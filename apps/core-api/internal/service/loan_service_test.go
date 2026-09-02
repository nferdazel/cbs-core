package service_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestGenerateFlatSchedule_ExactRemainderAbsorption(t *testing.T) {
	loanID := uuid.New()
	// Principal 10,000,000 / 3 months = 3,333,333.3333... per month
	principal := decimal.NewFromInt(10000000)
	annualRate := decimal.NewFromInt(12) // 12% per year = 1% per month = 100,000 interest/month
	termMonths := 3

	schedules, totalPayable, _ := domain.GenerateFlatSchedule(loanID, principal, annualRate, termMonths, time.Now())

	var sumPrincipal, sumInterest decimal.Decimal
	for _, sc := range schedules {
		sumPrincipal = sumPrincipal.Add(sc.PrincipalAmount)
		sumInterest = sumInterest.Add(sc.InterestAmount)
	}

	// Verify exact sum match without a single cent rounding discrepancy
	if !sumPrincipal.Equal(principal) {
		t.Fatalf("expected sum of schedule principal (%s) == principal (%s)", sumPrincipal.String(), principal.String())
	}
	expectedInterest := decimal.NewFromInt(300000)
	if !sumInterest.Equal(expectedInterest) {
		t.Fatalf("expected sum of schedule interest (%s) == expected interest (%s)", sumInterest.String(), expectedInterest.String())
	}
	expectedTotalPayable := principal.Add(expectedInterest)
	if !totalPayable.Equal(expectedTotalPayable) {
		t.Fatalf("expected total payable (%s) == %s", totalPayable.String(), expectedTotalPayable.String())
	}
}

func TestGenerateMurabahahSchedule_ExactRemainderAbsorption(t *testing.T) {
	loanID := uuid.New()
	principal := decimal.NewFromInt(10000000)
	margin := decimal.NewFromInt(1500000) // 1,500,000 margin / 3 months = 500,000 margin/month
	termMonths := 3

	schedules, totalPayable, _ := domain.GenerateMurabahahSchedule(loanID, principal, margin, termMonths, time.Now())

	var sumPrincipal, sumMargin decimal.Decimal
	for _, sc := range schedules {
		sumPrincipal = sumPrincipal.Add(sc.PrincipalAmount)
		sumMargin = sumMargin.Add(sc.InterestAmount)
	}

	if !sumPrincipal.Equal(principal) {
		t.Fatalf("expected sum principal %s == %s", sumPrincipal.String(), principal.String())
	}
	if !sumMargin.Equal(margin) {
		t.Fatalf("expected sum margin %s == %s", sumMargin.String(), margin.String())
	}
	if !totalPayable.Equal(principal.Add(margin)) {
		t.Fatalf("expected total payable %s == %s", totalPayable.String(), principal.Add(margin).String())
	}
}
