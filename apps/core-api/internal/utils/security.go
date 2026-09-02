package utils

import (
	"strings"
)

// MaskNIK masks 16-digit Indonesian NIK for UU PDP privacy compliance (e.g., "3171012345670001" -> "3171************")
func MaskNIK(nik string) string {
	nik = strings.TrimSpace(nik)
	if len(nik) <= 4 {
		return nik
	}
	if len(nik) <= 8 {
		return nik[:4] + strings.Repeat("*", len(nik)-4)
	}
	return nik[:4] + strings.Repeat("*", len(nik)-4)
}

// MaskAccountNumber masks bank account numbers for public statements (e.g., "201001002003" -> "201******003")
func MaskAccountNumber(accNo string) string {
	accNo = strings.TrimSpace(accNo)
	if len(accNo) <= 6 {
		return accNo
	}
	prefix := accNo[:3]
	suffix := accNo[len(accNo)-3:]
	maskedLen := len(accNo) - 6
	return prefix + strings.Repeat("*", maskedLen) + suffix
}
