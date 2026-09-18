package postgres

import (
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// buildPendingListQuery adalah bagian murni dari filter daftar maker-checker
// PENDING: ia menentukan klausa cabang, argumen, dan bentuk query. SQL akhirnya
// hanya dapat diuji dengan database; di sini yang diuji adalah penentuan scope
// cabang dan kehadiran penjaga branch_id IS NULL.
func TestBuildPendingListQuery(t *testing.T) {
	t.Run("aktor lintas cabang tidak difilter", func(t *testing.T) {
		for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAuditor, domain.RoleSystem} {
			query, args := buildPendingListQuery(domain.Actor{Role: role, BranchCode: "001"})
			if args != nil {
				t.Fatalf("role %s seharusnya tanpa argumen filter, dapat %v", role, args)
			}
			if strings.Contains(query, "branch_id") {
				t.Fatalf("role %s seharusnya tanpa klausa cabang: %q", role, query)
			}
			if !strings.Contains(query, "WHERE status = 'PENDING'") {
				t.Fatalf("filter status PENDING harus tetap ada: %q", query)
			}
		}
	})

	t.Run("aktor cabang difilter dan baris NULL tetap terlihat", func(t *testing.T) {
		query, args := buildPendingListQuery(domain.Actor{Role: domain.RoleSupervisor, BranchCode: "001"})

		if !strings.Contains(query, "branch_id IS NULL") {
			t.Fatalf("klausa harus menyertakan baris branch_id NULL: %q", query)
		}
		if !strings.Contains(query, "branch_id = (SELECT id FROM branches WHERE code = $1)") {
			t.Fatalf("klausa harus memetakan kode cabang ke id cabang: %q", query)
		}
		if !strings.Contains(query, "AND") {
			t.Fatalf("klausa cabang harus digabung ke filter status: %q", query)
		}
		if len(args) != 1 || args[0] != "001" {
			t.Fatalf("args %v, ingin [001]", args)
		}
	})

	t.Run("aktor cabang lain memakai kode cabangnya sendiri", func(t *testing.T) {
		_, args := buildPendingListQuery(domain.Actor{Role: domain.RoleSupervisor, BranchCode: "002"})
		if len(args) != 1 || args[0] != "002" {
			t.Fatalf("args %v, ingin [002]", args)
		}
	})

	t.Run("aktor tanpa kode cabang hanya menyisakan baris NULL", func(t *testing.T) {
		query, args := buildPendingListQuery(domain.Actor{Role: domain.RoleSupervisor})
		if !strings.Contains(query, "branch_id IS NULL") {
			t.Fatalf("aktor operasional tanpa kode cabang tetap harus difilter: %q", query)
		}
		if len(args) != 1 || args[0] != "" {
			t.Fatalf("args %v, ingin [\"\"]", args)
		}
	})
}
