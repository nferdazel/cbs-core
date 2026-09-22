package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ScheduleParams adalah masukan murni untuk pembentukan jadwal angsuran.
type ScheduleParams struct {
	Principal  decimal.Decimal
	AnnualRate decimal.Decimal // persen per tahun (konvensional)
	Margin     decimal.Decimal // margin total (murabahah)
	TermMonths int
	StartDate  time.Time
	Method     ScheduleMethod
	ProfitType ProfitType
}

// BuildSchedule menghasilkan jadwal angsuran sesuai metode produk. Semua nominal
// dibulatkan ke rupiah penuh, dan sisa pembulatan diserap angsuran terakhir agar
// total pokok selalu sama dengan principal.
func BuildSchedule(loanID uuid.UUID, p ScheduleParams) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	term := p.TermMonths
	if term <= 0 {
		term = 12
	}

	switch p.Method {
	case ScheduleAnnuity:
		return buildAnnuity(loanID, p, term)
	case ScheduleSliding:
		return buildSliding(loanID, p, term)
	case ScheduleBagiHasil:
		return buildBagiHasil(loanID, p, term)
	case ScheduleFlat:
		return buildFlat(loanID, p, term)
	default:
		return buildFlat(loanID, p, term)
	}
}

func buildFlat(loanID uuid.UUID, p ScheduleParams, term int) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	var totalProfit decimal.Decimal
	if p.Margin.IsPositive() {
		totalProfit = RoundToRupiah(p.Margin)
	} else {
		totalProfit = RoundToRupiah(interestForTerm(p.Principal, p.AnnualRate, term))
	}
	totalPayable := p.Principal.Add(totalProfit)

	basePrincipal := RoundToRupiah(p.Principal.Div(decimal.NewFromInt(int64(term))))
	baseProfit := RoundToRupiah(totalProfit.Div(decimal.NewFromInt(int64(term))))

	schedules := make([]LoanSchedule, 0, term)
	accPrincipal, accProfit := decimal.Zero, decimal.Zero
	for i := 1; i <= term; i++ {
		var principalPart, profitPart decimal.Decimal
		if i == term {
			principalPart = p.Principal.Sub(accPrincipal)
			profitPart = totalProfit.Sub(accProfit)
		} else {
			principalPart = basePrincipal
			profitPart = baseProfit
			accPrincipal = accPrincipal.Add(principalPart)
			accProfit = accProfit.Add(profitPart)
		}
		schedules = append(schedules, newSchedule(loanID, i, p.StartDate, principalPart, profitPart, p.ProfitType))
	}
	return schedules, totalPayable, schedules[0].TotalInstallment
}

func buildAnnuity(loanID uuid.UUID, p ScheduleParams, term int) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	monthlyRate := p.AnnualRate.Div(decimal.NewFromInt(100)).Div(decimal.NewFromInt(12))
	n := decimal.NewFromInt(int64(term))

	// Angsuran = P * r / (1 - (1+r)^-n). Bila r = 0, jatuh ke pokok dibagi rata.
	var installment decimal.Decimal
	if monthlyRate.IsZero() {
		installment = RoundToRupiah(p.Principal.Div(n))
	} else {
		onePlusR := decimal.NewFromInt(1).Add(monthlyRate)
		factor := onePlusR.Pow(n)
		installment = RoundToRupiah(p.Principal.Mul(monthlyRate).Mul(factor).Div(factor.Sub(decimal.NewFromInt(1))))
	}

	schedules := make([]LoanSchedule, 0, term)
	outstanding := p.Principal
	totalProfit := decimal.Zero
	for i := 1; i <= term; i++ {
		profitPart := RoundToRupiah(outstanding.Mul(monthlyRate))
		principalPart := installment.Sub(profitPart)

		if i == term {
			principalPart = outstanding
			profitPart = installment.Sub(principalPart)
			if profitPart.IsNegative() {
				profitPart = decimal.Zero
			}
		}
		if principalPart.GreaterThan(outstanding) {
			principalPart = outstanding
		}
		outstanding = outstanding.Sub(principalPart)
		totalProfit = totalProfit.Add(profitPart)
		schedules = append(schedules, newSchedule(loanID, i, p.StartDate, principalPart, profitPart, p.ProfitType))
	}
	return schedules, p.Principal.Add(totalProfit), installment
}

