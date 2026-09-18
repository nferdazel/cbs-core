package postgres

import (
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// buildJournalListQuery adalah bagian murni dari filter daftar jurnal: ia
// menentukan klausa cabang, argumen, dan bentuk query COUNT/SELECT. SQL akhirnya
// hanya dapat diuji dengan database; di sini yang diuji adalah penentuan scope
// cabang dan konsistensi klausa antara COUNT dan SELECT.
func TestBuildJournalListQuery(t *testing.T) {
	t.Run("aktor lintas cabang tidak difilter", func(t *testing.T) {
		for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAuditor, domain.RoleSystem} {
			countQuery, listQuery, args := buildJournalListQuery(domain.Actor{Role: role, BranchCode: "001"})
			if args != nil {
				t.Fatalf("role %s seharusnya tanpa argumen filter, dapat %v", role, args)
			}
			if strings.Contains(countQuery, "WHERE") || strings.Contains(listQuery, "WHERE") {
				t.Fatalf("role %s seharusnya tanpa klausa WHERE: count=%q list=%q", role, countQuery, listQuery)
			}
			if !strings.Contains(listQuery, "LIMIT $1 OFFSET $2") {
				t.Fatalf("tanpa filter, limit/offset harus placeholder pertama: %q", listQuery)
			}
		}
	})

	t.Run("aktor cabang difilter dan baris NULL tetap terlihat", func(t *testing.T) {
		countQuery, listQuery, args := buildJournalListQuery(domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})

		for _, q := range []string{countQuery, listQuery} {
			if !strings.Contains(q, "branch_id IS NULL") {
				t.Fatalf("klausa harus menyertakan baris branch_id NULL: %q", q)
			}
			if !strings.Contains(q, "branch_id = (SELECT id FROM branches WHERE code = $1)") {
				t.Fatalf("klausa harus memetakan kode cabang ke id cabang: %q", q)
			}
		}
		if len(args) != 1 || args[0] != "001" {
			t.Fatalf("args %v, ingin [001]", args)
		}
		if !strings.Contains(listQuery, "LIMIT $2 OFFSET $3") {
			t.Fatalf("dengan satu argumen filter, limit/offset harus $2/$3: %q", listQuery)
		}
	})

	t.Run("aktor cabang lain memakai kode cabangnya sendiri", func(t *testing.T) {
		_, _, args := buildJournalListQuery(domain.Actor{Role: domain.RoleTeller, BranchCode: "002"})
		if len(args) != 1 || args[0] != "002" {
			t.Fatalf("args %v, ingin [002]", args)
		}
	})

	t.Run("aktor tanpa kode cabang hanya menyisakan baris NULL", func(t *testing.T) {
		countQuery, _, args := buildJournalListQuery(domain.Actor{Role: domain.RoleTeller})
		if !strings.Contains(countQuery, "WHERE") {
			t.Fatal("aktor operasional tanpa kode cabang tetap harus difilter")
		}
		if len(args) != 1 || args[0] != "" {
			t.Fatalf("args %v, ingin [\"\"]", args)
		}
	})
}

// buildAccountStatementQuery memakai klausa cabang yang sama, tetapi dengan
// accountID sebagai argumen tambahan setelah kode cabang. Yang diuji: scope
// cabang dan posisi placeholder pagination agar COUNT dan SELECT konsisten.
func TestBuildAccountStatementQuery(t *testing.T) {
	accountID := uuid.New()

	t.Run("aktor lintas cabang tanpa filter cabang", func(t *testing.T) {
		countQuery, listQuery, args := buildAccountStatementQuery(domain.Actor{Role: domain.RoleSystem}, accountID)
		if strings.Contains(countQuery, "branch_id") || strings.Contains(listQuery, "branch_id") {
			t.Fatalf("aktor lintas cabang tidak boleh difilter: count=%q list=%q", countQuery, listQuery)
		}
		if len(args) != 1 || args[0] != accountID {
			t.Fatalf("args %v, ingin [%s]", args, accountID)
		}
		if !strings.Contains(listQuery, "LIMIT $2 OFFSET $3") {
			t.Fatalf("tanpa filter cabang, pagination harus $2/$3: %q", listQuery)
		}
	})

	t.Run("aktor cabang difilter pada cabang rekening", func(t *testing.T) {
		countQuery, listQuery, args := buildAccountStatementQuery(domain.Actor{Role: domain.RoleTeller, BranchCode: "001"}, accountID)
		for _, q := range []string{countQuery, listQuery} {
			if !strings.Contains(q, "a.branch_id IS NULL") {
				t.Fatalf("klausa harus menyertakan rekening tanpa cabang: %q", q)
			}
			if !strings.Contains(q, "code = $1") {
				t.Fatalf("kode cabang harus placeholder $1: %q", q)
			}
		}
		if len(args) != 2 || args[0] != "001" || args[1] != accountID {
			t.Fatalf("args %v, ingin [001 %s]", args, accountID)
		}
		if !strings.Contains(listQuery, "LIMIT $3 OFFSET $4") {
			t.Fatalf("dengan satu argumen cabang, pagination harus $3/$4: %q", listQuery)
		}
	})
}
