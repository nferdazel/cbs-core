package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CollateralType adalah jenis agunan. Jenis ini menentukan kebijakan haircut yang dipakai
// sebagai bawaan saat agunan dicatat; nilainya sendiri tidak mengubah hak hukum, sehingga
// jenis "lainnya" tetap boleh dipakai asal bukti ikatannya ada.
type CollateralType string

const (
	CollateralTanahBangunan  CollateralType = "TANAH_BANGUNAN"
	CollateralKendaraan      CollateralType = "KENDARAAN"
	CollateralDeposit        CollateralType = "DEPOSIT"
	CollateralMesinPeralatan CollateralType = "MESIN_PERALATAN"
	CollateralLainnya        CollateralType = "LAINNYA"

	// Jenis berikut melengkapi daftar Pasal 20 ayat (1) POJK No. 1 Tahun 2024. Sebelum
	// ada, agunannya jatuh ke tarif konservatif (nol) sehingga penyisihan lebih besar
	// dari kewajiban; tarifnya ditegakkan Pasal20Rate, bukan di sini.
	CollateralEmasPerhiasan   CollateralType = "EMAS_PERHIASAN"    // huruf a: 85% nilai pasar
	CollateralResiGudang      CollateralType = "RESI_GUDANG"       // huruf c/h/j: 70/50/30%
	CollateralTanahAdat       CollateralType = "TANAH_ADAT"        // huruf e: 50% NJOP
	CollateralTempatUsaha     CollateralType = "TEMPAT_USAHA"      // huruf f: 50%
	CollateralJaminanBumnBumd CollateralType = "JAMINAN_BUMN_BUMD" // huruf i: 50%
)

// CollateralStatus adalah keadaan agunan. Hanya ACTIVE yang dihitung sebagai pengurang.
type CollateralStatus string

const (
	CollateralActive   CollateralStatus = "ACTIVE"
	CollateralReleased CollateralStatus = "RELEASED"
	CollateralExecuted CollateralStatus = "EXECUTED"
)

// DefaultHaircutPercent adalah haircut bawaan: 100 berarti nilai taksasi tidak dihitung
// sebagai pengurang sama sekali. Bawaan ini disengaja — rilis modul agunan tidak boleh
// menggeser angka PPAP sebelum bank menetapkan kebijakannya sendiri.
func DefaultHaircutPercent() decimal.Decimal {
	return decimal.NewFromInt(100)
}

// HaircutConfigKey memetakan jenis agunan ke kunci konfigurasi kebijakan haircut.
func HaircutConfigKey(t CollateralType) string {
	suffix := "lainnya"
	switch t {
	case CollateralTanahBangunan:
		suffix = "tanah_bangunan"
	case CollateralKendaraan:
		suffix = "kendaraan"
	case CollateralDeposit:
		suffix = "deposit"
	case CollateralMesinPeralatan:
		suffix = "mesin_peralatan"
	}
	return "collateral.haircut." + suffix
}

// PPAPCollateralEnabledKey adalah saklar pengurangan agunan pada perhitungan PPAP. Selama
// nilainya false, agunan boleh tercatat lengkap tetapi tidak mengurangi eksposur.
const PPAPCollateralEnabledKey = "ppap.collateral.enabled"

// Kesalahan agunan. Dipisah agar handler dapat memetakan status HTTP tanpa menebak pesan.
var (
	ErrCollateralNotFound             = errors.New("agunan tidak ditemukan")
	ErrCollateralDocumentRequired     = errors.New("nomor bukti ikatan agunan wajib diisi")
	ErrCollateralOwnerRequired        = errors.New("nama pemilik agunan wajib diisi")
	ErrCollateralAppraisalInvalid     = errors.New("nilai taksasi agunan harus lebih besar dari nol")
	ErrCollateralAppraisalDateInvalid = errors.New("tanggal taksasi agunan tidak boleh di masa depan")
	ErrCollateralHaircutInvalid       = errors.New("haircut agunan harus antara 0 dan 100 persen")
	ErrCollateralNotActive            = errors.New("hanya agunan berstatus aktif yang dapat diubah")
	// Agunan tunai mengubah perlakuan PPKA umum (Pasal 19 ayat (4) huruf b jo. Pasal 17),
	// sehingga rekening tempat dananya diblokir wajib diketahui agar pengecualiannya
	// dapat ditelusuri; penanda tanpa rekening tidak dapat diaudit.
	ErrCollateralCashAccountRequired = errors.New("agunan tunai wajib dikaitkan ke rekening tempat dana diblokir")
)

