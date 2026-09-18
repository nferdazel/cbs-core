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
}
