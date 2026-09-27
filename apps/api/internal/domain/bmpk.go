package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// bmpk.go memuat fondasi Batas Maksimum Pemberian Kredit (BMPK): penandaan pihak
// terkait, penyimpanan batas, agregasi paparan per pihak, dan uji batas/pelampauan.
//
// Prinsip yang dipegang berkas ini:
//   - TIDAK mengarang aturan/angka regulasi. Nomor/format kolom resmi Laporan BMPK
//     belum ada di repo; karena itu batas disimpan eksplisit (bmpk_limits) dan tidak
//     ada persentase modal yang di-hardcode.
//   - Batas yang belum diset BUKAN nol. Statusnya BATAS_BELUM_DISET sehingga laporan
//     menuliskannya sebagai belum tersedia, bukan seolah sesuai batas.
//   - Taksonomi jenis hubungan dan pemetaan status ke istilah OJK ("pelanggaran" vs
//     "pelampauan") adalah keputusan bank/OJK, bukan nilai yang dikarang mesin.
//
// Saklar bmpk.enabled bawaan 'false' (migrasi 000103): modul tidak menegakkan apa pun
// sampai bank mengisi pihak terkait dan batasnya.

const (
	// BMPKEnabledKey adalah saklar utama modul BMPK. Bawaan false = non-enforcing.
	BMPKEnabledKey = "bmpk.enabled"
)

// Status batas satu pihak terkait. Dipisah sebagai konstanta agar laporan dan uji
// memakai kata yang sama, bukan literal yang tersebar.
const (
	// BMPKStatusBatasBelumDiset berarti bmpk_limits belum memuat baris pihak ini.
	// Paparan tetap dilaporkan, tetapi tidak ada yang dapat dibandingkan.
	BMPKStatusBatasBelumDiset = "BATAS_BELUM_DISET"
	// BMPKStatusDalamBatas berarti paparan tidak melebihi batas yang diisi bank.
	BMPKStatusDalamBatas = "DALAM_BATAS"
	// BMPKStatusMelampauiBatas berarti paparan melebihi batas yang diisi bank.
	// Pemetaannya ke istilah resmi OJK (pelanggaran/pelampauan) belum diputuskan.
	BMPKStatusMelampauiBatas = "MELAMPAUI_BATAS"
)

