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

// Penanda lintas buku meniru lintas cabang: hanya peran pengawas dan SYSTEM.
func TestActorIsCrossBookSemuaPeran(t *testing.T) {
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
		a := Actor{Role: tc.role, Book: BookConventional}
		if got := a.IsCrossBook(); got != tc.want {
			t.Errorf("IsCrossBook(%q) = %v, ingin %v", tc.role, got, tc.want)
		}
	}
}

func TestActorCanAccessBook(t *testing.T) {
	cases := []struct {
		name  string
		actor Actor
		book  COABook
		want  bool
	}{
		{"buku sama", Actor{Role: RoleAO, Book: BookSyariah}, BookSyariah, true},
		{"buku berbeda", Actor{Role: RoleAO, Book: BookSyariah}, BookConventional, false},
		{"pengawas lintas buku", Actor{Role: RoleAuditor, Book: BookConventional}, BookSyariah, true},
		{"superadmin lintas buku", Actor{Role: RoleSuperAdmin}, BookConventional, true},
		{"system lintas buku untuk batch", Actor{Role: RoleSystem}, BookSyariah, true},
		{"buku aktor belum ditentukan tidak dibatasi", Actor{Role: RoleAO}, BookSyariah, true},
		{"data pra-buku tanpa buku dibiarkan", Actor{Role: RoleAO, Book: BookSyariah}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.actor.CanAccessBook(tc.book); got != tc.want {
				t.Fatalf("CanAccessBook(%q) = %v, ingin %v", tc.book, got, tc.want)
			}
		})
	}
}

func TestActorConstrainedBook(t *testing.T) {
	if got := (Actor{Role: RoleAO, Book: BookSyariah}).ConstrainedBook("CONVENTIONAL"); got != "SYARIAH" {
		t.Fatalf("aktor satu buku harus dipaksa ke bukunya, dapat %q", got)
	}
	if got := (Actor{Role: RoleSuperAdmin}).ConstrainedBook("SYARIAH"); got != "SYARIAH" {
		t.Fatalf("aktor lintas buku harus menghormati pilihan, dapat %q", got)
	}
	if got := (Actor{Role: RoleAO}).ConstrainedBook("SYARIAH"); got != "SYARIAH" {
		t.Fatalf("buku aktor belum ditentukan harus menghormati pilihan, dapat %q", got)
	}
}

// ToActor wajib meneruskan buku dari klaim; tanpa ini filter repository kehilangan
// sumbernya pada permintaan HTTP nyata.
func TestJWTClaimsToActorMeneruskanBuku(t *testing.T) {
	actor := (&JWTClaims{Book: BookSyariah}).ToActor("127.0.0.1", "req-1")
	if actor.Book != BookSyariah {
		t.Fatalf("Book=%q, ingin SYARIAH", actor.Book)
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
