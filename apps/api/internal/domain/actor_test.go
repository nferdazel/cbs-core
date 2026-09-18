package domain

import "testing"

// Semua peran harus punya jawaban yang jelas: pengawas lintas cabang boleh,
// peran operasional tidak.
func TestActorIsCrossBranchSemuaPeran(t *testing.T) {
	cases := []struct {
		role StaffRole
		want bool
	}{
		{RoleSuperAdmin, true},
		{RoleAuditor, true},
		{RoleSystem, true},
		{RoleAdmin, false},
		{RoleSupervisor, false},
		{RoleTeller, false},
		{RoleCS, false},
		{RoleAO, false},
		{StaffRole("TIDAK_DIKENAL"), false},
	}
	for _, tc := range cases {
		a := Actor{Role: tc.role, BranchCode: "001"}
		if got := a.IsCrossBranch(); got != tc.want {
			t.Errorf("IsCrossBranch(%q) = %v, ingin %v", tc.role, got, tc.want)
		}
	}
}

func TestActorCanAccessBranch(t *testing.T) {
	cases := []struct {
		name       string
		actor      Actor
		branchCode string
		want       bool
	}{
		{"cabang sama", Actor{Role: RoleTeller, BranchCode: "001"}, "001", true},
		{"cabang berbeda", Actor{Role: RoleTeller, BranchCode: "001"}, "002", false},
		{"aktor tanpa cabang tidak boleh", Actor{Role: RoleTeller}, "001", false},
		{"pengawas lintas cabang", Actor{Role: RoleAuditor, BranchCode: "001"}, "002", true},
		{"superadmin lintas cabang", Actor{Role: RoleSuperAdmin, BranchCode: "001"}, "002", true},
		{"system lintas cabang untuk batch", Actor{Role: RoleSystem}, "002", true},
		{"data pra-migrasi tanpa cabang dibiarkan", Actor{Role: RoleTeller, BranchCode: "001"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.actor.CanAccessBranch(tc.branchCode); got != tc.want {
				t.Fatalf("CanAccessBranch(%q) = %v, ingin %v", tc.branchCode, got, tc.want)
			}
		})
	}
}
