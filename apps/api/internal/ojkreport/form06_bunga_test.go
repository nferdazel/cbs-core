package ojkreport

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestForm06KolomPendapatanBungaAkanDiterima membuktikan kolom XXXV kini terisi
// dari LoanRow.AccruedProfit (piutang bunga 10400 yang masih tercatat), bukan lagi
// terdaftar sebagai kolom tidak tersedia. Nominal ditulis rupiah penuh lewat
// FormatRupiah; kredit tanpa akruan menulis "0", bukan "-".
func TestForm06KolomPendapatanBungaAkanDiterima(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-AKRU", AccruedProfit: decimal.RequireFromString("1234567.8900")},
		{Status: "DISBURSED", LoanNumber: "LN-BERSIH"},
	}
	sec := buildForm06(rows)

	if got := findCell(t, sec, "LN-AKRU", form06SandiBungaAkanTer).Value; got != "1234568" {
		t.Errorf("kolom XXXV = %q, ingin 1234568 (dibulatkan ke rupiah penuh)", got)
	}
	if got := findCell(t, sec, "LN-BERSIH", form06SandiBungaAkanTer).Value; got != "0" {
		t.Errorf("kolom XXXV tanpa akruan = %q, ingin 0", got)
	}

	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiBungaAkanTer {
			t.Errorf("kolom XXXV masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}

// TestForm06KolomBungaDalamPenyelesaianTetapTidakTersedia menjaga keputusan triase:
// pendapatan bunga dalam penyelesaian (XXXVI) belum dimodelkan sebagai nominal, jadi
// harus tetap terdaftar beserta alasannya, bukan diisi angka karangan.
func TestForm06KolomBungaDalamPenyelesaianTetapTidakTersedia(t *testing.T) {
	sec := buildForm06([]LoanRow{{Status: "DISBURSED", LoanNumber: "LN-001", AccruedProfit: decimal.NewFromInt(1)}})
	if u := unavailableColumn(t, sec, form06SandiBungaProses); u.Reason == "" {
		t.Fatal("kolom XXXVI harus tetap terdaftar dengan alasan")
	}
}
