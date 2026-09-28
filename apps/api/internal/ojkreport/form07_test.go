package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// aydaOJKRepoStub mengimplementasikan domain.AYDARegisterRepository untuk menguji
// Form 07.00 lewat RepoSource tanpa basis data.
type aydaOJKRepoStub struct {
	domain.AYDARegisterRepository
	rows []domain.AYDAItem
	err  error
}

func (s aydaOJKRepoStub) ListAYDAForOJK(context.Context, time.Time) ([]domain.AYDAItem, error) {
	return s.rows, s.err
}

var _ domain.AYDARegisterRepository = aydaOJKRepoStub{}

// buildForm07 merinci tiap AYDA dan menghitung Jumlah sebagai nilai yang lebih rendah
// dari NRV atau nilai tercatat; kaki tabel JUMLAH diisi bila seluruh baris lengkap.
func TestBuildForm07MenghitungJumlahDanTotal(t *testing.T) {
	rows := []domain.AYDAItem{
		{
			CollateralTypeCode:      "02",
			CollateralAddress:       "Jl. Merdeka 1",
			AcquisitionDate:         time.Date(2025, time.June, 30, 0, 0, 0, 0, time.UTC),
			InitialRecognitionValue: decimal.NewFromInt(1_000),
			AccumulatedImpairment:   decimal.NewFromInt(100),
			NetRealizableValue:      decimal.NewFromInt(800),
		},
		{
			CollateralTypeCode:      "05",
			CollateralAddress:       "Jl. Sudirman 2",
			AcquisitionDate:         time.Date(2025, time.July, 15, 0, 0, 0, 0, time.UTC),
			InitialRecognitionValue: decimal.NewFromInt(2_000),
			AccumulatedImpairment:   decimal.NewFromInt(50),
			NetRealizableValue:      decimal.NewFromInt(2_500),
		},
	}
	sec := BuildForm07(rows, ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})

	if sec.Form != "07.00" {
		t.Fatalf("form = %q, ingin 07.00", sec.Form)
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom Sandi Kantor tidak ditandai belum tersedia")
	}
	if len(sec.Rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3 (dua AYDA + JUMLAH)", len(sec.Rows))
	}

	// Baris pertama: Jumlah = min(NRV 800, tercatat 900) = 800.
	first := rowByKey(t, sec, "2025-06-30|Jl. Merdeka 1")
	if got := cellValue(first, "II"); got != "02" {
		t.Errorf("jenis agunan = %q, ingin 02", got)
	}
	if got := cellValue(first, "IV"); got != "30-06-2025" {
		t.Errorf("tanggal = %q, ingin 30-06-2025 (TT-BB-TTTT)", got)
	}
	if got := cellValue(first, "VII"); got != "800" {
		t.Errorf("jumlah = %q, ingin 800", got)
	}
	if got := cellValue(first, "VIII"); got != "800" {
		t.Errorf("nrv = %q, ingin 800", got)
	}

	// Kaki tabel JUMLAH = penjumlahan tiap kolom angka.
	total := rowByKey(t, sec, form07TotalKey)
	if total.Reason != "" {
		t.Fatalf("JUMLAH harus terisi saat semua baris lengkap: %s", total.Reason)
	}
	for sandi, want := range map[string]string{"V": "3000", "VI": "150", "VII": "2750", "VIII": "3300"} {
		if got := cellValue(total, sandi); got != want {
			t.Errorf("JUMLAH kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
}

// Baris yang nilai tercatatnya negatif tidak boleh diisi angka: kolom VII ditulis "-"
// dan kaki tabel JUMLAH juga ditulis "-" dengan alasan, bukan total sebagian.
func TestBuildForm07JumlahTidakLengkap(t *testing.T) {
	rows := []domain.AYDAItem{{
		CollateralTypeCode:      "02",
		CollateralAddress:       "Jl. Merdeka 1",
		AcquisitionDate:         time.Date(2025, time.June, 30, 0, 0, 0, 0, time.UTC),
		InitialRecognitionValue: decimal.NewFromInt(100),
		AccumulatedImpairment:   decimal.NewFromInt(500),
		NetRealizableValue:      decimal.NewFromInt(400),
	}}
	sec := BuildForm07(rows, ReportingOffice{Sandi: "001"})

	baris := rowByKey(t, sec, "2025-06-30|Jl. Merdeka 1")
	if got := cellValue(baris, "VII"); got != "-" {
		t.Fatalf("jumlah baris tidak lengkap = %q, ingin -", got)
	}
	if baris.Reason == "" {
		t.Fatal("baris tidak lengkap harus beralasan")
	}
	total := rowByKey(t, sec, form07TotalKey)
	if total.Reason == "" {
		t.Fatal("JUMLAH harus beralasan saat ada baris tidak lengkap")
	}
	for _, sandi := range []string{"V", "VI", "VII", "VIII"} {
		if got := cellValue(total, sandi); got != "-" {
			t.Errorf("JUMLAH kolom %s = %q, ingin -", sandi, got)
		}
	}
}

// Kolom I Form 07.00 bersumber dari kantor pelapor tunggal pada bank_offices. Lewat
// RepoSource, kolom I menjadi kolom nyata paling kiri dan terisi sandi kantor itu.
func TestBuilderForm07KolomSandiKantorLewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		Kelembagaan: kelembagaanRepoStub{offices: []domain.BankOffice{
			{Code: "001", Name: "Kantor Pusat", Status: domain.KelembagaanKantorAktif},
		}},
		AYDA: aydaOJKRepoStub{rows: []domain.AYDAItem{{
			CollateralTypeCode:      "02",
			CollateralAddress:       "Jl. Merdeka 1",
			AcquisitionDate:         time.Date(2025, time.June, 30, 0, 0, 0, 0, time.UTC),
			InitialRecognitionValue: decimal.NewFromInt(1_000),
			AccumulatedImpairment:   decimal.NewFromInt(100),
			NetRealizableValue:      decimal.NewFromInt(900),
		}}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "07.00")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if got := cellValue(rowByKey(t, sec, "2025-06-30|Jl. Merdeka 1"), "I"); got != "001" {
		t.Errorf("kolom I = %q, ingin 001", got)
	}
}

// Register AYDA kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada
// SkippedForms dengan alasan spesifik.
func TestBuilderForm07RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), AYDA: aydaOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "07.00" {
			t.Fatal("Form 07.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "07.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 07.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 07.00 kosong harus tercatat pada SkippedForms")
}
