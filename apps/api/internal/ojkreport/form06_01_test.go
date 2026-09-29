package ojkreport

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// agunanRow membangun satu baris agunan lengkap untuk uji perakit Form 06.01.
func agunanRow(kode string) domain.AgunanRow {
	return domain.AgunanRow{
		ID:               uuid.New(),
		LoanNumber:       "LN-001",
		KodeRegister:     kode,
		JenisAgunanCode:  "202",
		AlamatAgunan:     "Jl. Merdeka 1",
		NilaiDiagunkan:   decimal.NewFromInt(500000000),
		NilaiAgunan:      decimal.NewFromInt(600000000),
		PenilaiCode:      domain.AgunanPenilaiIndependen,
		TanggalPenilaian: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		PPKAAmount:       decimal.NewFromInt(350000000),
		Status:           "ACTIVE",
	}
}

// TestBuildForm06_01PunyaBarisJumlah memastikan Form 06.01 memiliki baris JUMLAH yang
// menjumlahkan kolom VI, VII Nilai Agunan, dan VIII (PDF #169).
func TestBuildForm06_01PunyaBarisJumlah(t *testing.T) {
	rows := []domain.AgunanRow{agunanRow("AG-1"), agunanRow("AG-2")}
	sec := BuildForm06_01(rows, ReportingOffice{Sandi: "1001"})
	if sec.Form != "06.01" {
		t.Fatalf("form = %q, ingin 06.01", sec.Form)
	}
	// 2 baris agunan + 1 baris JUMLAH.
	if len(sec.Rows) != 3 {
		t.Fatalf("ingin 3 baris (2 agunan + JUMLAH), dapat %d", len(sec.Rows))
	}
	total := sec.Rows[2]
	if !strings.EqualFold(total.Key, form06_01TotalKey) {
		t.Fatalf("baris terakhir harus JUMLAH, dapat %q", total.Key)
	}
	if total.Reason != "" {
		t.Fatalf("JUMLAH baris lengkap tidak boleh beralasan: %q", total.Reason)
	}
	// Kolom semua terisi (sandi I disisipkan paling kiri): VI index 5, VII index 6, VIII index 9.
	if total.Cells[5].Value != "1000000000" {
		t.Errorf("JUMLAH Nilai Diagunkan = %q, ingin 1000000000", total.Cells[5].Value)
	}
	if total.Cells[9].Value != "700000000" {
		t.Errorf("JUMLAH PPKA = %q, ingin 700000000", total.Cells[9].Value)
	}
}

// TestBuildForm06_01JumlahTidakSebagian memastikan JUMLAH tidak dihitung sebagian bila ada
// baris yang kolomnya belum diisi bank — angka sebagian akan menyesatkan.
func TestBuildForm06_01JumlahTidakSebagian(t *testing.T) {
	kosong := agunanRow("")
	kosong.KodeRegister = ""
	sec := BuildForm06_01([]domain.AgunanRow{agunanRow("AG-1"), kosong}, ReportingOffice{Sandi: "1001"})
	total := sec.Rows[len(sec.Rows)-1]
	if total.Reason == "" {
		t.Fatal("JUMLAH harus beralasan bila ada baris belum lengkap")
	}
	if total.Cells[5].Value != "-" || total.Cells[9].Value != "-" {
		t.Fatalf("JUMLAH tidak boleh diisi sebagian: VI=%q VIII=%q", total.Cells[5].Value, total.Cells[9].Value)
	}
}

// TestBuildForm06_01PenilaiDiterjemahkanSebagaiLabel memastikan sandi VII.b ditulis label baku.
func TestBuildForm06_01PenilaiDiterjemahkanSebagaiLabel(t *testing.T) {
	sec := BuildForm06_01([]domain.AgunanRow{agunanRow("AG-1")}, ReportingOffice{Sandi: "1001"})
	// I(0) II(1) III(2) IV(3) V(4) VI(5) VII-nilai(6) VII-penilai(7)
	if got := sec.Rows[0].Cells[7].Value; got != "1 - Penilai Independen" {
		t.Fatalf("kolom VII Penilai = %q, ingin '1 - Penilai Independen'", got)
	}
}

// TestBuildForm06_01KantorTidakTersediaTetapJujur memastikan kolom I didaftarkan belum
// tersedia bila kantor pelapor tidak dapat dipilih, bukan dikarang.
func TestBuildForm06_01KantorTidakTersediaTetapJujur(t *testing.T) {
	sec := BuildForm06_01([]domain.AgunanRow{agunanRow("AG-1")},
		ReportingOffice{Reason: "kantor pelapor tidak dapat ditentukan"})
	if len(sec.Unavailable) != 1 || sec.Unavailable[0].Sandi != "I" {
		t.Fatalf("ingin tepat satu kolom tidak tersedia (I), dapat %+v", sec.Unavailable)
	}
}

// TestBuildForm06_01CatatanLikuidNonLikuid memastikan catatan menegaskan Likuid/Non Likuid
// adalah kategori dalam kolom IV, bukan kolom tersendiri (keputusan SME 29 Sep 2026).
func TestBuildForm06_01CatatanLikuidNonLikuid(t *testing.T) {
	sec := BuildForm06_01(nil, ReportingOffice{Sandi: "1001"})
	joined := strings.Join(sec.Notes, " ")
	if !strings.Contains(joined, "Likuid") || !strings.Contains(joined, "bukan kolom tersendiri") {
		t.Fatalf("catatan harus menjelaskan Likuid/Non Likuid bukan kolom tersendiri: %s", joined)
	}
	for _, c := range sec.Columns {
		if strings.Contains(strings.ToLower(c.Nama), "likuid") && !strings.Contains(c.Nama, "Jenis") {
			t.Fatalf("Likuid tidak boleh menjadi kolom tersendiri: %+v", c)
		}
	}
}