var (
	// ErrBMPKBankWide menandai permintaan laporan BMPK oleh aktor yang tidak berwenang
	// atas seluruh bank. Laporan BMPK bersifat bank-wide.
	ErrBMPKBankWide = NewLocalizedError("bmpk_bank_wide",
		"laporan BMPK bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrBMPKInputInvalid menandai data pihak terkait/batas yang tidak sah. Dipakai
	// penjaga bentuk bila kelak pengisian dipindahkan dari SQL ke API.
	ErrBMPKInputInvalid = NewLocalizedError("bmpk_input_invalid",
		"data pihak terkait/batas BMPK tidak valid")
	// ErrBMPKNotFound menandai baris pihak terkait/batas (atau nasabahnya) yang tidak
	// ada. Penghapusan yang tidak menemukan baris ditolak agar tidak tampak berhasil.
	ErrBMPKNotFound = NewLocalizedError("bmpk_not_found",
		"data pihak terkait/batas BMPK tidak ditemukan")
)

// BMPKRelationshipType adalah taksonomi jenis hubungan pihak terkait. Nilainya adalah
// kebijakan bank, jadi disimpan sebagai teks bebas dan hanya dijaga tidak kosong serta
// panjangnya wajar; kode TIDAK memetakan ke sandi apa pun.
type BMPKRelatedParty struct {
	CustomerID       uuid.UUID `json:"customer_id"`
	RelationshipType string    `json:"relationship_type"`
	// Note ikut dipaparkan meski kosong agar bentuk keluaran master seragam untuk UI.
	Note string `json:"note"`
}

// ValidateRelatedParty menegakkan syarat yang tidak dapat dijaga pemanggil: nasabah
// teridentifikasi dan jenis hubungan terisi.
func (p BMPKRelatedParty) Validate() error {
	if p.CustomerID == uuid.Nil {
		return fmt.Errorf("%w: customer_id wajib diisi", ErrBMPKInputInvalid)
	}
	if strings.TrimSpace(p.RelationshipType) == "" {
		return fmt.Errorf("%w: jenis hubungan wajib diisi", ErrBMPKInputInvalid)
	}
	if len(p.RelationshipType) > 64 {
		return fmt.Errorf("%w: jenis hubungan maksimal 64 karakter", ErrBMPKInputInvalid)
	}
	return nil
}

// BMPKLimit adalah batas satu pihak terkait yang ditetapkan bank. MaxAmount dalam
// rupiah penuh. baris yang tidak ada berarti batas belum diset; nol SAH dan berarti
// tidak boleh ada eksposur.
type BMPKLimit struct {
	CustomerID uuid.UUID       `json:"customer_id"`
	MaxAmount  decimal.Decimal `json:"max_amount"`
	// EffectiveDate nil ditulis null (belum dicatat), bukan tanggal tebakan.
	EffectiveDate *time.Time `json:"effective_date"`
	Note          string     `json:"note"`
}

// UpdateBMPKRelatedPartyInput adalah isi upsert SATU penandaan pihak terkait. Satu
// nasabah hanya punya satu baris (PRIMARY KEY customer_id); baris lama ditimpa.
// CustomerID dan RelationshipType wajib.
type UpdateBMPKRelatedPartyInput struct {
	CustomerID       string `json:"customer_id"`
	RelationshipType string `json:"relationship_type"`
	Note             string `json:"note"`
}

// UpdateBMPKLimitInput adalah isi upsert SATU batas per nasabah. MaxAmount dalam
// rupiah penuh, wajib dikirim, dan >= 0 (nol sah: tidak boleh ada eksposur). Ditulis
// sebagai pointer agar bidang yang hilang ditolak 422, bukan diam-diam menjadi batas
// nol. EffectiveDate memakai format YYYY-MM-DD; kosong = belum dicatat.
type UpdateBMPKLimitInput struct {
	CustomerID    string           `json:"customer_id"`
	MaxAmount     *decimal.Decimal `json:"max_amount"`
	EffectiveDate string           `json:"effective_date"`
	Note          string           `json:"note"`
}

// BMPKMaster adalah data mentah pihak terkait dan batas yang bank isi, dipakai UI
// pengaturan (bukan laporan terhitung). Urutan deterministik menurut customer_id.
type BMPKMaster struct {
	RelatedParties []BMPKRelatedParty `json:"related_parties"`
	Limits         []BMPKLimit        `json:"limits"`
}

// BuildBMPKRelatedParty memvalidasi masukan dan membentuk baris siap simpan. Aturan
// bentuk dipusatkan pada BMPKRelatedParty.Validate agar API dan laporan memakai
// syarat yang sama.
func BuildBMPKRelatedParty(in UpdateBMPKRelatedPartyInput) (BMPKRelatedParty, error) {
	out := BMPKRelatedParty{RelationshipType: strings.TrimSpace(in.RelationshipType), Note: strings.TrimSpace(in.Note)}
	id, err := parseBMPKCustomerID(in.CustomerID)
	if err != nil {
		return out, err
	}
	out.CustomerID = id
	if err := out.Validate(); err != nil {
		return out, err
	}
	return out, nil
}

// BuildBMPKLimit memvalidasi masukan dan membentuk batas siap simpan. MaxAmount
// wajib dikirim; nol sah, negatif ditolak.
func BuildBMPKLimit(in UpdateBMPKLimitInput) (BMPKLimit, error) {
	out := BMPKLimit{Note: strings.TrimSpace(in.Note)}
	if in.MaxAmount == nil {
		return out, fmt.Errorf("%w: max_amount wajib diisi", ErrBMPKInputInvalid)
	}
	out.MaxAmount = *in.MaxAmount
	id, err := parseBMPKCustomerID(in.CustomerID)
	if err != nil {
		return out, err
	}
	out.CustomerID = id
	if out.MaxAmount.IsNegative() {
		return out, fmt.Errorf("%w: max_amount tidak boleh negatif", ErrBMPKInputInvalid)
	}
	if out.EffectiveDate, err = parseBMPKEffectiveDate(in.EffectiveDate); err != nil {
		return out, err
	}
	return out, nil
}

// parseBMPKCustomerID mewajibkan customer_id terisi dan berbentuk UUID.
func parseBMPKCustomerID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("%w: customer_id wajib diisi", ErrBMPKInputInvalid)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: customer_id bukan UUID yang sah", ErrBMPKInputInvalid)
	}
	return id, nil
}

