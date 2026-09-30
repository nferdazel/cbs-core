package ojkreport

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// hapusBukuItem membangun satu baris register hapus buku untuk uji perakit Form 15.00.
func hapusBukuItem(jenisAset, hubungan string, pokok int64) domain.HapusBukuItem {
	return domain.HapusBukuItem{
		ID:                  uuid.New(),
		JenisAsetCode:       jenisAset,
		HubunganBankCode:    hubungan,
		TanggalHapusBuku:    time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC),
		SaldoPokokSaatHapus: decimal.NewFromInt(pokok),
	}
}

// TestBuildForm15_00DenganJumlah memastikan Form 15.00 punya 16 sel kolom, baris JUMLAH
// menjumlahkan kolom nominal, dan kolom teks pada baris JUMLAH "-".
func TestBuildForm15_00DenganJumlah(t *testing.T) {
	item1 := hapusBukuItem(domain.HapusBukuJenisAsetKredit, domain.HapusBukuHubunganTidakTerkait, 1000000)
	item1.NomorRekening = "KRD-001"
	item1.BungaSaatHapus = decimal.NewFromInt(250000)
	item1.AgunanNilai = decimal.NewFromInt(5000000)
	items := []domain.HapusBukuItem{
		item1,
		hapusBukuItem(domain.HapusBukuJenisAsetPenempatan, domain.HapusBukuHubunganTerkaitLain, 2000000),
	}
	sec := BuildForm15_00(items, ReportingOffice{Sandi: "1001"})

	if sec.Form != "15.00" {
		t.Fatalf("form = %q, ingin 15.00", sec.Form)
	}
	// 2 baris data + 1 baris JUMLAH.
	if len(sec.Rows) != 3 {
		t.Fatalf("ingin 3 baris, dapat %d", len(sec.Rows))
	}
	last := sec.Rows[len(sec.Rows)-1]
	if last.Key != form15_00TotalKey {
		t.Fatalf("baris terakhir = %q, ingin JUMLAH", last.Key)
	}
	// Kolom I dipasang paling kiri, jadi 17 sel (I + 16 kolom).
	if len(last.Cells) != 17 {
		t.Fatalf("baris JUMLAH punya %d sel, ingin 17", len(last.Cells))
	}
	// Cell[7] = VIII.a (kolom I di index 0, lalu II..VII di 1..6).
	if got := last.Cells[7].Value; got != FormatRupiah(decimal.NewFromInt(3000000)) {
		t.Fatalf("JUMLAH VIII.a = %q, ingin 3.000.000", got)
	}
	if last.Cells[2].Value != "JUMLAH" {
		t.Fatalf("label JUMLAH salah: %q", last.Cells[2].Value)
	}
	if last.Cells[1].Value != "-" {
		t.Fatalf("kolom II baris JUMLAH harus -, dapat %q", last.Cells[1].Value)
	}
}

// TestBuildForm15_00SandiLabel memastikan sandi IV dan VI ditulis sebagai label baku.
func TestBuildForm15_00SandiLabel(t *testing.T) {
	sec := BuildForm15_00([]domain.HapusBukuItem{
		{JenisAsetCode: "10", HubunganBankCode: "11", TanggalHapusBuku: time.Now()},
	}, ReportingOffice{Reason: "tanpa kantor (uji)"})
	// Tanpa kolom I: sel 0=II, 1=III, 2=IV, ... 5=VII.
	row := sec.Rows[0]
	if row.Cells[2].Value != "10 - Kredit yang Diberikan" {
		t.Fatalf("jenis aset = %q", row.Cells[2].Value)
	}
	if row.Cells[4].Value != "11 - Terkait Dalam Rangka Kesejahteraan" {
		t.Fatalf("hubungan = %q", row.Cells[4].Value)
	}
}

// TestBuildForm15_00Kosong memastikan tanpa data baris JUMLAH ada dengan kolom nominal "-".
func TestBuildForm15_00Kosong(t *testing.T) {
	sec := BuildForm15_00(nil, ReportingOffice{Sandi: "1001"})
	if len(sec.Rows) != 1 {
		t.Fatalf("tanpa data harus tetap 1 baris JUMLAH, dapat %d", len(sec.Rows))
	}
	if got := sec.Rows[0].Cells[7].Value; got != "-" {
		t.Fatalf("JUMLAH tanpa data = %q, ingin -", got)
	}
}
