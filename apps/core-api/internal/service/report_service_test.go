package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

type stubReportRepo struct{}

func (s *stubReportRepo) GetTrialBalance(ctx context.Context) ([]domain.TrialBalanceItem, error) {
	return []domain.TrialBalanceItem{
		{COACode: "101", COAName: "Kas Vault", AccountType: domain.COATypeAsset, DebitBalance: decimal.NewFromInt(100000), CreditBalance: decimal.Zero},
		{COACode: "201", COAName: "Simpanan Wadiah", AccountType: domain.COATypeLiability, DebitBalance: decimal.Zero, CreditBalance: decimal.NewFromInt(100000)},
	}, nil
}

func (s *stubReportRepo) GetCOABalances(ctx context.Context) (map[string]decimal.Decimal, map[string]domain.COAType, map[string]string, error) {
	balances := map[string]decimal.Decimal{
		"101": decimal.NewFromInt(150000000), // Asset
		"201": decimal.NewFromInt(100000000), // Liability
		"301": decimal.NewFromInt(50000000),  // Equity
		"401": decimal.NewFromInt(20000000),  // Revenue
		"501": decimal.NewFromInt(5000000),   // Expense
	}
	types := map[string]domain.COAType{
		"101": domain.COATypeAsset,
		"201": domain.COATypeLiability,
		"301": domain.COATypeEquity,
		"401": domain.COATypeRevenue,
		"501": domain.COATypeExpense,
	}
	names := map[string]string{
		"101": "Kas Vault",
		"201": "Simpanan Wadiah",
		"301": "Modal Disetor",
		"401": "Pendapatan Margin",
		"501": "Beban Ops",
	}
	return balances, types, names, nil
}

func TestGenerateTrialBalance(t *testing.T) {
	svc := service.NewReportService(&stubReportRepo{})
	report, err := svc.GenerateTrialBalance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.IsBalanced {
		t.Fatal("expected Trial Balance to be balanced")
	}
	if !report.TotalDebit.Equal(decimal.NewFromInt(100000)) {
		t.Fatalf("expected total debit 100k, got %s", report.TotalDebit.String())
	}
}

func TestGenerateBalanceSheet(t *testing.T) {
	svc := service.NewReportService(&stubReportRepo{})
	report, err := svc.GenerateBalanceSheet(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.IsBalanced {
		t.Fatalf("expected Assets (150M) == Liabilities + Equity (100M + 50M = 150M)")
	}
	if !report.TotalAssets.Equal(decimal.NewFromInt(150000000)) {
		t.Fatalf("expected 150M total assets, got %s", report.TotalAssets.String())
	}
}

func TestGenerateIncomeStatement(t *testing.T) {
	svc := service.NewReportService(&stubReportRepo{})
	now := time.Now()
	report, err := svc.GenerateIncomeStatement(context.Background(), now.AddDate(0, -1, 0), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Net Income = 20M Revenue - 5M Expense = 15M
	expectedNet := decimal.NewFromInt(15000000)
	if !report.NetIncome.Equal(expectedNet) {
		t.Fatalf("expected Net Income 15M, got %s", report.NetIncome.String())
	}
}
