package service

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// reportService menyusun laporan keuangan agregat.
//
// Laporan di sini (neraca saldo, neraca, laba/rugi, arus kas) SENGAJA tidak
// difilter per cabang meskipun journal_entries kini punya branch_id. Ini adalah
// laporan bank-wide sesuai regulasi: neraca dan laba/rugi harus mencerminkan
// posisi dan kinerja seluruh bank secara utuh. Membatasinya pada cabang pengguna
// akan menyembunyikan posisi bank dari pengguna yang berhak melihatnya. Filter
// cabang hanya berlaku untuk bacaan operasional, bukan laporan agregat.
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

// GetTrialBalance menghitung neraca saldo periode dari jurnal. Repositori melakukan
// agregasi SQL dan saldo akhir memakai domain.ClosingBalance.
func (s *reportService) GetTrialBalance(ctx context.Context, from, to time.Time, book string) ([]domain.TrialBalanceRow, error) {
	return s.reportRepo.TrialBalance(ctx, from, to, book)
}

// GetIncomeStatement menghitung laba/rugi periode dari jurnal.
func (s *reportService) GetIncomeStatement(ctx context.Context, from, to time.Time, book string) (*domain.IncomeStatement, error) {
	report, err := s.reportRepo.IncomeStatement(ctx, from, to, book)
	if err != nil {
		return nil, err
	}
	// Penjumlahan akhir laba/rugi memakai fungsi murni domain.
	report.NetIncome = domain.NetIncome(report.TotalRevenue, report.TotalExpense)
	return &report, nil
}

// GetBalanceSheet menyusun neraca per tanggal dari jurnal.
func (s *reportService) GetBalanceSheet(ctx context.Context, asOf time.Time, book string) (*domain.BalanceSheet, error) {
	report, err := s.reportRepo.BalanceSheet(ctx, asOf, book)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// GetCashFlow menyusun arus kas periode dari jurnal pada akun kas/bank.
func (s *reportService) GetCashFlow(ctx context.Context, from, to time.Time, book string) (*domain.CashFlow, error) {
	report, err := s.reportRepo.CashFlow(ctx, from, to, book)
	if err != nil {
		return nil, err
	}
	// Netto arus kas selalu sama dengan jumlah ketiga aktivitas.
	report.NetChange = report.Operating.Add(report.Investing).Add(report.Financing)
	return &report, nil
}
