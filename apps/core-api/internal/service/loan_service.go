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
	col, ppapRate, accrualSt := domain.CalculateCollectibility(0)
	loan := &domain.Loan{
		ID:                    loanID,
		LoanNumber:            generateLoanNumber(input.Type),
		CustomerID:            input.CustomerID,
		DisbursementAccountID: input.DisbursementAccountID,
		Type:                  input.Type,
		Status:                domain.LoanStatusPendingApproval,
		Collectibility:        col,
		DPD:                   0,
		AccrualStatus:         accrualSt,
		RequiredPPAP:          input.PrincipalAmount.Mul(ppapRate),
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
	// 1. Credit customer's savings account & Debit Loan Receivable Asset (10300)
	// 1. Post Double-Entry GL Journal: DEBIT Loan Portfolio Asset (10300), CREDIT Customer Savings (20100)
	refNo := fmt.Sprintf("DISB-%s", loan.LoanNumber)
	desc := fmt.Sprintf("Disbursement for Loan %s", loan.LoanNumber)

	acc, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, fmt.Errorf("disbursement account not found: %w", err)
	}

	if s.ledgerSvc != nil {
		_, err := s.ledgerSvc.PostCompoundJournal(ctx, domain.CustomJournalRequest{
			TransactionType: domain.TxTypeTransferInternal,
			Description:     desc,
			IdempotencyKey:  refNo,
			CreatedBy:       disburserID.String(),
			Lines: []domain.CustomJournalLineInput{
				{
					AccountNumber: "10300", // Loan Receivable Portfolio Asset
					Direction:     domain.DirectionDebit,
					Amount:        loan.PrincipalAmount,
					Description:   desc,
				},
				{
					AccountNumber: acc.AccountNumber, // Customer Savings Deposit Account
					Direction:     domain.DirectionCredit,
					Amount:        loan.PrincipalAmount,
					Description:   desc,
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to post disbursement GL journal: %w", err)
		}
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

	// 1. Post Compound GL Journal for Installment Repayment:
	// DEBIT Customer Savings (20100) (or Vault), CREDIT Loan Portfolio Asset (10300) (Principal), CREDIT Interest Income (40100) (Interest/Margin)
	if s.ledgerSvc != nil {
		refNo := fmt.Sprintf("PAY-INST-%s-%d", loan.LoanNumber, input.InstallmentNo)
		desc := fmt.Sprintf("Installment %d Payment for Loan %s", input.InstallmentNo, loan.LoanNumber)
		acc, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
		if err == nil && acc != nil {
			var journalLines []domain.CustomJournalLineInput
			journalLines = append(journalLines, domain.CustomJournalLineInput{
				AccountNumber: acc.AccountNumber,
				Direction:     domain.DirectionDebit,
				Amount:        target.TotalInstallment,
				Description:   desc,
			})
			if target.PrincipalAmount.GreaterThan(decimal.Zero) {
				journalLines = append(journalLines, domain.CustomJournalLineInput{
					AccountNumber: "10300", // Loan Receivable Portfolio Asset
					Direction:     domain.DirectionCredit,
					Amount:        target.PrincipalAmount,
					Description:   fmt.Sprintf("Principal reduction for Loan %s", loan.LoanNumber),
				})
			}
			if target.InterestAmount.GreaterThan(decimal.Zero) {
				journalLines = append(journalLines, domain.CustomJournalLineInput{
					AccountNumber: "40100", // Loan Interest / Margin Revenue Income
					Direction:     domain.DirectionCredit,
					Amount:        target.InterestAmount,
					Description:   fmt.Sprintf("Interest/Margin revenue for Loan %s", loan.LoanNumber),
				})
			}

			_, err = s.ledgerSvc.PostCompoundJournal(ctx, domain.CustomJournalRequest{
				TransactionType: domain.TxTypeTransferInternal,
				Description:     desc,
				IdempotencyKey:  refNo,
				CreatedBy:       tellerID.String(),
				Lines:           journalLines,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to post installment repayment GL journal: %w", err)
			}
		}
	}

	// 2. Update schedule status
	newStatus := domain.InstallmentStatusPaid
	if err := s.loanRepo.UpdateSchedulePayment(ctx, target.ID, target.PrincipalAmount, target.InterestAmount, newStatus); err != nil {
		return nil, fmt.Errorf("failed to record installment payment: %w", err)
	}

	target.Status = newStatus
	now := time.Now().UTC()
	target.PaidAt = &now

	// Check if all schedules are now paid to mark loan as PAID_OFF
	allPaid := true
	for _, sc := range schedules {
		if sc.ID == target.ID {
			continue
		}
		if sc.Status != domain.InstallmentStatusPaid {
			allPaid = false
			break
		}
	}
	if allPaid {
		_ = s.loanRepo.UpdateStatus(ctx, loan.ID, domain.LoanStatusPaidOff, &tellerID)
	}

	return target, nil
}

func (s *loanService) RestructureLoan(ctx context.Context, input domain.RestructureLoanInput, supervisorID uuid.UUID) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, errors.New("only active disbursed loans can be restructured")
	}
	if input.NewTermMonths <= 0 {
		return nil, errors.New("new term months must be positive")
	}

	now := time.Now().UTC()
	loan.IsRestructured = true
	loan.RestructuredCount++
	loan.RestructuredAt = &now
	loan.RestructuringReason = input.Reason
	loan.TermMonths = input.NewTermMonths

	if input.NewInterestRateAnnual.GreaterThan(decimal.Zero) {
		loan.InterestRateAnnual = input.NewInterestRateAnnual
	}
	if input.NewMarginAmount.GreaterThan(decimal.Zero) {
		loan.MarginAmount = input.NewMarginAmount
	}

	// OJK Rule: Post-restructuring initial collectibility is Kol 2 DPK
	col, ppapRate, accrualSt := domain.CalculateCollectibility(45) // DPD 45 = Kol 2 DPK
	loan.Collectibility = col
	loan.AccrualStatus = accrualSt
	loan.RequiredPPAP = loan.PrincipalAmount.Mul(ppapRate)

	// Re-calculate schedules with new parameters
	var schedules []domain.LoanSchedule
	var totalPayable, monthlyInstallment decimal.Decimal

	switch loan.Type {
	case domain.LoanTypeMurabahah:
		schedules, totalPayable, monthlyInstallment = domain.GenerateMurabahahSchedule(
			loan.ID, loan.PrincipalAmount, loan.MarginAmount, loan.TermMonths, now,
		)
	default:
		schedules, totalPayable, monthlyInstallment = domain.GenerateFlatSchedule(
			loan.ID, loan.PrincipalAmount, loan.InterestRateAnnual, loan.TermMonths, now,
		)
	}

	loan.TotalPayable = totalPayable
	loan.MonthlyInstallment = monthlyInstallment
	loan.Schedules = schedules

	return loan, nil
}

func (s *loanService) WriteOffLoan(ctx context.Context, input domain.WriteOffLoanInput, supervisorID uuid.UUID) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, errors.New("only active disbursed loans can be written off")
	}

	// 1. Post GL Journal: DEBIT PPAP Reserve (10900), CREDIT Loan Portfolio Asset (10300)
	if s.ledgerSvc != nil {
		refNo := fmt.Sprintf("WOFF-%s-%s", loan.LoanNumber, uuid.New().String()[:8])
		desc := fmt.Sprintf("Write-off of Loan %s - Reason: %s", loan.LoanNumber, input.Reason)
		_, err := s.ledgerSvc.PostCompoundJournal(ctx, domain.CustomJournalRequest{
			TransactionType: domain.TxTypeTransferInternal,
			Description:     desc,
			IdempotencyKey:  refNo,
			CreatedBy:       supervisorID.String(),
			Lines: []domain.CustomJournalLineInput{
				{
					AccountNumber: "10900", // Allowance for Impairment / PPAP Reserve
					Direction:     domain.DirectionDebit,
					Amount:        loan.PrincipalAmount,
					Description:   desc,
				},
				{
					AccountNumber: "10300", // Loan Portfolio Asset
					Direction:     domain.DirectionCredit,
					Amount:        loan.PrincipalAmount,
					Description:   desc,
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to post write-off GL journal: %w", err)
		}
	}

	if err := s.loanRepo.UpdateStatus(ctx, loan.ID, domain.LoanStatusWrittenOff, &supervisorID); err != nil {
		return nil, fmt.Errorf("failed to update loan status to WRITTEN_OFF: %w", err)
	}

	loan.Status = domain.LoanStatusWrittenOff
	return loan, nil
}

func (s *loanService) RecoverWrittenOffLoan(ctx context.Context, input domain.RecoverWrittenOffLoanInput, tellerID uuid.UUID) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if loan.Status != domain.LoanStatusWrittenOff {
		return nil, errors.New("loan is not in WRITTEN_OFF status")
	}
	if input.RecoveryAmount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("recovery amount must be positive")
	}

	// 1. Post Recovery Double-Entry GL Journal: DEBIT Customer Savings (20100), CREDIT Recovery Revenue (40900)
	if s.ledgerSvc != nil {
		refNo := fmt.Sprintf("RECOV-%s-%s", loan.LoanNumber, uuid.New().String()[:8])
		desc := fmt.Sprintf("Recovery Income of Written-Off Loan %s", loan.LoanNumber)
		acc, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
		if err == nil && acc != nil {
			_, err := s.ledgerSvc.PostCompoundJournal(ctx, domain.CustomJournalRequest{
				TransactionType: domain.TxTypeTransferInternal,
				Description:     desc,
				IdempotencyKey:  refNo,
				CreatedBy:       tellerID.String(),
				Lines: []domain.CustomJournalLineInput{
					{
						AccountNumber: acc.AccountNumber, // Customer Savings Account
						Direction:     domain.DirectionDebit,
						Amount:        input.RecoveryAmount,
						Description:   desc,
					},
					{
						AccountNumber: "40900", // Other Operational Recovery Income
						Direction:     domain.DirectionCredit,
						Amount:        input.RecoveryAmount,
						Description:   desc,
					},
				},
			})
			if err != nil {
				return nil, fmt.Errorf("failed to post recovery GL journal: %w", err)
			}
		}
	}

	return loan, nil
}
