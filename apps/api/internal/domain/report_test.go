package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func TestClosingBalance_Debit(t *testing.T) {
	// Akun DEBIT: saldo akhir = saldo awal + debit - kredit.
	opening := decimal.NewFromInt(1000)
	debit := decimal.NewFromInt(500)
	credit := decimal.NewFromInt(200)

	got := domain.ClosingBalance(domain.BalanceTypeDebit, opening, debit, credit)
	want := decimal.NewFromInt(1300)
	if !got.Equal(want) {
		t.Fatalf("saldo akhir DEBIT: want %s, got %s", want, got)
	}
}

func TestClosingBalance_Credit(t *testing.T) {
	// Akun CREDIT: saldo akhir = saldo awal + kredit - debit.
	opening := decimal.NewFromInt(1000)
	debit := decimal.NewFromInt(200)
	credit := decimal.NewFromInt(500)

	got := domain.ClosingBalance(domain.BalanceTypeCredit, opening, debit, credit)
	want := decimal.NewFromInt(1300)
	if !got.Equal(want) {
		t.Fatalf("saldo akhir CREDIT: want %s, got %s", want, got)
	}
}

func TestNetIncome(t *testing.T) {
	// Campuran angka: pendapatan 750, beban 320 -> laba 430.
	revenue := decimal.NewFromInt(750)
	expense := decimal.NewFromInt(320)

	got := domain.NetIncome(revenue, expense)
	want := decimal.NewFromInt(430)
	if !got.Equal(want) {
		t.Fatalf("net income: want %s, got %s", want, got)
	}
}

// TestBalanceSheetEquation menguji identitas Aset = Kewajiban + Ekuitas + Laba/Rugi
// berjalan memakai rangkaian mutasi jurnal yang seimbang.
func TestBalanceSheetEquation(t *testing.T) {
	// Data uji (setiap baris seimbang debit == kredit, agregat per akun):
	//   Kas (ASSET)          debit 1000
	//   Kredit (ASSET)       debit 500
	//   Tabungan (LIABILITY) credit 1200
	//   Modal (EQUITY)       credit 200
	//   Pendapatan (REVENUE) credit 150
	//   Beban (EXPENSE)      debit 50
	// Total debit = 1000+500+50 = 1550; total kredit = 1200+200+150 = 1550 (seimbang).
	type acc struct {
		coaType domain.COAType
		debit   int64
		credit  int64
	}
	accounts := []acc{
		{domain.COATypeAsset, 1000, 0},
		{domain.COATypeAsset, 500, 0},
		{domain.COATypeLiability, 0, 1200},
		{domain.COATypeEquity, 0, 200},
		{domain.COATypeRevenue, 0, 150},
		{domain.COATypeExpense, 50, 0},
	}

	assets := decimal.Zero
	liabilities := decimal.Zero
	equity := decimal.Zero
	revenue := decimal.Zero
	expense := decimal.Zero

	for _, a := range accounts {
		amount := domain.ClosingBalance(domain.NatureOf(a.coaType), decimal.Zero,
			decimal.NewFromInt(a.debit), decimal.NewFromInt(a.credit))
		switch a.coaType {
		case domain.COATypeAsset:
			assets = assets.Add(amount)
		case domain.COATypeLiability:
			liabilities = liabilities.Add(amount)
		case domain.COATypeEquity:
			equity = equity.Add(amount)
		case domain.COATypeRevenue:
			revenue = revenue.Add(amount)
		case domain.COATypeExpense:
			expense = expense.Add(amount)
		}
	}

	netIncome := domain.NetIncome(revenue, expense)

	if !assets.Equal(decimal.NewFromInt(1500)) {
		t.Fatalf("assets: want 1500, got %s", assets)
	}
	if !liabilities.Equal(decimal.NewFromInt(1200)) {
		t.Fatalf("liabilities: want 1200, got %s", liabilities)
	}
	if !equity.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("equity: want 200, got %s", equity)
	}
	if !netIncome.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("net income: want 100, got %s", netIncome)
	}

	if !assets.Equal(liabilities.Add(equity).Add(netIncome)) {
		t.Fatalf("neraca tidak seimbang: aset %s != kewajiban+ekuitas+laba %s",
			assets, liabilities.Add(equity).Add(netIncome))
	}
}
