package ojkreport

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Sandi kolom "Jenis CKPN" (sandi XXI Form 06.00) memetakan segel metode per kredit
// ke sandi menurut penjelasan umum kolom Q SEOJK 16/2024: sandi 1 = individual,
// sandi 2 = kolektif. Kredit lama tanpa segel dilaporkan kolektif (bawaan mesin),
// bukan dikosongkan, dan segel pengecualian aset baik dilaporkan sesuai jalurnya
// (kolektif: pengecualian bukan penilaian individual).
func TestSandiJenisCKPN(t *testing.T) {
	cases := []struct {
		method string
		mau    string
		nama   string
	}{
		{"INDIVIDUAL_DCF", "1", "DCF individual"},
		{"INDIVIDUAL_COLLATERAL", "1", "agunan individual"},
		{"INDIVIDUAL_MAX", "1", "MAX individual"},
		{"COLLECTIVE", "2", "kolektif eksplisit"},
		{"", "2", "kredit lama tanpa segel (bawaan kolektif)"},
		{"EXCLUDED_ASET_BAIK", "2", "pengecualian aset baik bukan jalur individual"},
		{"SEMAU-SEMENYA", "2", "segel tak dikenal tidak boleh keliru individual"},
	}
	for _, c := range cases {
		if got := sandiJenisCKPN(c.method); got != c.mau {
			t.Errorf("%s (%s): sandi %q, mau %q", c.nama, c.method, got, c.mau)
		}
	}
}

// Kolom XXI Form 06.00 tidak lagi dilaporkan unavailable: data yang dibutuhkannya
// (loans.ckpn_method) sudah tersimpan sejak T3/T4.
func TestForm06KolomJenisCKPNTersedia(t *testing.T) {
	sec := buildForm06([]LoanRow{{
		LoanNumber:   "KRD-UJI",
		CKPNMethod:   "INDIVIDUAL_MAX",
		RequiredCKPN: decimal.NewFromInt(50_000),
	}})
	for _, col := range sec.Columns {
		if col.Sandi == form06SandiJenisCKPN {
			return
		}
	}
	t.Fatalf("kolom sandi XXI tidak ada pada daftar kolom Form 06.00")
}
