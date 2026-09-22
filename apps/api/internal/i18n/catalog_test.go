package i18n

import (
	"strings"
	"testing"
)

// TestKatalogLengkap menjaga agar setiap kode punya terjemahan ID dan EN, tidak
// ada kode ganda, dan tidak ada entri katalog yang tidak terdaftar. Tanpa ini,
// pesan yang lupa diterjemahkan akan diam-diam jatuh ke bahasa lain.
func TestKatalogLengkap(t *testing.T) {
	seen := map[Code]bool{}
	for _, c := range codeList {
		if seen[c] {
			t.Errorf("kode ganda: %s", c)
		}
		seen[c] = true

		terjemahan, ok := catalog[c]
		if !ok {
			t.Errorf("kode %q tidak ada di katalog", c)
			continue
		}
		for _, lang := range []Lang{ID, EN} {
			if strings.TrimSpace(terjemahan[lang]) == "" {
				t.Errorf("kode %s: terjemahan %s kosong", c, lang)
			}
		}
		if strings.Count(terjemahan[ID], "%s") != strings.Count(terjemahan[EN], "%s") {
			t.Errorf("kode %s: jumlah placeholder %%s ID (%d) dan EN (%d) tidak sama",
				c, strings.Count(terjemahan[ID], "%s"), strings.Count(terjemahan[EN], "%s"))
		}
	}
	for c := range catalog {
		if !seen[c] {
			t.Errorf("entri katalog %q tidak terdaftar di codeList", c)
		}
	}
}

// TestDefaultBahasa memastikan default adalah ID, dan CBS_LANGUAGE=en mengalihkan
// ke EN. Bahasa sengaja ditentukan di tingkat instalasi, bukan per permintaan.
func TestDefaultBahasa(t *testing.T) {
	t.Setenv(languageEnv, "")
	if got := Default(); got != ID {
		t.Fatalf("default = %q, mau %q", got, ID)
	}
	t.Setenv(languageEnv, "en")
	if got := Default(); got != EN {
		t.Fatalf("CBS_LANGUAGE=en menghasilkan %q, mau %q", got, EN)
	}
	t.Setenv(languageEnv, "EN")
	if got := Default(); got != EN {
		t.Fatalf("CBS_LANGUAGE=EN (huruf besar) menghasilkan %q, mau %q", got, EN)
	}
	t.Setenv(languageEnv, "tidak-dikenal")
	if got := Default(); got != ID {
		t.Fatalf("bahasa tak dikenal menghasilkan %q, mau %q (ID)", got, ID)
	}
}

// TestTerjemahan memeriksa resolusi bahasa dan pengisian placeholder.
func TestTerjemahan(t *testing.T) {
	t.Setenv(languageEnv, "id")
	if got := Text(MsgLoginSuccessful); got != "login berhasil" {
		t.Errorf("Text ID = %q", got)
	}
	if got := Textf(MsgInvalidRequestBodyWithErr, "boom"); got != "isi permintaan tidak valid: boom" {
		t.Errorf("Textf ID = %q", got)
	}
	t.Setenv(languageEnv, "en")
	if got := Text(MsgLoginSuccessful); got != "login successful" {
		t.Errorf("Text EN = %q", got)
	}
	if got := Textf(MsgForbiddenRolePermission, "TELLER", "loans:approve"); got != "forbidden: your role (TELLER) does not have 'loans:approve' permission" {
		t.Errorf("Textf EN = %q", got)
	}
	// T() mengeksplisitkan bahasa, terlepas dari default instalasi.
	if got := T(ID, MsgLoginSuccessful); got != "login berhasil" {
		t.Errorf("T(ID) = %q", got)
	}
}
