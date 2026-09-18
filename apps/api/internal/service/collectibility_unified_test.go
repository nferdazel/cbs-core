package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// collectTestConfig adalah SystemConfigService in-memory untuk test.
type collectTestConfig struct {
	ints map[string]int
	decs map[string]decimal.Decimal
}

func (c *collectTestConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := c.decs[key]; ok {
		return v
	}
	return fallback
}

func (c *collectTestConfig) GetInt(_ context.Context, key string, fallback int) int {
	if v, ok := c.ints[key]; ok {
		return v
	}
	return fallback
}

func (c *collectTestConfig) GetString(_ context.Context, _ string, fallback string) string {
	return fallback
}

func (c *collectTestConfig) GetBool(_ context.Context, _ string, fallback bool) bool { return fallback }

func (c *collectTestConfig) Invalidate(_ string) {}

var _ domain.SystemConfigService = (*collectTestConfig)(nil)

// collectTestLoanRepo adalah LoanRepository minimal; hanya method yang dipakai
// RestructureLoan yang diimplementasikan.
type collectTestLoanRepo struct {
	domain.LoanRepository
	loan *domain.Loan
}

func (s *collectTestLoanRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Loan, error) {
	return s.loan, nil
}

func (s *collectTestLoanRepo) UpdateRestructure(_ context.Context, loan *domain.Loan, _ []domain.LoanSchedule) error {
	s.loan = loan
	return nil
}

// TestCollectibility_SatuAturanPPAPDanRestrukturisasi membuktikan DPD yang sama
// menghasilkan golongan yang sama lewat proses PPAP harian (Preview) maupun lewat
// restrukturisasi kredit, dengan konfigurasi yang sama. Tidak boleh ada dua aturan
// kolektibilitas di satu sistem.
func TestCollectibility_SatuAturanPPAPDanRestrukturisasi(t *testing.T) {
	ctx := context.Background()
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		dpd  int
		cfg  domain.SystemConfigService
		want domain.Collectibility
	}{
		{"DPD 45 default POJK", 45, nil, domain.KolKurangLancar},
		{"DPD 100 default POJK", 100, nil, domain.KolDiragukan},
		{"DPD 25 ambang konfigurasi khusus", 25, &collectTestConfig{
			ints: map[string]int{cfgCollectDPKDays: 10, cfgCollectKurangLancarDays: 20, cfgCollectDiragukanDays: 30},
		}, domain.KolDiragukan},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			due := asOf.AddDate(0, 0, -tc.dpd)

			// Jalur PPAP harian. Preview menghitung tanpa memposting/mengubah state.
			loanID := uuid.New()
			ppapRepo := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{{
				LoanID:         loanID,
				LoanNumber:     "KRD-SAMA",
				Outstanding:    decimal.NewFromInt(10_000_000),
				Collectibility: domain.KolLancar,
				RequiredPPAP:   decimal.NewFromInt(50_000),
				LastDueDate:    &due,
			}}}
			ppapSvc := newTestPPAPService(ppapRepo, &stubProductRepo{}, &stubPosting{})
			ppapSvc.config = tc.cfg
			summary, err := ppapSvc.Preview(ctx, asOf)
			if err != nil {
				t.Fatalf("Preview PPAP: %v", err)
			}
			if len(summary.Items) != 1 {
				t.Fatalf("item PPAP %d, ingin 1", len(summary.Items))
			}
			fromPPAP := summary.Items[0].Collectibility

			// Jalur restrukturisasi dengan DPD yang sama.
			productID := uuid.New()
			loanRepo := &collectTestLoanRepo{loan: &domain.Loan{
				ID:                   loanID,
				Status:               domain.LoanStatusDisbursed,
				ProductID:            &productID,
				DPD:                  tc.dpd,
				PrincipalAmount:      decimal.NewFromInt(10_000_000),
				OutstandingPrincipal: decimal.NewFromInt(10_000_000),
				TermMonths:           12,
				InterestRateAnnual:   decimal.NewFromInt(12),
			}}
			loanSvc := &loanService{
				loanRepo: loanRepo,
				productRepo: &stubProductRepo{product: &domain.BankingProduct{
					ID:             productID,
					Code:           "KRD-FLAT",
					ProfitScheme:   domain.SchemeInterest,
					ScheduleMethod: domain.ScheduleFlat,
					RateAnnual:     decimal.NewFromInt(12),
				}},
				config: tc.cfg,
			}
			restructured, err := loanSvc.RestructureLoan(ctx, domain.RestructureLoanInput{
				LoanID:        loanID,
				NewTermMonths: 12,
			}, domain.Actor{})
			if err != nil {
				t.Fatalf("RestructureLoan: %v", err)
			}
			fromRestrukturisasi := domain.CollectibilityFromOJK(restructured.Collectibility)

			if fromPPAP != fromRestrukturisasi {
				t.Fatalf("aturan berbeda: PPAP %s vs restrukturisasi %s",
					fromPPAP.Label(), fromRestrukturisasi.Label())
			}
			if fromPPAP != tc.want {
				t.Fatalf("DPD %d: got %s, want %s", tc.dpd, fromPPAP.Label(), tc.want.Label())
			}
		})
	}
}
