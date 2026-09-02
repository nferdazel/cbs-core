package service

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

type reportService struct {
	reportRepo domain.ReportRepository
}

func NewReportService(reportRepo domain.ReportRepository) domain.ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) GenerateTrialBalance(ctx context.Context) (*domain.TrialBalanceReport, error) {
	items, err := s.reportRepo.GetTrialBalance(ctx)
	if err != nil {
		return nil, err
	}

	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, item := range items {
		totalDebit = totalDebit.Add(item.DebitBalance)
		totalCredit = totalCredit.Add(item.CreditBalance)
	}

	return &domain.TrialBalanceReport{
		GeneratedAt: time.Now().UTC(),
		Items:       items,
		TotalDebit:  totalDebit,
		TotalCredit: totalCredit,
		IsBalanced:  totalDebit.Equal(totalCredit),
	}, nil
}

func (s *reportService) GenerateBalanceSheet(ctx context.Context, asOfDate time.Time) (*domain.BalanceSheetReport, error) {
	balances, types, names, err := s.reportRepo.GetCOABalances(ctx)
	if err != nil {
		return nil, err
	}

	var assets, liabilities, equity []domain.AccountBalanceSummary
	totalAssets := decimal.Zero
	totalLiabilities := decimal.Zero
	totalEquity := decimal.Zero

	for code, bal := range balances {
		acctype := types[code]
		name := names[code]

		summary := domain.AccountBalanceSummary{
			COACode: code,
			COAName: name,
			Balance: bal,
		}

		switch acctype {
		case domain.COATypeAsset:
			assets = append(assets, summary)
			totalAssets = totalAssets.Add(bal)
		case domain.COATypeLiability:
			liabilities = append(liabilities, summary)
			totalLiabilities = totalLiabilities.Add(bal)
		case domain.COATypeEquity:
			equity = append(equity, summary)
			totalEquity = totalEquity.Add(bal)
		}
	}

	totalLiabEquity := totalLiabilities.Add(totalEquity)

	return &domain.BalanceSheetReport{
		AsOfDate:                asOfDate,
		Assets:                  assets,
		Liabilities:             liabilities,
		Equity:                  equity,
		TotalAssets:             totalAssets,
		TotalLiabilities:        totalLiabilities,
		TotalEquity:             totalEquity,
		TotalLiabilitiesAndEquity: totalLiabEquity,
		IsBalanced:              totalAssets.Equal(totalLiabEquity),
	}, nil
}

func (s *reportService) GenerateIncomeStatement(ctx context.Context, startDate, endDate time.Time) (*domain.IncomeStatementReport, error) {
	balances, types, names, err := s.reportRepo.GetCOABalances(ctx)
	if err != nil {
		return nil, err
	}

	var revenues, expenses []domain.AccountBalanceSummary
	totalRevenue := decimal.Zero
	totalExpense := decimal.Zero

	for code, bal := range balances {
		acctype := types[code]
		name := names[code]

		summary := domain.AccountBalanceSummary{
			COACode: code,
			COAName: name,
			Balance: bal,
		}

		switch acctype {
		case domain.COATypeRevenue:
			revenues = append(revenues, summary)
			totalRevenue = totalRevenue.Add(bal)
		case domain.COATypeExpense:
			expenses = append(expenses, summary)
			totalExpense = totalExpense.Add(bal)
		}
	}

	netIncome := totalRevenue.Sub(totalExpense)

	return &domain.IncomeStatementReport{
		StartDate:    startDate,
		EndDate:      endDate,
		Revenues:     revenues,
		Expenses:     expenses,
		TotalRevenue: totalRevenue,
		TotalExpense: totalExpense,
		NetIncome:    netIncome,
	}, nil
}
