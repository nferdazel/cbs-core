package postgres

import (
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
)

// bookReadClause menyusun klausa WHERE untuk membatasi pembacaan baris pada buku
// COA (konvensional/syariah) aktor, bersama argumen posisionalnya. Ini padanan
// bookReadClause dari branchReadClause: batas keamanan buku ditegakkan di query,
// bukan diserahkan ke filter tampilan klien.
//
// column adalah ekspresi kolom buku baris, mis. subquery ke banking_products.book
// untuk kredit/deposito atau chart_of_accounts.book untuk rekening. startArg
// memberi nomor placeholder awal sehingga klausa dapat digabung dengan filter lain
// (mis. cabang) yang juga memakai argumen posisional.
//
// Aktor lintas buku (SUPERADMIN, AUDITOR, SYSTEM) tidak difilter: mereka memang
// melayani kedua buku. Aktor yang bukunya belum ditentukan (book kosong) juga
// tidak difilter, supaya pengguna lama tidak langsung kehilangan akses. Baris
// dengan buku NULL tetap disertakan (mis. kredit lama tanpa produk) dengan alasan
// yang sama, mengikuti semantik Actor.CanAccessBook.
func bookReadClause(column string, actor domain.Actor, startArg int) (string, []any) {
	if actor.IsCrossBook() || actor.Book == "" {
		return "", nil
	}
	clause := fmt.Sprintf("(%s IS NULL OR %s = $%d::coa_book)", column, column, startArg)
	return clause, []any{string(actor.Book)}
}

// journalBookFilter membatasi jurnal pada buku aktor. Buku jurnal TIDAK satu kolom
// melainkan gabungan buku baris-barisnya, sehingga bookReadClause (yang menguji satu
// ekspresi kolom) tidak cukup: jurnal yang menyentuh buku lain harus dikecualikan,
// bukan disamarkan oleh satu baris yang kebetulan sebuku. Jurnal tanpa baris ber-buku
// tetap disertakan, dan aktor lintas buku atau yang bukunya belum ditentukan tidak
// difilter, mengikuti semantik Actor.CanAccessJournal. alias adalah alias tabel
// journal_entries pada query pemanggil.
func journalBookFilter(alias string, actor domain.Actor, startArg int) (string, []any) {
	if actor.IsCrossBook() || actor.Book == "" {
		return "", nil
	}
	clause := fmt.Sprintf(`NOT EXISTS (
		SELECT 1 FROM journal_lines jl
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts c ON c.id = a.coa_id
		WHERE jl.journal_entry_id = %s.id
			AND c.book IS NOT NULL
			AND c.book <> $%d::coa_book)`, alias, startArg)
	return clause, []any{string(actor.Book)}
}
