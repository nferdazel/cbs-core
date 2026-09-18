package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestDepositDailyProfitKonvensional(t *testing.T) {
	// 10.000.000 x 4% / 365 = 1.095,89 dibulatkan menjadi 1.096.
	profit := DepositDailyProfit(
		decimal.NewFromInt(10_000_000),
		decimal.NewFromInt(4),
		decimal.Zero,
		false,
	)
	want := decimal.NewFromInt(1096)
	if !profit.Equal(want) {
		t.Fatalf("bunga harian = %s, mau %s", profit, want)
	}
}

func TestDepositDailyProfitBagiHasilNisbah(t *testing.T) {
	// 10.000.000 x 6% x 0,6 / 365 = 986,30 dibulatkan menjadi 986.
	profit := DepositDailyProfit(
		decimal.NewFromInt(10_000_000),
		decimal.NewFromFloat(0.6), // nisbah pemilik dana
		decimal.NewFromInt(6),     // proyeksi imbal hasil tahunan (%)
		true,
	)
	want := decimal.NewFromInt(986)
	if !profit.Equal(want) {
		t.Fatalf("bagi hasil harian = %s, mau %s", profit, want)
	}
}

func TestDepositDailyProfitTanpaTarif(t *testing.T) {
	profit := DepositDailyProfit(decimal.NewFromInt(10_000_000), decimal.Zero, decimal.NewFromInt(6), true)
	if !profit.IsZero() {
		t.Fatalf("bagi hasil tanpa nisbah harus nol, dapat %s", profit)
	}
}

func TestDepositDailyTaxPPhFinal(t *testing.T) {
	// PPh final 20% dari 1.096 = 219,2 dibulatkan menjadi 219.
	tax := DepositDailyTax(decimal.NewFromInt(1096), decimal.NewFromInt(20))
	want := decimal.NewFromInt(219)
	if !tax.Equal(want) {
		t.Fatalf("pajak harian = %s, mau %s", tax, want)
	}
}

func TestDepositDailyTaxNolBilaBebasPajak(t *testing.T) {
	if tax := DepositDailyTax(decimal.NewFromInt(1096), decimal.Zero); !tax.IsZero() {
		t.Fatalf("pajak harus nol, dapat %s", tax)
	}
}

func TestDepositMaturityProceeds(t *testing.T) {
	// Pokok 10.000.000 + akrual bruto 1.200.000 - pajak 240.000 = 10.960.000.
	proceeds := DepositMaturityProceeds(
		decimal.NewFromInt(10_000_000),
		decimal.NewFromInt(1_200_000),
		decimal.NewFromInt(240_000),
	)
	want := decimal.NewFromInt(10_960_000)
	if !proceeds.Equal(want) {
		t.Fatalf("hak jatuh tempo = %s, mau %s", proceeds, want)
	}
}

func TestDepositMaturityProceedsTanpaPajakSyariah(t *testing.T) {
	proceeds := DepositMaturityProceeds(
		decimal.NewFromInt(5_000_000),
		decimal.NewFromInt(300_000),
		decimal.Zero,
	)
	want := decimal.NewFromInt(5_300_000)
	if !proceeds.Equal(want) {
		t.Fatalf("hak jatuh tempo syariah = %s, mau %s", proceeds, want)
	}
}

func TestDepositMaturityDate(t *testing.T) {
	start := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	got := DepositMaturityDate(start, 3)
	want := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("jatuh tempo = %s, mau %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestDepositMaturityDateTermNolMinimumSatuBulan(t *testing.T) {
	start := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	got := DepositMaturityDate(start, 0)
	if !got.Equal(time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("term nol harus minimal satu bulan, dapat %s", got.Format("2006-01-02"))
	}
}

func TestIsBagiHasilProfit(t *testing.T) {
	if !IsBagiHasilProfit(ProfitTypeBagiHasil) {
		t.Fatal("BAGI_HASIL harus dikenali sebagai bagi hasil")
	}
	if IsBagiHasilProfit(ProfitTypeInterest) {
		t.Fatal("INTEREST bukan bagi hasil")
	}
}
