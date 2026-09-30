package ojkreport

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// modalItem membangun satu baris register modal untuk uji perakit Form 00.06.
func modalItem(jenis, jenisModal string, jumlah int64) domain.ModalItem {
	return domain.ModalItem{
		ID:             uuid.New(),
		JenisCode:      jenis,
		JenisModalCode: jenisModal,
		Jumlah:         decimal.NewFromInt(jumlah),
	}
}

// TestBuildForm00_06DenganJumlah memastikan Form 00.06 punya 3 sel kolom logis (II-IV),
// baris JUMLAH menjumlahkan kolom IV, dan kolom II kosong ditulis "-".
func TestBuildForm00_06DenganJumlah(t *testing.T) {
	tanggal := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	item1 := modalItem(domain.ModalJenisDana, domain.ModalJenisModalDisetor, 1000000000)
	item1.TanggalPersetujuan = &tanggal
	items := []domain.ModalItem{
		item1,
		modalItem(domain.ModalJenisTanahBangunanInti, domain.ModalJenisModalSumbangan, 500000000),
	}
	sec := BuildForm00_06(items, ReportingOffice{Sandi: "1001"})

	if sec.Form != "00.06" {
		t.Fatalf("form = %q, ingin 00.06", sec.Form)
	}
	// 2 baris data + 1 baris JUMLAH.
	if len(sec.Rows) != 3 {
		t.Fatalf("ingin 3 baris, dapat %d", len(sec.Rows))
	}
	last := sec.Rows[len(sec.Rows)-1]
	if last.Key != form00_06TotalKey {
		t.Fatalf("baris terakhir = %q, ingin JUMLAH", last.Key)
	}
	// Kolom I dipasang paling kiri, jadi sel = I, II, III, IV (4 sel).
	if len(last.Cells) != 4 {
		t.Fatalf("baris JUMLAH punya %d sel, ingin 4 (I-IV)", len(last.Cells))
	}
	if got := last.Cells[3].Value; got != FormatRupiah(decimal.NewFromInt(1500000000)) {
		t.Fatalf("JUMLAH kolom IV = %q, ingin 1.500.000.000", got)
	}
	if last.Cells[1].Value != "-" || last.Cells[2].Value != "JUMLAH" {
		t.Fatalf("baris JUMLAH kolom II/III salah: %+v", last.Cells)
	}
	// Baris pertama: kolom II tanggal terisi.
	if got := sec.Rows[0].Cells[1].Value; got != "2026-03-15" {
		t.Fatalf("kolom II baris 1 = %q, ingin 2026-03-15", got)
	}
	// Baris kedua: tanggal kosong -> "-".
	if got := sec.Rows[1].Cells[1].Value; got != "-" {
		t.Fatalf("kolom II baris 2 = %q, ingin -", got)
	}
}

// TestBuildForm00_06SandiLabel memastikan sandi I dan III ditulis sebagai label baku dan
// sandi tak dikenal tidak diterka.
func TestBuildForm00_06SandiLabel(t *testing.T) {
	cases := []struct {
		jenis, jenisModal string
		wantJenis         string
		wantJenisModal    string
	}{
		{domain.ModalJenisDana, domain.ModalJenisDanaSetoranEkuitas,
			"01 - Dana", "03 - Dana Setoran Modal - Ekuitas"},
		{domain.ModalJenisTanahBangunanNonInti, domain.ModalJenisModalSumbangan,
			"03 - Tanah/Bangunan (Bukan Modal Inti)", "02 - Modal Sumbangan"},
		{"77", "99", "77", "99"},
	}
	for _, c := range cases {
		sec := BuildForm00_06([]domain.ModalItem{modalItem(c.jenis, c.jenisModal, 100)},
			ReportingOffice{Reason: "tanpa kantor (uji)"})
		sec0 := sec
		_ = sec0
		// Tanpa kolom I: sel 0=II, 1=III, 2=IV. Jenis (I) tampil di Key.
		row := sec.Rows[0]
		if !strings.Contains(row.Key, c.wantJenis) {
			t.Fatalf("key %q tidak memuat jenis %q", row.Key, c.wantJenis)
		}
		if row.Cells[1].Value != c.wantJenisModal {
			t.Fatalf("jenis modal %q = %q, ingin %q", c.jenisModal, row.Cells[1].Value, c.wantJenisModal)
		}
	}
}

// TestBuildForm00_06KosongTanpaBarisJUMLAHBervalue memastikan tanpa baris data, baris
// JUMLAH tetap ada tetapi kolom IV "-" (tidak menulis nol yang menyesatkan).
func TestBuildForm00_06Kosong(t *testing.T) {
	sec := BuildForm00_06(nil, ReportingOffice{Sandi: "1001"})
	if len(sec.Rows) != 1 {
		t.Fatalf("tanpa data harus tetap 1 baris JUMLAH, dapat %d", len(sec.Rows))
	}
	if got := sec.Rows[0].Cells[3].Value; got != "-" {
		t.Fatalf("JUMLAH tanpa data = %q, ingin -", got)
	}
}
