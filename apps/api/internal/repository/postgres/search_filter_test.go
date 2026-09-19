package postgres

import "testing"

// Karakter khusus LIKE harus di-escape: tanpa ini, mengetik "%" pada kotak
// pencarian akan cocok dengan seluruh nasabah, dan "_" cocok dengan sembarang satu
// karakter. Keduanya bukan pencarian yang diminta pengguna.
func TestLikePrefixPattern(t *testing.T) {
	cases := []struct {
		name string
		term string
		want string
	}{
		{"kosong", "", ""},
		{"hanya spasi", "   ", ""},
		{"biasa", "budi", "budi%"},
		{"spasi tepi dipangkas", "  budi  ", "budi%"},
		{"persen di-escape", "%", `\%` + "%"},
		{"garis bawah di-escape", "_", `\_` + "%"},
		{"backslash di-escape", `\`, `\\` + "%"},
		{"campuran", "a%b_c\\d", `a\%b\_c\\d` + "%"},
		{"cif", "001122", "001122%"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := likePrefixPattern(tc.term); got != tc.want {
				t.Fatalf("likePrefixPattern(%q) = %q, ingin %q", tc.term, got, tc.want)
			}
		})
	}
}

func TestAndCondition(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		cond     string
		want     string
	}{
		{"keduanya kosong", "", "", ""},
		{"hanya existing", "a = 1", "", "a = 1"},
		{"hanya cond", "", "b = 2", "b = 2"},
		{"digabung", "a = 1", "b = 2", "a = 1 AND b = 2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := andCondition(tc.existing, tc.cond); got != tc.want {
				t.Fatalf("andCondition(%q, %q) = %q, ingin %q", tc.existing, tc.cond, got, tc.want)
			}
		})
	}
}
