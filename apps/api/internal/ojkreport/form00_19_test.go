package ojkreport

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// Dokumen Form 00.19 memuat identitas bank, jaringan kantor, dan susunan pengurus per
// kategori, serta menandai bagian yang belum dimodelkan (divisi/satuan kerja).
func TestBuildForm00_19DocumentMemuatIsi(t *testing.T) {
	started := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	report := domain.KelembagaanReport{
		AsOf: time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
		Offices: []domain.BankOffice{
			{Code: "002", OfficeType: "KCP", Name: "KCP Bandung", City: "Bandung", Status: "AKTIF"},
			{Code: "001", OfficeType: "KC", Name: "KC Pusat", City: "Jakarta", Status: "AKTIF"},
		},
		Management: []domain.BankManagement{
			{Category: "DIREKSI", Name: "Budi", Position: "Direktur Utama", OJKPositionCode: "01", StartedAt: &started, Status: "AKTIF"},
			{Category: "KOMISARIS", Name: "Siti", Position: "Komisaris Utama", Status: "AKTIF"},
		},
	}

	doc := BuildForm00_19Document(report, "BPR Contoh Sejahtera")

	for _, want := range []string{
		"BPR Contoh Sejahtera",
		"STRUKTUR ORGANISASI BPR",
		"KC Pusat", "KCP Bandung",
		"Direksi", "Komisaris",
		"Budi", "Siti",
		"01-03-2025",
		"Divisi atau Satuan Kerja",
		"Belum dimodelkan",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("dokumen tidak memuat %q", want)
		}
	}
}

// Tanpa identitas bank, dokumen TIDAK mengarang nama bank.
func TestBuildForm00_19DocumentTanpaNamaBank(t *testing.T) {
	doc := BuildForm00_19Document(domain.KelembagaanReport{}, "")
	if !strings.Contains(doc, "PROFIL BANK BELUM DIKONFIGURASI") {
		t.Fatal("dokumen harus menyatakan profil bank belum dikonfigurasi, bukan mengarang nama")
	}
}

// Kantor diurutkan menurut sandi secara deterministik; kelembagaan kosong menghasilkan
// peringatan, bukan dokumen yang tampak lengkap.
func TestBuildForm00_19DocumentDeterministikDanKosong(t *testing.T) {
	report := domain.KelembagaanReport{
		Offices: []domain.BankOffice{
			{Code: "002", Name: "B"},
			{Code: "001", Name: "A"},
		},
	}
	doc := BuildForm00_19Document(report, "BPR X")
	if idxA, idxB := strings.Index(doc, ">A<"), strings.Index(doc, ">B<"); idxA == -1 || idxB == -1 || idxA > idxB {
		t.Fatalf("kantor harus urut menurut sandi; idxA=%d idxB=%d", idxA, idxB)
	}

	kosong := BuildForm00_19Document(domain.KelembagaanReport{}, "BPR X")
	if !strings.Contains(kosong, "Belum ada kantor") || !strings.Contains(kosong, "Belum ada pejabat") {
		t.Fatal("dokumen tanpa data harus menyatakan register kosong")
	}
}

// Nilai yang belum diisi ditulis "-", dan HTML dari data bank di-escape.
func TestBuildForm00_19DocumentEscapeDanDash(t *testing.T) {
	report := domain.KelembagaanReport{
		Offices: []domain.BankOffice{{Code: "", Name: `<script>alert(1)</script>`}},
	}
	doc := BuildForm00_19Document(report, "BPR <Y>")
	if strings.Contains(doc, "<script>alert(1)</script>") {
		t.Fatal("data bank harus di-escape, tidak boleh menyuntik HTML")
	}
	if !strings.Contains(doc, "&lt;script&gt;") {
		t.Fatal("nama kantor harus muncul dalam bentuk ter-escape")
	}
	if !strings.Contains(doc, "BPR &lt;Y&gt;") {
		t.Fatal("nama bank harus di-escape")
	}
	// Sandi kantor kosong -> "-"
	if !strings.Contains(doc, "<td>-</td>") {
		t.Fatal("nilai kosong harus ditulis \"-\"")
	}
}

// Peringatan dari laporan kelembagaan ikut tercetak, tidak disembunyikan.
func TestBuildForm00_19DocumentMeneruskanPeringatan(t *testing.T) {
	report := domain.KelembagaanReport{Warnings: []string{"bagian X belum punya sumber"}}
	doc := BuildForm00_19Document(report, "BPR X")
	if !strings.Contains(doc, "bagian X belum punya sumber") {
		t.Fatal("peringatan laporan kelembagaan harus tercetak di dokumen")
	}
}
