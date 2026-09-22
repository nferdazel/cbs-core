// Package i18n menyatukan pesan yang ditampilkan API dalam satu katalog berkode
// dengan terjemahan Indonesia dan Inggris. Handler tidak membaca kamus: mereka
// memakai konstanta Code dan helper respons (Success/ErrorCode) yang
// menerjemahkannya, sehingga pesan tidak lagi berserak sebagai literal.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Lang adalah bahasa pesan.
type Lang string

const (
	// ID adalah bahasa utama aplikasi.
	ID Lang = "id"
	// EN adalah terjemahan Inggris.
	EN Lang = "en"
)

// languageEnv adalah nama variabel lingkungan penentu bahasa instalasi.
const languageEnv = "CBS_LANGUAGE"

// Default mengembalikan bahasa yang dipakai proses ini.
//
// Bahasa ditetapkan di tingkat instalasi lewat CBS_LANGUAGE, bukan dinegosiasikan
// per permintaan. Alasannya: keputusan pemilik adalah satu instalasi per bank,
// sehingga preferensi bahasa adalah setelan bank, dan web (satu-satunya klien)
// menyimpan pilihan bahasanya di sisi klien tanpa meneruskan header apa pun ke
// API. Memakai Accept-Language justru berbahaya karena browser mengirim header itu
// otomatis; pesan galat bisa berganti ke Inggris tanpa pengguna memilihnya dan
// tidak lagi seragam dengan antarmuka. Tersedia bila nanti dipilih per permintaan.
func Default() Lang {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(languageEnv)), string(EN)) {
		return EN
	}
	return ID
}

// Text menerjemahkan kode ke bahasa default instalasi.
func Text(c Code) string {
	return render(Default(), c, false, nil)
}

// Textf menerjemahkan kode lalu mengisi placeholder formatnya (%s).
func Textf(c Code, args ...any) string {
	return render(Default(), c, true, args)
}

// T menerjemahkan kode ke bahasa tertentu. Disediakan agar pemilihan bahasa per
// permintaan dapat ditambahkan tanpa mengubah setiap handler.
func T(lang Lang, c Code) string {
	return render(lang, c, false, nil)
}

// render memilih terjemahan lalu, bila ada argumen, mengisi formatnya. Bahasa yang
// tidak dikenal jatuh ke ID, dan kode yang tidak ada di katalog dikembalikan apa
// adanya agar tidak diam-diam menjadi pesan kosong; uji katalog menutup celah ini.
func render(lang Lang, c Code, format bool, args []any) string {
	translations, ok := catalog[c]
	if !ok {
		return string(c)
	}
	text, ok := translations[lang]
	if !ok || text == "" {
		text = translations[ID]
	}
	if format {
		return fmt.Sprintf(text, args...)
	}
	return text
}
