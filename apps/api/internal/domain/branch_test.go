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

// Jenjang organisasi (W14): hierarki menurun wilayah -> area -> cabang. Atasan
// wajib lebih tinggi; setingkat atau lebih rendah ditolak. Jenjang tak dikenal
// tidak boleh lolos ke database.
func TestOrgUnitLevelRankDanParent(t *testing.T) {
	if !UnitLevelBranch.Valid() || !UnitLevelArea.Valid() || !UnitLevelRegion.Valid() {
		t.Fatal("tiga jenjang harus valid")
	}
	if OrgUnitLevel("AREA_LAIN").Valid() {
		t.Fatal("jenjang tak dikenal harus invalid")
	}
	cases := []struct {
		parent OrgUnitLevel
		child  OrgUnitLevel
		want   bool
	}{
		{UnitLevelRegion, UnitLevelArea, true},
		{UnitLevelRegion, UnitLevelBranch, true},
		{UnitLevelArea, UnitLevelBranch, true},
		{UnitLevelArea, UnitLevelArea, false},
		{UnitLevelBranch, UnitLevelBranch, false},
		{UnitLevelBranch, UnitLevelArea, false},
		{OrgUnitLevel("X"), UnitLevelBranch, false},
	}
	for _, tc := range cases {
		if got := tc.parent.CanBeParentOf(tc.child); got != tc.want {
			t.Errorf("%s.CanBeParentOf(%s) = %v, ingin %v", tc.parent, tc.child, got, tc.want)
		}
	}
}

// Kode area/wilayah bebas (bukan 3 digit, karena tidak dipakai untuk nomor
// rekening), tetapi wajib tidak kosong.
func TestValidateOrgUnitCode(t *testing.T) {
	for _, code := range []string{"AREA-1", "WILAYAH_JABAR", "A1"} {
		if err := ValidateOrgUnitCode(code); err != nil {
			t.Errorf("ValidateOrgUnitCode(%q) = %v, ingin nil", code, err)
		}
	}
	if err := ValidateOrgUnitCode(""); !errors.Is(err, ErrInvalidBranchCode) {
		t.Errorf("kode kosong: err = %v, ingin ErrInvalidBranchCode", err)
	}
}