// parseBMPKEffectiveDate menerima tanggal kosong (belum dicatat) atau YYYY-MM-DD.
func parseBMPKEffectiveDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, fmt.Errorf("%w: effective_date harus format YYYY-MM-DD", ErrBMPKInputInvalid)
	}
	return &t, nil
}

// BMPKPartyExposure adalah satu baris paparan per pihak terkait beserta batasnya.
// Ini hasil dari SATU query agregat (kredit + penempatan) yang di-left-join ke
// penandaan pihak terkait dan batas. Nama nasabah TIDAK ikut query karena tersimpan
// terenkripsi; service melengkapinya lewat pembacaan nama secara batch.
type BMPKPartyExposure struct {
	CustomerID       uuid.UUID `json:"customer_id"`
	CustomerName     string    `json:"customer_name,omitempty"`
	RelationshipType string    `json:"relationship_type"`
	// LoanExposure adalah baki debet pokok kredit (loans.outstanding_principal).
	LoanExposure decimal.Decimal `json:"loan_exposure"`
	// PlacementExposure adalah outstanding penempatan pada bank lain yang ditautkan
	// ke nasabah ini (lps_placements.customer_id).
	PlacementExposure decimal.Decimal `json:"placement_exposure"`
	// LimitAmount dan HasLimit berasal dari bmpk_limits. HasLimit=false berarti
	// batas belum diset, dan LimitAmount tidak bermakna (tetap ditulis nol).
	LimitAmount        decimal.Decimal `json:"limit_amount"`
	LimitEffectiveDate *time.Time      `json:"limit_effective_date,omitempty"`
	LimitNote          string          `json:"limit_note,omitempty"`
	HasLimit           bool            `json:"has_limit"`
}

// Total menjumlahkan paparan kredit dan penempatan satu pihak. Ini agregasi yang
// dibandingkan dengan batas.
func (e BMPKPartyExposure) Total() decimal.Decimal {
	return e.LoanExposure.Add(e.PlacementExposure)
}

// BMPKPartyCheck adalah hasil uji batas satu pihak: total paparan, kelebihan (bila
// melampaui), status, dan alasan yang dapat dibaca operator.
type BMPKPartyCheck struct {
	BMPKPartyExposure
	TotalExposure decimal.Decimal `json:"total_exposure"`
	// Excess adalah total paparan dikurangi batas; nol bila tidak melampaui atau
	// batas belum diset.
	Excess decimal.Decimal `json:"excess"`
	Status string          `json:"status"`
	// StatusReason menjelaskan sebab status (terutama saat batas belum diset atau
	// saat melampaui), bukan hanya mencetak status.
	StatusReason string `json:"status_reason,omitempty"`
}

// CheckBMPKParty membandingkan paparan satu pihak dengan batasnya. Batas yang belum
// diset menghasilkan BATAS_BELUM_DISET; batas nol tetap sah dan setiap paparan positif
// dinyatakan melampaui. Fungsi ini murni: tidak menyentuh basis data.
func CheckBMPKParty(e BMPKPartyExposure) BMPKPartyCheck {
	total := e.Total()
	check := BMPKPartyCheck{BMPKPartyExposure: e, TotalExposure: total}
	if !e.HasLimit {
		check.Status = BMPKStatusBatasBelumDiset
		check.StatusReason = "batas BMPK pihak ini belum diset bank pada bmpk_limits; paparan dilaporkan tanpa penilaian batas"
		return check
	}
	if total.GreaterThan(e.LimitAmount) {
		check.Excess = total.Sub(e.LimitAmount)
		check.Status = BMPKStatusMelampauiBatas
		check.StatusReason = fmt.Sprintf("paparan %s melebihi batas %s sebesar %s (nominal rupiah penuh)",
			total.Round(0), e.LimitAmount.Round(0), check.Excess.Round(0))
		return check
	}
	check.Status = BMPKStatusDalamBatas
	return check
}