// CollateralInput adalah permintaan pencatatan agunan. Haircut tidak wajib diisi: bila
// kosong, kebijakan bank untuk jenis agunan itu yang dipakai.
type CollateralInput struct {
	LoanID         uuid.UUID
	CollateralType CollateralType
	Description    string
	DocumentNumber string
	OwnerName      string
	AppraisalValue decimal.Decimal
	AppraisalDate  time.Time
	Appraiser      string
	// HaircutPercent kosong berarti pakai kebijakan jenis agunan dari konfigurasi.
	HaircutPercent *decimal.Decimal
	// Penanda kepatuhan Pasal 20/21 POJK No. 1 Tahun 2024. ExistsKnown dan Executable
	// berupa pointer: nil berarti belum diisi operator dan dianggap true (keadaan normal);
	// yang lain bernilai false bila tidak disebut, karena secara hukum justru itu yang
	// membuat agunan TIDAK boleh jadi pengurang (mis. belum ber-hak tanggungan).
	AppraiserIndependent bool
	Certified            bool
	Mortgaged            bool
	MortgageValue        decimal.Decimal
	ExistsKnown          *bool
	Executable           *bool
	ThirdPartyOwner      bool
	OwnerConsent         bool
	// IsCash menandai agunan tunai Pasal 17 (tabungan, deposito, logam mulia, atau
	// surat berharga BI/Pemerintah) yang memenuhi syarat ayat (3). Penanda ini TIDAK
	// dipakai sebagai pengurang PPKA khusus — agunan tunai bukan daftar Pasal 20(1) —
	// melainkan untuk mengecualikan bagian yang dijamin dari PPKA umum (Pasal 19(4)(b)).
	IsCash bool
	// CashAccountID adalah rekening tempat agunan tunai disimpan/diblokir. Wajib
	// terhubung ke satu rekening agar pemblokiran Pasal 17(3) dapat diaudit.
	CashAccountID *uuid.UUID
	// WarehouseReceiptValuedAt adalah tanggal penilaian resi gudang bila berbeda dari
	// tanggal taksasi agunan. Kosong berarti AppraisalDate yang dipakai.
	WarehouseReceiptValuedAt *time.Time
	Notes                    string
}

// CollateralSummary adalah rekap agunan aktif per jenis. Dipakai manajemen untuk melihat
// sebaran jaminan dan berapa nilai pengurang yang tersedia, bukan untuk perhitungan PPAP
// (perhitungan itu per kredit).
type CollateralSummary struct {
	CollateralType CollateralType  `json:"collateral_type"`
	Count          int             `json:"count"`
	AppraisalValue decimal.Decimal `json:"appraisal_value"`
	BoundAmount    decimal.Decimal `json:"bound_amount"`
}

// CollateralService mengelola agunan kredit. Fase ini hanya pencatatan dan pembacaan;
// pengurangan PPAP masih dimatikan lewat ppap.collateral.enabled.
type CollateralService interface {
	Create(ctx context.Context, input CollateralInput, actor Actor) (*LoanCollateral, error)
	GetByID(ctx context.Context, id uuid.UUID, actor Actor) (*LoanCollateral, error)
	ListByLoan(ctx context.Context, loanID uuid.UUID, actor Actor) ([]LoanCollateral, error)
	// Summary merekap agunan berstatus ACTIVE per jenis. branchCode kosong berarti
	// seluruh cabang (hanya untuk aktor lintas cabang).
	Summary(ctx context.Context, actor Actor) ([]CollateralSummary, error)
}

// CollateralRepository menyimpan agunan kredit. Perhitungan PPKA tidak cukup memakai
// jumlah nilai pengurang: tarif Pasal 20(1), syarat Pasal 21(2), dan penurunan waktu
// Pasal 20(3)/(5) dinilai per agunan, jadi jalur PPAP membaca daftar agunan aktif.
type CollateralRepository interface {
	// branchCode kosong berarti cabang tidak diketahui dan disimpan sebagai NULL.
	Create(ctx context.Context, c *LoanCollateral, branchCode string) error
	GetByID(ctx context.Context, id uuid.UUID) (*LoanCollateral, error)
	ListByLoan(ctx context.Context, loanID uuid.UUID) ([]LoanCollateral, error)
	Update(ctx context.Context, c *LoanCollateral) error
	// ListActiveByLoans mengambil seluruh agunan berstatus ACTIVE untuk sekumpulan
	// kredit dalam SATU query, agar perhitungan PPAP tidak menjadi N+1.
	ListActiveByLoans(ctx context.Context, loanIDs []uuid.UUID) ([]LoanCollateral, error)
	// SummaryActive merekap agunan ACTIVE per jenis, disaring cabang bila branchCode
	// tidak kosong. Hanya ACTIVE yang dihitung: agunan yang sudah dilepas atau dieksekusi
	// tidak lagi menjamin apa pun, dan menampilkannya pada rekap jaminan akan melebihkan
	// nilai jaminan bank.
	SummaryActive(ctx context.Context, branchCode string) ([]CollateralSummary, error)
}

