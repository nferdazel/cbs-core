package postgres

import (
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
)

// branchReadClause menyusun klausa WHERE untuk membatasi pembacaan baris pada
// cabang aktor, bersama argumen posisionalnya. Aktor lintas cabang (SUPERADMIN,
// AUDITOR, SYSTEM) tidak difilter: mereka memang mengawasi seluruh bank.
//
// Baris dengan branch_id NULL sengaja TETAP disertakan. Migrasi 000020 hanya
// mengisi branch_id bila ada dasar cabang yang sah, sehingga NULL berarti data
// pra-migrasi yang cabangnya belum diketahui; menyembunyikannya akan membuat
// data lama hilang dari operasional. Ini mengikuti semantik Actor.CanAccessBranch.
//
// Pemetaan kode cabang aktor ke id memakai subquery, bukan query terpisah di
// service, agar filter tetap berada di dalam satu query dan pagination benar.
// Bila kode cabang aktor kosong, subquery menghasilkan NULL sehingga
// perbandingan branch_id = NULL tidak pernah benar dan hanya baris NULL yang
// tersisa — konsisten dengan CanAccessBranch yang hanya mengizinkan cabang kosong.
func branchReadClause(column string, actor domain.Actor) (string, []any) {
	if actor.IsCrossBranch() {
		return "", nil
	}
	clause := fmt.Sprintf("(%s IS NULL OR %s = (SELECT id FROM branches WHERE code = $1))", column, column)
	return clause, []any{actor.BranchCode}
}
