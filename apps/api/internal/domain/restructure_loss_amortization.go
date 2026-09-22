package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Amortisasi saldo kerugian restrukturisasi memakai metode suku bunga efektif (EIR).
//
// Dasar dokumen (bukan ingatan):
//   - POJK No. 1 Tahun 2024 Pasal 32: BPR wajib menerapkan perlakuan akuntansi
//     Restrukturisasi Kredit sesuai standar akuntansi keuangan dan pedoman akuntansi BPR.
//   - SEOJK No. 21/SEOJK.03/2024 (PA BPR) Bab 5.2 hlm. 61 (ilustrasi jurnal): saldo
//     kerugian dipulihkan perlahan melalui pengakuan pendapatan bunga pada suku bunga
//     efektif orisinal dengan jurnal Db. Kredit yang diberikan / Kr. Pendapatan bunga.
//
// Istilah:
//
//	principalBefore   = sisa pokok KONTRAKTUAL sebelum angsuran ini (jumlah pokok
//	                    angsuran 1..n-1 dikurangkan dari total pokok jadwal).
//	lossBalanceBefore = loans.restructure_loss_balance sebelum amortisasi periode ini.
//	carryingBefore    = principalBefore - lossBalanceBefore (nilai tercatat).
//	eirInterest       = carryingBefore x EIR bulanan orisinal, dibulatkan ke rupiah.
//	contractualProfit = porsi bunga jadwal angsuran periode ini (sudah/bakal diakui
//	                    jalur akrual kontraktual yang ada).
//	amortization      = eirInterest - contractualProfit.
//
// Amortisasi TIDAK menggantikan akrual kontraktual: jalur akrual/pembayaran yang ada
// tetap mengakui contractualProfit. Berkas ini hanya mengakui SELISIHNYA, sehingga
// total pendapatan bunga yang diakui = contractualProfit + (eirInterest -
// contractualProfit) = eirInterest. Karena eirInterest penuh tidak pernah dijurnal di
// sini, tidak ada pendapatan bunga yang dihitung dua kali.

// RestructureLossAmortizationInput adalah masukan murni perhitungan satu periode.
type RestructureLossAmortizationInput struct {
	PrincipalBefore   decimal.Decimal
	LossBalanceBefore decimal.Decimal
	EIRMonthly        decimal.Decimal
	ContractualProfit decimal.Decimal
	// FinalPeriod true berarti angsuran ini yang terakhir pada tenor: sisa saldo
	// kerugian diamortisasi seluruhnya agar tidak ada rupiah yang menggantung.
	FinalPeriod bool
}

// RestructureLossAmortizationResult adalah hasil satu periode.
type RestructureLossAmortizationResult struct {
	EIRInterest      decimal.Decimal
	Amortization     decimal.Decimal
	LossBalanceAfter decimal.Decimal
}

// CalculateRestructureLossAmortization menghitung amortisasi satu periode. Amortisasi
// tidak pernah negatif (pendapatan tidak pernah dibalik) dan tidak pernah melebihi
// saldo kerugian (saldo tidak menjadi negatif). Pada periode terakhir seluruh sisa
// saldo dihabiskan, sehingga pembulatan periode-periode sebelumnya tidak meninggalkan
// sisa menggantung.
//
// Suku bunga efektif yang hilang DITOLAK dengan ErrEIRMissing; perhitungan tidak jatuh
// ke suku bunga kontraktual atau nol.
func CalculateRestructureLossAmortization(in RestructureLossAmortizationInput) (RestructureLossAmortizationResult, error) {
	if !in.EIRMonthly.IsPositive() {
		return RestructureLossAmortizationResult{}, fmt.Errorf(
			"%w: amortisasi saldo kerugian butuh suku bunga efektif bulanan positif, dapat %s",
			ErrEIRMissing, in.EIRMonthly)
	}
	if in.PrincipalBefore.IsNegative() {
		return RestructureLossAmortizationResult{}, fmt.Errorf(
			"%w: sisa pokok sebelum amortisasi negatif %s",
			ErrRestructureLossParamInvalid, in.PrincipalBefore)
	}
	if in.LossBalanceBefore.IsNegative() {
		return RestructureLossAmortizationResult{}, fmt.Errorf(
			"%w: saldo kerugian sebelum amortisasi negatif %s",
			ErrRestructureLossParamInvalid, in.LossBalanceBefore)
	}
	if in.ContractualProfit.IsNegative() {
		return RestructureLossAmortizationResult{}, fmt.Errorf(
			"%w: porsi bunga kontraktual negatif %s",
			ErrRestructureLossParamInvalid, in.ContractualProfit)
	}

	carrying := in.PrincipalBefore.Sub(in.LossBalanceBefore)
	if carrying.IsNegative() {
		return RestructureLossAmortizationResult{}, fmt.Errorf(
			"%w: nilai tercatat %s negatif (pokok %s - saldo kerugian %s)",
			ErrRestructureLossParamInvalid, carrying, in.PrincipalBefore, in.LossBalanceBefore)
	}

	eirInterest := RoundToRupiah(carrying.Mul(in.EIRMonthly))
	amortization := eirInterest.Sub(in.ContractualProfit)
	if amortization.IsNegative() {
		// Bunga kontraktual sudah melebihi bunga efektif nilai tercatat. Tidak ada
		// yang diamortisasi dan pendapatan tidak boleh dikurangi (tidak ada
		// amortisasi negatif).
		amortization = decimal.Zero
	}
	if amortization.GreaterThan(in.LossBalanceBefore) {
		amortization = in.LossBalanceBefore
	}
	if in.FinalPeriod {
		// Akhir tenor: habiskan sisa saldo tepat nol, apa pun hasil pembulatan
		// periode-periode sebelumnya.
		amortization = in.LossBalanceBefore
	}

	return RestructureLossAmortizationResult{
		EIRInterest:      eirInterest,
		Amortization:     amortization,
		LossBalanceAfter: in.LossBalanceBefore.Sub(amortization),
	}, nil
}

