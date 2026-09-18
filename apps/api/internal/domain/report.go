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

// --- Laporan berbasis jurnal (journal_lines sebagai sumber kebenaran) ---

// TrialBalanceRow adalah baris neraca saldo per akun COA daun.
type TrialBalanceRow struct {
	AccountCode    string          `json:"account_code"`
	AccountName    string          `json:"account_name"`
	Book           string          `json:"book"`
	NormalBalance  BalanceType     `json:"normal_balance"`
	TotalDebit     decimal.Decimal `json:"total_debit"`
	TotalCredit    decimal.Decimal `json:"total_credit"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
	ClosingBalance decimal.Decimal `json:"closing_balance"`
}

// ReportRow adalah baris ringkas satu akun pada laporan laba/rugi, neraca, atau arus kas.
type ReportRow struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Book        string          `json:"book"`
	Amount      decimal.Decimal `json:"amount"`
}

type IncomeStatement struct {
	TotalRevenue decimal.Decimal `json:"total_revenue"`
	TotalExpense decimal.Decimal `json:"total_expense"`
	NetIncome    decimal.Decimal `json:"net_income"`
	Rows         []ReportRow     `json:"rows"`
}

type BalanceSheet struct {
	TotalAssets      decimal.Decimal `json:"total_assets"`
	TotalLiabilities decimal.Decimal `json:"total_liabilities"`
	TotalEquity      decimal.Decimal `json:"total_equity"`
	NetIncome        decimal.Decimal `json:"net_income"`
	Rows             []ReportRow     `json:"rows"`
}

type CashFlow struct {
	Operating decimal.Decimal `json:"operating"`
	Investing decimal.Decimal `json:"investing"`
	Financing decimal.Decimal `json:"financing"`
	NetChange decimal.Decimal `json:"net_change"`
	Rows      []ReportRow     `json:"rows"`
}

// ClosingBalance menghitung saldo akhir menurut sifat saldo normal akun:
// DEBIT  = saldo awal + debit - kredit
// CREDIT = saldo awal + kredit - debit
// Fungsi murni tanpa DB agar perhitungan saldo dapat diuji terpisah.
func ClosingBalance(normalBalance BalanceType, opening, debit, credit decimal.Decimal) decimal.Decimal {
	if normalBalance == BalanceTypeCredit {
		return opening.Add(credit).Sub(debit)
	}
	return opening.Add(debit).Sub(credit)
}

// NetIncome menghitung laba/rugi = total pendapatan - total beban.
func NetIncome(totalRevenue, totalExpense decimal.Decimal) decimal.Decimal {
	return totalRevenue.Sub(totalExpense)
}

// NatureOf mengembalikan sifat saldo alami menurut tipe akun. Neraca dan laba/rugi
// memakai sifat ini (bukan normal_balance tersimpan), sehingga akun kontra seperti
// PPAP (ASSET/CREDIT) dan akumulasi penyusutan tetap mengurangi kelompok asetnya.
func NatureOf(t COAType) BalanceType {
	switch t {
	case COATypeAsset, COATypeExpense:
		return BalanceTypeDebit
	default:
		return BalanceTypeCredit
	}
}

// --- Interfaces ---

type ReportRepository interface {
	GetTrialBalance(ctx context.Context) ([]TrialBalanceItem, error)
	GetCOABalances(ctx context.Context) (map[string]decimal.Decimal, map[string]COAType, map[string]string, error)

	// Laporan berbasis jurnal.
	TrialBalance(ctx context.Context, from, to time.Time, book string) ([]TrialBalanceRow, error)
	IncomeStatement(ctx context.Context, from, to time.Time, book string) (IncomeStatement, error)
	BalanceSheet(ctx context.Context, asOf time.Time, book string) (BalanceSheet, error)
	CashFlow(ctx context.Context, from, to time.Time, book string) (CashFlow, error)
}

type ReportService interface {
	GenerateTrialBalance(ctx context.Context) (*TrialBalanceReport, error)
	GenerateBalanceSheet(ctx context.Context, asOfDate time.Time) (*BalanceSheetReport, error)
	GenerateIncomeStatement(ctx context.Context, startDate, endDate time.Time) (*IncomeStatementReport, error)

	// Laporan berbasis jurnal.
	GetTrialBalance(ctx context.Context, from, to time.Time, book string) ([]TrialBalanceRow, error)
	GetIncomeStatement(ctx context.Context, from, to time.Time, book string) (*IncomeStatement, error)
	GetBalanceSheet(ctx context.Context, asOf time.Time, book string) (*BalanceSheet, error)
	GetCashFlow(ctx context.Context, from, to time.Time, book string) (*CashFlow, error)
}
