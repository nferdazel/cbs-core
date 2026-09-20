package service

import (
	"encoding/base64"
	"testing"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
)

func searchTestCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.NewCipher("k1", base64.StdEncoding.EncodeToString(make([]byte, 32)), nil)
	if err != nil {
		t.Fatalf("membangun cipher: %v", err)
	}
	return c
}

// Hanya kata kunci berbentuk NIK (tepat 16 digit) yang boleh dihitung blind
// index-nya. Bentuk lain harus dilewati, bukan dipaksa menjadi hash yang tidak akan
// pernah cocok dengan data mana pun.
func TestIsNIK(t *testing.T) {
	cases := []struct {
		term string
		want bool
	}{
		{"3201234567890001", true},
		{"0000000000000000", true},
		{"320123456789000", false},   // 15 digit
		{"32012345678900012", false}, // 17 digit
		{"320123456789000a", false},  // ada huruf
		{"", false},
		{"CIF-0001", false},
		{"budi", false},
	}

	for _, tc := range cases {
		if got := isNIK(tc.term); got != tc.want {
			t.Fatalf("isNIK(%q) = %v, ingin %v", tc.term, got, tc.want)
		}
	}
}

// Kata kunci NIK dicari lewat blind index SAJA. Nomor CIF tidak pernah berbentuk 16
// digit, sehingga bila filter CIF ikut diisi dan digabung dengan AND, seluruh hasil
// pasti tersaring habis — pencarian NIK selalu kosong.
func TestSearchQuery_ForNIKUsesBlindIndexOnly(t *testing.T) {
	cipher := searchTestCipher(t)
	svc := &customerService{cipher: cipher}

	const nik = "3201234567890001"
	q := svc.searchQuery(nik)

	if q.CIF != "" {
		t.Fatalf("kata kunci NIK tidak boleh ikut menyaring nomor CIF, dapat %q", q.CIF)
	}
	if want := cipher.BlindIndex(nik); q.IDCardIndex != want {
		t.Fatalf("blind index NIK = %q, ingin %q", q.IDCardIndex, want)
	}
}

// Kata kunci non-NIK dicari sebagai awalan nomor CIF, tanpa menyentuh blind index.
func TestSearchQuery_ForCIFUsesPrefixOnly(t *testing.T) {
	svc := &customerService{cipher: searchTestCipher(t)}

	q := svc.searchQuery("  CIF000000123  ")
	if q.CIF != "CIF000000123" {
		t.Fatalf("CIF harus dipangkas spasi, dapat %q", q.CIF)
	}
	if q.IDCardIndex != "" {
		t.Fatalf("bukan NIK tidak boleh jadi blind index, dapat %q", q.IDCardIndex)
	}

	if empty := svc.searchQuery("   "); empty != (domain.CustomerQuery{}) {
		t.Fatalf("kata kunci kosong harus menghasilkan filter kosong, dapat %+v", empty)
	}
}

// Tanpa cipher, kata kunci NIK tidak boleh menghasilkan blind index (akan panik bila
// dipaksa); pencarian jatuh ke awalan CIF apa adanya.
func TestSearchQuery_WithoutCipherFallsBackToCIF(t *testing.T) {
	svc := &customerService{}

	q := svc.searchQuery("3201234567890001")
	if q.IDCardIndex != "" {
		t.Fatalf("tanpa cipher tidak boleh ada blind index, dapat %q", q.IDCardIndex)
	}
	if q.CIF != "3201234567890001" {
		t.Fatalf("jatuh ke CIF apa adanya, dapat %q", q.CIF)
	}
}
