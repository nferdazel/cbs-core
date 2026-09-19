package postgres

import "strings"

// likePrefixPattern menyiapkan pola pencarian awalan untuk ILIKE. Karakter khusus
// LIKE (%, _, \) di-escape agar kata kunci dari pengguna tidak berubah menjadi
// wildcard: mengetik "%" tidak boleh berarti "semua baris". Pemanggil wajib memakai
// klausa dengan ESCAPE '\' agar escape ini berlaku.
//
// Kata kunci kosong (atau hanya spasi) menghasilkan string kosong, yang bagi
// pemanggil berarti "tanpa filter pencarian".
func likePrefixPattern(term string) string {
	term = strings.TrimSpace(term)
	if term == "" {
		return ""
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
	return escaped + "%"
}

// andCondition menggabungkan klausa WHERE opsional. Klausa kosong diabaikan, dan
// hasil kosong berarti tidak ada kondisi sama sekali.
func andCondition(existing, cond string) string {
	if cond == "" {
		return existing
	}
	if existing == "" {
		return cond
	}
	return existing + " AND " + cond
}
