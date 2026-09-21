package domain

import (
	"reflect"
	"testing"
)

func TestNormalizeNameTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"dua kata", "Siti Rahayu", []string{"siti", "rahayu"}},
		{"huruf kecil", "siti", []string{"siti"}},
		{"huruf besar", "SITI", []string{"siti"}},
		{"spasi & tanda baca berlebih", "  SITI , Rahayu!  ", []string{"siti", "rahayu"}},
		{"kata berulang disimpan sekali", "Siti Siti Rahayu", []string{"siti", "rahayu"}},
		{"gelar depan dibuang", "Hj. Siti Rahayu", []string{"siti", "rahayu"}},
		{"gelar belakang bertitik dibuang", "Siti Rahayu, S.E.", []string{"siti", "rahayu"}},
		{"singkatan panjang dibuang", "Drs. Bambang S.Kom.", []string{"bambang"}},
		{"tanda hubung memisah", "Siti-Rahayu", []string{"siti", "rahayu"}},
		{"apostrof memisah, inisial dibuang", "O'Brien Siti", []string{"brien", "siti"}},
		{"inisial satu huruf dibuang", "A B C", nil},
		{"hanya tanda baca", "!!! ???", nil},
		{"kosong", "", nil},
		{"hanya spasi", "   ", nil},
		{"hanya gelar", "Hj. Drs.", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeNameTokens(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeNameTokens(%q) = %v, ingin %v", tc.in, got, tc.want)
			}
		})
	}
}

// Nama kosong atau tidak berisi kata bermakna tidak boleh menghasilkan token.
// Tanpa jaminan ini, nama yang gagal validasi tetap bisa menulis baris indeks
// sampah yang membuat pencarian cocok dengan hal yang salah.
func TestNormalizeNameTokens_NoGarbageForInvalidName(t *testing.T) {
	for _, in := range []string{"", "   ", "---", "..", "A."} {
		if got := NormalizeNameTokens(in); len(got) != 0 {
			t.Fatalf("NormalizeNameTokens(%q) = %v, ingin tanpa token", in, got)
		}
	}
}
