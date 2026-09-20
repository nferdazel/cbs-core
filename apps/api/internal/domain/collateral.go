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
	Notes          string
}

// CollateralService mengelola agunan kredit. Fase ini hanya pencatatan dan pembacaan;
// pengurangan PPAP masih dimatikan lewat ppap.collateral.enabled.
type CollateralService interface {
	Create(ctx context.Context, input CollateralInput, actor Actor) (*LoanCollateral, error)
	GetByID(ctx context.Context, id uuid.UUID, actor Actor) (*LoanCollateral, error)
	ListByLoan(ctx context.Context, loanID uuid.UUID, actor Actor) ([]LoanCollateral, error)
}

// CollateralRepository menyimpan agunan kredit. Ringkas dengan sengaja: yang dibutuhkan
// perhitungan PPAP hanyalah jumlah nilai pengurang agunan aktif per kredit.
type CollateralRepository interface {
	// branchCode kosong berarti cabang tidak diketahui dan disimpan sebagai NULL.
	Create(ctx context.Context, c *LoanCollateral, branchCode string) error
	GetByID(ctx context.Context, id uuid.UUID) (*LoanCollateral, error)
	ListByLoan(ctx context.Context, loanID uuid.UUID) ([]LoanCollateral, error)
	Update(ctx context.Context, c *LoanCollateral) error
	// SumActiveBoundByLoan menjumlahkan bound_amount agunan berstatus ACTIVE untuk
	// sekumpulan kredit dalam SATU query, agar perhitungan PPAP tidak menjadi N+1.
	SumActiveBoundByLoan(ctx context.Context, loanIDs []uuid.UUID) (map[uuid.UUID]decimal.Decimal, error)
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
	BranchID       *uuid.UUID       `json:"branch_id,omitempty"`
	BranchCode     string           `json:"branch_code,omitempty"`
	CollateralType CollateralType   `json:"collateral_type"`
	Description    string           `json:"description"`
	DocumentNumber string           `json:"document_number"`
	OwnerName      string           `json:"owner_name"`
	AppraisalValue decimal.Decimal  `json:"appraisal_value"`
	AppraisalDate  time.Time        `json:"appraisal_date"`
	Appraiser      string           `json:"appraiser,omitempty"`
	HaircutPercent decimal.Decimal  `json:"haircut_percent"`
	BoundAmount    decimal.Decimal  `json:"bound_amount"`
	Status         CollateralStatus `json:"status"`
	ReleasedAt     *time.Time       `json:"released_at,omitempty"`
	Notes          string           `json:"notes,omitempty"`
	CreatedBy      string           `json:"created_by"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedBy      string           `json:"updated_by,omitempty"`
	UpdatedAt      time.Time        `json:"updated_at"`
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
	return nil
}
