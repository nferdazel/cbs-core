package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Ambang pembebasan membandingkan SALDO tabungan, bukan bunga: saldo tepat di ambang
// tetap bebas potongan, satu rupiah di atasnya dikenakan tarif bruto.
func TestSavingsInterestTax_AmbangDibandingkanDenganSaldo(t *testing.T) {
	exempt := decimal.NewFromInt(7_500_000)
	rate := decimal.NewFromInt(20)
	gross := decimal.NewFromInt(100_000)

	if tax := SavingsInterestTax(gross, exempt, rate, exempt); !tax.IsZero() {
		t.Fatalf("saldo tepat di ambang harus bebas potongan, dapat %s", tax)
	}
	tax := SavingsInterestTax(gross, decimal.NewFromInt(7_500_001), rate, exempt)
	if !tax.Equal(decimal.NewFromInt(20_000)) {
		t.Fatalf("pajak %s, ingin 20000 (20%% dari 100000)", tax)
	}
}

// Ambang 0 mematikan pembebasan: seluruh tabungan dikenakan tarif.
func TestSavingsInterestTax_TanpaAmbangTetapMemotong(t *testing.T) {
	tax := SavingsInterestTax(decimal.NewFromInt(100_000), decimal.NewFromInt(1_000), decimal.NewFromInt(20), decimal.Zero)
	if !tax.Equal(decimal.NewFromInt(20_000)) {
		t.Fatalf("pajak %s, ingin 20000", tax)
	}
}

// Bunga nol atau tarif nol tidak pernah menghasilkan potongan.
func TestSavingsInterestTax_TanpaDasarPotongan(t *testing.T) {
	exempt := decimal.NewFromInt(7_500_000)
	if tax := SavingsInterestTax(decimal.Zero, decimal.NewFromInt(99_000_000), decimal.NewFromInt(20), exempt); !tax.IsZero() {
		t.Fatalf("bunga nol tidak boleh dipajaki, dapat %s", tax)
	}
	if tax := SavingsInterestTax(decimal.NewFromInt(100_000), decimal.NewFromInt(99_000_000), decimal.Zero, exempt); !tax.IsZero() {
		t.Fatalf("tarif nol tidak boleh memotong, dapat %s", tax)
	}
}
