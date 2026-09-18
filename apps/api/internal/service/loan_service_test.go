package service_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// buildScheduleContract memverifikasi invariant setiap metode jadwal: jumlah pokok
// selalu sama dengan principal, dan jumlah angsuran konsisten dengan total payable.
func assertScheduleInvariants(t *testing.T, name string, schedules []domain.LoanSchedule, principal decimal.Decimal, total decimal.Decimal) {
	t.Helper()

	sumPrincipal := decimal.Zero
	sumTotal := decimal.Zero
	for i, sc := range schedules {
		if sc.InstallmentNo != i+1 {
			t.Fatalf("%s: nomor angsuran tidak berurutan pada indeks %d", name, i)
		}
		if sc.TotalInstallment.LessThan(decimal.Zero) {
			t.Fatalf("%s: total angsuran negatif pada angsuran %d", name, sc.InstallmentNo)
		}
		sumPrincipal = sumPrincipal.Add(sc.PrincipalAmount)
		sumTotal = sumTotal.Add(sc.TotalInstallment)
	}

	if !sumPrincipal.Equal(principal) {
		t.Fatalf("%s: jumlah pokok %s != principal %s", name, sumPrincipal.String(), principal.String())
	}
	if !sumTotal.Equal(total) {
		t.Fatalf("%s: jumlah angsuran %s != total payable %s", name, sumTotal.String(), total.String())
	}
}

func TestBuildSchedule_Flat(t *testing.T) {
	principal := decimal.NewFromInt(10000000)
	schedules, total, _ := domain.BuildSchedule(uuid.New(), domain.ScheduleParams{
		Principal:  principal,
		AnnualRate: decimal.NewFromInt(12),
		TermMonths: 12,
		StartDate:  time.Now(),
		Method:     domain.ScheduleFlat,
		ProfitType: domain.ProfitTypeInterest,
	})
	if len(schedules) != 12 {
		t.Fatalf("jumlah angsuran %d, ingin 12", len(schedules))
	}
	expectedTotal := decimal.NewFromInt(11200000)
	if !total.Equal(expectedTotal) {
		t.Fatalf("total payable %s, ingin %s", total.String(), expectedTotal.String())
	}
	assertScheduleInvariants(t, "flat", schedules, principal, total)
}

func TestBuildSchedule_AnnuityEqualsPrincipalWithZeroRate(t *testing.T) {
	principal := decimal.NewFromInt(12000000)
	schedules, total, _ := domain.BuildSchedule(uuid.New(), domain.ScheduleParams{
		Principal:  principal,
		AnnualRate: decimal.Zero,
		TermMonths: 12,
		StartDate:  time.Now(),
		Method:     domain.ScheduleAnnuity,
		ProfitType: domain.ProfitTypeInterest,
	})
	if !total.Equal(principal) {
		t.Fatalf("bunga nol: total %s != principal %s", total.String(), principal.String())
	}
	assertScheduleInvariants(t, "annuity-nol", schedules, principal, total)
}

func TestBuildSchedule_AnnuityMonotonicPrincipal(t *testing.T) {
	principal := decimal.NewFromInt(50000000)
	schedules, total, _ := domain.BuildSchedule(uuid.New(), domain.ScheduleParams{
		Principal:  principal,
		AnnualRate: decimal.NewFromInt(12),
		TermMonths: 24,
		StartDate:  time.Now(),
		Method:     domain.ScheduleAnnuity,
		ProfitType: domain.ProfitTypeInterest,
	})
	assertScheduleInvariants(t, "annuity", schedules, principal, total)

	// Pada anuitas, porsi pokok naik dan porsi bunga turun seiring waktu.
	if schedules[0].PrincipalAmount.GreaterThan(schedules[1].PrincipalAmount) {
		t.Fatal("porsi pokok anuitas seharusnya menaik")
	}
	if schedules[0].ProfitAmount.LessThan(schedules[1].ProfitAmount) {
		t.Fatal("porsi bunga anuitas seharusnya menurun")
	}
}

func TestBuildSchedule_Sliding(t *testing.T) {
	principal := decimal.NewFromInt(24000000)
	schedules, total, _ := domain.BuildSchedule(uuid.New(), domain.ScheduleParams{
		Principal:  principal,
		AnnualRate: decimal.NewFromInt(12),
		TermMonths: 24,
		StartDate:  time.Now(),
		Method:     domain.ScheduleSliding,
		ProfitType: domain.ProfitTypeInterest,
	})
	assertScheduleInvariants(t, "sliding", schedules, principal, total)

	if schedules[0].ProfitAmount.LessThan(schedules[1].ProfitAmount) {
		t.Fatal("bunga sliding seharusnya menurun karena mengikuti sisa pokok")
	}
}

func TestBuildSchedule_MurabahahFlatMargin(t *testing.T) {
	principal := decimal.NewFromInt(10000000)
	margin := decimal.NewFromInt(1500000)
	schedules, total, _ := domain.BuildSchedule(uuid.New(), domain.ScheduleParams{
		Principal:  principal,
		Margin:     margin,
		TermMonths: 12,
		StartDate:  time.Now(),
		Method:     domain.ScheduleFlat,
		ProfitType: domain.ProfitTypeMargin,
	})
	assertScheduleInvariants(t, "murabahah", schedules, principal, total)
	if !total.Equal(principal.Add(margin)) {
		t.Fatalf("murabahah: total %s != principal+margin", total.String())
	}
	if schedules[0].ProfitType != domain.ProfitTypeMargin {
		t.Fatalf("profit type %s, ingin MARGIN", schedules[0].ProfitType)
	}
}

func TestBuildSchedule_BagiHasil(t *testing.T) {
	principal := decimal.NewFromInt(20000000)
	projected := decimal.NewFromInt(2000000)
	schedules, total, _ := domain.BuildSchedule(uuid.New(), domain.ScheduleParams{
		Principal:  principal,
		Margin:     projected,
		TermMonths: 10,
		StartDate:  time.Now(),
		Method:     domain.ScheduleBagiHasil,
		ProfitType: domain.ProfitTypeBagiHasil,
	})
	assertScheduleInvariants(t, "bagi-hasil", schedules, principal, total)
	if schedules[0].ProfitType != domain.ProfitTypeBagiHasil {
		t.Fatalf("profit type %s, ingin BAGI_HASIL", schedules[0].ProfitType)
	}
}

func TestCalculateCollectibility_POJK1Tahun2024(t *testing.T) {
	cases := []struct {
		dpd     int
		wantKol domain.OJKCollectibility
		wantRate string
		accrual domain.AccrualStatus
	}{
		{0, domain.CollectibilityKol1, "0.005", domain.AccrualStatusAccrual},
		{90, domain.CollectibilityKol2, "0.01", domain.AccrualStatusAccrual},
		{120, domain.CollectibilityKol3, "0.15", domain.AccrualStatusCash},
		{180, domain.CollectibilityKol4, "0.5", domain.AccrualStatusCash},
		{181, domain.CollectibilityKol5, "1", domain.AccrualStatusCash},
	}
	for _, c := range cases {
		kol, ppap, accrual := domain.CalculateCollectibility(c.dpd)
		if kol != c.wantKol {
			t.Fatalf("DPD %d: kolektibilitas %s, ingin %s", c.dpd, kol, c.wantKol)
		}
		if ppap.String() != c.wantRate {
			t.Fatalf("DPD %d: PPAP %s, ingin %s", c.dpd, ppap.String(), c.wantRate)
		}
		if accrual != c.accrual {
			t.Fatalf("DPD %d: akrual %s, ingin %s", c.dpd, accrual, c.accrual)
		}
	}
}
