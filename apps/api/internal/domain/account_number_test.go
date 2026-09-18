package domain_test

import (
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

func TestBuildAccountNumber(t *testing.T) {
	num, err := domain.BuildAccountNumber("001", "101", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(num) != domain.AccountNumberLength {
		t.Fatalf("panjang nomor rekening = %d, mau %d", len(num), domain.AccountNumberLength)
	}
	if !strings.HasPrefix(num, "001101") {
		t.Fatalf("nomor rekening %s tidak diawali kode cabang+produk", num)
	}
	if !domain.ValidateAccountNumber(num) {
		t.Fatalf("nomor rekening %s yang baru dibangun gagal validasi", num)
	}
}

// Cek digit Luhn harus menangkap salah ketik satu digit di posisi mana pun.
func TestValidateAccountNumberDetectsSingleDigitTypo(t *testing.T) {
	num, err := domain.BuildAccountNumber("001", "111", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < len(num)-1; i++ {
		altered := []byte(num)
		if altered[i] == '9' {
			altered[i] = '0'
		} else {
			altered[i]++
		}
		if domain.ValidateAccountNumber(string(altered)) {
			t.Fatalf("salah ketik di posisi %d tidak terdeteksi: %s", i, string(altered))
		}
	}
}

func TestBuildAccountNumberRejectsInvalidBranch(t *testing.T) {
	if _, err := domain.BuildAccountNumber("1", "101", 1); err == nil {
		t.Fatal("kode cabang 1 digit seharusnya ditolak")
	}
	if _, err := domain.BuildAccountNumber("A01", "101", 1); err == nil {
		t.Fatal("kode cabang non-angka seharusnya ditolak")
	}
	if _, err := domain.BuildAccountNumber("001", "101", 0); err == nil {
		t.Fatal("seri nol seharusnya ditolak")
	}
}

func TestProductSequenceForProductCannotCollide(t *testing.T) {
	seen := map[string]string{}
	cases := []struct {
		family domain.ProductFamily
		book   domain.COABook
	}{
		{domain.FamilySavings, domain.BookConventional},
		{domain.FamilySavings, domain.BookSyariah},
		{domain.FamilyCurrentAccount, domain.BookConventional},
		{domain.FamilyCurrentAccount, domain.BookSyariah},
		{domain.FamilyTimeDeposit, domain.BookConventional},
		{domain.FamilyTimeDeposit, domain.BookSyariah},
		{domain.FamilyLoan, domain.BookConventional},
		{domain.FamilyLoan, domain.BookSyariah},
	}

	for _, c := range cases {
		p := &domain.BankingProduct{Family: c.family, Book: c.book}
		code, err := domain.ProductSequenceForProduct(p)
		if err != nil {
			t.Fatalf("%s/%s: %v", c.family, c.book, err)
		}
		key := string(c.family) + "/" + string(c.book)
		if other, ok := seen[code]; ok {
			t.Fatalf("kode produk %s dipakai oleh %s dan %s", code, other, key)
		}
		seen[code] = key
	}
}

func TestMaskAccountNumber(t *testing.T) {
	masked := domain.MaskAccountNumber("0011010000000017")
	if !strings.HasSuffix(masked, "0017") {
		t.Fatalf("4 digit terakhir harus terlihat, dapat %s", masked)
	}
	if strings.Contains(masked[:len(masked)-4], "1") && strings.Trim(masked[:len(masked)-4], "*") != "" {
		t.Fatalf("bagian depan harus seluruhnya bertopeng, dapat %s", masked)
	}
}
