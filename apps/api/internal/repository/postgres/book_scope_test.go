package postgres

import (
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// bookReadClause adalah fungsi murni yang menentukan klausa filter buku COA.
// Query SQL akhirnya hanya dapat diuji dengan database; di sini yang diuji adalah
// penentuan parameter filternya (dan kehadiran penjaga buku NULL).
func TestBookReadClause(t *testing.T) {
	t.Run("aktor lintas buku tidak difilter", func(t *testing.T) {
		for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAuditor, domain.RoleSystem} {
			clause, args := bookReadClause("coa.book", domain.Actor{Role: role, Book: domain.BookConventional}, 1)
			if clause != "" || args != nil {
				t.Fatalf("role %s seharusnya tanpa filter, dapat clause=%q args=%v", role, clause, args)
			}
		}
	})

	t.Run("buku aktor kosong tidak difilter", func(t *testing.T) {
		clause, args := bookReadClause("coa.book", domain.Actor{Role: domain.RoleAO}, 1)
		if clause != "" || args != nil {
			t.Fatalf("buku kosong seharusnya tanpa filter, dapat clause=%q args=%v", clause, args)
		}
	})

	t.Run("aktor satu buku difilter dan baris buku NULL tetap terlihat", func(t *testing.T) {
		clause, args := bookReadClause("coa.book", domain.Actor{Role: domain.RoleAO, Book: domain.BookSyariah}, 2)
		if !strings.Contains(clause, "coa.book IS NULL") {
			t.Fatalf("klausa harus menyertakan baris buku NULL: %q", clause)
		}
		if !strings.Contains(clause, "= $2::coa_book") {
			t.Fatalf("klausa harus memakai placeholder startArg dengan cast enum: %q", clause)
		}
		if len(args) != 1 || args[0] != "SYARIAH" {
			t.Fatalf("args %v, ingin [SYARIAH]", args)
		}
	})
}
