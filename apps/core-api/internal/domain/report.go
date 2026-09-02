package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// --- Financial Statement Models ---

type TrialBalanceItem struct {
	COACode       string          `json:"coa_code"`
	COAName       string          `json:"coa_name"`
	AccountType   COAType         `json:"account_type"`
	DebitBalance  decimal.Decimal `json:"debit_balance"`
	CreditBalance decimal.Decimal `json:"credit_balance"`
}

type TrialBalanceReport struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	Items        []TrialBalanceItem `json:"items"`
	TotalDebit   decimal.Decimal    `json:"total_debit"`
	TotalCredit  decimal.Decimal    `json:"total_credit"`
	IsBalanced   bool               `json:"is_balanced"`
}

type AccountBalanceSummary struct {
	COACode string          `json:"coa_code"`
	COAName string          `json:"coa_name"`
	Balance decimal.Decimal `json:"balance"`
}

type BalanceSheetReport struct {
	AsOfDate                time.Time               `json:"as_of_date"`
	Assets                  []AccountBalanceSummary `json:"assets"`
	Liabilities             []AccountBalanceSummary `json:"liabilities"`
	Equity                  []AccountBalanceSummary `json:"equity"`
	TotalAssets             decimal.Decimal         `json:"total_assets"`
	TotalLiabilities        decimal.Decimal         `json:"total_liabilities"`
	TotalEquity             decimal.Decimal         `json:"total_equity"`
	TotalLiabilitiesAndEquity decimal.Decimal       `json:"total_liabilities_and_equity"`
	IsBalanced              bool                    `json:"is_balanced"`
}

type IncomeStatementReport struct {
	StartDate    time.Time               `json:"start_date"`
	EndDate      time.Time               `json:"end_date"`
	Revenues     []AccountBalanceSummary `json:"revenues"`
	Expenses     []AccountBalanceSummary `json:"expenses"`
	TotalRevenue decimal.Decimal         `json:"total_revenue"`
	TotalExpense decimal.Decimal         `json:"total_expense"`
	NetIncome    decimal.Decimal         `json:"net_income"`
}

// --- Interfaces ---

type ReportRepository interface {
	GetTrialBalance(ctx context.Context) ([]TrialBalanceItem, error)
	GetCOABalances(ctx context.Context) (map[string]decimal.Decimal, map[string]COAType, map[string]string, error)
}

type ReportService interface {
	GenerateTrialBalance(ctx context.Context) (*TrialBalanceReport, error)
	GenerateBalanceSheet(ctx context.Context, asOfDate time.Time) (*BalanceSheetReport, error)
	GenerateIncomeStatement(ctx context.Context, startDate, endDate time.Time) (*IncomeStatementReport, error)
}
