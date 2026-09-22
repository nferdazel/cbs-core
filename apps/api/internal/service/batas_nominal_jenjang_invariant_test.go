package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keputusan pemilik sistem: HIERARKI = CAKUPAN DATA, GRUP = KEWENANGAN. Tingkat
// cabang/area/wilayah TIDAK menentukan batas nominal. Batas nominal hanya dibaca dari
// matriks `limit.<peran>.<jenis>.*` (migrasi 000051), dan kaitan grup
// (`user_groups.approval_limit_role`, migrasi 000079) menunjuk baris PERAN pada matriks
// yang sama — bukan matriks kedua per jenjang.
//
// Uji ini adalah PENJAGA (bukan matriks baru): ia gagal bila kode yang menegakkan
// kewenangan nominal mulai menyimpulkan batas dari jenjang/`unit_level`, atau bila ada
// kunci batas per jenjang masuk ke seed. Bank bebas memakai area/wilayah, tetapi itu
// hanya memperluas cakupan data, bukan menaikkan/menurunkan nominal.
func TestBatasNominalTidakDariJenjang(t *testing.T) {
	root := configSeedRepoRoot(t)

	// (1) Berkas yang menegakkan kewenangan nominal tidak boleh menyentuh jenjang unit.
	for _, rel := range []string{
		filepath.Join("apps", "api", "internal", "service", "limit_service.go"),
		filepath.Join("apps", "api", "internal", "service", "maker_checker_service.go"),
		filepath.Join("apps", "api", "internal", "domain", "limit.go"),
		filepath.Join("apps", "api", "internal", "domain", "maker_checker.go"),
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("membaca %s: %v", rel, err)
		}
		for _, token := range []string{"UnitLevel", "unit_level", "OrgUnitLevel"} {
			if strings.Contains(string(data), token) {
				t.Fatalf("%s menyebut %q: batas nominal TIDAK boleh disimpulkan dari jenjang unit "+
					"(hierarki = cakupan data, grup = kewenangan); gunakan matriks limit.<peran>.* yang sama",
					rel, token)
			}
		}
	}

	// (2) Tidak boleh ada kunci batas per jenjang di seed. Nama jenjang dalam huruf kecil
	//     dipakai karena kunci konfigurasi W3 selalu lowercase.
	seeded := parseSystemConfigSeed(t, filepath.Join(root, "packages", "db-migrations"))
	levels := map[string]bool{"cabang": true, "area": true, "wilayah": true, "branch": true, "region": true}
	for key := range seeded {
		parts := strings.Split(key, ".")
		if len(parts) >= 2 && parts[0] == "limit" && levels[parts[1]] {
			t.Fatalf("kunci batas per jenjang %q ada di seed; batas nominal hanya boleh per peran/grup", key)
		}
	}
}
