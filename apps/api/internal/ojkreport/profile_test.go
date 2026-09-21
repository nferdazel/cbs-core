package ojkreport

import (
	"strings"
	"testing"
)

// Profil bank yang belum diisi membuat Form 00.00 belum dapat dibangun.
func TestBuildForm00BelumDikonfigurasi(t *testing.T) {
	if _, ok := buildForm00(&BankProfileConfig{}); ok {
		t.Fatal("profil tanpa nama bank harus dinyatakan belum tersedia")
	}
	if _, ok := buildForm00(nil); ok {
		t.Fatal("profil nil harus dinyatakan belum tersedia")
	}
}

// Field yang tersedia diisi; field yang kosong menunjuk kunci konfigurasi yang harus
// diisi bank, bukan diisi nilai karangan.
func TestBuildForm00MengisiNilaiDanMenandaiKunci(t *testing.T) {
	cfg := &BankProfileConfig{
		Name:       "BPR Uji",
		Address:    "Jl. Uji 1",
		City:       "Bandung",
		Phone:      "022-123",
		NPWP:       "01.234.567.8-901.000",
		Configured: true,
	}
	sec, ok := buildForm00(cfg)
	if !ok {
		t.Fatal("profil terkonfigurasi harus dapat dibangun")
	}

	row := rowByKey(t, sec, "1. Nama BPR")
	if len(row.Cells) != 1 || row.Cells[0].Value != "BPR Uji" {
		t.Fatalf("baris nama BPR = %+v", row)
	}

	// Email kosong: barisnya tidak tersedia dan menyebut kunci konfigurasi.
	email := rowByKey(t, sec, "6. E-mail")
	if email.Reason == "" {
		t.Fatal("email kosong harus dinyatakan belum dikonfigurasi")
	}
	if !strings.Contains(email.Reason, OJKBankEmailKey) {
		t.Fatalf("alasan email harus menyebut kunci %s, dapat %q", OJKBankEmailKey, email.Reason)
	}

	// Field tanpa sumber sama sekali tetap didaftarkan dengan alasan.
	if r := rowByKey(t, sec, "12. Informasi Audit Laporan Keuangan Tahunan (KAP/AP)"); r.Reason == "" {
		t.Fatal("informasi audit harus dinyatakan belum tersedia")
	}
}
