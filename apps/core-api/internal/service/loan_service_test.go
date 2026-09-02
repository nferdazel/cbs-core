package service_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestGenerateFlatSchedule(t *testing.T) {
	loanID := uuid.New()
	principal := decimal.NewFromInt(12000000) // Rp 12.000.000
	annualRate := decimal.NewFromFloat(12.0)   // 12% p.a.
	termMonths := 12                           // 12 bulan
	startDate := time.Now()

	schedules, totalPayable, monthlyInstallment := domain.GenerateFlatSchedule(
		loanID, principal, annualRate, termMonths, startDate,
	)

	if len(schedules) != 12 {
		t.Fatalf("expected 12 installment schedules, got %d", len(schedules))
	}

	// Principal = 12M, Interest = 1.44M, Total = 13.44M
	expectedTotalPayable := decimal.NewFromFloat(13440000)
	if !totalPayable.Equal(expectedTotalPayable) {
		t.Fatalf("expected total payable %s, got %s", expectedTotalPayable.String(), totalPayable.String())
	}

	// Monthly = 12M/12 + 1.44M/12 = 1M + 120k = 1.12M
	expectedMonthly := decimal.NewFromFloat(1120000)
	if !monthlyInstallment.Equal(expectedMonthly) {
		t.Fatalf("expected monthly installment %s, got %s", expectedMonthly.String(), monthlyInstallment.String())
	}

	first := schedules[0]
	if !first.PrincipalAmount.Equal(decimal.NewFromFloat(1000000)) {
		t.Errorf("expected monthly principal 1M, got %s", first.PrincipalAmount.String())
	}
	if !first.InterestAmount.Equal(decimal.NewFromFloat(120000)) {
		t.Errorf("expected monthly interest 120k, got %s", first.InterestAmount.String())
	}
}

func TestGenerateMurabahahSchedule(t *testing.T) {
	loanID := uuid.New()
	principal := decimal.NewFromInt(10000000) // Rp 10.000.000 (Harga Pokok)
	margin := decimal.NewFromInt(2000000)      // Rp 2.000.000 (Keuntungan Murabahah)
	termMonths := 10                           // 10 bulan
	startDate := time.Now()

	schedules, totalPayable, monthlyInstallment := domain.GenerateMurabahahSchedule(
		loanID, principal, margin, termMonths, startDate,
	)

	if len(schedules) != 10 {
		t.Fatalf("expected 10 installment schedules, got %d", len(schedules))
	}

	// Total Payable = 10M + 2M = 12M
	expectedTotal := decimal.NewFromFloat(12000000)
	if !totalPayable.Equal(expectedTotal) {
		t.Fatalf("expected total payable 12M, got %s", totalPayable.String())
	}

	// Monthly = 12M / 10 = 1.2M
	expectedMonthly := decimal.NewFromFloat(1200000)
	if !monthlyInstallment.Equal(expectedMonthly) {
		t.Fatalf("expected monthly installment 1.2M, got %s", monthlyInstallment.String())
	}

	first := schedules[0]
	if !first.PrincipalAmount.Equal(decimal.NewFromFloat(1000000)) {
		t.Errorf("expected monthly principal 1M, got %s", first.PrincipalAmount.String())
	}
	if !first.InterestAmount.Equal(decimal.NewFromFloat(200000)) {
		t.Errorf("expected monthly margin 200k, got %s", first.InterestAmount.String())
	}
}
