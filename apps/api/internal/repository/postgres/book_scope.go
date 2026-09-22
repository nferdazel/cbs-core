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
