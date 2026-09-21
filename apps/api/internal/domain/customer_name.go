package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// nameTitleTokens adalah kata yang dikenal sebagai gelar/singkatan dan tidak
// membedakan nasabah satu dengan lain. Dibuang dari token pencarian agar "Hj. Siti"
// dan "Siti" menghasilkan token yang sama. Daftar sengaja pendek dan hanya berisi
// bentuk yang lazim di Indonesia; gelar di luar daftar tetap menjadi token biasa,
// yang hanya menambah sedikit noise dan tidak merusak pencocokan.
//
// Ditulis tanpa titik karena tanda titik diperlakukan sebagai bagian singkatan
// ("S.Kom." -> "skom"), bukan pemisah kata.
var nameTitleTokens = map[string]struct{}{
	"hj": {}, "rd": {}, "rr": {},
	"dr": {}, "drs": {}, "dra": {}, "drg": {}, "prof": {},
	"ir": {}, "ns": {}, "apt": {}, "sdr": {}, "sdri": {},
	// gelar akademik/profesi yang umum menempel di depan atau belakang nama
	"sh": {}, "se": {}, "st": {}, "ss": {}, "ssi": {}, "ssos": {},
	"skom": {}, "spd": {}, "sei": {}, "sip": {}, "sked": {}, "skg": {},
	"mm": {}, "ms": {}, "msi": {}, "mh": {}, "mkn": {}, "mkom": {},
	"mkes": {}, "mpd": {}, "mt": {}, "mba": {}, "msc": {}, "phd": {},
	"spa": {}, "spb": {}, "spog": {}, "sppd": {}, "span": {},
}

// NormalizeNameTokens memecah nama lengkap menjadi kata kunci pencarian.
//
// Aturannya:
//   - huruf/digit menjadi huruf kecil; titik dihapus karena bagian singkatan
//     ("S.Kom." -> "skom"); pemisah lain (spasi, koma, tanda hubung, apostrof)
//     menjadi pemisah kata;
//   - spasi berlebih otomatis rapat karena pemecahan memakai Fields;
//   - gelar/singkatan pada nameTitleTokens dibuang;
//   - token satu huruf (inisial) dibuang karena terlalu umum untuk menyaring;
//   - token kembar dalam satu nama hanya disimpan sekali, sehingga nama seperti
//     "Siti Siti" tidak menggandakan baris indeks.
//
// Hasil selalu deterministik dan urut kemunculan, sehingga indeks yang dihitung
// dari hasil ini stabil. Fungsi ini murni: tanpa kunci, tanpa database.
func NormalizeNameTokens(fullName string) []string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return unicode.ToLower(r)
		case r == '.':
			return -1 // hapus, jangan pisahkan: "S.Kom" tetap satu singkatan
		default:
			return ' '
		}
	}, fullName)

	fields := strings.Fields(mapped)
	seen := make(map[string]struct{}, len(fields))
	var tokens []string
	for _, field := range fields {
		if utf8.RuneCountInString(field) < 2 {
			continue
		}
		if _, isTitle := nameTitleTokens[field]; isTitle {
			continue
		}
		if _, dup := seen[field]; dup {
			continue
		}
		seen[field] = struct{}{}
		tokens = append(tokens, field)
	}
	return tokens
}
