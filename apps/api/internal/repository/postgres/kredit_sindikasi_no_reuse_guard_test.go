package postgres

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Penjaga no reuse/no recycle No. Rekening Form 06.02. Aturan OJK (PDF #page 177)
// mewajibkan No. Rekening "unik dan tidak boleh sama", dan penomoran register pada
// kredit sindikasi mengikuti kaidah no reuse/no recycle yang sama dengan register
// lain. Jaminan itu bertumpu pada dua hal yang tidak boleh hilang tanpa disadari:
//   - migrasi 000124 memberi UNIQUE penuh pada kolom no_rekening, sehingga baris
//     NONAKTIF tetap memegang nomornya; dan
//   - penghapusan register adalah soft-delete (UPDATE status = NONAKTIF), bukan DELETE
//     fisik yang akan membebaskan nomor.
//
// Uji ini membaca berkas sumber, bukan basis data, sehingga tetap berjalan di CI tanpa
// PostgreSQL. Register ber-nomor-unik lain (penyertaan, properti, aset keuangan) sudah
// punya penjaga serupa; kredit sindikasi ditambahkan agar aturan no-reuse tidak bisa
// dilanggar tanpa tertangkap.
func TestKreditSindikasiNoReuseTerjagaDiSumber(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal menemukan lokasi uji")
	}
	dir := filepath.Dir(thisFile)

	migrasi := filepath.Join(dir, "..", "..", "..", "..", "..",
		"packages", "db-migrations", "000124_kredit_sindikasi_register.up.sql")
	isiMigrasi, err := os.ReadFile(migrasi)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", migrasi, err)
	}
	if !strings.Contains(string(isiMigrasi), "CONSTRAINT kredit_sindikasi_no_rekening_unique UNIQUE (no_rekening)") {
		t.Fatalf("migrasi 000124 harus memberi UNIQUE penuh pada no_rekening agar nomor tidak dapat dipakai ulang")
	}

	repo, err := os.ReadFile(filepath.Join(dir, "kredit_sindikasi_repo.go"))
	if err != nil {
		t.Fatalf("membaca kredit_sindikasi_repo.go: %v", err)
	}
	isiRepo := string(repo)
	if !strings.Contains(isiRepo, "SET status = 'NONAKTIF'") {
		t.Fatal("penghapusan register kredit sindikasi harus soft-delete (SET status = 'NONAKTIF')")
	}
	if strings.Contains(isiRepo, "DELETE FROM kredit_sindikasi_register") {
		t.Fatal("penghapusan fisik (DELETE FROM kredit_sindikasi_register) dilarang: nomor rekening akan dapat dipakai ulang")
	}
	if !strings.Contains(isiRepo, "status = 'AKTIF'") {
		t.Fatal("sumber Form 06.02 harus menyaring status = 'AKTIF' agar baris NONAKTIF tidak muncul di form")
	}
}
