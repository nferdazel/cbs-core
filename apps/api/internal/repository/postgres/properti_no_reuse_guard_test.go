package postgres

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Penjaga no reuse/no recycle No. Register Form 17.00. Aturan OJK (PDF #page 231)
// mewajibkan nomor register unik dan tidak boleh dipakai ulang. Jaminan itu bertumpu
// pada dua hal yang tidak boleh hilang tanpa disadari:
//   - migrasi 000118 memberi UNIQUE penuh pada kolom no_register, sehingga baris
//     NONAKTIF tetap memegang nomornya; dan
//   - penghapusan register adalah soft-delete (UPDATE status = NONAKTIF), bukan DELETE
//     fisik yang akan membebaskan nomor.
//
// Uji ini membaca berkas sumber, bukan basis data, sehingga tetap berjalan di CI tanpa
// PostgreSQL; uji perilaku sesungguhnya ada di
// service/properti_no_reuse_integration_test.go (di-skip tanpa CBS_TEST_DB_DSN).
func TestPropertiNoReuseTerjagaDiSumber(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal menemukan lokasi uji")
	}
	dir := filepath.Dir(thisFile)

	migrasi := filepath.Join(dir, "..", "..", "..", "..", "..",
		"packages", "db-migrations", "000118_properti_terbengkalai_register.up.sql")
	isiMigrasi, err := os.ReadFile(migrasi)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", migrasi, err)
	}
	if !strings.Contains(string(isiMigrasi), "UNIQUE (no_register)") {
		t.Fatalf("migrasi 000118 harus memberi UNIQUE penuh pada no_register agar nomor tidak dapat dipakai ulang")
	}

	repo, err := os.ReadFile(filepath.Join(dir, "properti_repo.go"))
	if err != nil {
		t.Fatalf("membaca properti_repo.go: %v", err)
	}
	isiRepo := string(repo)
	if !strings.Contains(isiRepo, "SET status = 'NONAKTIF'") {
		t.Fatal("penghapusan register properti harus soft-delete (SET status = 'NONAKTIF')")
	}
	if strings.Contains(isiRepo, "DELETE FROM properti_terbengkalai_register") {
		t.Fatal("penghapusan fisik (DELETE FROM properti_terbengkalai_register) dilarang: nomor register akan dapat dipakai ulang")
	}
	if !strings.Contains(isiRepo, "status = 'AKTIF'") {
		t.Fatal("sumber Form 17.00 harus menyaring status = 'AKTIF' agar baris NONAKTIF tidak muncul di form")
	}
}
