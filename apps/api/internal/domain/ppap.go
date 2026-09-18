package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Sentinel error modul PPAP. Dipakai agar pemanggil bisa membedakan kegagalan
// konfigurasi/data dari kegagalan tak terduga.
var (
	ErrPPAPLoanNotFound    = errors.New("kredit untuk perhitungan PPAP tidak ditemukan")
	ErrPPAPReserveNotFound = errors.New("akun cadangan PPAP tidak ditemukan")
	ErrPPAPExpenseNotFound = errors.New("akun beban PPAP tidak ditemukan")
)

// Collectibility adalah kualitas aset versi numerik: 1 Lancar, 2 Dalam Perhatian
// Khusus, 3 Kurang Lancar, 4 Diragukan, 5 Macet. Nilai ini yang dipakai mesin PPAP;
// kolom database tetap menyimpan bentuk lama (mis. "2_DPK") demi kompatibilitas.
type Collectibility int

const (
	KolLancar       Collectibility = 1
	KolDPK          Collectibility = 2
	KolKurangLancar Collectibility = 3
	KolDiragukan    Collectibility = 4
	KolMacet        Collectibility = 5
)

func (c Collectibility) Valid() bool {
	return c >= KolLancar && c <= KolMacet
}

func (c Collectibility) Label() string {
	switch c {
	case KolLancar:
		return "Lancar"
	case KolDPK:
		return "Dalam Perhatian Khusus"
	case KolKurangLancar:
		return "Kurang Lancar"
	case KolDiragukan:
		return "Diragukan"
	case KolMacet:
		return "Macet"
	default:
		return "Tidak Dikenal"
	}
}

// IsNPL menandai kredit tidak lancar. Mulai kolektibilitas 3 (Kurang Lancar),
// pendapatan bunga/margin tidak boleh lagi diakui secara akrual (cash basis).
func (c Collectibility) IsNPL() bool {
	return c >= KolKurangLancar
}

// OJKCode memetakan nilai numerik ke kode lama yang tersimpan di kolom loans.collectibility.
func (c Collectibility) OJKCode() OJKCollectibility {
	switch c {
	case KolLancar:
		return CollectibilityKol1
	case KolDPK:
		return CollectibilityKol2
	case KolKurangLancar:
		return CollectibilityKol3
	case KolDiragukan:
		return CollectibilityKol4
	case KolMacet:
		return CollectibilityKol5
	default:
		return CollectibilityKol1
	}
}

// CollectibilityFromOJK mengubah kode kolektibilitas tersimpan menjadi nilai numerik.
// Nilai tak dikenal dianggap Lancar agar kredit lama tetap ikut terpantau.
func CollectibilityFromOJK(o OJKCollectibility) Collectibility {
	switch o {
	case CollectibilityKol2:
		return KolDPK
	case CollectibilityKol3:
		return KolKurangLancar
	case CollectibilityKol4:
		return KolDiragukan
	case CollectibilityKol5:
		return KolMacet
	default:
		return KolLancar
	}
}

// CollectibilityThresholds adalah batas atas DPD (hari) per kolektibilitas, dari
// konfigurasi. Batas lancar selalu DPD 0; yang dikonfigurasi adalah batas atas
// golongan 2, 3, dan 4.
type CollectibilityThresholds struct {
	DPK          int
	KurangLancar int
	Diragukan    int
}

// DefaultCollectibilityThresholds mengikuti POJK 40/2019 (kualitas aset BPR).
func DefaultCollectibilityThresholds() CollectibilityThresholds {
	return CollectibilityThresholds{DPK: 30, KurangLancar: 90, Diragukan: 180}
}

