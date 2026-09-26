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
)

// BMPKRelationshipType adalah taksonomi jenis hubungan pihak terkait. Nilainya adalah
// kebijakan bank, jadi disimpan sebagai teks bebas dan hanya dijaga tidak kosong serta
// panjangnya wajar; kode TIDAK memetakan ke sandi apa pun.
type BMPKRelatedParty struct {
	CustomerID       uuid.UUID `json:"customer_id"`
	RelationshipType string    `json:"relationship_type"`
	Note             string    `json:"note,omitempty"`
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
	CustomerID    uuid.UUID       `json:"customer_id"`
	MaxAmount     decimal.Decimal `json:"max_amount"`
	EffectiveDate *time.Time      `json:"effective_date,omitempty"`
	Note          string          `json:"note,omitempty"`
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

// BMPKRepository membaca fondasi BMPK. Implementasi WAJIB membaca paparan gabungan
// dalam SATU query (kredit + penempatan), bukan satu query per pihak.
type BMPKRepository interface {
	// ListPartyExposures mengembalikan satu baris per pihak terkait beserta paparan
	// kredit/penempatan dan batasnya. Pihak terkait tanpa batas tetap muncul dengan
	// HasLimit=false.
	ListPartyExposures(ctx context.Context) ([]BMPKPartyExposure, error)
}

// BMPKCustomerNamer melengkapi nama nasabah yang sudah didekripsi secara batch. Kontrak
// dipisah dari domain.CustomerService agar modul BMPK tidak menarik seluruh permukaan
// layanan nasabah dan agar uji dapat menyuntikkan stub.
type BMPKCustomerNamer interface {
	NamesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// BMPKService merakit laporan BMPK. Nama method dibuat eksplisit (BMPKReport) supaya
// dapat dipakai juga sebagai sumber laporan OJK.
type BMPKService interface {
	BMPKReport(ctx context.Context, asOf time.Time, actor Actor) (BMPKReport, error)
}
