package service_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Invarian seed izin (kode vs migrasi).
//
// Masalah yang dicegah: pemetaan izin kini hidup di database (keputusan pemilik
// sistem), sedangkan domain.RolePermissions hanya menjadi SEED AWAL. Bila kode
// diubah tanpa memperbarui migrasi 000079, database dan kode berbeda. Uji ini
// membandingkan KEDUANYA sehingga ketidaksinkronan terlihat sebagai kegagalan uji,
// bukan sebagai izin yang diam-diam hilang atau muncul di produksi.
//
// Sumber kebenaran pemetaan izin adalah database; uji ini mengunci seed agar
// reproduksi RolePermissions yang berlaku saat migrasi dibuat.

var (
	permissionConstRe = regexp.MustCompile(`(\w+)\s+Permission\s*=\s*"([^"]+)"`)
	groupPermSeedRe   = regexp.MustCompile(`\('(ROLE_[A-Z]+)',\s*'([^']+)'\)`)
	menuPermSeedRe    = regexp.MustCompile(`\('([a-z_]+)',\s*'([a-z:_]+)'\)`)
	menuCatalogRowRe  = regexp.MustCompile(`\('([a-z_]+)',\s*'[^']*'\)`)
	webMenuKeyRe      = regexp.MustCompile(`menuKey:\s*"([^"]+)"`)
	migrationSeedFile = "000079_user_groups_permissions.up.sql"
	// migrationOverrideFile mengubah matriks izin setelah 000079. Invarian
	// membandingkan kode dengan 000079 + overridenya, bukan hanya seed awal, supaya
	// keputusan panel (000091) tetap terjaga.
	migrationOverrideFile = "000091_recover_permission_and_account_freeze.up.sql"
	domainStaffGoRel      = filepath.Join("apps", "api", "internal", "domain", "staff.go")
	webNavTsRel           = filepath.Join("apps", "web", "src", "components", "layout", "nav.ts")
	migrationsDirRel      = filepath.Join("packages", "db-migrations")
)

// declaredPermissionConstants membaca konstanta Perm* dari sumber produksi domain.
// Dipakai untuk memastikan tidak ada izin seed yang salah tulis (bukan konstanta).
func declaredPermissionConstants(t *testing.T, repoRoot string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, domainStaffGoRel))
	if err != nil {
		t.Fatalf("membaca staff.go: %v", err)
	}
	out := map[string]bool{}
	for _, m := range permissionConstRe.FindAllStringSubmatch(string(data), -1) {
		out[m[2]] = true
	}
	if len(out) < 20 {
		t.Fatalf("hanya %d konstanta izin terbaca dari staff.go; pemindai rusak", len(out))
	}
	return out
}

func readMigrationSeed(t *testing.T, repoRoot string) (groupPerms map[string]map[string]bool, menuPerms map[string]map[string]bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, migrationsDirRel, migrationSeedFile))
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", migrationSeedFile, err)
	}
	sql := string(data)

	groupPerms = map[string]map[string]bool{}
	for _, m := range groupPermSeedRe.FindAllStringSubmatch(sql, -1) {
		if groupPerms[m[1]] == nil {
			groupPerms[m[1]] = map[string]bool{}
		}
		groupPerms[m[1]][m[2]] = true
	}
	menuPerms = map[string]map[string]bool{}
	for _, m := range menuPermSeedRe.FindAllStringSubmatch(sql, -1) {
		if menuPerms[m[1]] == nil {
			menuPerms[m[1]] = map[string]bool{}
		}
		menuPerms[m[1]][m[2]] = true
	}
	return groupPerms, menuPerms
}

// readMenuCatalogKeys mengambil kunci menu dari blok INSERT INTO menu_catalog saja,
// supaya tidak tertukar dengan baris menu_permissions.
func readMenuCatalogKeys(t *testing.T, repoRoot string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, migrationsDirRel, migrationSeedFile))
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", migrationSeedFile, err)
	}
	block := string(data)
	idx := strings.Index(block, "INSERT INTO menu_catalog")
	if idx < 0 {
		t.Fatalf("blok INSERT INTO menu_catalog tidak ditemukan di %s", migrationSeedFile)
	}
	block = block[idx:]
	if end := strings.Index(block, ";"); end >= 0 {
		block = block[:end]
	}
	out := map[string]bool{}
	for _, m := range menuCatalogRowRe.FindAllStringSubmatch(block, -1) {
		out[m[1]] = true
	}
	return out
}

