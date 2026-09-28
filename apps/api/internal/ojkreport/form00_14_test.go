package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Laporan internal jenis nasabah per produk (dulu diberi nomor Form 00.14 — nomor itu
// tidak ada di SEOJK 16/2024, lihat docs/CELAH-FORM-OJK.md §6) mengagregasi jumlah
// rekening dan total nominal per produk simpanan menurut golongan nasabah (sandi
// Lampiran 02). Golongan kosong ditulis "-".
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
	if sec.Form != "JENIS_NASABAH_PRODUK" {
		t.Fatalf("form = %s, ingin JENIS_NASABAH_PRODUK", sec.Form)
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

// savingsStubSource melengkapi ojkStubSource dengan agregasi internal jenis nasabah
// per produk (dulu diberi nomor Form 00.14) dan merekam posisi akhir periode.
type savingsStubSource struct {
	*ojkStubSource
	savings  []SavingsCustomerTypeRow
	lastAsOf time.Time
}

func (s *savingsStubSource) ListSavingsCustomerTypes(_ context.Context, asOf time.Time, _ domain.Actor) ([]SavingsCustomerTypeRow, error) {
	s.lastAsOf = asOf
	return s.savings, nil
}

// Agregasi internal jenis nasabah per produk tetap tersedia sebagai data internal,
// tetapi SENGAJA tidak diikutkan pada bundel bulanan: label "Form 00.14" sudah
// ditarik dari pelaporan OJK (docs/CELAH-FORM-OJK.md §6).
func TestGenerateMonthlyTidakMemuatAgregasiJenisNasabah(t *testing.T) {
	base := newStubSource()
	src := &savingsStubSource{
		ojkStubSource: &ojkStubSource{stubSource: base},
		savings: []SavingsCustomerTypeRow{
			{ProductFamily: "SAVINGS", ProductCode: "TAB-01", ProductName: "Tabungan", CustomerTypeCode: "871", AccountCount: 1, TotalAmount: decimal.NewFromInt(5000)},
		},
	}

	// Sumbernya tetap dapat diagregasi untuk keperluan internal, pada posisi akhir
	// periode yang diminta (bukan waktu ekspor).
	asOf := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	rows, err := src.ListSavingsCustomerTypes(context.Background(), asOf, domain.Actor{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("agregasi internal = %d baris (err %v), ingin 1", len(rows), err)
	}
	if !src.lastAsOf.Equal(asOf) {
		t.Fatalf("asOf agregasi internal = %v, ingin %v", src.lastAsOf, asOf)
	}

	// Tetapi agregasinya tidak boleh muncul pada keluaran ekspor bulanan.
	b, err := NewBuilder(src).GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "JENIS_NASABAH_PRODUK" {
			t.Fatalf("agregasi internal %s tidak boleh ikut bundel ekspor", sec.Form)
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "00.14" {
			t.Fatalf("form 00.14 tidak boleh muncul di SkippedForms bundel ekspor")
		}
	}
}
