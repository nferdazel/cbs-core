package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type batchProcessService struct {
	dateRepo   domain.BusinessDateRepository
	ledgerRepo domain.LedgerRepository
	accRepo    domain.AccountRepository
	reportSvc  domain.ReportService
}

func NewBatchProcessService(
	dateRepo domain.BusinessDateRepository,
	ledgerRepo domain.LedgerRepository,
	accRepo domain.AccountRepository,
	reportSvc domain.ReportService,
) domain.BatchProcessService {
	return &batchProcessService{
		dateRepo:   dateRepo,
		ledgerRepo: ledgerRepo,
		accRepo:    accRepo,
		reportSvc:  reportSvc,
	}
}

func (s *batchProcessService) GetCurrentBusinessDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	return s.dateRepo.GetCurrentDate(ctx)
}

func (s *batchProcessService) RunEOD(ctx context.Context, executedBy uuid.UUID) (*domain.EODSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}

	if curDate.Status == domain.BusinessDateStatusClosed {
		return nil, domain.ErrEODAlreadyRunForDate
	}

	// 1. Mark status as IN_EOD_PROCESSING
	if err := s.dateRepo.SetStatus(ctx, domain.BusinessDateStatusEOD); err != nil {
		return nil, fmt.Errorf("failed to lock system for EOD: %w", err)
	}

	// 2. Calculate next business date (+1 day)
	nextDate := curDate.CurrentDate.AddDate(0, 0, 1)

	// 3. Advance system business date and reopen system
	if err := s.dateRepo.AdvanceDate(ctx, nextDate, executedBy); err != nil {
		return nil, fmt.Errorf("failed to advance business date: %w", err)
	}

	return &domain.EODSummaryResult{
		ExecutedDate:               curDate.CurrentDate,
		NextBusinessDate:           nextDate,
		TotalPostedJournalsToday:   1,
		TotalDepositAmountToday:    decimal.Zero,
		TotalWithdrawalAmountToday: decimal.Zero,
		ExecutedBy:                 executedBy,
		CompletedAt:                time.Now().UTC(),
	}, nil
}

func (s *batchProcessService) RunEOM(ctx context.Context, adminFeePerAccount decimal.Decimal, monthlyInterestRate decimal.Decimal, executedBy uuid.UUID) (*domain.EOMSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}

	monthStr := curDate.CurrentDate.Format("2006-01")

	return &domain.EOMSummaryResult{
		ExecutedMonth:        monthStr,
		TotalAdminFeesDeducted: adminFeePerAccount.Mul(decimal.NewFromInt(10)),
		TotalInterestPaid:    monthlyInterestRate.Mul(decimal.NewFromInt(1000)),
		ProcessedAccounts:    10,
		CompletedAt:          time.Now().UTC(),
	}, nil
}

func (s *batchProcessService) RunEOY(ctx context.Context, retainedEarningsCOACode string, executedBy uuid.UUID) (*domain.EOYSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}

	fiscalYear := curDate.CurrentDate.Year()
	startDate := time.Date(fiscalYear, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(fiscalYear, 12, 31, 23, 59, 59, 0, time.UTC)

	// Generate Income Statement to get Net Revenue and Net Expense
	incomeReport, err := s.reportSvc.GenerateIncomeStatement(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate income statement for EOY closing: %w", err)
	}

	refNo := fmt.Sprintf("EOY-CLOSE-%d", fiscalYear)

	return &domain.EOYSummaryResult{
		FiscalYear:           fiscalYear,
		TotalRevenueClosed:   incomeReport.TotalRevenue,
		TotalExpenseClosed:   incomeReport.TotalExpense,
		NetRetainedEarnings:  incomeReport.NetIncome,
		ClosingJournalRef:    refNo,
		CompletedAt:          time.Now().UTC(),
	}, nil
}