// CollectibilityFromDPD menentukan golongan dari jumlah hari keterlambatan.
// Ambang yang tidak wajar (<= batas sebelumnya) dikoreksi ke default agar
// konfigurasi rusak tidak pernah menghasilkan golongan yang lebih baik.
func CollectibilityFromDPD(dpd int, t CollectibilityThresholds) Collectibility {
	def := DefaultCollectibilityThresholds()
	if t.DPK <= 0 {
		t.DPK = def.DPK
	}
	if t.KurangLancar <= t.DPK {
		t.KurangLancar = def.KurangLancar
	}
	if t.Diragukan <= t.KurangLancar {
		t.Diragukan = def.Diragukan
	}

	switch {
	case dpd <= 0:
		return KolLancar
	case dpd <= t.DPK:
		return KolDPK
	case dpd <= t.KurangLancar:
		return KolKurangLancar
	case dpd <= t.Diragukan:
		return KolDiragukan
	default:
		return KolMacet
	}
}

// PPAPRate adalah tarif penyisihan minimum atas pokok terutang, dalam fraksi
// (0.005 = 0,5%). Disimpan sebagai decimal agar tidak ada galat pembulatan biner.
type PPAPRate decimal.Decimal

// PPAPRates memetakan kolektibilitas ke tarif minimumnya.
type PPAPRates map[Collectibility]PPAPRate

// DefaultPPAPRates mengikuti ketentuan PPAP minimum BPR atas pokok terutang:
// Lancar 0,5%; Dalam Perhatian Khusus 10%; Kurang Lancar 15%; Diragukan 50%; Macet 100%.
func DefaultPPAPRates() PPAPRates {
	return PPAPRates{
		KolLancar:       PPAPRate(decimal.NewFromFloat(0.005)),
		KolDPK:          PPAPRate(decimal.NewFromFloat(0.10)),
		KolKurangLancar: PPAPRate(decimal.NewFromFloat(0.15)),
		KolDiragukan:    PPAPRate(decimal.NewFromFloat(0.50)),
		KolMacet:        PPAPRate(decimal.NewFromFloat(1.00)),
	}
}

// PPAPCalculation adalah hasil perhitungan PPAP satu kredit.
type PPAPCalculation struct {
	Outstanding    decimal.Decimal
	Collectibility Collectibility
	Rate           PPAPRate
	Target         decimal.Decimal
	Existing       decimal.Decimal
	Adjustment     decimal.Decimal
}

// PPAPAmount menghitung target PPAP atas saldo pokok terutang (bukan total tagihan)
// dan membulatkannya ke rupiah penuh. Tarif yang tidak dikenal menghasilkan nol.
func PPAPAmount(outstanding decimal.Decimal, c Collectibility, rates PPAPRates) decimal.Decimal {
	rate, ok := rates[c]
	if !ok || outstanding.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	return RoundToRupiah(outstanding.Mul(decimal.Decimal(rate)))
}

// PPAPAdjustment adalah selisih yang harus diposting: positif berarti menambah
// cadangan (provisi), negatif berarti memulihkan cadangan (reversal), nol berarti
// cadangan sudah sesuai target.
func PPAPAdjustment(target, existing decimal.Decimal) decimal.Decimal {
	return RoundToRupiah(target.Sub(existing))
}

// CalculatePPAP merangkai perhitungan target dan selisih dari saldo cadangan yang
// sudah diakui untuk kredit tersebut.
func CalculatePPAP(outstanding decimal.Decimal, c Collectibility, existing decimal.Decimal, rates PPAPRates) PPAPCalculation {
	target := PPAPAmount(outstanding, c, rates)
	return PPAPCalculation{
		Outstanding:    outstanding,
		Collectibility: c,
		Rate:           rates[c],
		Target:         target,
		Existing:       existing,
		Adjustment:     PPAPAdjustment(target, existing),
	}
}

// PPAPLoanSnapshot adalah data kredit aktif yang dibutuhkan proses PPAP harian.
type PPAPLoanSnapshot struct {
	LoanID         uuid.UUID
	LoanNumber     string
	ProductID      *uuid.UUID
	Outstanding    decimal.Decimal
	Collectibility Collectibility // kolektibilitas tersimpan
	DPD            int            // DPD tersimpan
	AccrualStatus  AccrualStatus
	RequiredPPAP   decimal.Decimal // cadangan per kredit yang sudah diakui (target terakhir)
	// LastDueDate adalah jatuh tempo angsuran terlama yang belum dibayar; nil bila
	// seluruh angsuran sudah lunas.
	LastDueDate *time.Time
}

