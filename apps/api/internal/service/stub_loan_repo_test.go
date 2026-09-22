package service_test

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubLoanRepo adalah implementasi minimal domain.LoanRepository untuk test yang
// hanya membutuhkan kehadiran tipe, bukan perilaku database.
type stubLoanRepo struct {
	loan              *domain.Loan
	schedules         []domain.LoanSchedule
	penaltyCandidates []domain.LoanPenaltyCandidate
	penaltyKeys       map[string]bool
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

func (s *stubLoanRepo) List(ctx context.Context, limit, offset int, actor domain.Actor) ([]domain.Loan, int, error) {
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

func (s *stubLoanRepo) UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	return s.UpdateStatus(ctx, id, status, approvedBy)
}

func (s *stubLoanRepo) RejectLoanTx(ctx context.Context, tx any, id uuid.UUID, reason string) error {
	if s.loan != nil {
		s.loan.Status = domain.LoanStatusRejected
		s.loan.RejectionReason = reason
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

func (s *stubLoanRepo) MarkDisbursedTx(ctx context.Context, tx any, id uuid.UUID, outstanding decimal.Decimal) error {
	return s.MarkDisbursed(ctx, id, outstanding)
}

func (s *stubLoanRepo) GetSchedules(ctx context.Context, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	return s.schedules, nil
}

func (s *stubLoanRepo) LockLoanTx(ctx context.Context, tx any, id uuid.UUID) (*domain.Loan, error) {
	return s.GetByID(ctx, id)
}

func (s *stubLoanRepo) HasAccrualPostingsTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error) {
	return false, nil
}

func (s *stubLoanRepo) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	return nil
}

func (s *stubLoanRepo) UpdateSchedulePaymentTx(ctx context.Context, _ any, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	return s.UpdateSchedulePayment(ctx, scheduleID, paidPrincipal, paidProfit, settleAccrued, status)
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

func (s *stubLoanRepo) UpdateOutstandingTx(ctx context.Context, _ any, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	return s.UpdateOutstanding(ctx, id, outstanding, penalty)
}

func (s *stubLoanRepo) ListPenaltyCandidates(ctx context.Context, asOf time.Time, _ domain.Actor) ([]domain.LoanPenaltyCandidate, error) {
	return s.penaltyCandidates, nil
}

// AddPenaltyAccruedTx meniru idempotensi berbasis idempotency_key jurnal.
func (s *stubLoanRepo) AddPenaltyAccruedTx(ctx context.Context, tx any, loanID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedOn time.Time) (bool, error) {
	if s.penaltyKeys == nil {
		s.penaltyKeys = map[string]bool{}
	}
	if s.penaltyKeys[idempotencyKey] {
		return false, nil
	}
	s.penaltyKeys[idempotencyKey] = true
	if s.loan != nil && s.loan.ID == loanID {
		s.loan.PenaltyAccrued = s.loan.PenaltyAccrued.Add(amount)
	}
	return true, nil
}

func (s *stubLoanRepo) ListInterestAccrualCandidates(ctx context.Context, asOf time.Time, _ domain.Actor) ([]domain.LoanInterestAccrualCandidate, error) {
	return nil, nil
}

func (s *stubLoanRepo) AddScheduleProfitAccruedTx(ctx context.Context, tx any, scheduleID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedAt time.Time) (bool, error) {
	return false, nil
}

func (s *stubLoanRepo) HasInstallmentPaymentTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error) {
	return false, nil
}

func (s *stubLoanRepo) DeleteSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) error {
	s.schedules = nil
	return nil
}

func (s *stubLoanRepo) GetDisbursementJournalRefTx(ctx context.Context, tx any, loanNumber string) (string, error) {
	return "REF-DISB", nil
}

func (s *stubLoanRepo) GetSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	return s.schedules, nil
}

func (s *stubLoanRepo) CorrectLoanAmountTx(ctx context.Context, tx any, loan *domain.Loan, schedules []domain.LoanSchedule) error {
	s.loan = loan
	s.schedules = schedules
	return nil
}

// NextCorrectionCountTx memenuhi kontrak repo; test yang benar-benar mengoreksi
// nominal memakai correctionLoanRepo yang menaikkan penghitungnya.
func (s *stubLoanRepo) NextCorrectionCountTx(ctx context.Context, tx any, loanID uuid.UUID) (int, error) {
	return 1, nil
}

var _ domain.LoanRepository = (*stubLoanRepo)(nil)
