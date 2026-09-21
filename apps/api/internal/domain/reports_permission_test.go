package domain

import "testing"

// Laporan keuangan bank-wide dipisahkan dari ledger:read: hanya peran yang
// berkepentingan (pengawas/admin/auditor) yang boleh membacanya.
func TestPermReportsFinancialReadHanyaPeranBerkepentingan(t *testing.T) {
	for _, role := range []StaffRole{RoleSuperAdmin, RoleAdmin, RoleSupervisor, RoleAuditor} {
		if !role.HasPermission(PermReportsFinancialRead) {
			t.Fatalf("peran %s harus memegang reports:financial:read", role)
		}
	}
	for _, role := range []StaffRole{RoleTeller, RoleCS, RoleAO} {
		if role.HasPermission(PermReportsFinancialRead) {
			t.Fatalf("peran cabang %s tidak boleh memegang reports:financial:read", role)
		}
	}
}

// AO (dan peran cabang lain) tetap memegang ledger:read untuk mutasi/statement
// rekening maupun laporan operasional due-obligations.
func TestPeranCabangTetapMemegangLedgerRead(t *testing.T) {
	for _, role := range []StaffRole{RoleTeller, RoleCS, RoleAO} {
		if !role.HasPermission(PermLedgerRead) {
			t.Fatalf("peran %s harus tetap memegang ledger:read", role)
		}
	}
}
