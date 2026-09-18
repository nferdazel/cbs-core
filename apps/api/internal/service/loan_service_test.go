package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
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

// stubLoanProductRepo adalah ProductRepository minimal untuk test restrukturisasi.
type stubLoanProductRepo struct {
	domain.ProductRepository
	product *domain.BankingProduct
}

func (s *stubLoanProductRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.BankingProduct, error) {
	return s.product, nil
}

var _ domain.ProductRepository = (*stubLoanProductRepo)(nil)

// Restrukturisasi memakai ambang DPD dan tarif dari konfigurasi yang sama dengan
// PPAP harian. DPD 45 masuk rentang 31-90 (Kurang Lancar), bukan lagi DPK seperti
// aturan lama yang memakai DPK 1-90.
func TestRestructureLoan_MemakaiAturanPOJKDariKonfigurasi(t *testing.T) {
	loanID := uuid.New()
	productID := uuid.New()
	repo := &stubLoanRepo{loan: &domain.Loan{
		ID:                   loanID,
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		DPD:                  45,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
	}}
	products := &stubLoanProductRepo{product: &domain.BankingProduct{
		ID:             productID,
		Code:           "KRD-FLAT",
		ProfitScheme:   domain.SchemeInterest,
		ScheduleMethod: domain.ScheduleFlat,
		RateAnnual:     decimal.NewFromInt(12),
	}}
	// Konfigurasi kosong memakai fallback POJK: DPK 30, Kurang Lancar 90, Diragukan 180.
	config := &stubLimitConfig{values: map[string]decimal.Decimal{}}
	svc := service.NewLoanService(nil, repo, products, nil, nil, nil, config)

	loan, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:        loanID,
		NewTermMonths: 12,
	}, domain.Actor{})
	if err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if loan.Collectibility != domain.CollectibilityKol3 {
		t.Fatalf("DPD 45 harus Kurang Lancar, dapat %s", loan.Collectibility)
	}
	if loan.AccrualStatus != domain.AccrualStatusCash {
		t.Fatalf("NPL harus cash basis, dapat %s", loan.AccrualStatus)
	}
	// required_ppap tidak dihitung ulang di sini: nilainya berarti cadangan yang sudah
	// dibukukan dan hanya batch PPAP yang boleh mengubahnya, karena batch itulah yang
	// memposting selisih jurnalnya. Kredit ini belum pernah dicadangkan, jadi tetap nol.
	if !loan.RequiredPPAP.IsZero() {
		t.Fatalf("required_ppap %s, ingin tetap 0 sampai batch PPAP berjalan", loan.RequiredPPAP)
	}
}

// Restrukturisasi kredit cabang lain harus ditolak bagi peran operasional, tetapi
// tetap boleh dijalankan oleh pelaku lintas cabang (pengawas/batch).
func TestRestructureLoan_MenolakKreditCabangLain(t *testing.T) {
	loanID := uuid.New()
	productID := uuid.New()
	loanBranchID := uuid.New()

	newService := func(loanBranch string) domain.LoanService {
		repo := &stubLoanRepo{loan: &domain.Loan{
			ID:                   loanID,
			Status:               domain.LoanStatusDisbursed,
			ProductID:            &productID,
			BranchID:             &loanBranchID,
			BranchCode:           loanBranch,
			PrincipalAmount:      decimal.NewFromInt(10_000_000),
			OutstandingPrincipal: decimal.NewFromInt(10_000_000),
			TermMonths:           12,
			InterestRateAnnual:   decimal.NewFromInt(12),
		}}
		products := &stubLoanProductRepo{product: &domain.BankingProduct{
			ID:             productID,
			Code:           "KRD-FLAT",
			ProfitScheme:   domain.SchemeInterest,
			ScheduleMethod: domain.ScheduleFlat,
			RateAnnual:     decimal.NewFromInt(12),
		}}
		config := &stubLimitConfig{values: map[string]decimal.Decimal{}}
		return service.NewLoanService(nil, repo, products, nil, nil, nil, config)
	}
	input := domain.RestructureLoanInput{LoanID: loanID, NewTermMonths: 12}

	t.Run("teller cabang berbeda ditolak", func(t *testing.T) {
		_, err := newService("002").RestructureLoan(context.Background(), input,
			domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
		if !errors.Is(err, domain.ErrCrossBranchAccess) {
			t.Fatalf("mau ErrCrossBranchAccess, dapat %v", err)
		}
	})

	t.Run("system lintas cabang boleh", func(t *testing.T) {
		if _, err := newService("002").RestructureLoan(context.Background(), input,
			domain.SystemActor(uuid.New())); err != nil {
			t.Fatalf("batch seharusnya boleh merestrukturisasi lintas cabang: %v", err)
		}
	})
}
