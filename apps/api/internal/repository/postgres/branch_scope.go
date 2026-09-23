package postgres

import (
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
)

// branchReadClause menyusun klausa WHERE untuk membatasi pembacaan baris pada
// cabang aktor, bersama argumen posisionalnya. Aktor lintas cabang (SUPERADMIN,
// AUDITOR, SYSTEM) tidak difilter: mereka memang mengawasi seluruh bank.
//
// Bila cakupan unit aktor sudah diresolusi middleware (Actor.BranchScope.Loaded),
// filter memakai himpunan kode cabang yang boleh diakses (cabang/area/wilayah),
// bukan satu kode: area melihat semua cabang di areanya, wilayah seluruh
// wilayahnya. Karena kolom yang difilter bertipe uuid sedangkan cakupan berisi
// kode, kode dipetakan lebih dulu ke id cabang lewat subquery. Bank tanpa
// area/wilayah tetap menghasilkan himpunan satu kode, sehingga perilakunya sama
// dengan sebelumnya. Jalur lama (cakupan belum diresolusi, mis. aktor yang
// dibangun di uji unit) dipertahankan apa adanya.
//
// Baris dengan branch_id NULL sengaja TETAP disertakan. Migrasi 000020 hanya
// mengisi branch_id bila ada dasar cabang yang sah, sehingga NULL berarti data
// pra-migrasi yang cabangnya belum diketahui; menyembunyikannya akan membuat
// data lama hilang dari operasional. Ini mengikuti semantik Actor.CanAccessBranch.
func branchReadClause(column string, actor domain.Actor) (string, []any) {
	if actor.IsCrossBranch() {
		return "", nil
	}
	if actor.HasResolvedBranchScope() {
		// Kolom branch_id bertipe uuid, sedangkan cakupan berisi kode unit. Kode
		// dipetakan ke id lewat subquery, bukan dibandingkan langsung: `branch_id =
		// ANY(kode)` membuat PostgreSQL menolak input kode sebagai uuid (SQLSTATE
		// 22P02) sehingga setiap endpoint daftar gagal 500 untuk staf bercabang.
		clause := fmt.Sprintf("(%s IS NULL OR %s IN (SELECT id FROM branches WHERE code = ANY($1)))", column, column)
		return clause, []any{actor.BranchScope.Codes()}
	}
	// Nilai lama: pemetaan kode cabang aktor ke id memakai subquery, bukan query
	// terpisah di service, agar filter tetap berada di dalam satu query dan
	// pagination benar. Bila kode cabang aktor kosong, subquery menghasilkan NULL
	// sehingga perbandingan branch_id = NULL tidak pernah benar dan hanya baris
	// NULL yang tersisa — konsisten dengan CanAccessBranch yang hanya mengizinkan
	// cabang kosong.
	clause := fmt.Sprintf("(%s IS NULL OR %s = (SELECT id FROM branches WHERE code = $1))", column, column)
	return clause, []any{actor.BranchCode}
}
