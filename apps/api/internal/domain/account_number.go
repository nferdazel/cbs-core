package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Format nomor rekening BPR:
//
//	KK PPP SSSSSSSSS D
//	│  │   │         └─ cek digit (Luhn)
//	│  │   └─────────── seri urut per produk per cabang (9 digit)
//	│  └─────────────── kode produk (3 digit)
//	└────────────────── kode cabang (3 digit)
//
// Total 16 digit. Cek digit membuat salah ketik satu digit selalu terdeteksi.
const (
	AccountNumberLength  = 16
	accountSerialDigits  = 9
	accountBranchDigits  = 3
	accountProductDigits = 3
)

var (
	ErrInvalidBranchCode  = errors.New("kode cabang harus 3 digit angka")
	ErrInvalidProductCode = errors.New("kode produk harus 3 digit angka")
	ErrInvalidAccountNum  = errors.New("nomor rekening tidak valid")
)

// productSequence memetakan family + buku ke kode produk 3 digit. Pemetaan dibuat
// eksplisit dan deterministik agar nomor rekening bermakna dan stabil lintas
// deployment. Produk baru harus ditambahkan di sini, bukan dihitung dari hash,
// supaya tidak ada kolisi diam-diam.
func productSequence(family ProductFamily, book COABook) (string, bool) {
	switch {
	case book == BookConventional && family == FamilySavings:
		return "101", true
	case book == BookSyariah && family == FamilySavings:
		return "111", true
	case book == BookConventional && family == FamilyCurrentAccount:
		return "102", true
	case book == BookSyariah && family == FamilyCurrentAccount:
		return "112", true
	case book == BookConventional && family == FamilyTimeDeposit:
		return "103", true
	case book == BookSyariah && family == FamilyTimeDeposit:
		return "113", true
	case book == BookConventional && family == FamilyLoan:
		return "201", true
	case book == BookSyariah && family == FamilyLoan:
		return "211", true
	default:
		return "", false
	}
}

// ProductSequenceForProduct mengembalikan kode produk 3 digit untuk nomor rekening.
func ProductSequenceForProduct(p *BankingProduct) (string, error) {
	seq, ok := productSequence(p.Family, p.Book)
	if !ok {
		return "", fmt.Errorf("tidak ada kode nomor rekening untuk family %s book %s", p.Family, p.Book)
	}
	return seq, nil
}

// BuildAccountNumber menyusun nomor rekening lengkap beserta cek digit Luhn.
func BuildAccountNumber(branchCode, productCode string, serial int64) (string, error) {
	if !isDigits(branchCode, accountBranchDigits) {
		return "", ErrInvalidBranchCode
	}
	if !isDigits(productCode, accountProductDigits) {
		return "", ErrInvalidProductCode
	}
	if serial < 1 {
		return "", ErrInvalidAccountNum
	}

	body := fmt.Sprintf("%s%s%0*d", branchCode, productCode, accountSerialDigits, serial)
	if len(body) != accountBranchDigits+accountProductDigits+accountSerialDigits {
		// Seri melebihi kapasitas digit; hentikan daripada memotong diam-diam.
		return "", fmt.Errorf("seri rekening %d melebihi kapasitas %d digit", serial, accountSerialDigits)
	}
	return body + fmt.Sprintf("%d", luhnCheckDigit(body)), nil
}

// ValidateAccountNumber memastikan panjang benar dan cek digit cocok.
func ValidateAccountNumber(number string) bool {
	if len(number) != AccountNumberLength || !isDigits(number, AccountNumberLength) {
		return false
	}
	body, check := number[:len(number)-1], int(number[len(number)-1]-'0')
	return luhnCheckDigit(body) == check
}

// luhnCheckDigit menghitung cek digit Luhn (mod 10) dari deretan angka.
func luhnCheckDigit(digits string) int {
	sum := 0
	double := true // digit paling kanan body dikali 2 karena cek digit menyusul
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return (10 - sum%10) % 10
}

func isDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// MaskAccountNumber menyembunyikan sebagian besar nomor rekening untuk tampilan/log.
func MaskAccountNumber(number string) string {
	if len(number) <= 4 {
		return strings.Repeat("*", len(number))
	}
	return strings.Repeat("*", len(number)-4) + number[len(number)-4:]
}
