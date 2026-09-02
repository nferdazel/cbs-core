package utils

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// RoundMoney is the single canonical rounding function used across the core banking system.
// It applies the standard Banker's Rounding (Round-Half-Even, IEEE 754) to 2 decimal places.
func RoundMoney(val decimal.Decimal) decimal.Decimal {
	return val.RoundBank(2)
}

// RoundRupiah rounds monetary values to the nearest exact Rupiah (0 decimal places)
func RoundRupiah(val decimal.Decimal) decimal.Decimal {
	return val.RoundBank(0)
}

// RoundingMode defines financial decimal rounding strategies
type RoundingMode string

const (
	RoundingModeBankers RoundingMode = "BANKERS" // Half-Even (IEEE 754)
	RoundingModeHalfUp  RoundingMode = "HALF_UP" // Standard Commercial Half-Up
	RoundingModeFloor   RoundingMode = "FLOOR"   // Truncation / Cut-off
)

// BankersRound performs Round-Half-Even (Banker's Rounding)
func BankersRound(val decimal.Decimal, scale int32) decimal.Decimal {
	return val.RoundBank(scale)
}

// HalfUpRound performs Round-Half-Away-From-Zero (Commercial Rounding)
func HalfUpRound(val decimal.Decimal, scale int32) decimal.Decimal {
	return val.Round(scale)
}

// TruncateDown truncates fractional decimals down to scale
func TruncateDown(val decimal.Decimal, scale int32) decimal.Decimal {
	return val.Truncate(scale)
}

// FormatIDR formats decimal into Indonesian Rupiah currency string (e.g., "Rp 12.500.000,00")
func FormatIDR(val decimal.Decimal) string {
	intPart := val.Truncate(0).IntPart()
	fracPart := val.Sub(decimal.NewFromInt(intPart)).Abs().Mul(decimal.NewFromInt(100)).IntPart()

	// Format integer part with thousands separator (dots)
	sign := ""
	if intPart < 0 {
		sign = "-"
		intPart = -intPart
	}

	intStr := fmt.Sprintf("%d", intPart)
	var result []string
	length := len(intStr)

	for i := length; i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{intStr[start:i]}, result...)
	}

	formattedInt := strings.Join(result, ".")
	return fmt.Sprintf("Rp %s%s,%02d", sign, formattedInt, fracPart)
}

// TerbilangRupiah converts decimal amounts to official Indonesian words
func TerbilangRupiah(val decimal.Decimal) string {
	if val.Equal(decimal.Zero) {
		return "Nol Rupiah"
	}

	intVal := val.Abs().Truncate(0).IntPart()
	if intVal == 0 {
		return "Nol Rupiah"
	}

	rawWords := convertNumberToWords(intVal)
	cleanWords := strings.Join(strings.Fields(rawWords), " ")

	prefix := ""
	if val.IsNegative() {
		prefix = "Minus "
	}

	return fmt.Sprintf("%s%s Rupiah", prefix, cleanWords)
}

var units = []string{"", "Satu", "Dua", "Tiga", "Empat", "Lima", "Enam", "Tujuh", "Delapan", "Sembilan", "Sepuluh", "Sebelas"}

func convertNumberToWords(n int64) string {
	switch {
	case n < 12:
		return units[n]
	case n < 20:
		return convertNumberToWords(n-10) + " Belas"
	case n < 100:
		return convertNumberToWords(n/10) + " Puluh " + convertNumberToWords(n%10)
	case n < 200:
		return "Seratus " + convertNumberToWords(n-100)
	case n < 1000:
		return convertNumberToWords(n/100) + " Ratus " + convertNumberToWords(n%100)
	case n < 2000:
		return "Seribu " + convertNumberToWords(n-1000)
	case n < 1000000:
		return convertNumberToWords(n/1000) + " Ribu " + convertNumberToWords(n%1000)
	case n < 1000000000:
		return convertNumberToWords(n/1000000) + " Juta " + convertNumberToWords(n%1000000)
	case n < 1000000000000:
		return convertNumberToWords(n/1000000000) + " Miliar " + convertNumberToWords(n%1000000000)
	default:
		return convertNumberToWords(n/1000000000000) + " Triliun " + convertNumberToWords(n%1000000000000)
	}
}