// PPAPLoanUpdate membawa perubahan state kredit hasil proses PPAP.
type PPAPLoanUpdate struct {
	LoanID         uuid.UUID
	Collectibility Collectibility
	DPD            int
	AccrualStatus  AccrualStatus
	StopAccrual    bool
	RequiredPPAP   decimal.Decimal
}

// PPAPRunItem adalah hasil pemrosesan satu kredit.
type PPAPRunItem struct {
	LoanID                uuid.UUID       `json:"loan_id"`
	LoanNumber            string          `json:"loan_number"`
	DPD                   int             `json:"dpd"`
	Collectibility        Collectibility  `json:"collectibility"`
	Outstanding           decimal.Decimal `json:"outstanding"`
	Target                decimal.Decimal `json:"target"`
	Existing              decimal.Decimal `json:"existing"`
	Adjustment            decimal.Decimal `json:"adjustment"`
	CollectibilityChanged bool            `json:"collectibility_changed"`
	StopAccrual           bool            `json:"stop_accrual"`
	Posted                bool            `json:"posted"`
}

// PPAPRunFailure mencatat kredit yang gagal diproses tanpa menggagalkan batch.
type PPAPRunFailure struct {
	LoanID     uuid.UUID `json:"loan_id"`
	LoanNumber string    `json:"loan_number"`
	Error      string    `json:"error"`
}

// PPAPRunSummary adalah ringkasan satu kali proses PPAP.
type PPAPRunSummary struct {
	AsOf            time.Time        `json:"as_of"`
	Total           int              `json:"total"`
	Processed       int              `json:"processed"`
	Failed          int              `json:"failed"`
	Skipped         int              `json:"skipped"` // tidak berubah, tidak ada posting
	TotalAdjustment decimal.Decimal  `json:"total_adjustment"`
	Items           []PPAPRunItem    `json:"items"`
	Failures        []PPAPRunFailure `json:"failures"`
	ReserveBefore   decimal.Decimal  `json:"reserve_before"` // saldo GL cadangan sebelum proses (rekonsiliasi)
	ReserveAfter    decimal.Decimal  `json:"reserve_after"`  // saldo GL cadangan setelah proses (rekonsiliasi)
	Preview         bool             `json:"preview"`        // true bila hanya simulasi, tanpa posting
}

// PPAPRepository adalah akses data proses PPAP. Seluruh SQL jurnal tetap lewat
// posting engine, bukan di sini.
type PPAPRepository interface {
	// ListDueLoans mengambil kredit aktif beserta outstanding dan jatuh tempo
	// angsuran terlama yang belum dibayar, per tanggal asOf.
	ListDueLoans(ctx context.Context, asOf time.Time) ([]PPAPLoanSnapshot, error)
	// UpdateCollectibility menyimpan perubahan kolektibilitas (beserta status akrual
	// turunannya) di dalam transaksi pemanggil.
	UpdateCollectibility(ctx context.Context, tx any, loanID uuid.UUID, c Collectibility) error
	// UpdateLoanState menyimpan seluruh state PPAP kredit dalam satu update.
	UpdateLoanState(ctx context.Context, tx any, update PPAPLoanUpdate) error
	// GetPPAPReserveBalance membaca saldo akun cadangan GL untuk kode COA tertentu.
	GetPPAPReserveBalance(ctx context.Context, tx any, coaCode string) (decimal.Decimal, error)
}

// PPAPService menjalankan penyisihan PPAP dan pembaruan kolektibilitas kredit.
type PPAPService interface {
	RunDaily(ctx context.Context, asOf time.Time, actor Actor) (PPAPRunSummary, error)
	Preview(ctx context.Context, asOf time.Time) (PPAPRunSummary, error)
}