// LoanRestructureLossAmortizationCandidate adalah satu kredit yang punya saldo kerugian
// restrukturisasi bukan nol dan perlu diamortisasi pada tanggal bisnis tertentu.
// Penentuan angsuran mana yang diproses dilakukan service dari jadwal tersimpan.
type LoanRestructureLossAmortizationCandidate struct {
	LoanID     uuid.UUID
	LoanNumber string
}

// RestructureLossAmortizationRepository adalah irisan opsional domain.LoanRepository
// untuk amortisasi saldo kerugian. Dipisah dari LoanRepository agar implementasi uji
// yang tidak membutuhkan amortisasi tidak wajib berubah; service memakai type assertion.
type RestructureLossAmortizationRepository interface {
	// ListRestructureLossAmortizationCandidates mengambil kredit bersaldo kerugian
	// bukan nol yang punya angsuran belum diamortisasi dan sudah jatuh tempo, atau
	// yang sudah lunas/dihapusbukukan tetapi saldonya belum nol (penutupan sisa).
	// actor membatasi hasil pada buku yang aktif di instalasi.
	ListRestructureLossAmortizationCandidates(ctx context.Context, asOf time.Time, actor Actor) ([]LoanRestructureLossAmortizationCandidate, error)
	// ApplyRestructureLossAmortizationTx mengurangi saldo kerugian dan menandai
	// angsuran sudah diamortisasi, hanya bila jurnal dengan idempotencyKey tersebut
	// belum ada. scheduleID nil berarti penutupan sisa saldo saat kredit lunas/
	// dihapusbukukan (tanpa angsuran). Nilai kembali false berarti pengulangan
	// idempoten: tidak ada pengurangan kedua.
	ApplyRestructureLossAmortizationTx(ctx context.Context, tx any, loanID uuid.UUID, scheduleID *uuid.UUID, amount decimal.Decimal, idempotencyKey string, amortizedAt time.Time) (bool, error)
}

// RestructureLossAmortizationItem adalah hasil pemrosesan satu angsuran (atau satu
// penutupan sisa saldo) pada satu kredit.
type RestructureLossAmortizationItem struct {
	LoanID           uuid.UUID
	LoanNumber       string
	InstallmentNo    int
	Amount           decimal.Decimal
	JournalReference string
	Status           string
	Message          string
}

// RestructureLossAmortizationFailure mencatat kredit yang gagal diproses tanpa
// menggagalkan seluruh batch.
type RestructureLossAmortizationFailure struct {
	LoanID     uuid.UUID
	LoanNumber string
	Error      string
}

// RestructureLossAmortizationSummary merangkum satu kali eksekusi amortisasi. Warnings
// memuat kondisi yang bukan kegagalan teknis, mis. kredit yang EIR-nya belum tersimpan.
type RestructureLossAmortizationSummary struct {
	AsOf           time.Time
	Total          int
	Processed      int
	Amortized      int
	Skipped        int
	Failed         int
	TotalAmortized decimal.Decimal
	Items          []RestructureLossAmortizationItem
	Failures       []RestructureLossAmortizationFailure
	Warnings       []string
}
