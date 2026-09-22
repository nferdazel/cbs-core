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

// Cakupan unit hierarki (W14): aktor di area melihat seluruh cabang di areanya,
// aktor di wilayah seluruh wilayahnya, aktor cabang tetap hanya cabangnya. Bank
// tanpa area/wilayah memakai cakupan satu kode, jadi perilakunya identik.
func TestActorCanAccessBranchDenganHierarki(t *testing.T) {
	area := Actor{Role: RoleSupervisor, BranchCode: "810", BranchScope: NewBranchScope([]string{"810", "811", "812"})}
	region := Actor{Role: RoleSupervisor, BranchCode: "900", BranchScope: NewBranchScope([]string{"900", "810", "811", "812"})}
	branch := Actor{Role: RoleTeller, BranchCode: "811", BranchScope: NewBranchScope([]string{"811"})}

	cases := []struct {
		name  string
		actor Actor
		code  string
		want  bool
	}{
		{"cabang hanya cabangnya", branch, "811", true},
		{"cabang tidak melihat cabang lain di area", branch, "812", false},
		{"area melihat cabang di areanya", area, "811", true},
		{"area melihat cabang lain di areanya", area, "812", true},
		{"area tidak melihat di luar areanya", area, "001", false},
		{"wilayah melihat cabang anak", region, "812", true},
		{"wilayah melihat area anak", region, "810", true},
		{"wilayah tidak melihat di luar wilayahnya", region, "001", false},
		// Cakupan kosong yang sudah diresolusi = aktor tanpa unit: tidak melihat apa pun.
		{"cakupan kosong menolak", Actor{Role: RoleTeller, BranchCode: "811", BranchScope: NewBranchScope(nil)}, "811", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.actor.CanAccessBranch(tc.code); got != tc.want {
				t.Fatalf("CanAccessBranch(%q) = %v, ingin %v", tc.code, got, tc.want)
			}
		})
	}
}

// Tanpa resolusi (mis. aktor di uji unit/legacy), perilaku lama dipertahankan:
// hanya cabangnya sendiri. Ini bukti pengguna lama tidak kehilangan akses.
func TestActorCanAccessBranchTanpaResolusiIdentikLama(t *testing.T) {
	legacy := Actor{Role: RoleTeller, BranchCode: "811"}
	resolved := Actor{Role: RoleTeller, BranchCode: "811", BranchScope: NewBranchScope([]string{"811"})}
	for _, code := range []string{"811", "812", "001", ""} {
		if legacy.CanAccessBranch(code) != resolved.CanAccessBranch(code) {
			t.Fatalf("kode %q: legacy=%v resolved=%v, harus sama", code, legacy.CanAccessBranch(code), resolved.CanAccessBranch(code))
		}
	}
}

// BranchScope disimpan tergabung agar Actor tetap dapat dibandingkan dengan ==.
// Uji ini mengunci dedup dan pemisahan kode, termasuk kode yang mengandung tanda
// hubung (kode area/wilayah bank bebas, jadi pemisah koma wajib aman).
func TestBranchScopeDedupDanIsi(t *testing.T) {
	scope := NewBranchScope([]string{"811", "811", "", "AREA-1", "812"})
	if !scope.Loaded {
		t.Fatal("cakupan hasil konstruktor harus Loaded")
	}
	if got := scope.Codes(); len(got) != 3 || got[0] != "811" || got[1] != "AREA-1" || got[2] != "812" {
		t.Fatalf("Codes() = %v, ingin [811 AREA-1 812]", got)
	}
	if !scope.Contains("AREA-1") || scope.Contains("999") {
		t.Fatal("Contains tidak konsisten")
	}
	if (BranchScope{}).Loaded || (BranchScope{}).Contains("811") {
		t.Fatal("nilai nol BranchScope harus berarti belum diresolusi")
	}
}

// Actor harus tetap dapat dibandingkan (dipakai uji pemetaan actor). Slice akan
// membuat operasi == gagal kompilasi; BranchScope sengaja string-based.
func TestActorDapatDibandingkan(t *testing.T) {
	a := Actor{Role: RoleTeller, BranchCode: "811", BranchScope: NewBranchScope([]string{"811", "812"})}
	b := Actor{Role: RoleTeller, BranchCode: "811", BranchScope: NewBranchScope([]string{"811", "812"})}
	if a != b {
		t.Fatal("Actor dengan cakupan sama harus sama")
	}
}
