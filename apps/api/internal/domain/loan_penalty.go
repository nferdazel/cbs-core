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

// LoanPenaltyCap menghitung plafon denda PER KREDIT: capPercent persen dari pokok
// angsuran yang tertunggak saat ini. Basisnya sama dengan basis denda (pokok tunggakan,
// bukan seluruh sisa pokok), sehingga plafon selalu sebanding dengan kewajiban yang
// benar-benar menunggak. Plafon diterapkan pada denda TERAKRU YANG BELUM DIBAYAR
// (loans.penalty_accrued): begitu denda dibayar, ruang plafon terbuka lagi — yang
// dibatasi adalah saldo denda berjalan, bukan akumulasi seumur kredit.
//
// capPercent <= 0 berarti tanpa plafon dan fungsi mengembalikan nol; pemanggil wajib
// memeriksa capPercent.IsPositive() lebih dulu agar nol tidak dibaca sebagai "plafon
// nol" (tidak pernah mengakru).
func LoanPenaltyCap(overduePrincipal, capPercent decimal.Decimal) decimal.Decimal {
	if !overduePrincipal.IsPositive() || !capPercent.IsPositive() {
		return decimal.Zero
	}
	return RoundToRupiah(overduePrincipal.Mul(capPercent).Div(decimal.NewFromInt(100)))
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
	// PenaltyAccrued adalah saldo denda terakru yang belum dibayar pada kredit ini
	// (loans.penalty_accrued). Dipakai membandingkan dengan plafon: ruang plafon yang
	// tersisa = LoanPenaltyCap(OverduePrincipal) - PenaltyAccrued.
	PenaltyAccrued decimal.Decimal
	// OldestDueDate adalah jatuh tempo angsuran tertua yang belum dibayar; nil bila
	// tidak ada tunggakan.
	OldestDueDate *time.Time
	// FinalDueDate adalah jatuh tempo Kredit (angsuran terakhir), dipakai menilai
	// dimensi "Kredit telah jatuh tempo" POJK 1/2024 Lampiran II saat menentukan
	// kolektibilitas. nil berarti jadwal belum diketahui dan dimensi jatuh tempo
	// dianggap belum lewat.
	FinalDueDate *time.Time
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
	// Capped menandai bahwa nominal (atau seluruh akrual) hari ini dibatasi plafon
	// denda. Terlihat di ringkasan agar tunggakan berbulan-bulan tidak menghentikan
	// denda diam-diam.
	Capped bool
	// CapAmount adalah plafon denda yang berlaku (0 bila plafon tidak diaktifkan).
	CapAmount decimal.Decimal
	// SyariahSocialFund menandai denda pembiayaan syariah yang diposting ke Dana
	// Kebajikan (bukan pendapatan bank). Sikap sementara menunggu keputusan DPS.
	SyariahSocialFund bool
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
	// CapPercent adalah plafon denda berlaku dalam persen dari pokok tunggakan;
	// 0 berarti plafon tidak diaktifkan.
	CapPercent decimal.Decimal
	// Capped adalah jumlah kredit yang akrualnya dibatasi atau dihentikan plafon
	// pada eksekusi ini. Wajib terlihat di ringkasan EOD, bukan berhenti diam-diam.
	Capped int
	// SyariahSocialFund adalah jumlah kredit syariah yang dendanya diposting ke Dana
	// Kebajikan, bukan pendapatan bank. Sikap sementara menunggu keputusan DPS.
	SyariahSocialFund int
}

// LoanPenaltyService adalah kapabilitas akrual denda yang dapat dipanggil orchestrator
// batch tanpa harus bergantung pada seluruh domain.LoanService. loanService
// mengimplementasikan keduanya.
type LoanPenaltyService interface {
	AccruePenalties(ctx context.Context, asOf time.Time, actor Actor) (LoanPenaltySummary, error)
}
