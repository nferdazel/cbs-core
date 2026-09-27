package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func decimalPtr(t *testing.T, v string) *decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(v)
	if err != nil {
		t.Fatalf("nilai desimal %q tidak valid: %v", v, err)
	}
	return &d
}

// TestForm06AgunanPPKADariKolomPenyimpanan membuktikan kolom XXV kini terisi dari
// kolom penyimpanan per kredit (migrasi 000109) dan ditulis "-" bila belum
// diisi/belum dihitung. Nol yang benar-benar tersimpan tetap ditulis "0", berbeda
// dari NULL: nilai ini tidak lagi terdaftar tidak tersedia dan TIDAK dihitung ulang
// dari agunan/plafon di perakit laporan.
func TestForm06AgunanPPKADariKolomPenyimpanan(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-ISI", OJKAgunanPPKAAmount: decimalPtr(t, "4000000")},
		{Status: "DISBURSED", LoanNumber: "LN-NOL", OJKAgunanPPKAAmount: decimalPtr(t, "0")},
		{Status: "DISBURSED", LoanNumber: "LN-KOSONG"},
	}
	sec := buildForm06(rows)

	if got := findCell(t, sec, "LN-ISI", form06SandiAgunanPPKA).Value; got != "4000000" {
		t.Errorf("kolom XXV terisi = %q, ingin 4000000", got)
	}
	if got := findCell(t, sec, "LN-NOL", form06SandiAgunanPPKA).Value; got != "0" {
		t.Errorf("kolom XXV nol = %q, ingin 0 (bukan -)", got)
	}
	if got := findCell(t, sec, "LN-KOSONG", form06SandiAgunanPPKA).Value; got != "-" {
		t.Errorf("kolom XXV kosong = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiAgunanPPKA {
			t.Errorf("kolom XXV masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}

// TestForm06KelonggaranTarikDariKolomPenyimpanan membuktikan kolom XXVI terisi dari
// kolom penyimpanan nullable dan ditulis "-" bila bank belum mengisi; nilainya tidak
// diturunkan dari plafon dikurangi baki.
func TestForm06KelonggaranTarikDariKolomPenyimpanan(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-ISI", OJKKelonggaranTarikAmount: decimalPtr(t, "1500000")},
		{Status: "DISBURSED", LoanNumber: "LN-KOSONG"},
	}
	sec := buildForm06(rows)

	if got := findCell(t, sec, "LN-ISI", form06SandiKelonggaran).Value; got != "1500000" {
		t.Errorf("kolom XXVI terisi = %q, ingin 1500000", got)
	}
	if got := findCell(t, sec, "LN-KOSONG", form06SandiKelonggaran).Value; got != "-" {
		t.Errorf("kolom XXVI kosong = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiKelonggaran {
			t.Errorf("kolom XXVI masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}

// TestListLoansForOJKMengisiAgunanPPKAKelonggaran menutup rantai adaptor kredit:
// nilai agunan PPKA dan kelonggaran tarik per kredit dipetakan dari baris kredit
// tanpa query tambahan (keduanya ikut kolom kredit yang sudah ada).
func TestListLoansForOJKMengisiAgunanPPKAKelonggaran(t *testing.T) {
	repo := &loanAggStub{loans: []domain.Loan{{
		LoanNumber:                "LN-AGN",
		Status:                    domain.LoanStatusDisbursed,
		OJKAgunanPPKAAmount:       decimalPtr(t, "2500000"),
		OJKKelonggaranTarikAmount: decimalPtr(t, "1000000"),
	}}}
	src := RepoSource{Loans: repo}

	rows, err := src.ListLoansForOJK(context.Background(), time.Now().UTC(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListLoansForOJK: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(rows))
	}
	if rows[0].OJKAgunanPPKAAmount == nil || !rows[0].OJKAgunanPPKAAmount.Equal(decimal.NewFromInt(2_500_000)) {
		t.Errorf("agunan PPKA = %v, ingin 2500000", rows[0].OJKAgunanPPKAAmount)
	}
	if rows[0].OJKKelonggaranTarikAmount == nil || !rows[0].OJKKelonggaranTarikAmount.Equal(decimal.NewFromInt(1_000_000)) {
		t.Errorf("kelonggaran tarik = %v, ingin 1000000", rows[0].OJKKelonggaranTarikAmount)
	}
}
