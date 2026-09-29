package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// kasValasOJKRepoStub mengimplementasikan domain.KasValasRegisterRepository untuk
// menguji Form 03.00 lewat RepoSource tanpa basis data.
type kasValasOJKRepoStub struct {
	domain.KasValasRegisterRepository
	rows []domain.KasValasItem
	err  error
}

func (s kasValasOJKRepoStub) ListKasValasForOJK(context.Context, time.Time) ([]domain.KasValasItem, error) {
	return s.rows, s.err
}

var _ domain.KasValasRegisterRepository = kasValasOJKRepoStub{}

func kasValasFormItem(jenis string, nominal int64) domain.KasValasItem {
	return domain.KasValasItem{
		JenisValasCode: jenis,
		Nominal:        decimal.NewFromInt(nominal),
		KursTengah:     decimal.NewFromInt(16_000),
	}
}

// Form 03.00 menampilkan satu baris per jenis valas DITAMBAH baris JUMLAH; kolom V adalah
// turunan III x IV; baris JUMLAH menjumlahkan Nominal dan Nilai Rupiah tetapi menulis "-"
// pada Kurs Tengah (penjumlahan kurs tidak bermakna).
func TestBuildForm03_00JumlahDanNilaiTurunan(t *testing.T) {
	rows := []domain.KasValasItem{
		kasValasFormItem("USD", 1_000),
		kasValasFormItem("SGD", 2_000),
	}
	sec := BuildForm03_00(rows, ReportingOffice{Sandi: "1101"})

	if len(sec.Rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3 (2 jenis + JUMLAH)", len(sec.Rows))
	}
	row := rowByKey(t, sec, "USD")
	for sandi, want := range map[string]string{
		"I": "1101", "II": "USD", "III": "1000.00", "IV": "16000.00",
		"V": "16000000",
	} {
		if got := cellValue(row, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}

	jumlah := rowByKey(t, sec, "JUMLAH")
	if got := cellValue(jumlah, "II"); got != "JUMLAH" {
		t.Fatalf("baris JUMLAH kolom II = %q", got)
	}
	if got := cellValue(jumlah, "III"); got != "3000.00" {
		t.Fatalf("JUMLAH kolom III = %q, ingin 3000.00", got)
	}
	if got := cellValue(jumlah, "IV"); got != "-" {
		t.Fatalf("JUMLAH kolom IV = %q, ingin - (kurs tidak dijumlahkan)", got)
	}
	if got := cellValue(jumlah, "V"); got != "48000000" {
		t.Fatalf("JUMLAH kolom V = %q, ingin 48000000", got)
	}
}

// Bila Nominal/Kurs belum diisi, kolom V ditulis "-" dan JUMLAH tidak dihitung sebagian.
func TestBuildForm03_00NilaiTakTerhitungTidakDikarang(t *testing.T) {
	rows := []domain.KasValasItem{
		kasValasFormItem("USD", 1_000),
		{JenisValasCode: "SGD", Nominal: decimal.NewFromInt(500), KursTengah: decimal.Zero},
	}
	sec := BuildForm03_00(rows, ReportingOffice{Sandi: "1101"})

	barisSGD := rowByKey(t, sec, "SGD")
	if got := cellValue(barisSGD, "V"); got != "-" {
		t.Fatalf("baris berkurs kosong kolom V = %q, ingin -", got)
	}
	if barisSGD.Reason == "" {
		t.Fatal("baris berkurs kosong harus punya alasan kolom V tidak tersedia")
	}

	jumlah := rowByKey(t, sec, "JUMLAH")
	if got := cellValue(jumlah, "III"); got != "-" {
		t.Fatalf("JUMLAH tidak lengkap kolom III = %q, ingin - (tidak menghitung sebagian)", got)
	}
	if jumlah.Reason == "" {
		t.Fatal("JUMLAH tidak lengkap harus punya alasan")
	}
}

// Tanpa registri, Form 03.00 tetap membawa baris JUMLAH berisi "-" dan tidak mengarang.
func TestBuildForm03_00KosongTetapBawaJumlah(t *testing.T) {
	sec := BuildForm03_00(nil, ReportingOffice{Sandi: "1101"})
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1 (JUMLAH kosong)", len(sec.Rows))
	}
	if got := cellValue(rowByKey(t, sec, "JUMLAH"), "III"); got != "-" {
		t.Fatalf("JUMLAH tanpa baris kolom III = %q, ingin -", got)
	}
}

// Tata kelola kolom: 5 kolom (I + II..V); tanpa kantor pasti kolom I didaftarkan tidak
// tersedia.
func TestBuildForm03_00KolomDanSandiKantor(t *testing.T) {
	sec := BuildForm03_00(
		[]domain.KasValasItem{kasValasFormItem("USD", 100)},
		ReportingOffice{Sandi: "1101"})
	if len(sec.Columns) != 5 {
		t.Fatalf("jumlah kolom = %d, ingin 5 (I + II..V)", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[4].Sandi != "V" {
		t.Fatalf("kolom pertama/terakhir = %+v/%+v", sec.Columns[0], sec.Columns[4])
	}

	tanpaKantor := BuildForm03_00(
		[]domain.KasValasItem{kasValasFormItem("USD", 100)},
		ReportingOffice{Reason: "kantor belum dikonfigurasi"})
	if !punyaUnavailable(tanpaKantor.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Catatan form menyebut baris JUMLAH dan bahwa kolom V adalah turunan.
func TestBuildForm03_00Catatan(t *testing.T) {
	sec := BuildForm03_00(nil, ReportingOffice{Sandi: "1101"})
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "JUMLAH") {
		t.Fatalf("catatan harus menyebut baris JUMLAH: %q", gabung)
	}
	if !strings.Contains(gabung, "turunan") {
		t.Fatalf("catatan harus menyatakan kolom V turunan: %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 03.00 muncul di bundel bulanan.
func TestBuilderForm03_00LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		KasValas: kasValasOJKRepoStub{rows: []domain.KasValasItem{
			kasValasFormItem("USD", 1_000),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "03.00")
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 (1 + JUMLAH)", len(sec.Rows))
	}
}

// Register kosong tidak ditampilkan sebagai tabel; form dicatat pada SkippedForms dengan
// alasan yang menyebut status PVA.
func TestBuilderForm03_00RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), KasValas: kasValasOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "03.00" {
			t.Fatal("Form 03.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "03.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 03.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 03.00 kosong harus tercatat pada SkippedForms")
}
