package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// savingsAccountStubSource melengkapi stubSource dengan baris Form 11.00 dan merekam
// posisi akhir periode yang diminta perakit.
type savingsAccountStubSource struct {
	*stubSource
	rows     []SavingsAccountRow
	err      error
	lastAsOf time.Time
}

func (s *savingsAccountStubSource) ListSavingsAccountsForOJK(_ context.Context, asOf time.Time, _ domain.Actor) ([]SavingsAccountRow, error) {
	s.lastAsOf = asOf
	return s.rows, s.err
}

// Form 11.00 mengisi kolom yang punya sumber (CIF, no. rekening, hubungan, golongan,
// lokasi, suku bunga, nominal, jumlah) dan menandai kolom tanpa sumber sebagai belum
// tersedia, bukan nol. Nomor Identitas tetap tidak dipaparkan (keputusan privasi).
func TestBuildForm11KolomTersediaDanBelum(t *testing.T) {
	rows := []SavingsAccountRow{{
		AccountNumber:      "TAB-001",
		CounterpartyCIF:    "CIF-001",
		CustomerTypeCode:   "860",
		HubunganBankCode:   "20",
		LocationCode:       "0197",
		ProfitScheme:       "INTEREST",
		InterestRateAnnual: decimal.RequireFromString("3.50"),
		Balance:            decimal.NewFromInt(1_500_000),
	}}

	sec := buildForm11(rows, ReportingOffice{Sandi: "001", Nama: "Kantor Pusat"})
	if sec.Form != "11.00" {
		t.Fatalf("form = %s, ingin 11.00", sec.Form)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(sec.Rows))
	}
	key := "TAB-001"
	cases := map[string]string{
		form11SandiKantor:    "001",
		form11SandiIDPihak:   "CIF-001",
		form11SandiNoRek:     "TAB-001",
		form11SandiHubungan:  "20",
		form11SandiGolongan:  "860",
		form11SandiLokasi:    "0197",
		form11SandiSukuBunga: "3.50",
		form11SandiNominal:   "1500000",
		form11SandiJumlah:    "1500000",
	}
	for sandi, want := range cases {
		if got := findCell(t, sec, key, sandi).Value; got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}

	for _, sandi := range []string{
		form11SandiJenis, form11SandiJangka, form11SandiDiblokir, form11SandiAlasan,
		form11SandiBiaya, form11SandiNomorID, form11SandiPEP, form11SandiRisiko,
		form11SandiStatusData,
	} {
		u := unavailableColumn(t, sec, sandi)
		if strings.TrimSpace(u.Reason) == "" {
			t.Errorf("kolom %s harus mencantumkan alasan", sandi)
		}
	}
}

// Suku bunga hanya ditulis sebagai persen untuk produk berbunga; nisbah bagi hasil
// syariah dan tarif yang belum diisi ditulis "-", bukan "0.00".
func TestBuildForm11SukuBungaTanpaPersen(t *testing.T) {
	for nama, r := range map[string]SavingsAccountRow{
		"nisbah bagi hasil": {AccountNumber: "TAB-2", ProfitScheme: "MUDHARABAH", InterestRateAnnual: decimal.RequireFromString("0.4")},
		"tarif belum diisi": {AccountNumber: "TAB-3", ProfitScheme: "INTEREST"},
	} {
		sec := buildForm11([]SavingsAccountRow{r}, ReportingOffice{})
		if got := findCell(t, sec, r.AccountNumber, form11SandiSukuBunga).Value; got != "-" {
			t.Errorf("%s: suku bunga = %q, ingin -", nama, got)
		}
	}
}

// Tanpa sumber, Form 11.00 didaftarkan belum tersedia dengan alasan; bila sumber ada
// tetapi tidak ada rekening bersaldo, alasannya menyebut rekening tabungan.
func TestGenerateMonthlyForm11SkippedDenganAlasan(t *testing.T) {
	tanpaSumber, err := NewBuilder(newStubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly tanpa sumber: %v", err)
	}
	if got := skippedReason(t, tanpaSumber, "11.00"); !strings.Contains(got, "belum dikonfigurasi") {
		t.Errorf("alasan tanpa sumber = %q", got)
	}

	kosong, err := NewBuilder(&savingsAccountStubSource{stubSource: newStubSource()}).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly tanpa baris: %v", err)
	}
	if got := skippedReason(t, kosong, "11.00"); !strings.Contains(got, "rekening tabungan") {
		t.Errorf("alasan tanpa baris = %q", got)
	}
}

// Dengan baris tabungan, Form 11.00 muncul pada tabel bundle bulanan dan permintaan
// diteruskan pada posisi AKHIR PERIODE, bukan waktu ekspor dijalankan.
func TestGenerateMonthlyMemuatForm11DenganAkhirPeriode(t *testing.T) {
	period := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	src := &savingsAccountStubSource{
		stubSource: newStubSource(),
		rows: []SavingsAccountRow{{
			AccountNumber: "TAB-777", CounterpartyCIF: "CIF-777", CustomerTypeCode: "860",
			ProfitScheme: "INTEREST", InterestRateAnnual: decimal.RequireFromString("2.00"),
			Balance: decimal.NewFromInt(2_000_000),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(context.Background(), period, "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "11.00")
	if got := findCell(t, sec, "TAB-777", form11SandiNominal).Value; got != "2000000" {
		t.Fatalf("nominal Form 11.00 = %q, ingin 2000000", got)
	}
	if want := MonthEnd(period); !src.lastAsOf.Equal(want) {
		t.Fatalf("asOf ke sumber tabungan = %v, ingin akhir periode %v", src.lastAsOf, want)
	}
}

// Kolom I Form 11.00 memakai kantor pelapor yang sama dengan Form 09.00/01.01 lewat
// RepoSource (bank_offices). Bila kantor pelapor tunggal tersedia, kolom I menjadi
// kolom nyata ber-sandi kantor itu, bukan tidak tersedia.
func TestBuilderForm11KolomSandiKantorLewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		SavingsAccounts: &savingsAccountRepoStub{rows: []domain.SavingsAccountAggregate{{
			AccountNumber: "TAB-9", CounterpartyCIF: "CIF-9", ProfitScheme: "INTEREST",
			InterestRateAnnual: decimal.RequireFromString("2.00"), Balance: decimal.NewFromInt(100),
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
	sec := tableByForm(t, b, "11.00")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if got := findCell(t, sec, "TAB-9", "I").Value; got != "001" {
		t.Fatalf("kolom I Form 11.00 = %q, ingin 001", got)
	}
}
