package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// bankDepositStubSource melengkapi stubSource dengan agregasi Form 13.00 dan merekam
// posisi akhir periode yang diminta perakit.
type bankDepositStubSource struct {
	*stubSource
	bankDeposits []BankDepositRow
	err          error
	lastAsOf     time.Time
}

func (s *bankDepositStubSource) ListBankDepositsForOJK(_ context.Context, asOf time.Time, _ domain.Actor) ([]BankDepositRow, error) {
	s.lastAsOf = asOf
	return s.bankDeposits, s.err
}

// Form 13.00 mengisi kolom yang punya sumber (jenis bank, lokasi, jenis, nominal) dan
// menandai kolom yang tidak punya sumber sebagai belum tersedia, termasuk kolom
// "diblokir" yang tidak punya riwayat per akhir periode.
func TestBuildForm13KolomTersediaDanBelum(t *testing.T) {
	rows := []BankDepositRow{
		{
			BranchCode:       "001",
			CounterpartyCIF:  "CIF-001",
			JenisBankCode:    "700",
			HubunganBankCode: "20",
			LocationCode:     "0197",
			Jenis:            "02",
			AccountCount:     2,
			TotalNominal:     decimal.NewFromInt(250_000_000),
		},
	}

	sec := buildForm13(rows)
	if sec.Form != "13.00" {
		t.Fatalf("form = %s, ingin 13.00", sec.Form)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(sec.Rows))
	}
	key := "CIF-001 / 02 / 001"
	if got := findCell(t, sec, key, form13SandiJenisBank).Value; got != "700" {
		t.Errorf("jenis bank = %q, ingin 700", got)
	}
	if got := findCell(t, sec, key, form13SandiLokasi).Value; got != "0197" {
		t.Errorf("lokasi = %q, ingin 0197", got)
	}
	if got := findCell(t, sec, key, form13SandiJenis).Value; got != "02" {
		t.Errorf("jenis simpanan = %q, ingin 02", got)
	}
	if got := findCell(t, sec, key, form13SandiNominal).Value; got != "250000000" {
		t.Errorf("nominal = %q, ingin 250000000", got)
	}
	if got := findCell(t, sec, key, form13SandiJumlah).Value; got != "250000000" {
		t.Errorf("jumlah = %q, ingin 250000000", got)
	}

	for _, sandi := range []string{form13SandiNoRek, form13SandiSandiBank, form13SandiJangka, form13SandiSukuBunga, form13SandiDiblokir, form13SandiAlasan, form13SandiBiaya} {
		u := unavailableColumn(t, sec, sandi)
		if strings.TrimSpace(u.Reason) == "" {
			t.Errorf("kolom %s harus mencantumkan alasan", sandi)
		}
	}
}

// Tanpa sumber, Form 13.00 didaftarkan belum tersedia dengan alasan; bila sumber ada
// tetapi tidak ada simpanan bank lawan, alasannya menyebut golongan bank.
func TestGenerateMonthlyForm13SkippedDenganAlasan(t *testing.T) {
	tanpaSumber, err := NewBuilder(newStubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly tanpa sumber: %v", err)
	}
	if got := skippedReason(t, tanpaSumber, "13.00"); !strings.Contains(got, "belum dikonfigurasi") {
		t.Errorf("alasan tanpa sumber = %q", got)
	}

	kosong, err := NewBuilder(&bankDepositStubSource{stubSource: newStubSource()}).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly tanpa baris: %v", err)
	}
	if got := skippedReason(t, kosong, "13.00"); !strings.Contains(got, "pihak_lawan") {
		t.Errorf("alasan tanpa baris = %q", got)
	}
}

// Dengan baris simpanan bank lawan, Form 13.00 muncul pada tabel bundle bulanan.
func TestGenerateMonthlyMemuatForm13(t *testing.T) {
	src := &bankDepositStubSource{
		stubSource: newStubSource(),
		bankDeposits: []BankDepositRow{
			{
				BranchCode:      "001",
				CounterpartyCIF: "CIF-777",
				JenisBankCode:   "600",
				Jenis:           "01",
				AccountCount:    1,
				TotalNominal:    decimal.NewFromInt(5_000_000),
			},
		},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "13.00")
	if got := findCell(t, sec, "CIF-777 / 01 / 001", form13SandiNominal).Value; got != "5000000" {
		t.Fatalf("nominal Form 13.00 = %q, ingin 5000000", got)
	}
}

// TestGenerateMonthlyMeneruskanAkhirPeriodeKeSumberBank memastikan agregasi bank lawan
// diminta pada posisi AKHIR PERIODE laporan, bukan waktu ekspor dijalankan. Bila ekspor
// ditunda, angka tetap milik bulan yang dilaporkan.
func TestGenerateMonthlyMeneruskanAkhirPeriodeKeSumberBank(t *testing.T) {
	period := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	src := &bankDepositStubSource{
		stubSource: newStubSource(),
		bankDeposits: []BankDepositRow{{
			BranchCode: "001", CounterpartyCIF: "CIF-777", JenisBankCode: "600",
			Jenis: "01", AccountCount: 1, TotalNominal: decimal.NewFromInt(5_000_000),
		}},
	}
	if _, err := NewBuilder(src).GenerateMonthly(context.Background(), period, ""); err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	want := MonthEnd(period)
	if !src.lastAsOf.Equal(want) {
		t.Fatalf("asOf ke sumber bank = %v, ingin akhir periode %v", src.lastAsOf, want)
	}
}

// skippedReason mencari alasan satu form pada SkippedForms bundle.
func skippedReason(t *testing.T, b *Bundle, form string) string {
	t.Helper()
	for _, f := range b.SkippedForms {
		if f.Form == form {
			return f.UnavailableReason
		}
	}
	t.Fatalf("form %s tidak ada pada SkippedForms", form)
	return ""
}
