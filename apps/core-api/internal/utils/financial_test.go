package utils_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/utils"
	"github.com/shopspring/decimal"
)

func TestFinancialUtils_RoundingModes(t *testing.T) {
	// Test Banker's Rounding (Round Half to Even)
	val1 := decimal.NewFromFloat(2.5) // -> 2
	val2 := decimal.NewFromFloat(3.5) // -> 4

	if rounded := utils.BankersRound(val1, 0); !rounded.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("expected BankersRound(2.5) == 2, got %s", rounded)
	}
	if rounded := utils.BankersRound(val2, 0); !rounded.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("expected BankersRound(3.5) == 4, got %s", rounded)
	}

	// Test HalfUp Rounding (Commercial Rounding)
	if rounded := utils.HalfUpRound(val1, 0); !rounded.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("expected HalfUpRound(2.5) == 3, got %s", rounded)
	}

	// Test TruncateDown (Cut-off)
	val3 := decimal.NewFromFloat(12500.99)
	if truncated := utils.TruncateDown(val3, 0); !truncated.Equal(decimal.NewFromInt(12500)) {
		t.Fatalf("expected TruncateDown(12500.99) == 12500, got %s", truncated)
	}
}

func TestFinancialUtils_FormatIDR(t *testing.T) {
	val := decimal.NewFromInt(12500000)
	formatted := utils.FormatIDR(val)
	expected := "Rp 12.500.000,00"

	if formatted != expected {
		t.Fatalf("expected FormatIDR == %s, got %s", expected, formatted)
	}
}

func TestFinancialUtils_TerbilangRupiah(t *testing.T) {
	tests := []struct {
		amount   decimal.Decimal
		expected string
	}{
		{decimal.NewFromInt(0), "Nol Rupiah"},
		{decimal.NewFromInt(12500000), "Dua Belas Juta Lima Ratus Ribu Rupiah"},
		{decimal.NewFromInt(50000000), "Lima Puluh Juta Rupiah"},
		{decimal.NewFromInt(1500), "Seribu Lima Ratus Rupiah"},
	}

	for _, tt := range tests {
		result := utils.TerbilangRupiah(tt.amount)
		if result != tt.expected {
			t.Errorf("TerbilangRupiah(%s) = %q; want %q", tt.amount, result, tt.expected)
		}
	}
}

func TestSecurityUtils_Masking(t *testing.T) {
	nik := "3171012345670001"
	maskedNIK := utils.MaskNIK(nik)
	if maskedNIK != "3171************" {
		t.Fatalf("expected masked NIK 3171************, got %s", maskedNIK)
	}

	accNo := "201001002003"
	maskedAcc := utils.MaskAccountNumber(accNo)
	if maskedAcc != "201******003" {
		t.Fatalf("expected masked account 201******003, got %s", maskedAcc)
	}
}
