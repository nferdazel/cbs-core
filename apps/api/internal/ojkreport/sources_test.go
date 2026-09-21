package ojkreport

import (
	"testing"

	"github.com/shopspring/decimal"
)

// NPLDariLoanRows menjumlahkan baki debet menurut kualitas dan hanya mengurangkan
// CKPN kredit tidak lancar (Lampiran II hlm. 204 butir 3 dan 4).
func TestNPLDariLoanRowsMenjumlahkanKualitasDanCKPN(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", Collectibility: "1_LANCAR", Outstanding: decimal.NewFromInt(500)},
		{Status: "DISBURSED", Collectibility: "2_DPK", Outstanding: decimal.NewFromInt(100)},
		{Status: "DISBURSED", Collectibility: "3_KURANG_LANCAR", Outstanding: decimal.NewFromInt(100), RequiredCKPN: decimal.NewFromInt(10)},
		{Status: "DEFAULTED", Collectibility: "4_DIRAGUKAN", Outstanding: decimal.NewFromInt(150), RequiredCKPN: decimal.NewFromInt(30)},
		{Status: "DEFAULTED", Collectibility: "5_MACET", Outstanding: decimal.NewFromInt(150), RequiredCKPN: decimal.NewFromInt(50)},
	}
	k := NPLDariLoanRows(rows)
	if !k.Tersedia {
		t.Fatal("komponen kredit harus tersedia")
	}
	if want := decimal.NewFromInt(1000); !k.TotalKredit.Equal(want) {
		t.Errorf("total kredit = %s, ingin %s", k.TotalKredit, want)
	}
	if want := decimal.NewFromInt(400); !k.KurangLancar.Add(k.Diragukan).Add(k.Macet).Equal(want) {
		t.Errorf("kredit tidak lancar = %s, ingin %s",
			k.KurangLancar.Add(k.Diragukan).Add(k.Macet), want)
	}
	if want := decimal.NewFromInt(90); !k.CKPNNPL.Equal(want) {
		t.Errorf("CKPN NPL = %s, ingin %s", k.CKPNNPL, want)
	}

	// NPL gross = 400/1000 = 40%; NPL neto = (400-90)/1000 = 31%.
	gross, ok := RumusNPLGross(k.KurangLancar, k.Diragukan, k.Macet, k.TotalKredit)
	if !ok || !gross.Equal(decimal.NewFromInt(40)) {
		t.Errorf("NPL gross = %s, ingin 40", gross)
	}
	neto, ok := RumusNPLNeto(k.KurangLancar, k.Diragukan, k.Macet, k.CKPNNPL, k.TotalKredit)
	if !ok || !neto.Equal(decimal.NewFromInt(31)) {
		t.Errorf("NPL neto = %s, ingin 31", neto)
	}
}

// Kredit di luar status berjalan tidak masuk total maupun kualitas.
func TestNPLDariLoanRowsMelewatiKreditTidakAktif(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", Collectibility: "3_KURANG_LANCAR", Outstanding: decimal.NewFromInt(100)},
		{Status: "WRITTEN_OFF", Collectibility: "5_MACET", Outstanding: decimal.NewFromInt(9999)},
		{Status: "PENDING_APPROVAL", Collectibility: "1_LANCAR", Outstanding: decimal.NewFromInt(50)},
	}
	k := NPLDariLoanRows(rows)
	if want := decimal.NewFromInt(100); !k.TotalKredit.Equal(want) {
		t.Fatalf("total kredit = %s, ingin %s (kredit tidak aktif harus dilewati)", k.TotalKredit, want)
	}
}

// Rata-rata tanpa nilai dinyatakan tidak tersedia, bukan nol.
func TestRataRataKosongTidakTersedia(t *testing.T) {
	got, ok := RataRata(nil)
	if ok {
		t.Fatal("rata-rata tanpa nilai harus tidak tersedia")
	}
	if !got.IsZero() {
		t.Fatalf("nilai = %s, ingin nol", got)
	}

	avg, ok := RataRata([]decimal.Decimal{decimal.NewFromInt(100), decimal.NewFromInt(300)})
	if !ok || !avg.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("rata-rata = %s ok=%v, ingin 200", avg, ok)
	}
}
