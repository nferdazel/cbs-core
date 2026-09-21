package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// LoanPenaltyEligible menentukan kredit yang boleh dikenai denda. Hanya kredit aktif
// (DISBURSED) dan kredit yang sudah dinyatakan default yang punya tunggakan; kredit
// PENDING/APPROVED belum dicairkan, sedangkan PAID_OFF/WRITTEN_OFF/REJECTED tidak
// boleh lagi ditambah dendanya. Daftar positif disengaja agar status baru yang tidak
// dikenal tidak otomatis dikenai denda.
func LoanPenaltyEligible(status LoanStatus) bool {
	switch status {
	case LoanStatusDisbursed, LoanStatusDefaulted:
		return true
	default:
		return false
	}
}

// LoanPenaltyAmount menghitung denda satu kredit.
//
// Dasar pengenaan: pokok angsuran yang sudah lewat jatuh tempo dan belum dibayar
// (outstanding_principal per jadwal = principal_amount - paid_principal), BUKAN
// seluruh sisa pokok kredit. Alasannya: denda adalah sanksi atas keterlambatan, jadi
// hanya bagian yang benar-benar menunggak yang dikenai; angsuran yang belum jatuh
// tempo tidak boleh ikut dihitung. Angka tunggakan dihitung di query repo dengan
// GREATEST(principal_amount - paid_principal, 0) agar angsuran sebagian tetap benar.
//
// ratePerMille adalah tarif harian per-mille (‰) dari pokok tunggakan: nilai 1 berarti
// 0,1% per hari. daysToAccrue adalah JUMLAH HARI YANG DIAKRU PADA PEMANGGILAN INI,
// bukan total DPD kredit: pemanggil menghitung selisih hari sejak akrual terakhir
// agar akumulasi tetap linier dan tidak kuadratik saat dijalankan setiap hari EOD.
// Hasil dibulatkan ke rupiah penuh dengan RoundToRupiah. Tarif atau hari nol
// menghasilkan nol: tidak ada denda yang dikarangkan di kode.
//
// Idempotensi per (kredit, tanggal) menjaga agar tanggal yang sama hanya menambah
// sekali; pemanggil bertanggung jawab menjalankannya sekali per tanggal bisnis (EOD).
func LoanPenaltyAmount(overduePrincipal decimal.Decimal, daysToAccrue int, ratePerMille decimal.Decimal) decimal.Decimal {
	if daysToAccrue <= 0 || !overduePrincipal.IsPositive() || !ratePerMille.IsPositive() {
		return decimal.Zero
	}
	dailyRate := ratePerMille.Div(decimal.NewFromInt(1000))
	return RoundToRupiah(overduePrincipal.Mul(dailyRate).Mul(decimal.NewFromInt(int64(daysToAccrue))))
}

// LoanPenaltyCandidate adalah satu kredit menunggak yang perlu dihitung dendanya.
type LoanPenaltyCandidate struct {
	LoanID                uuid.UUID
	LoanNumber            string
	ProductID             *uuid.UUID
	DisbursementAccountID uuid.UUID
	Status                LoanStatus
	// OverduePrincipal adalah total pokok angsuran yang lewat jatuh tempo dan belum
	// dibayar pada asOf.
	OverduePrincipal decimal.Decimal
	// OldestDueDate adalah jatuh tempo angsuran tertua yang belum dibayar; nil bila
	// tidak ada tunggakan.
	OldestDueDate *time.Time
	// LastAccruedOn adalah tanggal akrual denda terakhir kredit ini; nil bila belum
	// pernah diakru. Dipakai untuk menghitung selisih hari yang belum diakru.
	LastAccruedOn *time.Time
}

// LoanPenaltyItem adalah hasil pemrosesan satu kredit.
type LoanPenaltyItem struct {
	LoanID           uuid.UUID
	LoanNumber       string
	DPD              int // total hari tunggakan sejak jatuh tempo
	DaysAccrued      int // hari yang diakru pada pemanggilan ini
	OverduePrincipal decimal.Decimal
	Penalty          decimal.Decimal
	JournalReference string
	Status           string // BatchItemAccrued / BatchItemSkipped / BatchItemFailed
	Message          string
}

// LoanPenaltyFailure mencatat kredit yang gagal diproses tanpa menggagalkan batch.
type LoanPenaltyFailure struct {
	LoanID     uuid.UUID
	LoanNumber string
	Error      string
}

// LoanPenaltySummary merangkum satu kali eksekusi akrual denda.
//
// RateConfigured=false berarti tarif denda harian masih 0 (belum diisi operator):
// tidak ada denda yang diakru. Bila ada kredit menunggak, Warning menyebut berapa
// kredit yang terdampak; bila tidak ada tunggakan, Warning kosong agar tidak berisik.
// Overdue adalah jumlah kredit menunggak (jatuh tempo terlewat dan berpokok tunggakan)
// yang akan dikenai denda bila tarif diisi.
type LoanPenaltySummary struct {
	AsOf           time.Time
	RatePerMille   decimal.Decimal
	RateConfigured bool
	Warning        string
	Overdue        int
	Total          int
	Processed      int
	Accrued        int
	Skipped        int
	Failed         int
	TotalPenalty   decimal.Decimal
	Items          []LoanPenaltyItem
	Failures       []LoanPenaltyFailure
}

// LoanPenaltyService adalah kapabilitas akrual denda yang dapat dipanggil orchestrator
// batch tanpa harus bergantung pada seluruh domain.LoanService. loanService
// mengimplementasikan keduanya.
type LoanPenaltyService interface {
	AccruePenalties(ctx context.Context, asOf time.Time, actor Actor) (LoanPenaltySummary, error)
}
