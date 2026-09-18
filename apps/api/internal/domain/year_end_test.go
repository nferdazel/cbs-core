package domain_test

import (
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func TestComputeClosingEntries_ProfitBalanced(t *testing.T) {
	balances := []domain.NominalAccountBalance{
		{COACode: "40100", AccountType: domain.COATypeRevenue, Amount: decimal.NewFromInt(1000)},
		{COACode: "50100", AccountType: domain.COATypeExpense, Amount: decimal.NewFromInt(400)},
	}

	entries, revenue, expense, net, err := domain.ComputeClosingEntries(balances, "30200")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !revenue.Equal(decimal.NewFromInt(1000)) || !expense.Equal(decimal.NewFromInt(400)) || !net.Equal(decimal.NewFromInt(600)) {
		t.Fatalf("total tidak sesuai: rev=%s exp=%s net=%s", revenue, expense, net)
	}

	totalDebit, totalCredit := decimal.Zero, decimal.Zero
	for _, e := range entries {
		if e.Direction == domain.DirectionDebit {
			totalDebit = totalDebit.Add(e.Amount)
		} else {
			totalCredit = totalCredit.Add(e.Amount)
		}
	}
	if !totalDebit.Equal(totalCredit) {
		t.Fatalf("jurnal penutup tidak seimbang: debit=%s kredit=%s", totalDebit, totalCredit)
	}

	// Laba harus dikredit ke laba ditahan 30200.
	foundRetained := false
	for _, e := range entries {
		if e.COACode == "30200" {
			foundRetained = true
			if e.Direction != domain.DirectionCredit || !e.Amount.Equal(decimal.NewFromInt(600)) {
				t.Fatalf("laba ditahan harus kredit 600, dapat %s %s", e.Direction, e.Amount)
			}
		}
	}
	if !foundRetained {
		t.Fatal("baris laba ditahan tidak ditemukan")
	}
}

func TestComputeClosingEntries_LossDebitsRetainedEarnings(t *testing.T) {
	balances := []domain.NominalAccountBalance{
		{COACode: "40100", AccountType: domain.COATypeRevenue, Amount: decimal.NewFromInt(100)},
		{COACode: "50100", AccountType: domain.COATypeExpense, Amount: decimal.NewFromInt(300)},
	}

	entries, _, _, net, err := domain.ComputeClosingEntries(balances, "13200")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !net.Equal(decimal.NewFromInt(-200)) {
		t.Fatalf("rugi harus -200, dapat %s", net)
	}
	for _, e := range entries {
		if e.COACode == "13200" {
			if e.Direction != domain.DirectionDebit || !e.Amount.Equal(decimal.NewFromInt(200)) {
				t.Fatalf("rugi harus didebit 200 ke laba ditahan, dapat %s %s", e.Direction, e.Amount)
			}
			return
		}
	}
	t.Fatal("baris laba ditahan tidak ditemukan")
}

func TestComputeClosingEntries_Empty(t *testing.T) {
	_, _, _, _, err := domain.ComputeClosingEntries(nil, "30200")
	if !errors.Is(err, domain.ErrNoNominalAccounts) {
		t.Fatalf("expected ErrNoNominalAccounts, got %v", err)
	}
}

func TestSavingsInterest_DailyBalance(t *testing.T) {
	// Saldo tetap 1.000.000 selama 30 hari, tarif 12% per tahun, 365 hari.
	daily := make([]domain.DailyBalance, 0, 30)
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		daily = append(daily, domain.DailyBalance{Date: start.AddDate(0, 0, i), Balance: decimal.NewFromInt(1000000)})
	}

	got := domain.SavingsInterest(daily, decimal.NewFromInt(12), 365)
	// 1.000.000 x 0,12 / 365 x 30 = 9.863,01 -> dibulatkan 9.863.
	want := decimal.NewFromInt(9863)
	if !got.Equal(want) {
		t.Fatalf("bunga salah: got %s want %s", got, want)
	}
}

func TestMudharabahShare(t *testing.T) {
	got := domain.MudharabahShare(
		decimal.NewFromInt(1000000),
		decimal.NewFromFloat(0.5),
		decimal.NewFromInt(10000000),
		decimal.NewFromInt(100000000),
	)
	if !got.Equal(decimal.NewFromInt(50000)) {
		t.Fatalf("bagi hasil salah: got %s want 50000", got)
	}
}
