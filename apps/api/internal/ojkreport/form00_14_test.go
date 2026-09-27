package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Form 00.14 mengagregasi jumlah rekening dan total nominal per produk simpanan
// menurut golongan nasabah (sandi Lampiran 02). Golongan kosong ditulis "-".
func TestBuildForm00_14BarisDanNominal(t *testing.T) {
	rows := []SavingsCustomerTypeRow{
		{
			ProductFamily:    "SAVINGS",
			ProductCode:      "TAB-01",
			ProductName:      "Tabungan Umum",
			CustomerTypeCode: "871",
			AccountCount:     3,
			TotalAmount:      decimal.NewFromInt(15_000_000),
		},
		{
			ProductFamily: "TIME_DEPOSIT",
			ProductCode:   "DEP-01",
			ProductName:   "Deposito Berjangka",
			// Golongan nasabah belum diisi bank.
			AccountCount: 2,
			TotalAmount:  decimal.NewFromInt(100_000_000),
		},
	}

	sec := buildForm00_14(rows)
	if sec.Form != "00.14" {
		t.Fatalf("form = %s, ingin 00.14", sec.Form)
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(sec.Rows))
	}

	baris := findCell(t, sec, "TAB-01 / 871", form0014SandiTotalNominal)
	if baris.Value != "15000000" {
		t.Fatalf("total tabungan = %q, ingin 15000000", baris.Value)
	}
	if got := findCell(t, sec, "TAB-01 / 871", form0014SandiGolongan).Value; got != "871" {
		t.Fatalf("golongan = %q, ingin 871", got)
	}
	if got := findCell(t, sec, "TAB-01 / 871", form0014SandiJenisProduk).Value; got != "Tabungan" {
		t.Fatalf("jenis produk = %q, ingin Tabungan", got)
	}
	if got := findCell(t, sec, "DEP-01 / -", form0014SandiGolongan).Value; got != "-" {
		t.Fatalf("golongan kosong = %q, ingin '-'", got)
	}
	if got := findCell(t, sec, "DEP-01 / -", form0014SandiJenisProduk).Value; got != "Deposito Berjangka" {
		t.Fatalf("jenis produk deposito = %q", got)
	}
}

// savingsStubSource melengkapi ojkStubSource dengan agregasi Form 00.14.
type savingsStubSource struct {
	*ojkStubSource
	savings []SavingsCustomerTypeRow
}

func (s *savingsStubSource) ListSavingsCustomerTypes(_ context.Context, _ domain.Actor) ([]SavingsCustomerTypeRow, error) {
	return s.savings, nil
}

// Dengan sumber simpanan, Form 00.14 muncul pada tabel bundle bulanan.
func TestGenerateMonthlyMemuatForm00_14(t *testing.T) {
	base := newStubSource()
	src := &savingsStubSource{
		ojkStubSource: &ojkStubSource{stubSource: base},
		savings: []SavingsCustomerTypeRow{
			{ProductFamily: "SAVINGS", ProductCode: "TAB-01", ProductName: "Tabungan", CustomerTypeCode: "871", AccountCount: 1, TotalAmount: decimal.NewFromInt(5000)},
		},
	}
	b, err := NewBuilder(src).GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.14")
	if got := findCell(t, sec, "TAB-01 / 871", form0014SandiTotalNominal).Value; got != "5000" {
		t.Fatalf("total nominal = %q, ingin 5000", got)
	}
}