// LoanCollateral adalah satu agunan yang terikat pada satu kredit.
//
// BoundAmount tidak diisi dari sini: nilainya dihitung database dari AppraisalValue dan
// HaircutPercent sebagai kolom generated, supaya nilai pengurang yang tersimpan selalu
// konsisten dengan kebijakan pada saat pencatatan dan tetap dapat diaudit sesudahnya.
type LoanCollateral struct {
	ID     uuid.UUID `json:"id"`
	LoanID uuid.UUID `json:"loan_id"`
	// BranchID dan BranchCode diisi dari join ke branches. Pemeriksaan akses memakai
	// BranchCode karena Actor membawa kode cabang, bukan id.
	BranchID       *uuid.UUID      `json:"branch_id,omitempty"`
	BranchCode     string          `json:"branch_code,omitempty"`
	CollateralType CollateralType  `json:"collateral_type"`
	Description    string          `json:"description"`
	DocumentNumber string          `json:"document_number"`
	OwnerName      string          `json:"owner_name"`
	AppraisalValue decimal.Decimal `json:"appraisal_value"`
	AppraisalDate  time.Time       `json:"appraisal_date"`
	Appraiser      string          `json:"appraiser,omitempty"`
	// Penanda kepatuhan Pasal 20 dan Pasal 21 POJK No. 1 Tahun 2024. Ini yang menentukan
	// agunan boleh diperhitungkan sebagai pengurang PPKA: jenis dan ikatan hukumnya
	// (Pasal 20 ayat (1)), serta keadaannya pada saat perhitungan (Pasal 21 ayat (2)).
	AppraiserIndependent bool            `json:"appraiser_independent"`
	Certified            bool            `json:"certified"`
	Mortgaged            bool            `json:"mortgaged"`
	MortgageValue        decimal.Decimal `json:"mortgage_value"`
	ExistsKnown          bool            `json:"exists_known"`
	Executable           bool            `json:"executable"`
	ThirdPartyOwner      bool            `json:"third_party_owner"`
	OwnerConsent         bool            `json:"owner_consent"`
	// Penanda agunan tunai Pasal 17 dan rekening tempat dananya diblokir. Agunan tunai
	// mengubah perlakuan PPKA umum (dikecualikan, Pasal 19 ayat (4) huruf b), tetapi
	// TIDAK menambah pengurang PPKA khusus karena tidak tercantum pada Pasal 20 ayat (1).
	IsCash                   bool             `json:"is_cash"`
	CashAccountID            *uuid.UUID       `json:"cash_account_id,omitempty"`
	WarehouseReceiptValuedAt *time.Time       `json:"warehouse_receipt_valued_at,omitempty"`
	HaircutPercent           decimal.Decimal  `json:"haircut_percent"`
	BoundAmount              decimal.Decimal  `json:"bound_amount"`
	Status                   CollateralStatus `json:"status"`
	ReleasedAt               *time.Time       `json:"released_at,omitempty"`
	Notes                    string           `json:"notes,omitempty"`
	CreatedBy                string           `json:"created_by"`
	CreatedAt                time.Time        `json:"created_at"`
	UpdatedBy                string           `json:"updated_by,omitempty"`
	UpdatedAt                time.Time        `json:"updated_at"`
}

// IsActive menandai agunan yang masih dihitung sebagai pengurang.
func (c *LoanCollateral) IsActive() bool {
	return c.Status == CollateralActive
}

// Validate menegakkan syarat yang tidak dapat dijaga skema: bukti ikatan yang ada isinya,
// taksasi bernilai positif dan tidak bertanggal masa depan, serta haircut dalam rentang
// persen. Tanpa bukti ikatan, agunan tidak boleh dihitung sebagai pengurang sekaya apa pun
// taksasinya — karena itu syaratnya ditegakkan di sini, bukan diserahkan ke operator.
func (c *LoanCollateral) Validate(now time.Time) error {
	if c.LoanID == uuid.Nil {
		return fmt.Errorf("agunan harus terikat pada satu kredit")
	}
	if strings.TrimSpace(c.DocumentNumber) == "" {
		return ErrCollateralDocumentRequired
	}
	if strings.TrimSpace(c.OwnerName) == "" {
		return ErrCollateralOwnerRequired
	}
	if c.AppraisalValue.LessThanOrEqual(decimal.Zero) {
		return ErrCollateralAppraisalInvalid
	}
	if c.AppraisalDate.IsZero() {
		return ErrCollateralAppraisalDateInvalid
	}
	// Dibandingkan per tanggal, bukan per waktu: taksasi hari ini sah walau jamnya belum
	// lewat di zona waktu lain.
	hariIni := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	taksasi := time.Date(c.AppraisalDate.Year(), c.AppraisalDate.Month(), c.AppraisalDate.Day(), 0, 0, 0, 0, time.UTC)
	if taksasi.After(hariIni) {
		return ErrCollateralAppraisalDateInvalid
	}
	if c.HaircutPercent.IsNegative() || c.HaircutPercent.GreaterThan(decimal.NewFromInt(100)) {
		return ErrCollateralHaircutInvalid
	}
	// Agunan tunai tanpa rekening tidak dapat dibuktikan diblokir (Pasal 17 ayat (3)
	// huruf a dan d), sehingga pengecualian PPKA umumnya tidak boleh diakui.
	if c.IsCash && c.CashAccountID == nil {
		return ErrCollateralCashAccountRequired
	}
	return nil
}
