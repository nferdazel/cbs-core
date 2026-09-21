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

// Tiap kata nama menjadi satu kondisi EXISTS, dan semua kondisi wajib terpenuhi
// (AND). Inilah yang membuat "siti rahayu" tidak mencocokkan nasabah yang hanya
// memiliki salah satu kata.
func TestAddNameTokenFilters(t *testing.T) {
	oneToken := "EXISTS (SELECT 1 FROM customer_name_tokens cnt WHERE cnt.customer_id = customers.id AND cnt.token_index = $1)"
	twoTokens := oneToken + " AND " +
		"EXISTS (SELECT 1 FROM customer_name_tokens cnt WHERE cnt.customer_id = customers.id AND cnt.token_index = $2)"

	where, args := addNameTokenFilters("", nil, []string{"tok-a", "tok-b"})
	if where != twoTokens {
		t.Fatalf("klausa = %q, ingin %q", where, twoTokens)
	}
	if len(args) != 2 || args[0] != "tok-a" || args[1] != "tok-b" {
		t.Fatalf("argumen = %v, ingin [tok-a tok-b]", args)
	}

	// Satu token hanya menambah satu kondisi dan meneruskan nomor placeholder
	// setelah argumen yang sudah ada.
	where, args = addNameTokenFilters("branch_id IS NULL", []any{"cabang"}, []string{"tok-a"})
	wantWhere := "branch_id IS NULL AND " +
		"EXISTS (SELECT 1 FROM customer_name_tokens cnt WHERE cnt.customer_id = customers.id AND cnt.token_index = $2)"
	if where != wantWhere {
		t.Fatalf("klausa = %q, ingin %q", where, wantWhere)
	}
	if len(args) != 2 || args[0] != "cabang" || args[1] != "tok-a" {
		t.Fatalf("argumen = %v, ingin [cabang tok-a]", args)
	}

	// Tanpa token (atau token kosong) tidak ada kondisi tambahan sama sekali.
	where, args = addNameTokenFilters("", nil, nil)
	if where != "" || len(args) != 0 {
		t.Fatalf("tanpa token = (%q, %v), ingin kosong", where, args)
	}
	where, args = addNameTokenFilters("", nil, []string{"", ""})
	if where != "" || len(args) != 0 {
		t.Fatalf("token kosong = (%q, %v), ingin kosong", where, args)
	}
}
