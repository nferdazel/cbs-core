package postgres

import (
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// branchReadClause adalah fungsi murni yang menentukan klausa filter cabang.
// Query SQL akhirnya hanya dapat diuji dengan database; di sini yang diuji adalah
// penentuan parameter filternya (dan kehadiran penjaga branch_id IS NULL).
func TestBranchReadClause(t *testing.T) {
	t.Run("aktor lintas cabang tidak difilter", func(t *testing.T) {
		for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAuditor, domain.RoleSystem} {
			clause, args := branchReadClause("a.branch_id", domain.Actor{Role: role, BranchCode: "001"})
			if clause != "" || args != nil {
				t.Fatalf("role %s seharusnya tanpa filter, dapat clause=%q args=%v", role, clause, args)
			}
		}
	})

	t.Run("aktor cabang difilter dan baris NULL tetap terlihat", func(t *testing.T) {
		clause, args := branchReadClause("a.branch_id", domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
		if !strings.Contains(clause, "a.branch_id IS NULL") {
			t.Fatalf("klausa harus menyertakan baris branch_id NULL: %q", clause)
		}
		if !strings.Contains(clause, "code = $1") {
			t.Fatalf("klausa harus memetakan kode cabang ke id cabang: %q", clause)
		}
		if len(args) != 1 || args[0] != "001" {
			t.Fatalf("args %v, ingin [001]", args)
		}
	})

	t.Run("kode cabang kosong hanya menyisakan baris NULL", func(t *testing.T) {
		clause, args := branchReadClause("branch_id", domain.Actor{Role: domain.RoleTeller})
		if clause == "" {
			t.Fatal("aktor operasional tanpa kode cabang tetap harus difilter")
		}
		if len(args) != 1 || args[0] != "" {
			t.Fatalf("args %v, ingin [\"\"]", args)
		}
	})

	t.Run("cakupan hierarki diresolusi memakai himpunan kode", func(t *testing.T) {
		actor := domain.Actor{
			Role:        domain.RoleSupervisor,
			BranchCode:  "810",
			BranchScope: domain.NewBranchScope([]string{"810", "811", "812"}),
		}
		clause, args := branchReadClause("loans.branch_id", actor)
		if !strings.Contains(clause, "loans.branch_id IS NULL") {
			t.Fatalf("klausa harus menyertakan baris branch_id NULL: %q", clause)
		}
		// branch_id bertipe uuid, jadi kode TIDAK boleh dibandingkan langsung dengan
		// kolom itu; kode wajib dipetakan ke id lewat subquery. Klausa `= ANY($1)`
		// langsung ke branch_id menyebabkan SQLSTATE 22P02 dan endpoint 500.
		if !strings.Contains(clause, "IN (SELECT id FROM branches WHERE code = ANY($1))") {
			t.Fatalf("klausa cakupan hierarki harus memetakan kode ke id cabang: %q", clause)
		}
		if len(args) != 1 {
			t.Fatalf("argumen %v, ingin satu parameter himpunan", args)
		}
		codes, ok := args[0].([]string)
		if !ok || len(codes) != 3 || codes[0] != "810" || codes[2] != "812" {
			t.Fatalf("argumen himpunan %v, ingin [810 811 812]", args[0])
		}
	})

	t.Run("cakupan hierarki dan buku tetap berlaku bersamaan", func(t *testing.T) {
		// Sumbu unit dan sumbu buku terpisah: filter cabang memakai $1, filter buku
		// memakai $2, sehingga cakupan area tidak meniadakan penjagaan lini usaha.
		actor := domain.Actor{
			Role:        domain.RoleSupervisor,
			BranchCode:  "810",
			BranchScope: domain.NewBranchScope([]string{"810", "811"}),
			Book:        domain.BookConventional,
		}
		branchClause, branchArgs := branchReadClause("loans.branch_id", actor)
		bookClause, bookArgs := bookReadClause("p.book", actor, len(branchArgs)+1)
		if branchClause == "" || bookClause == "" {
			t.Fatalf("kedua klausa harus terisi: cabang=%q buku=%q", branchClause, bookClause)
		}
		if !strings.Contains(bookClause, "$2") {
			t.Fatalf("filter buku harus memakai placeholder $2 agar tidak bertabrakan: %q", bookClause)
		}
		if len(bookArgs) != 1 || bookArgs[0] != "CONVENTIONAL" {
			t.Fatalf("argumen buku %v, ingin [CONVENTIONAL]", bookArgs)
		}
	})
}
