package ojkreport

import (
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// pihakTerkaitItem membangun satu baris register pihak terkait untuk uji perakit Form 00.05.
func pihakTerkaitItem(nama, jenis, hubungan string) domain.PihakTerkaitItem {
	return domain.PihakTerkaitItem{
		ID:           uuid.New(),
		Nama:         nama,
		Alamat:       "Jl. Contoh No. 1",
		JenisCode:    jenis,
		HubunganCode: hubungan,
	}
}

// TestBuildForm00_05TanpaJumlah memastikan Form 00.05 punya 5 kolom logis, satu baris per
// pihak terkait, TANPA baris JUMLAH, dan kolom II No. Identitas selalu "-" (privasi).
func TestBuildForm00_05TanpaJumlah(t *testing.T) {
	rows := []domain.PihakTerkaitItem{
		pihakTerkaitItem("Budi", domain.PihakTerkaitJenisPerorangan, domain.PihakTerkaitHubunganPengendaliKeluarga),
		pihakTerkaitItem("PT Contoh", domain.PihakTerkaitJenisBadan, domain.PihakTerkaitHubunganPerusahaanBukanBank),
	}
	sec := BuildForm00_05(rows, ReportingOffice{Sandi: "1001"})

	if sec.Form != "00.05" {
		t.Fatalf("form = %q, ingin 00.05", sec.Form)
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("ingin 2 baris, dapat %d", len(sec.Rows))
	}
	// Kolom I dipasang paling kiri karena kantor pelapor tunggal tersedia, jadi baris
	// berisi 6 sel (I..V).
	if sec.Columns[0].Sandi != "I" {
		t.Fatalf("kolom pertama = %q, ingin I", sec.Columns[0].Sandi)
	}
	for _, row := range sec.Rows {
		if strings.EqualFold(row.Key, "JUMLAH") {
			t.Fatal("Form 00.05 tidak boleh memiliki baris JUMLAH")
		}
		if len(row.Cells) != 5 {
			t.Fatalf("baris %q punya %d sel, ingin 5 (I-V)", row.Key, len(row.Cells))
		}
		if row.Cells[1].Nama != "No. Identitas" || row.Cells[1].Value != "-" {
			t.Fatalf("kolom II harus selalu \"-\" (privasi), dapat %q", row.Cells[1].Value)
		}
	}
	for _, u := range sec.Unavailable {
		t.Fatalf("tidak boleh ada kolom tidak tersedia saat kantor tersedia: %+v", u)
	}
}

// TestBuildForm00_05SandiLabel memastikan sandi IV dan V ditulis sebagai label baku dan
// sandi tak dikenal tidak diterka.
func TestBuildForm00_05SandiLabel(t *testing.T) {
	cases := []struct {
		jenis, hubungan string
		wantJenis       string
		wantHubungan    string
	}{
		{domain.PihakTerkaitJenisPemerintah, domain.PihakTerkaitHubunganPeminjamDijamin,
			"03 - Pemerintah Daerah atau Pemerintah Pusat", "06 - Peminjam yang Dijamin Pengurus"},
		{domain.PihakTerkaitJenisBadan, domain.PihakTerkaitHubunganBPRRangkapKomisaris,
			"02 - Perusahaan atau Badan", "04 - BPR/BPRS Lain dengan Rangkap Komisaris"},
		{"77", "99", "77", "99"},
	}
	for _, c := range cases {
		// Kantor tanpa sandi diberi alasan agar kolom I tidak dipasang: sel 0=II, 1=III,
		// 2=IV, 3=V.
		sec := BuildForm00_05([]domain.PihakTerkaitItem{pihakTerkaitItem("X", c.jenis, c.hubungan)},
			ReportingOffice{Reason: "tanpa kantor pelapor (uji)"})
		row := sec.Rows[0]
		if row.Cells[2].Value != c.wantJenis {
			t.Fatalf("jenis %q = %q, ingin %q", c.jenis, row.Cells[2].Value, c.wantJenis)
		}
		if row.Cells[3].Value != c.wantHubungan {
			t.Fatalf("hubungan %q = %q, ingin %q", c.hubungan, row.Cells[3].Value, c.wantHubungan)
		}
	}
}

// TestBuildForm00_05KolomSandiKantorTidakTersedia memastikan kolom I dinyatakan tidak
// tersedia ketika kantor pelapor belum dapat dipilih, bukan dikarang.
func TestBuildForm00_05KolomSandiKantorTidakTersedia(t *testing.T) {
	sec := BuildForm00_05(
		[]domain.PihakTerkaitItem{pihakTerkaitItem("X", domain.PihakTerkaitJenisPerorangan, domain.PihakTerkaitHubunganPengendaliKeluarga)},
		ReportingOffice{Reason: "belum ada kantor pelapor"})
	found := false
	for _, u := range sec.Unavailable {
		if u.Sandi == "I" {
			found = true
		}
	}
	if !found {
		t.Fatal("kolom I harus terdaftar tidak tersedia saat kantor belum dapat dipilih")
	}
	if len(sec.Rows[0].Cells) != 4 {
		t.Fatalf("baris tanpa kolom I harus 4 sel (II-V), dapat %d", len(sec.Rows[0].Cells))
	}
}
