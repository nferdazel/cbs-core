package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// offBalanceRepoStub mengimplementasikan domain.OffBalanceRepository untuk menguji
// Form 01.01 lewat RepoSource tanpa basis data.
type offBalanceRepoStub struct {
	domain.OffBalanceRepository
	rows []domain.OffBalanceAggregate
	err  error
}

func (s offBalanceRepoStub) ListAggregates(context.Context, time.Time) ([]domain.OffBalanceAggregate, error) {
	return s.rows, s.err
}

var _ domain.OffBalanceRepository = offBalanceRepoStub{}

// Form 01.01 harus memakai nama pos resmi untuk sandi yang tercantum di PDF #page 110,
// memakai uraian bank untuk sandi tak dikenal, dan menandai kolom Sandi Kantor belum
// tersedia (register bersifat bank-wide).
func TestBuildForm01_01NamaPosDanKolomBelumTersedia(t *testing.T) {
	rows := []domain.OffBalanceAggregate{
		{Category: domain.OffBalanceCategoryKomitmen, PositionCode: "6102010000",
			TotalAmount: decimal.NewFromInt(1500000), ItemCount: 1},
		{Category: domain.OffBalanceCategoryLainnya, PositionCode: "",
			Description: "Pos manual bank", TotalAmount: decimal.NewFromInt(250000), ItemCount: 1},
	}

	sec := BuildForm01_01(rows, ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})
	if sec.Form != "01.01" {
		t.Fatalf("form = %q, ingin 01.01", sec.Form)
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom Sandi Kantor tidak ditandai belum tersedia")
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(sec.Rows))
	}

	// Baris bersandi resmi: nama resmi diambil dari peta, sandi tampil apa adanya.
	if got := cellValue(sec.Rows[0], "II"); got != "Fasilitas Kredit kepada Nasabah yang Belum Ditarik" {
		t.Errorf("nama pos bersandi = %q", got)
	}
	if got := cellValue(sec.Rows[0], "III"); got != "6102010000" {
		t.Errorf("sandi pos = %q", got)
	}
	// Baris tanpa sandi: nama dari uraian bank, sandi ditulis "-".
	if got := cellValue(sec.Rows[1], "II"); got != "Pos manual bank" {
		t.Errorf("nama pos tanpa sandi = %q", got)
	}
	if got := cellValue(sec.Rows[1], "III"); got != "-" {
		t.Errorf("sandi pos kosong = %q, ingin -", got)
	}
}

// cellValue mengambil nilai satu sel menurut sandi kolomnya.
func cellValue(row TableRow, sandi string) string {
	for _, c := range row.Cells {
		if c.Sandi == sandi {
			return c.Value
		}
	}
	return ""
}

// Kolom I Form 01.01 kini bersumber dari kantor pelapor tunggal pada bank_offices.
// Lewat RepoSource dengan register off-balance, kolom I menjadi kolom nyata paling
// kiri ber-sandi kantor itu; baris tanpa sandi pos tetap menulis "-" pada kolom III.
func TestBuilderForm01_01KolomSandiKantorLewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		Kelembagaan: kelembagaanRepoStub{offices: []domain.BankOffice{
			{Code: "001", Name: "Kantor Pusat", Status: domain.KelembagaanKantorAktif},
		}},
		OffBalance: offBalanceRepoStub{rows: []domain.OffBalanceAggregate{
			{Category: domain.OffBalanceCategoryKomitmen, PositionCode: "6102010000",
				TotalAmount: decimal.NewFromInt(1500000), ItemCount: 1},
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "01.01")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(sec.Rows))
	}
	if got := cellValue(sec.Rows[0], "I"); got != "001" {
		t.Errorf("kolom I = %q, ingin 001", got)
	}
	if got := cellValue(sec.Rows[0], "III"); got != "6102010000" {
		t.Errorf("kolom III = %q, ingin 6102010000", got)
	}
}
