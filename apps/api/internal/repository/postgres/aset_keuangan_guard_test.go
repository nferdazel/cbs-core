package postgres

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Penjaga No. Rekening unik Form 18.00. Aturan form (PDF #page 237) mewajibkan nomor
// rekening "tidak boleh sama" untuk setiap rekening. Jaminan itu bertumpu pada dua hal
// yang tidak boleh hilang tanpa disadari:
//   - migrasi 000121 memberi UNIQUE penuh pada kolom no_rekening, sehingga baris NONAKTIF
//     tetap memegang nomornya; dan
//   - penghapusan register adalah soft-delete (UPDATE status = NONAKTIF), bukan DELETE
//     fisik yang akan membebaskan nomornya.
//
// Uji ini membaca berkas sumber, bukan basis data, sehingga tetap berjalan di CI tanpa
// PostgreSQL.
func TestAsetKeuanganNoRekeningUnikTerjagaDiSumber(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal menemukan lokasi uji")
	}
	dir := filepath.Dir(thisFile)

	migrasi := filepath.Join(dir, "..", "..", "..", "..", "..",
		"packages", "db-migrations", "000121_aset_keuangan_lainnya_register.up.sql")
	isiMigrasi, err := os.ReadFile(migrasi)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", migrasi, err)
	}
	if !strings.Contains(string(isiMigrasi), "UNIQUE (no_rekening)") {
		t.Fatalf("migrasi 000121 harus memberi UNIQUE penuh pada no_rekening agar nomor tidak dapat dipakai ulang")
	}

	repo, err := os.ReadFile(filepath.Join(dir, "aset_keuangan_repo.go"))
	if err != nil {
		t.Fatalf("membaca aset_keuangan_repo.go: %v", err)
	}
	isiRepo := string(repo)
	if !strings.Contains(isiRepo, "SET status = 'NONAKTIF'") {
		t.Fatal("penghapusan register aset keuangan harus soft-delete (SET status = 'NONAKTIF')")
	}
	if strings.Contains(isiRepo, "DELETE FROM aset_keuangan_lainnya_register") {
		t.Fatal("penghapusan fisik (DELETE FROM aset_keuangan_lainnya_register) dilarang: nomor rekening akan dapat dipakai ulang")
	}
	if !strings.Contains(isiRepo, "status = 'AKTIF'") {
		t.Fatal("sumber Form 18.00 harus menyaring status = 'AKTIF' agar baris NONAKTIF tidak muncul di form")
	}
}
