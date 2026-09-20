package domain_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// NIK yang ditulis dengan pemisah harus dinormalisasi sebelum dienkripsi dan
// diindeks. Tanpa ini, blind index berbeda untuk nasabah yang sama: duplikat lolos
// dan pencarian tidak menemukan apa pun.
func TestNormalizeIDCardNumber(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"polos", "3201234567890001", "3201234567890001", false},
		{"berspasi", "3201 2345 6789 0001", "3201234567890001", false},
		{"bertanda hubung", "3201-2345-6789-0001", "3201234567890001", false},
		{"bertitik", "3201.2345.6789.0001", "3201234567890001", false},
		{"campuran", "3201 - 2345.6789 0001", "3201234567890001", false},
		{"kosong", "", "", true},
		{"hanya pemisah", " - . ", "", true},
		{"kurang satu digit", "320123456789000", "", true},
		{"lebih satu digit", "32012345678900012", "", true},
		{"ada huruf", "320123456789000A", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.NormalizeIDCardNumber(tc.raw)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidIDCard) {
					t.Fatalf("NormalizeIDCardNumber(%q): ingin ErrInvalidIDCard, dapat %v", tc.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeIDCardNumber(%q): tak terduga %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeIDCardNumber(%q) = %q, ingin %q", tc.raw, got, tc.want)
			}
		})
	}
}