func buildSliding(loanID uuid.UUID, p ScheduleParams, term int) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	monthlyRate := p.AnnualRate.Div(decimal.NewFromInt(100)).Div(decimal.NewFromInt(12))
	basePrincipal := RoundToRupiah(p.Principal.Div(decimal.NewFromInt(int64(term))))

	schedules := make([]LoanSchedule, 0, term)
	outstanding := p.Principal
	totalProfit := decimal.Zero
	accPrincipal := decimal.Zero
	for i := 1; i <= term; i++ {
		principalPart := basePrincipal
		if i == term {
			principalPart = p.Principal.Sub(accPrincipal)
		}
		profitPart := RoundToRupiah(outstanding.Mul(monthlyRate))
		outstanding = outstanding.Sub(principalPart)
		accPrincipal = accPrincipal.Add(principalPart)
		totalProfit = totalProfit.Add(profitPart)
		schedules = append(schedules, newSchedule(loanID, i, p.StartDate, principalPart, profitPart, p.ProfitType))
	}

	first := schedules[0].TotalInstallment
	return schedules, p.Principal.Add(totalProfit), first
}

// buildBagiHasil membagi hasil berdasarkan pendapatan yang diproyeksikan. Karena
// pendapatan aktual baru diketahui saat berjalan, jadwal memakai proyeksi proporsional
// terhadap margin yang disepakati; koreksi dilakukan saat realisasi.
func buildBagiHasil(loanID uuid.UUID, p ScheduleParams, term int) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	basePrincipal := RoundToRupiah(p.Principal.Div(decimal.NewFromInt(int64(term))))
	baseProfit := RoundToRupiah(p.Margin.Div(decimal.NewFromInt(int64(term))))

	schedules := make([]LoanSchedule, 0, term)
	accPrincipal, accProfit := decimal.Zero, decimal.Zero
	for i := 1; i <= term; i++ {
		var principalPart, profitPart decimal.Decimal
		if i == term {
			principalPart = p.Principal.Sub(accPrincipal)
			profitPart = p.Margin.Sub(accProfit)
		} else {
			principalPart = basePrincipal
			profitPart = baseProfit
			accPrincipal = accPrincipal.Add(principalPart)
			accProfit = accProfit.Add(profitPart)
		}
		schedules = append(schedules, newSchedule(loanID, i, p.StartDate, principalPart, profitPart, p.ProfitType))
	}

	totalPayable := p.Principal.Add(
		schedules[len(schedules)-1].ProfitAmount.Add(accProfit),
	)
	first := schedules[0].TotalInstallment
	return schedules, totalPayable, first
}

// ValidateProfitSharingRatio memeriksa nisbah bagi hasil berada pada rentang yang
// diizinkan skema, yaitu 0 <= n <= 1. Satu sumber kebenaran yang sama dipakai jalur
// pembentukan jadwal (ProjectBagiHasilMargin) dan jalur pengubahan parameter produk,
// supaya ambang batasnya tidak pernah bercabang.
//
// Nilai 0 berarti parameter belum diisi dan tetap sah disimpan untuk produk
// non-bagi-hasil; yang menolak nisbah nol adalah ProjectBagiHasilMargin, bukan fungsi
// ini, karena jadwal bagi hasil tidak boleh dibentuk tanpa nisbah.
func ValidateProfitSharingRatio(nisbah decimal.Decimal) error {
	if nisbah.IsNegative() || nisbah.GreaterThan(decimal.NewFromInt(1)) {
		return ErrBagiHasilNisbahOutOfRange
	}
	return nil
}

