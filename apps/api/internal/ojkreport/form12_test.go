package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// timeDepositStubSource melengkapi stubSource dengan baris Form 12.00 dan merekam
// posisi akhir periode yang diminta perakit.
type timeDepositStubSource struct {
	*stubSource
	rows     []TimeDepositRow
	err      error
	lastAsOf time.Time
}

func (s *timeDepositStubSource) ListTimeDepositsForOJK(_ context.Context, asOf time.Time, _ domain.Actor) ([]TimeDepositRow, error) {
	s.lastAsOf = asOf
	return s.rows, s.err
}

// Form 12.00 mengisi kolom yang punya sumber (CIF, no. rekening, hubungan, golongan,
// lokasi, jangka waktu, suku bunga, nominal, jumlah) dan menandai kolom tanpa sumber
// sebagai belum tersedia, bukan nol.
func TestBuildForm12KolomTersediaDanBelum(t *testing.T) {
	rows := []TimeDepositRow{{
		AccountNumber:    "DEP-001",
		CounterpartyCIF:  "CIF-001",
		CustomerTypeCode: "860",
		HubunganBankCode: "20",
		LocationCode:     "0197",
		ProfitType:       "INTEREST",
		PlacementAmount:  decimal.NewFromInt(10_000_000),
		StartDate:        time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC),
		MaturityDate:     time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC),
		ProfitRate:       decimal.RequireFromString("4.25"),
	}}

	sec := buildForm12(rows, ReportingOffice{Sandi: "001", Nama: "Kantor Pusat"})
	if sec.Form != "12.00" {
		t.Fatalf("form = %s, ingin 12.00", sec.Form)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(sec.Rows))
	}
	key := "DEP-001"
	cases := map[string]string{
		form12SandiKantor:    "001",
		form12SandiIDPihak:   "CIF-001",
		form12SandiNoRek:     "DEP-001",
		form12SandiHubungan:  "20",
		form12SandiGolongan:  "860",
		form12SandiLokasi:    "0197",
		form12SandiJangka:    "10-01-2026 s.d. 10-07-2026",
		form12SandiSukuBunga: "4.25",
		form12SandiNominal:   "10000000",
		form12SandiJumlah:    "10000000",
	}
	for sandi, want := range cases {
		if got := findCell(t, sec, key, sandi).Value; got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}

	for _, sandi := range []string{
		form12SandiDiblokir, form12SandiAlasan, form12SandiBiaya, form12SandiNomorID,
		form12SandiPEP, form12SandiRisiko, form12SandiStatusData,
	} {
		u := unavailableColumn(t, sec, sandi)
		if strings.TrimSpace(u.Reason) == "" {
			t.Errorf("kolom %s harus mencantumkan alasan", sandi)
		}
	}
}

// Suku bunga deposito hanya ditulis sebagai persen untuk profit_type INTEREST;
// margin/bagi hasil syariah ditulis "-".
func TestBuildForm12SukuBungaTanpaPersen(t *testing.T) {
	sec := buildForm12([]TimeDepositRow{{
		AccountNumber: "DEP-2", ProfitType: "BAGI_HASIL", ProfitRate: decimal.RequireFromString("0.4"),
	}}, ReportingOffice{})
	if got := findCell(t, sec, "DEP-2", form12SandiSukuBunga).Value; got != "-" {
		t.Errorf("suku bunga bagi hasil = %q, ingin -", got)
	}
}

// Tanpa sumber, Form 12.00 didaftarkan belum tersedia dengan alasan; bila sumber ada
// tetapi tidak ada kontrak berjalan, alasannya menyebut deposito berjangka.
func TestGenerateMonthlyForm12SkippedDenganAlasan(t *testing.T) {
	tanpaSumber, err := NewBuilder(newStubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly tanpa sumber: %v", err)
	}
	if got := skippedReason(t, tanpaSumber, "12.00"); !strings.Contains(got, "belum dikonfigurasi") {
		t.Errorf("alasan tanpa sumber = %q", got)
	}

	kosong, err := NewBuilder(&timeDepositStubSource{stubSource: newStubSource()}).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly tanpa baris: %v", err)
	}
	if got := skippedReason(t, kosong, "12.00"); !strings.Contains(got, "deposito berjangka") {
		t.Errorf("alasan tanpa baris = %q", got)
	}
}

// Dengan baris deposito, Form 12.00 muncul pada tabel bundle bulanan dan permintaan
// diteruskan pada posisi AKHIR PERIODE, bukan waktu ekspor dijalankan.
func TestGenerateMonthlyMemuatForm12DenganAkhirPeriode(t *testing.T) {
	period := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	src := &timeDepositStubSource{
		stubSource: newStubSource(),
		rows: []TimeDepositRow{{
			AccountNumber: "DEP-777", CounterpartyCIF: "CIF-777", CustomerTypeCode: "860",
			ProfitType: "INTEREST", PlacementAmount: decimal.NewFromInt(5_000_000),
			StartDate:    time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC),
			MaturityDate: time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC),
			ProfitRate:   decimal.RequireFromString("4.00"),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(context.Background(), period, "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "12.00")
	if got := findCell(t, sec, "DEP-777", form12SandiNominal).Value; got != "5000000" {
		t.Fatalf("nominal Form 12.00 = %q, ingin 5000000", got)
	}
	if want := MonthEnd(period); !src.lastAsOf.Equal(want) {
		t.Fatalf("asOf ke sumber deposito = %v, ingin akhir periode %v", src.lastAsOf, want)
	}
}

// Kolom I Form 12.00 memakai kantor pelapor yang sama dengan Form 09.00/01.01 lewat
// RepoSource (bank_offices).
func TestBuilderForm12KolomSandiKantorLewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		TimeDeposits: &timeDepositRepoStub{rows: []domain.TimeDepositAggregate{{
			AccountNumber: "DEP-9", CounterpartyCIF: "CIF-9", ProfitType: "INTEREST",
			PlacementAmount: decimal.NewFromInt(100), ProfitRate: decimal.RequireFromString("2.00"),
			StartDate:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			MaturityDate: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		}}},
		Kelembagaan: kelembagaanRepoStub{offices: []domain.BankOffice{
			{Code: "001", Name: "Kantor Pusat", Status: domain.KelembagaanKantorAktif},
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "12.00")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if got := findCell(t, sec, "DEP-9", "I").Value; got != "001" {
		t.Fatalf("kolom I Form 12.00 = %q, ingin 001", got)
	}
}
