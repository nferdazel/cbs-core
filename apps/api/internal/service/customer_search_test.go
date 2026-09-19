package service

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

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
		{"320123456789000", false},  // 15 digit
		{"32012345678900012", false}, // 17 digit
		{"320123456789000a", false}, // ada huruf
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

// Tanpa cipher, kata kunci NIK tidak boleh menghasilkan blind index (akan panik bila
// dipaksa); pencarian CIF tetap berjalan apa adanya.
func TestSearchQueryWithoutCipher(t *testing.T) {
	svc := &customerService{}

	numeric := svc.searchQuery("3201234567890001")
	if numeric.CIF != "3201234567890001" {
		t.Fatalf("CIF harus tetap diisi, dapat %q", numeric.CIF)
	}
	if numeric.IDCardIndex != "" {
		t.Fatalf("tanpa cipher tidak boleh ada blind index, dapat %q", numeric.IDCardIndex)
	}

	alnum := svc.searchQuery("  CIF-0001  ")
	if alnum.CIF != "CIF-0001" {
		t.Fatalf("kata kunci harus dipangkas spasi, dapat %q", alnum.CIF)
	}
	if alnum.IDCardIndex != "" {
		t.Fatalf("bukan NIK tidak boleh jadi blind index, dapat %q", alnum.IDCardIndex)
	}

	if empty := svc.searchQuery("   "); empty != (domain.CustomerQuery{}) {
		t.Fatalf("kata kunci kosong harus menghasilkan filter kosong, dapat %+v", empty)
	}
}
