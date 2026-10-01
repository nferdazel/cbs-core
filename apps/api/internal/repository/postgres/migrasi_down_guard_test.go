package postgres

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Penjaga pasangan migrasi. Kebijakan repo: setiap migrasi .up.sql punya pasangan
// .down.sql yang simetris (praktik sejak 000099), sehingga jalur rollback
// (scripts/migrate.sh --down) dapat dipakai untuk migrasi yang tercatat. Migrasi lama
// (000001-000098) belum punya pasangan dan dikecualikan agar penjaga ini tidak menuntut
// perubahan berkas yang sudah dirilis.
//
// Uji ini membaca berkas sumber, bukan basis data, sehingga tetap berjalan di CI tanpa
// PostgreSQL; uji perilaku up->down->up memerlukan DB dan dijalankan manual di VPS.
func TestMigrasiBaruPunyaPasanganDown(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal menemukan lokasi uji")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..",
		"packages", "db-migrations")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("membaca %s: %v", dir, err)
	}

	down := map[string]bool{}
	var ups []string
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".down.sql"):
			down[strings.TrimSuffix(name, ".down.sql")] = true
		case strings.HasSuffix(name, ".up.sql"):
			ups = append(ups, strings.TrimSuffix(name, ".up.sql"))
		}
	}
	if len(ups) == 0 {
		t.Fatal("tidak ada migrasi .up.sql terbaca; penjaga tidak bekerja")
	}

	// Ambang migrasi wajib-berpasangan: sejak 000099. Migrasi dengan nomor lebih kecil
	// dirilis sebelum kebijakan ini dan tidak dituntut.
	const wajibSejak = "000099"

	sort.Strings(ups)
	var tanpa []string
	for _, base := range ups {
		nomor := strings.SplitN(base, "_", 2)[0]
		if nomor < wajibSejak {
			continue
		}
		if !down[base] {
			tanpa = append(tanpa, base)
		}
	}
	if len(tanpa) > 0 {
		t.Fatalf("migrasi sejak %s wajib punya pasangan .down.sql, tetapi tidak ada:\n  %s",
			wajibSejak, strings.Join(tanpa, "\n  "))
	}
}