// readWebMenuKeys membaca menuKey dari nav.ts: kunci inilah yang dikirim server
// (hanya menu yang ada di katalog DB yang bisa tampil). Bila web dan DB berbeda,
// ada menu yang tak pernah tampil atau sebaliknya; uji menutup celah itu.
func readWebMenuKeys(t *testing.T, repoRoot string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, webNavTsRel))
	if err != nil {
		t.Fatalf("membaca nav.ts: %v", err)
	}
	out := map[string]bool{}
	for _, m := range webMenuKeyRe.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	if len(out) == 0 {
		t.Fatal("tidak ada menuKey terbaca dari nav.ts; pemindai rusak")
	}
	return out
}

// applyPermissionOverrides membaca perubahan matriks izin dari migrasi setelah
// 000079 dan menerapkannya pada seed 000079: blok DELETE FROM group_permissions
// adalah pencabutan, blok INSERT INTO group_permissions adalah pemberian. Dengan
// begitu invarian membandingkan kode dengan seluruh migrasi yang berlaku, bukan
// hanya 000079, dan migrasi izin baru yang lupa disertakan tertangkap.
func applyPermissionOverrides(t *testing.T, repoRoot string, seed map[string]map[string]bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, migrationsDirRel, migrationOverrideFile))
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", migrationOverrideFile, err)
	}
	sql := string(data)
	const revokeMarker = "DELETE FROM group_permissions"
	const grantMarker = "INSERT INTO group_permissions"
	revokeAt := strings.Index(sql, revokeMarker)
	grantAt := strings.Index(sql, grantMarker)
	if revokeAt < 0 || grantAt < 0 || grantAt < revokeAt {
		t.Fatalf("%s harus memuat blok %q lalu %q agar perubahan izin dapat diperiksa",
			migrationOverrideFile, revokeMarker, grantMarker)
	}
	for _, m := range groupPermSeedRe.FindAllStringSubmatch(sql[revokeAt:grantAt], -1) {
		if seed[m[1]] != nil {
			delete(seed[m[1]], m[2])
		}
	}
	for _, m := range groupPermSeedRe.FindAllStringSubmatch(sql[grantAt:], -1) {
		if seed[m[1]] == nil {
			seed[m[1]] = map[string]bool{}
		}
		seed[m[1]][m[2]] = true
	}
}

// TestPermissionSeedInvariant memastikan seed migrasi mereproduksi RolePermissions
// tepat sama untuk setiap peran, dan tidak ada izin seed yang bukan konstanta kode.
func TestPermissionSeedInvariant(t *testing.T) {
	root := configSeedRepoRoot(t)
	known := declaredPermissionConstants(t, root)
	seededGroups, _ := readMigrationSeed(t, root)
	applyPermissionOverrides(t, root, seededGroups)

	if len(seededGroups) == 0 {
		t.Fatalf("tidak ada seed group_permissions terbaca dari %s", migrationSeedFile)
	}

	roles := []domain.StaffRole{
		domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleSupervisor,
		domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor,
	}
	for _, role := range roles {
		code := "ROLE_" + string(role)
		want := map[string]bool{}
		for _, p := range domain.RolePermissions[role] {
			want[string(p)] = true
		}
		got := seededGroups[code]
		if len(got) == 0 {
			t.Fatalf("grup %s tidak di-seed di %s", code, migrationSeedFile)
		}
		if !equalStringSet(want, got) {
			t.Fatalf("seed %s tidak sama dengan RolePermissions[%s]:\n  hanya kode : %v\n  hanya seed : %v\n"+
				"Perbarui seed migrasi %s agar mereproduksi kode, atau sebaliknya.",
				code, role, diffStringSet(want, got), diffStringSet(got, want), migrationSeedFile)
		}
		for p := range got {
			if !known[p] {
				t.Errorf("izin seed %q pada %s bukan konstanta Perm* di domain", p, code)
			}
		}
	}

	// Setiap izin yang dipetakan menu juga harus konstanta yang dikenal.
	_, seededMenus := readMigrationSeed(t, root)
	for menuKey, perms := range seededMenus {
		for p := range perms {
			if !known[p] {
				t.Errorf("izin menu %q pada menu %q bukan konstanta Perm* di domain", p, menuKey)
			}
		}
	}

	// Katalog menu DB dan menuKey web harus sama himpunannya: menu di luar katalog
	// DB tidak akan pernah dikirim server, sehingga item web-nya tak pernah tampil.
	catalog := readMenuCatalogKeys(t, root)
	web := readWebMenuKeys(t, root)
	if !equalStringSet(catalog, web) {
		t.Fatalf("kunci menu web dan katalog DB berbeda:\n  hanya DB : %v\n  hanya web: %v\n"+
			"Samakan menuKey di nav.ts dengan seed menu_catalog.",
			diffStringSet(catalog, web), diffStringSet(web, catalog))
	}
}

func equalStringSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func diffStringSet(have, other map[string]bool) []string {
	var out []string
	for k := range have {
		if !other[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
