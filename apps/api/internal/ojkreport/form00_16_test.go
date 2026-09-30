package ojkreport

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// pihakLawanItem membangun satu baris register pihak lawan untuk uji perakit Form 00.16.
func pihakLawanItem() domain.PihakLawanItem {
	return domain.PihakLawanItem{
		ID:           uuid.New(),
		PihakLawanID: "CP-1",
		Nama:         "PT Contoh",
	}
}

// TestBuildForm00_16TanpaJumlahDanPrivasi memastikan Form 00.16 punya 19 sel kolom (II-XX),
// TANPA baris JUMLAH, dan kolom III Nomor Identitas + VI NPWP selalu "-" (privasi).
func TestBuildForm00_16TanpaJumlahDanPrivasi(t *testing.T) {
	sec := BuildForm00_16([]domain.PihakLawanItem{pihakLawanItem()}, ReportingOffice{Sandi: "1001"})

	if sec.Form != "00.16" {
		t.Fatalf("form = %q, ingin 00.16", sec.Form)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("ingin 1 baris, dapat %d", len(sec.Rows))
	}
	row := sec.Rows[0]
	if strings.EqualFold(row.Key, "JUMLAH") {
		t.Fatal("Form 00.16 tidak boleh memiliki baris JUMLAH")
	}
	// Kolom I dipasang paling kiri, jadi 20 sel (I + II..XX).
	if len(row.Cells) != 20 {
		t.Fatalf("baris punya %d sel, ingin 20 (I-XX)", len(row.Cells))
	}
	if row.Cells[2].Nama != "Nomor Identitas" || row.Cells[2].Value != "-" {
		t.Fatalf("kolom III harus selalu \"-\" (privasi), dapat %q", row.Cells[2].Value)
	}
	if row.Cells[5].Nama != "NPWP" || row.Cells[5].Value != "-" {
		t.Fatalf("kolom VI harus selalu \"-\" (privasi), dapat %q", row.Cells[5].Value)
	}
}

// TestBuildForm00_16SandiLabel memastikan sandi II, IV, IX, X ditulis sebagai label baku,
// dan tanggal kosong ditulis "-".
func TestBuildForm00_16SandiLabel(t *testing.T) {
	item := pihakLawanItem()
	item.JenisIdentitasCode = domain.PihakLawanIdentitasKTP
	item.JenisKelaminCode = domain.PihakLawanKelaminPerempuan
	item.JenisUsahaCode = domain.PihakLawanUsahaSyariah
	item.HubunganBankCode = domain.PihakLawanHubunganTerkait
	sec := BuildForm00_16([]domain.PihakLawanItem{item},
		ReportingOffice{Reason: "tanpa kantor (uji)"})
	// Tanpa kolom I: sel 0=II, 1=III, 2=IV, 3=V, 4=VI, 5=VII, 6=VIII, 7=IX, 8=X.
	row := sec.Rows[0]
	if row.Cells[0].Value != "1 - Kartu Tanda Penduduk" {
		t.Fatalf("jenis identitas = %q", row.Cells[0].Value)
	}
	if row.Cells[2].Value != "2 - Perempuan" {
		t.Fatalf("jenis kelamin = %q", row.Cells[2].Value)
	}
	if row.Cells[7].Value != "2 - Syariah" {
		t.Fatalf("jenis usaha = %q", row.Cells[7].Value)
	}
	if row.Cells[8].Value != "12 - Pihak Terkait" {
		t.Fatalf("hubungan = %q", row.Cells[8].Value)
	}
	// XIV Tanggal Pemeringkatan = sel index 12; kosong -> "-".
	if row.Cells[12].Value != "-" {
		t.Fatalf("tanggal pemeringkatan kosong = %q, ingin -", row.Cells[12].Value)
	}
}

// TestBuildForm00_16TanggalTerisi memastikan tanggal pemeringkatan dilaporkan apa adanya.
func TestBuildForm00_16TanggalTerisi(t *testing.T) {
	item := pihakLawanItem()
	ts := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	item.TanggalPemeringkatan = &ts
	sec := BuildForm00_16([]domain.PihakLawanItem{item},
		ReportingOffice{Reason: "tanpa kantor (uji)"})
	if got := sec.Rows[0].Cells[12].Value; got != "2026-05-01" {
		t.Fatalf("tanggal pemeringkatan = %q, ingin 2026-05-01", got)
	}
}
