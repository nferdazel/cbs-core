package domain_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Email nasabah opsional: kosong harus diterima, sedangkan format yang jelas salah
// ditolak agar teller langsung tahu saat salah ketik.
func TestValidateEmail(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"kosong diterima", "", false},
		{"spasi saja diterima", "   ", false},
		{"biasa", "nasabah@contoh.co.id", false},
		{"tanpa titik domain", "nasabah@contoh", true},
		{"tanpa at", "nasabah.contoh.id", true},
		{"lokal kosong", "@contoh.id", true},
		{"domain kosong", "nasabah@", true},
		{"dua at", "a@b@contoh.id", true},
		{"ada spasi", "nasabah @contoh.id", true},
		{"titik di akhir domain", "nasabah@contoh.", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateEmail(tc.email)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidEmail) {
					t.Fatalf("ValidateEmail(%q): ingin ErrInvalidEmail, dapat %v", tc.email, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateEmail(%q): tak terduga %v", tc.email, err)
			}
		})
	}
}