// BMPKReport adalah keluaran baca-saja modul BMPK untuk satu posisi.
type BMPKReport struct {
	AsOf               time.Time        `json:"as_of"`
	EnforcementEnabled bool             `json:"enforcement_enabled"`
	Rows               []BMPKPartyCheck `json:"rows"`
	// Warnings mencatat batas modul yang perlu diketahui pembaca laporan (mis. saklar
	// mati, batas belum diset), bukan menutupi kekurangan data.
	Warnings []string `json:"warnings,omitempty"`
}

// BMPKRepository membaca fondasi BMPK dan menyimpan pengaturan pihak terkait/batas.
// Implementasi WAJIB membaca paparan gabungan dalam SATU query (kredit + penempatan),
// bukan satu query per pihak. Penulisan menerima transaksi dari layanan agar audit
// berada pada transaksi yang sama.
type BMPKRepository interface {
	// ListPartyExposures mengembalikan satu baris per pihak terkait beserta paparan
	// kredit/penempatan dan batasnya. Pihak terkait tanpa batas tetap muncul dengan
	// HasLimit=false.
	ListPartyExposures(ctx context.Context) ([]BMPKPartyExposure, error)
	// ListRelatedParties dan ListLimits membaca data pengaturan mentah untuk UI.
	ListRelatedParties(ctx context.Context) ([]BMPKRelatedParty, error)
	ListLimits(ctx context.Context) ([]BMPKLimit, error)
	// UpsertRelatedPartyTx/UpsertLimitTx menimpa baris nasabah (INSERT ... ON CONFLICT
	// customer_id DO UPDATE) memakai transaksi pemanggil. Tabel BMPK tidak menyimpan
	// kolom pelaku; jejak audit ditulis layanan pada transaksi yang sama.
	UpsertRelatedPartyTx(ctx context.Context, tx any, p BMPKRelatedParty) error
	UpsertLimitTx(ctx context.Context, tx any, l BMPKLimit) error
	// DeleteRelatedPartyTx/DeleteLimitTx menghapus baris nasabah; found=false bila
	// baris tidak ada.
	DeleteRelatedPartyTx(ctx context.Context, tx any, customerID uuid.UUID) (bool, error)
	DeleteLimitTx(ctx context.Context, tx any, customerID uuid.UUID) (bool, error)
}

// BMPKCustomerNamer melengkapi nama nasabah yang sudah didekripsi secara batch. Kontrak
// dipisah dari domain.CustomerService agar modul BMPK tidak menarik seluruh permukaan
// layanan nasabah dan agar uji dapat menyuntikkan stub.
type BMPKCustomerNamer interface {
	NamesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// BMPKService merakit laporan BMPK sekaligus melayani pengaturan pihak terkait dan
// batasnya. Nama method laporan dibuat eksplisit (BMPKReport) supaya dapat dipakai
// juga sebagai sumber laporan OJK.
type BMPKService interface {
	BMPKReport(ctx context.Context, asOf time.Time, actor Actor) (BMPKReport, error)
	// ListMaster mengembalikan data pengaturan mentah (pihak terkait + batas) untuk UI.
	ListMaster(ctx context.Context) (BMPKMaster, error)
	// UpsertRelatedParty menyimpan satu penandaan pihak terkait (upsert per nasabah),
	// teraudit. Menolak nasabah yang tidak ada dengan ErrBMPKNotFound.
	UpsertRelatedParty(ctx context.Context, input UpdateBMPKRelatedPartyInput, actor Actor) (*BMPKRelatedParty, error)
	// DeleteRelatedParty menghapus penandaan satu nasabah; baris tak ada →
	// ErrBMPKNotFound.
	DeleteRelatedParty(ctx context.Context, customerID uuid.UUID, actor Actor) error
	// UpsertLimit menyimpan satu batas per nasabah (upsert), teraudit. Menolak nasabah
	// yang tidak ada dengan ErrBMPKNotFound.
	UpsertLimit(ctx context.Context, input UpdateBMPKLimitInput, actor Actor) (*BMPKLimit, error)
	// DeleteLimit menghapus batas satu nasabah; baris tak ada → ErrBMPKNotFound.
	DeleteLimit(ctx context.Context, customerID uuid.UUID, actor Actor) error
}
