package domain

import (
	"errors"
	"testing"
)

// Kewajiban cabang terdaftar hanya berlaku untuk peran bercabang. SUPERADMIN,
// AUDITOR, dan SYSTEM bertindak lintas cabang: kode 'HO' milik akun kantor pusat
// bukan baris di tabel branches, sehingga operasinya tidak boleh ditolak.
func TestActorRequiresRegisteredBranch(t *testing.T) {
	cases := []struct {
		role StaffRole
		want bool
	}{
		{RoleSuperAdmin, false},
		{RoleAuditor, false},
		{RoleSystem, false},
		{RoleAdmin, true},
		{RoleSupervisor, true},
		{RoleTeller, true},
		{RoleCS, true},
		{RoleAO, true},
	}
	for _, tc := range cases {
		a := Actor{Role: tc.role, BranchCode: "HO"}
		if got := a.RequiresRegisteredBranch(); got != tc.want {
			t.Errorf("RequiresRegisteredBranch(%q) = %v, ingin %v", tc.role, got, tc.want)
		}
	}
}

// Kode cabang harus tepat 3 angka karena menjadi awalan nomor rekening.
func TestValidateBranchCode(t *testing.T) {
	for _, code := range []string{"001", "002", "999"} {
		if err := ValidateBranchCode(code); err != nil {
			t.Errorf("ValidateBranchCode(%q) = %v, ingin nil", code, err)
		}
	}
	for _, code := range []string{"", "HO", "12", "1234", "12A", "0A1"} {
		if err := ValidateBranchCode(code); !errors.Is(err, ErrInvalidBranchCode) {
			t.Errorf("ValidateBranchCode(%q) = %v, ingin ErrInvalidBranchCode", code, err)
		}
	}
}