// ValidateProjectedRevenueRateAnnual memeriksa proyeksi pendapatan tahunan berada
// pada rentang yang diizinkan skema, yaitu 0 <= p <= 100 (persen per tahun). Dipakai
// bersama oleh pembentukan jadwal dan pengubahan parameter produk.
//
// Nilai 0 berarti parameter belum diisi dan tetap sah disimpan; ProjectBagiHasilMargin
// yang menolak proyeksi nol bila jadwal bagi hasil akan dibentuk.
func ValidateProjectedRevenueRateAnnual(projectedRevenueRateAnnual decimal.Decimal) error {
	if projectedRevenueRateAnnual.IsNegative() || projectedRevenueRateAnnual.GreaterThan(MaxProjectedRevenueRateAnnual) {
		return ErrBagiHasilProjectionOutOfRange
	}
	return nil
}

// ProjectBagiHasilMargin menghitung PROYEKSI imbal hasil (Rp) untuk jadwal
// angsuran akad bagi hasil (mudharabah/musyarakah).
//
// Angka ini adalah PROYEKSI, bukan bagi hasil yang pasti. Bagi hasil sejatinya
// bergantung pada laba/pendapatan usaha yang dibiayai dan baru diketahui saat
// realisasi. Jadwal memakai tarif ekuivalen = nisbah x proyeksi pendapatan tahunan
// usaha sebagai dasar tagihan, dan angka itu WAJIB disesuaikan saat bagi hasil
// aktual dihitung. Jangan memperlakukan profit_amount pada jadwal ini sebagai laba
// final nasabah.
//
// Bila nisbah atau proyeksi pendapatan produk kosong/nol, fungsi menolak agar tidak
// mengarang angka (ErrBagiHasilNisbahMissing / ErrBagiHasilProjectionMissing).
func ProjectBagiHasilMargin(principal, nisbah, projectedRevenueRateAnnual decimal.Decimal, termMonths int) (decimal.Decimal, error) {
	if !nisbah.IsPositive() {
		return decimal.Zero, ErrBagiHasilNisbahMissing
	}
	if err := ValidateProfitSharingRatio(nisbah); err != nil {
		return decimal.Zero, err
	}
	if !projectedRevenueRateAnnual.IsPositive() {
		return decimal.Zero, ErrBagiHasilProjectionMissing
	}
	if err := ValidateProjectedRevenueRateAnnual(projectedRevenueRateAnnual); err != nil {
		return decimal.Zero, err
	}
	equivalentRate := nisbah.Mul(projectedRevenueRateAnnual)
	return RoundToRupiah(interestForTerm(principal, equivalentRate, termMonths)), nil
}

// interestForTerm menghitung bunga sederhana untuk tenor tertentu.
func interestForTerm(principal, annualRate decimal.Decimal, termMonths int) decimal.Decimal {
	rateFraction := annualRate.Div(decimal.NewFromInt(100))
	monthsFraction := decimal.NewFromInt(int64(termMonths)).Div(decimal.NewFromInt(12))
	return principal.Mul(rateFraction).Mul(monthsFraction)
}

// RoundToRupiah membulatkan nominal ke rupiah penuh dengan Banker's Rounding,
// satu-satunya fungsi pembulatan uang di sistem ini.
func RoundToRupiah(v decimal.Decimal) decimal.Decimal {
	return v.RoundBank(0)
}

func newSchedule(loanID uuid.UUID, no int, start time.Time, principal, profit decimal.Decimal, profitType ProfitType) LoanSchedule {
	return LoanSchedule{
		ID:               uuid.New(),
		LoanID:           loanID,
		InstallmentNo:    no,
		DueDate:          start.AddDate(0, no, 0),
		PrincipalAmount:  principal,
		ProfitAmount:     profit,
		TotalInstallment: principal.Add(profit),
		ProfitType:       profitType,
		Status:           InstallmentStatusPending,
		CreatedAt:        time.Now().UTC(),
	}
}
