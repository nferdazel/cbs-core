package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type stubLoanRepo struct {
	loan *domain.Loan
}

func (s *stubLoanRepo) Create(ctx context.Context, loan *domain.Loan, schedules []domain.LoanSchedule) error {
	s.loan = loan
	return nil
}

func (s *stubLoanRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Loan, error) {
	if s.loan != nil && s.loan.ID == id {
		return s.loan, nil
	}
	return nil, domain.ErrLoanNotFound
}

func (s *stubLoanRepo) GetByNumber(ctx context.Context, loanNumber string) (*domain.Loan, error) {
	return s.loan, nil
}

func (s *stubLoanRepo) List(ctx context.Context, limit, offset int) ([]domain.Loan, int, error) {
	return []domain.Loan{*s.loan}, 1, nil
}

func (s *stubLoanRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	if s.loan != nil {
		s.loan.Status = status
	}
	return nil
}

func (s *stubLoanRepo) MarkDisbursed(ctx context.Context, id uuid.UUID) error {
	if s.loan != nil {
		s.loan.Status = domain.LoanStatusDisbursed
	}
	return nil
}

func (s *stubLoanRepo) GetSchedules(ctx context.Context, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	if s != nil && s.loan != nil {
		return s.loan.Schedules, nil
	}
	return nil, nil
}

func (s *stubLoanRepo) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidInterest decimal.Decimal, status domain.InstallmentStatus) error {
	return nil
}

func TestRestructureLoan_OJKRules(t *testing.T) {
	loanID := uuid.New()
	loan := &domain.Loan{
		ID:                    loanID,
		LoanNumber:            "KRD-2026-00001",
		Status:                domain.LoanStatusDisbursed,
		Collectibility:        domain.CollectibilityKol5, // Previously NPL Kol 5
		DPD:                   200,
		AccrualStatus:         domain.AccrualStatusCash,
		PrincipalAmount:       decimal.NewFromInt(50000000),
		InterestRateAnnual:    decimal.NewFromInt(12),
		TermMonths:            12,
		DisbursementAccountID: uuid.New(),
	}

	repo := &stubLoanRepo{loan: loan}
	svc := service.NewLoanService(repo, nil, nil, nil)

	input := domain.RestructureLoanInput{
		LoanID:        loanID,
		NewTermMonths: 24, // Extend term to 24 months
		Reason:        "Restrukturisasi Covid/Dampak Ekonomi",
	}

	supervisorID := uuid.New()
	restructured, err := svc.RestructureLoan(context.Background(), input, supervisorID)
	if err != nil {
		t.Fatalf("unexpected error restructuring loan: %v", err)
	}

	if !restructured.IsRestructured {
		t.Fatal("expected IsRestructured flag to be true")
	}
	if restructured.RestructuredCount != 1 {
		t.Fatalf("expected RestructuredCount == 1, got %d", restructured.RestructuredCount)
	}

	// OJK Rule Check: Post-restructuring initial collectibility MUST be Kol 2 DPK
	if restructured.Collectibility != domain.CollectibilityKol2 {
		t.Fatalf("expected post-restructuring collectibility to be Kol 2 DPK, got %s", restructured.Collectibility)
	}
	if restructured.TermMonths != 24 {
		t.Fatalf("expected new term 24 months, got %d", restructured.TermMonths)
	}
}

func TestWriteOffLoan_And_Recovery(t *testing.T) {
	loanID := uuid.New()
	loan := &domain.Loan{
		ID:                    loanID,
		LoanNumber:            "KRD-2026-99999",
		Status:                domain.LoanStatusDisbursed,
		Collectibility:        domain.CollectibilityKol5, // Kol 5 Macet
		DPD:                   365,
		PrincipalAmount:       decimal.NewFromInt(20000000),
		DisbursementAccountID: uuid.New(),
	}

	repo := &stubLoanRepo{loan: loan}
	svc := service.NewLoanService(repo, nil, nil, nil)

	supervisorID := uuid.New()

	// 1. Write-off execution
	writtenOff, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: loanID,
		Reason: "Macet total, debitur melarikan diri (Hapus Buku Kol 5)",
	}, supervisorID)
	if err != nil {
		t.Fatalf("unexpected error executing write-off: %v", err)
	}
	if writtenOff.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("expected status WRITTEN_OFF, got %s", writtenOff.Status)
	}

	// 2. Recovery execution
	tellerID := uuid.New()
	recovered, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID:         loanID,
		RecoveryAmount: decimal.NewFromInt(5000000), // Recovery 5 Juta
	}, tellerID)
	if err != nil {
		t.Fatalf("unexpected error executing loan recovery: %v", err)
	}
	if recovered.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("expected status WRITTEN_OFF, got %s", recovered.Status)
	}
}

func TestGenerateFlatSchedule_ExactRemainderAbsorption(t *testing.T) {
	loanID := uuid.New()
	principal := decimal.NewFromInt(10000000)
	annualRate := decimal.NewFromInt(12)
	termMonths := 3

	schedules, totalPayable, _ := domain.GenerateFlatSchedule(loanID, principal, annualRate, termMonths, time.Now())

	var sumPrincipal, sumInterest decimal.Decimal
	for _, sc := range schedules {
		sumPrincipal = sumPrincipal.Add(sc.PrincipalAmount)
		sumInterest = sumInterest.Add(sc.InterestAmount)
	}

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
	margin := decimal.NewFromInt(1500000)
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
