package service_test

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubLoanRepo adalah implementasi minimal domain.LoanRepository untuk test yang
// hanya membutuhkan kehadiran tipe, bukan perilaku database.
type stubLoanRepo struct {
	loan      *domain.Loan
	schedules []domain.LoanSchedule
}

func (s *stubLoanRepo) Create(ctx context.Context, loan *domain.Loan, schedules []domain.LoanSchedule) error {
	s.loan = loan
	s.schedules = schedules
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
	if s.loan == nil {
		return nil, 0, nil
	}
	return []domain.Loan{*s.loan}, 1, nil
}

func (s *stubLoanRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	if s.loan != nil {
		s.loan.Status = status
	}
	return nil
}

func (s *stubLoanRepo) MarkDisbursed(ctx context.Context, id uuid.UUID, outstanding decimal.Decimal) error {
	if s.loan != nil {
		s.loan.Status = domain.LoanStatusDisbursed
		s.loan.OutstandingPrincipal = outstanding
	}
	return nil
}

func (s *stubLoanRepo) GetSchedules(ctx context.Context, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	return s.schedules, nil
}

func (s *stubLoanRepo) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidProfit decimal.Decimal, status domain.InstallmentStatus) error {
	return nil
}

func (s *stubLoanRepo) UpdateRestructure(ctx context.Context, loan *domain.Loan, schedules []domain.LoanSchedule) error {
	s.loan = loan
	s.schedules = schedules
	return nil
}

func (s *stubLoanRepo) UpdateCollectibility(ctx context.Context, id uuid.UUID, col domain.OJKCollectibility, dpd int, accrual domain.AccrualStatus, ppap decimal.Decimal) error {
	return nil
}

func (s *stubLoanRepo) UpdateOutstanding(ctx context.Context, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	return nil
}

var _ domain.LoanRepository = (*stubLoanRepo)(nil)
