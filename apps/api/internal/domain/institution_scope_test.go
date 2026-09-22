package domain

import "testing"

// Cakupan buku instalasi adalah lapisan pertama: lini usaha yang tidak dilayani
// instalasi harus tertutup bahkan bagi peran lintas buku, sedangkan DUAL/kosong
// harus identik dengan perilaku lama.
func TestInstitutionBookScopeActiveBooks(t *testing.T) {
	cases := []struct {
		scope InstitutionBookScope
		want  []COABook
	}{
		{ScopeConventional, []COABook{BookConventional}},
		{ScopeSyariah, []COABook{BookSyariah}},
		{ScopeDual, []COABook{BookConventional, BookSyariah}},
		{"", []COABook{BookConventional, BookSyariah}},
	}
	for _, tc := range cases {
		got := tc.scope.ActiveBooks()
		if len(got) != len(tc.want) {
			t.Fatalf("ActiveBooks(%q) = %v, ingin %v", tc.scope, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ActiveBooks(%q) = %v, ingin %v", tc.scope, got, tc.want)
			}
		}
	}
}

func TestParseInstitutionBookScope(t *testing.T) {
	cases := []struct {
		raw  string
		want InstitutionBookScope
	}{
		{"SYARIAH", ScopeSyariah},
		{"syariah", ScopeSyariah},
		{"KONVENSIONAL", ScopeConventional},
		{"CONVENTIONAL", ScopeConventional},
		{"DUAL", ScopeDual},
		{"", ScopeDual},
		{"nilai-rusak", ScopeDual},
	}
	for _, tc := range cases {
		if got := ParseInstitutionBookScope(tc.raw); got != tc.want {
			t.Errorf("ParseInstitutionBookScope(%q) = %q, ingin %q", tc.raw, got, tc.want)
		}
	}
}

// Pada instalasi satu buku, peran pengawas pun tidak lagi lintas buku agar filter
// repository ikut berlaku. Instalasi DUAL tidak boleh berubah.
func TestInstitutionBookScopeMenutupLintasBuku(t *testing.T) {
	for _, role := range []StaffRole{RoleSuperAdmin, RoleAuditor, RoleSystem} {
		if (Actor{Role: role}).IsCrossBook() != true {
			t.Fatalf("role %s pada DUAL harus lintas buku", role)
		}
		if (Actor{Role: role, BookScope: ScopeDual}).IsCrossBook() != true {
			t.Fatalf("role %s pada DUAL eksplisit harus lintas buku", role)
		}
		if (Actor{Role: role, BookScope: ScopeSyariah}).IsCrossBook() != false {
			t.Fatalf("role %s pada instalasi SYARIAH tidak boleh lintas buku", role)
		}
		if (Actor{Role: role, BookScope: ScopeConventional}).IsCrossBook() != false {
			t.Fatalf("role %s pada instalasi KONVENSIONAL tidak boleh lintas buku", role)
		}
	}
}

// Peran operasional terbatas buku tetap terbatas, dan cakupan instalasi menutup
// sisi yang tidak dilayani.
func TestInstitutionBookScopeCanAccessBook(t *testing.T) {
	if (Actor{Role: RoleAO, Book: BookConventional, BookScope: ScopeSyariah}).CanAccessBook(BookConventional) {
		t.Fatal("instalasi SYARIAH harus menolak buku konvensional")
	}
	if !(Actor{Role: RoleAO, Book: BookSyariah, BookScope: ScopeSyariah}).CanAccessBook(BookSyariah) {
		t.Fatal("instalasi SYARIAH harus mengizinkan buku syariah")
	}
	if (Actor{Role: RoleAO, Book: BookSyariah, BookScope: ScopeConventional}).CanAccessBook(BookSyariah) {
		t.Fatal("instalasi KONVENSIONAL harus menolak buku syariah")
	}
	// Aktor lintas buku pada instalasi DUAL tidak berubah.
	if !(Actor{Role: RoleAuditor, BookScope: ScopeDual}).CanAccessBook(BookSyariah) {
		t.Fatal("aktor lintas buku pada DUAL harus tetap boleh")
	}
	// Aktor lintas buku pada instalasi satu buku ikut ditutup untuk sisi lain.
	if (Actor{Role: RoleAuditor, BookScope: ScopeSyariah}).CanAccessBook(BookConventional) {
		t.Fatal("instalasi SYARIAH harus menolak buku konvensional bagi auditor")
	}
}

func TestInstitutionBookScopeConstrainedBook(t *testing.T) {
	if got := (Actor{Role: RoleSuperAdmin, BookScope: ScopeSyariah}).ConstrainedBook("CONVENTIONAL"); got != "SYARIAH" {
		t.Fatalf("instalasi SYARIAH harus memaksa laporan ke SYARIAH, dapat %q", got)
	}
	if got := (Actor{Role: RoleSuperAdmin, BookScope: ScopeConventional}).ConstrainedBook("SYARIAH"); got != "CONVENTIONAL" {
		t.Fatalf("instalasi KONVENSIONAL harus memaksa laporan ke CONVENTIONAL, dapat %q", got)
	}
	if got := (Actor{Role: RoleSuperAdmin, BookScope: ScopeDual}).ConstrainedBook("SYARIAH"); got != "SYARIAH" {
		t.Fatalf("DUAL harus menghormati pilihan, dapat %q", got)
	}
}

func TestJWTClaimsToActorMeneruskanCakupanInstalasi(t *testing.T) {
	actor := (&JWTClaims{Book: BookSyariah, BookScope: ScopeSyariah}).ToActor("127.0.0.1", "req-1")
	if actor.BookScope != ScopeSyariah {
		t.Fatalf("BookScope=%q, ingin SYARIAH", actor.BookScope)
	}
}
