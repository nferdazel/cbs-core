package domain_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// EIR kredit tanpa suku bunga efektif orisinal TIDAK boleh diperlakukan nol: harus
// gagal dengan ErrEIRMissing (keputusan panel butir 4.4.3), kecuali override tahunan
// diisi bank.
func TestCKPNIndividualDiscountRateMonthly(t *testing.T) {
	// EIR tersimpan dipakai apa adanya (fraksi/bulan).
	got, err := domain.CKPNIndividualDiscountRateMonthly(decimal.NewFromFloat(0.0125), "")
	if err != nil {
		t.Fatalf("EIR tersimpan tidak boleh gagal: %v", err)
	}
	if !got.Equal(decimal.NewFromFloat(0.0125)) {
		t.Fatalf("EIR %s, mau 0.0125", got)
	}

	// EIR kosong + override 12%/tahun -> 1%/bulan.
	got, err = domain.CKPNIndividualDiscountRateMonthly(decimal.Zero, "12")
	if err != nil {
		t.Fatalf("override sah tidak boleh gagal: %v", err)
	}
	if !got.Equal(decimal.NewFromFloat(1)) {
		t.Fatalf("override 12%%/tahun -> %s, mau 1/bulan", got)
	}

	// EIR kosong + override kosong -> ErrEIRMissing, BUKAN nol.
	got, err = domain.CKPNIndividualDiscountRateMonthly(decimal.Zero, "")
	if !errors.Is(err, domain.ErrEIRMissing) {
		t.Fatalf("err=%v, mau ErrEIRMissing", err)
	}
	if !got.IsZero() {
		t.Fatalf("nilai harus nol saat gagal, dapat %s", got)
	}

	// Override salah format ditolak sebagai parameter tidak valid.
	if _, err := domain.CKPNIndividualDiscountRateMonthly(decimal.Zero, "dua belas"); !errors.Is(err, domain.ErrCKPNParameterInvalid) {
		t.Fatalf("override salah format: err=%v, mau ErrCKPNParameterInvalid", err)
	}
}

func TestCKPNIndividualMethodValid(t *testing.T) {
	for _, m := range []domain.CKPNIndividualMethod{
		domain.CKPNIndividualMethodDCF,
		domain.CKPNIndividualMethodCollateral,
		domain.CKPNIndividualMethodMax,
	} {
		if !m.Valid() {
			t.Fatalf("metode %s seharusnya dikenal", m)
		}
	}
	if domain.CKPNIndividualMethod("SESUKA").Valid() {
		t.Fatal("metode tak dikenal tidak boleh dianggap sah")
	}
}
