package ojkreport

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// sindikasiItem membangun satu baris register kredit sindikasi untuk uji perakit Form 06.02.
func sindikasiItem(noRekening, perjanjian string) domain.SindikasiItem {
	return domain.SindikasiItem{
		CounterpartyID:             "CP-1",
		NoRekening:                 noRekening,
		JumlahPendanaanSindikasi:   decimal.NewFromInt(1000000000),
		BagianPendanaan:            decimal.NewFromInt(200000000),
		SandiBankPeserta:           "123456",
		Plafon:                     decimal.NewFromInt(500000000),
		BakiDebet:                  decimal.NewFromInt(450000000),
		StatusKepesertaanCode:      domain.SindikasiKepesertaanAnggota,
		NomorPerjanjianInduk:       perjanjian,
		PendanaanDiBankPelaporCode: domain.SindikasiPendanaanYa,
		KualitasCode:               domain.SindikasiKualitasLancar,
		TunggakanPokok:             decimal.Zero,
		TunggakanBunga:             decimal.Zero,
		HariTunggakanPokok:         0,
		HariTunggakanBunga:         0,
		AsOf:                       time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
		Status:                     domain.SindikasiStatusAktif,
	}
}

// TestBuildForm06_02DaftarTanpaJumlah memastikan form berisi 13 kolom logis (16 sel karena
// kolom VII dan XII/XIII bersub-kolom), satu baris per rekening, dan TIDAK ada baris JUMLAH.
func TestBuildForm06_02DaftarTanpaJumlah(t *testing.T) {
	rows := []domain.SindikasiItem{sindikasiItem("001", "ABC-1"), sindikasiItem("002", "ABC-2")}
	sec := BuildForm06_02(rows, ReportingOffice{Sandi: "1001"})
	if sec.Form != "06.02" {
		t.Fatalf("form = %q, ingin 06.02", sec.Form)
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("ingin 2 baris, dapat %d", len(sec.Rows))
	}
	for _, row := range sec.Rows {
		if strings.EqualFold(row.Key, "JUMLAH") {
			t.Fatal("Form 06.02 tidak boleh memiliki baris JUMLAH")
		}
		if row.Reason != "" {
			t.Fatalf("baris lengkap tidak boleh beralasan: %q", row.Reason)
		}
	}
	// Kolom I dipasang paling kiri karena kantor pelapor tunggal tersedia.
	if sec.Columns[0].Sandi != "I" {
		t.Fatalf("kolom pertama = %q, ingin I", sec.Columns[0].Sandi)
	}
	for _, u := range sec.Unavailable {
		t.Fatalf("tidak boleh ada kolom tidak tersedia saat kantor tersedia: %+v", u)
	}
}

// TestBuildForm06_02KolomIdentitasSelaluKosongBeralasan memastikan kolom III No. Identitas
// selalu ditulis "-" (keputusan privasi), bukan dikarang.
func TestBuildForm06_02KolomIdentitasSelaluKosongBeralasan(t *testing.T) {
	sec := BuildForm06_02([]domain.SindikasiItem{sindikasiItem("001", "ABC-1")}, ReportingOffice{Sandi: "1001"})
	// Setelah kolom I disisipkan, urutan sel: I(0), II(1), III(2).
	cell := sec.Rows[0].Cells[2]
	if cell.Sandi != "III" || cell.Value != "-" {
		t.Fatalf("kolom III = %q %q, ingin III -", cell.Sandi, cell.Value)
	}
	joined := strings.Join(sec.Notes, " ")
	if !strings.Contains(joined, "No. Identitas") || !strings.Contains(joined, "tidak disimpan") {
		t.Fatal("catatan harus menjelaskan kolom III sengaja tidak disimpan")
	}
}

// TestBuildForm06_02NoRekeningKosongSahBilaPendanaanTidak memastikan kolom IV "-" TANPA
// alasan "tidak tersedia" ketika pendanaan bukan di bank pelapor (X = sandi 2), sesuai
// penjelasan PDF #page 177.
func TestBuildForm06_02NoRekeningKosongSahBilaPendanaanTidak(t *testing.T) {
	item := sindikasiItem("", "ABC-1")
	item.PendanaanDiBankPelaporCode = domain.SindikasiPendanaanTidak
	sec := BuildForm06_02([]domain.SindikasiItem{item}, ReportingOffice{Sandi: "1001"})
	row := sec.Rows[0]
	// Kolom IV adalah indeks 3 setelah kolom I disisipkan (I, II, III, IV).
	if row.Cells[3].Sandi != "IV" || row.Cells[3].Value != "-" {
		t.Fatalf("kolom IV = %q %q, ingin IV -", row.Cells[3].Sandi, row.Cells[3].Value)
	}
	if row.Reason != "" {
		t.Fatalf("kekosongan No. Rekening sah menurut form, tidak boleh beralasan: %q", row.Reason)
	}
	if !strings.HasPrefix(row.Key, "perjanjian:") {
		t.Fatalf("kunci baris tanpa rekening harus memakai nomor perjanjian, dapat %q", row.Key)
	}
}

// TestBuildForm06_02SandiDiterjemahkanSebagaiLabel memastikan sandi VIII/X/XI ditulis
// sebagai label baku, bukan angka mentah yang membingungkan pembaca berkas.
func TestBuildForm06_02SandiDiterjemahkanSebagaiLabel(t *testing.T) {
	item := sindikasiItem("001", "ABC-1")
	item.StatusKepesertaanCode = domain.SindikasiKepesertaanArranger
	item.KualitasCode = domain.SindikasiKualitasMacet
	sec := BuildForm06_02([]domain.SindikasiItem{item}, ReportingOffice{Sandi: "1001"})
	row := sec.Rows[0]
	want := map[string]string{"VIII": "1 - Arranger", "XI": "5 - Macet"}
	got := map[string]string{}
	for _, c := range row.Cells {
		switch c.Sandi {
		case "VIII", "XI":
			got[c.Sandi] = c.Value
		}
	}
	for sandi, w := range want {
		if got[sandi] != w {
			t.Fatalf("kolom %s = %q, ingin %q", sandi, got[sandi], w)
		}
	}
}

// TestBuildForm06_02KantorTidakTersediaTetapJujur memastikan bila kantor pelapor tidak
// dapat dipilih, kolom I didaftarkan belum tersedia beralasan, bukan dikarang.
func TestBuildForm06_02KantorTidakTersediaTetapJujur(t *testing.T) {
	sec := BuildForm06_02([]domain.SindikasiItem{sindikasiItem("001", "ABC-1")},
		ReportingOffice{Reason: "kantor pelapor tidak dapat ditentukan"})
	if len(sec.Unavailable) != 1 || sec.Unavailable[0].Sandi != "I" {
		t.Fatalf("ingin tepat satu kolom tidak tersedia (I), dapat %+v", sec.Unavailable)
	}
}
