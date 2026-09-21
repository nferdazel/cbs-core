package postgres

import (
	"fmt"
	"strings"
)

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

// addNameTokenFilters menambahkan satu kondisi EXISTS untuk tiap token nama.
// Semua token digabung dengan AND: nasabah harus memiliki setiap kata yang
// diketik. Tiap elemen tokenCandidates adalah kandidat indeks lintas versi kunci
// untuk satu kata; ANY membuat baris yang diindeks dengan kunci lama atau baru
// sama-sama cocok. Daftar kosong berarti tanpa filter nama.
//
// Subquery berkorelasi ke customers.id (tabel luar), sehingga filter tetap berada
// di query List yang sama dan COUNT memakai klausa yang sama — total pagination
// tetap benar.
func addNameTokenFilters(where string, whereArgs []any, tokenCandidates [][]string) (string, []any) {
	for _, candidates := range tokenCandidates {
		if len(candidates) == 0 {
			continue
		}
		whereArgs = append(whereArgs, candidates)
		cond := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM customer_name_tokens cnt WHERE cnt.customer_id = customers.id AND cnt.token_index = ANY($%d))",
			len(whereArgs),
		)
		where = andCondition(where, cond)
	}
	return where, whereArgs
}
