package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type loanService struct {
	loanRepo    domain.LoanRepository
	accountRepo domain.AccountRepository
	customerRepo domain.CustomerRepository
	ledgerSvc   domain.LedgerService
}

func NewLoanService(
	loanRepo domain.LoanRepository,
	accountRepo domain.AccountRepository,
	customerRepo domain.CustomerRepository,
	ledgerSvc domain.LedgerService,
) domain.LoanService {
	return &loanService{
		loanRepo:     loanRepo,
		accountRepo:  accountRepo,
		customerRepo: customerRepo,
		ledgerSvc:    ledgerSvc,
	}
}

func generateLoanNumber(loanType domain.LoanType) string {
	prefix := "KRD"
	if loanType == domain.LoanTypeMurabahah || loanType == domain.LoanTypeMudharabah {
		prefix = "PMB"
	}
	return fmt.Sprintf("%s-%s-%05d", prefix, time.Now().Format("2006"), time.Now().Nanosecond()%100000)
}

func (s *loanService) ApplyLoan(ctx context.Context, input domain.ApplyLoanInput, aoID uuid.UUID) (*domain.Loan, error) {
	if input.PrincipalAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidLoanAmount
	}
	if input.TermMonths <= 0 {
		return nil, domain.ErrInvalidLoanTerm
	}

	// Verify customer & account exist
	if _, err := s.customerRepo.GetByID(ctx, input.CustomerID); err != nil {
		return nil, errors.New("customer not found")
	}
	acc, err := s.accountRepo.GetByID(ctx, input.DisbursementAccountID)
	if err != nil {
		return nil, errors.New("disbursement account not found")
	}
	if acc.CustomerID != nil && *acc.CustomerID != input.CustomerID {
		return nil, errors.New("disbursement account does not belong to specified customer")
	}

	loanID := uuid.New()
	startDate := time.Now().UTC()

	var schedules []domain.LoanSchedule
	var totalPayable, monthlyInstallment decimal.Decimal

	switch input.Type {
	case domain.LoanTypeMurabahah:
		schedules, totalPayable, monthlyInstallment = domain.GenerateMurabahahSchedule(
			loanID, input.PrincipalAmount, input.MarginAmount, input.TermMonths, startDate,
		)
	default:
		// Default to Flat schedule
		schedules, totalPayable, monthlyInstallment = domain.GenerateFlatSchedule(
			loanID, input.PrincipalAmount, input.InterestRateAnnual, input.TermMonths, startDate,
		)
	}

	now := time.Now().UTC()
	loan := &domain.Loan{
		ID:                    loanID,
		LoanNumber:            generateLoanNumber(input.Type),
		CustomerID:            input.CustomerID,
		DisbursementAccountID: input.DisbursementAccountID,
		Type:                  input.Type,
		Status:                domain.LoanStatusPendingApproval,
		PrincipalAmount:       input.PrincipalAmount,
		InterestRateAnnual:    input.InterestRateAnnual,
		MarginAmount:          input.MarginAmount,
		TotalPayable:          totalPayable,
		TermMonths:            input.TermMonths,
		MonthlyInstallment:    monthlyInstallment,
		AOID:                  &aoID,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := s.loanRepo.Create(ctx, loan, schedules); err != nil {
		return nil, fmt.Errorf("failed to create loan application: %w", err)
	}

	loan.Schedules = schedules
	return loan, nil
}

func (s *loanService) ApproveLoan(ctx context.Context, loanID uuid.UUID, supervisorID uuid.UUID) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusPendingApproval {
		return nil, domain.ErrLoanAlreadyApproved
	}

	if err := s.loanRepo.UpdateStatus(ctx, loanID, domain.LoanStatusApproved, &supervisorID); err != nil {
		return nil, fmt.Errorf("failed to approve loan: %w", err)
	}

	loan.Status = domain.LoanStatusApproved
	loan.ApprovedBy = &supervisorID
	now := time.Now().UTC()
	loan.ApprovedAt = &now
	return loan, nil
}

func (s *loanService) RejectLoan(ctx context.Context, loanID uuid.UUID, supervisorID uuid.UUID) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusPendingApproval {
		return nil, domain.ErrLoanAlreadyApproved
	}

	if err := s.loanRepo.UpdateStatus(ctx, loanID, domain.LoanStatusRejected, &supervisorID); err != nil {
		return nil, fmt.Errorf("failed to reject loan: %w", err)
	}

	loan.Status = domain.LoanStatusRejected
	return loan, nil
}

func (s *loanService) DisburseLoan(ctx context.Context, loanID uuid.UUID, disburserID uuid.UUID) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusApproved {
		if loan.Status == domain.LoanStatusDisbursed {
			return nil, domain.ErrLoanAlreadyDisbursed
		}
		return nil, domain.ErrLoanNotApproved
	}

	// 1. Credit the customer's savings account via Deposit service (increases balance)
	refNo := fmt.Sprintf("DISB-%s", loan.LoanNumber)
	desc := fmt.Sprintf("Disbursement for Loan %s", loan.LoanNumber)

	acc, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, fmt.Errorf("disbursement account not found: %w", err)
	}

	_, err = s.ledgerSvc.Deposit(ctx, domain.DepositRequest{
		AccountNumber:  acc.AccountNumber,
		Amount:         loan.PrincipalAmount,
		Currency:       acc.Currency,
		Description:    desc,
		IdempotencyKey: refNo,
		CreatedBy:      disburserID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to credit customer account during disbursement: %w", err)
	}

	// 2. Mark loan as disbursed
	if err := s.loanRepo.MarkDisbursed(ctx, loanID); err != nil {
		return nil, fmt.Errorf("failed to mark loan as disbursed: %w", err)
	}

	loan.Status = domain.LoanStatusDisbursed
	now := time.Now().UTC()
	loan.DisbursedAt = &now
	return loan, nil
}

func (s *loanService) GetLoan(ctx context.Context, id uuid.UUID) (*domain.Loan, error) {
	return s.loanRepo.GetByID(ctx, id)
}

func (s *loanService) ListLoans(ctx context.Context, page, pageSize int) ([]domain.Loan, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.loanRepo.List(ctx, pageSize, (page-1)*pageSize)
}

func (s *loanService) PayInstallment(ctx context.Context, input domain.PayInstallmentInput, tellerID uuid.UUID) (*domain.LoanSchedule, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, errors.New("loan is not in active disbursed state")
	}

	schedules, err := s.loanRepo.GetSchedules(ctx, loan.ID)
	if err != nil {
		return nil, err
	}

	var target *domain.LoanSchedule
	for i := range schedules {
		if schedules[i].InstallmentNo == input.InstallmentNo {
			target = &schedules[i]
			break
		}
	}
	if target == nil {
		return nil, errors.New("installment schedule not found")
	}

	if target.Status == domain.InstallmentStatusPaid {
		return nil, errors.New("this installment has already been paid in full")
	}

	// Update schedule status
	newStatus := domain.InstallmentStatusPaid
	if err := s.loanRepo.UpdateSchedulePayment(ctx, target.ID, target.PrincipalAmount, target.InterestAmount, newStatus); err != nil {
		return nil, fmt.Errorf("failed to record installment payment: %w", err)
	}

	target.Status = newStatus
	now := time.Now().UTC()
	target.PaidAt = &now
	return target, nil
}
