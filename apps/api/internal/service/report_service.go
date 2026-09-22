package service

import (
	"context"
	"sort"
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

// scopedReportBook membatasi buku yang diminta klien ke buku aktor. Laporan
// menerima query param book, dan tanpa pembatasan ini pengguna konvensional dapat
// membaca posisi syariah (dan sebaliknya) hanya dengan mengubah param. Aktor
// lintas buku atau yang bukunya belum ditentukan boleh memilih; aktor satu buku
// dipaksa ke bukunya sendiri. Nilai aktor dibaca dari claims di context; pemanggil
// non-HTTP (uji/service lain) tanpa claims tidak dibatasi.
func scopedReportBook(ctx context.Context, requested string) string {
	claims, ok := domain.ClaimsFromContext(ctx)
	if !ok {
		return requested
	}
	return claims.ToActor("", "").ConstrainedBook(requested)
}

// GetTrialBalance menghitung neraca saldo periode dari jurnal. Repositori melakukan
// agregasi SQL dan saldo akhir memakai domain.ClosingBalance.
func (s *reportService) GetTrialBalance(ctx context.Context, from, to time.Time, book string) ([]domain.TrialBalanceRow, error) {
	return s.reportRepo.TrialBalance(ctx, from, to, scopedReportBook(ctx, book))
}

// GetIncomeStatement menghitung laba/rugi periode dari jurnal.
func (s *reportService) GetIncomeStatement(ctx context.Context, from, to time.Time, book string) (*domain.IncomeStatement, error) {
	report, err := s.reportRepo.IncomeStatement(ctx, from, to, scopedReportBook(ctx, book))
	if err != nil {
		return nil, err
	}
	// Penjumlahan akhir laba/rugi memakai fungsi murni domain.
	report.NetIncome = domain.NetIncome(report.TotalRevenue, report.TotalExpense)
	return &report, nil
}

// GetBalanceSheet menyusun neraca per tanggal dari jurnal.
func (s *reportService) GetBalanceSheet(ctx context.Context, asOf time.Time, book string) (*domain.BalanceSheet, error) {
	report, err := s.reportRepo.BalanceSheet(ctx, asOf, scopedReportBook(ctx, book))
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// ListDueObligations menggabungkan angsuran kredit dan deposito berjangka yang
// jatuh tempo dalam withinDays hari ke depan, termasuk yang sudah lewat, lalu
// mengurutkannya dari tanggal terdekat. Pengurutan dilakukan di sini agar kedua
// sumber data tampil dalam satu urutan yang konsisten.
func (s *reportService) ListDueObligations(ctx context.Context, asOf time.Time, withinDays int, actor domain.Actor) ([]domain.DueObligation, error) {
	if withinDays < 0 {
		withinDays = 0
	}
	base := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	until := base.AddDate(0, 0, withinDays)

	installments, err := s.reportRepo.ListDueLoanInstallments(ctx, base, until, actor)
	if err != nil {
		return nil, err
	}
	deposits, err := s.reportRepo.ListDueDeposits(ctx, base, until, actor)
	if err != nil {
		return nil, err
	}

	items := make([]domain.DueObligation, 0, len(installments)+len(deposits))
	items = append(items, installments...)
	items = append(items, deposits...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DueDate.Equal(items[j].DueDate) {
			return items[i].Kind < items[j].Kind
		}
		return items[i].DueDate.Before(items[j].DueDate)
	})
	return items, nil
}

// GetCashFlow menyusun arus kas periode dari jurnal pada akun kas/bank.
func (s *reportService) GetCashFlow(ctx context.Context, from, to time.Time, book string) (*domain.CashFlow, error) {
	report, err := s.reportRepo.CashFlow(ctx, from, to, scopedReportBook(ctx, book))
	if err != nil {
		return nil, err
	}
	// Netto arus kas selalu sama dengan jumlah ketiga aktivitas.
	report.NetChange = report.Operating.Add(report.Investing).Add(report.Financing)
	return &report, nil
}
