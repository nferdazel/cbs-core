package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// TestForm06KolomAngsuranPertamaDanNominalTunggakan membuktikan kolom XIII dan XVII
// kini terisi dari LoanRow (bukan lagi terdaftar tidak tersedia): tanggal angsuran
// pertama dengan format 2006-01-02, dan nominal tunggakan pokok+bunga lewat
// FormatRupiah. Kredit tanpa jadwal menulis "-" dan "0".
func TestForm06KolomAngsuranPertamaDanNominalTunggakan(t *testing.T) {
	pertama := mustDate(t, "2025-04-10")
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-TUNGGAK", FirstInstallmentDate: &pertama, OverdueUnpaid: decimal.NewFromInt(1_234_567)},
		{Status: "DISBURSED", LoanNumber: "LN-LANCAR"},
	}
	sec := buildForm06(rows)

	if got := findCell(t, sec, "LN-TUNGGAK", form06SandiAngsuranRetni).Value; got != "2025-04-10" {
		t.Errorf("kolom XIII = %q, ingin 2025-04-10", got)
	}
	if got := findCell(t, sec, "LN-TUNGGAK", form06SandiNominalTungg).Value; got != "1234567" {
		t.Errorf("kolom XVII = %q, ingin 1234567", got)
	}
	if got := findCell(t, sec, "LN-LANCAR", form06SandiAngsuranRetni).Value; got != "-" {
		t.Errorf("kolom XIII tanpa jadwal = %q, ingin -", got)
	}
	if got := findCell(t, sec, "LN-LANCAR", form06SandiNominalTungg).Value; got != "0" {
		t.Errorf("kolom XVII tanpa tunggakan = %q, ingin 0", got)
	}

	// Kedua kolom tidak boleh lagi muncul sebagai kolom tidak tersedia.
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiAngsuranRetni || u.Sandi == form06SandiNominalTungg {
			t.Errorf("kolom %s masih terdaftar tidak tersedia: %s", u.Sandi, u.Reason)
		}
	}
}

// loanAggStub menyediakan dua kontrak yang dipakai RepoSource.ListLoansForOJK:
// daftar kredit dan agregat jadwal per kredit. Metode domain.LoanRepository lain
// tidak dipakai uji ini sehingga cukup lewat embedding.
type loanAggStub struct {
	domain.LoanRepository
	loans      []domain.Loan
	aggregates []domain.LoanScheduleAggregate
	gotAsOf    time.Time
}

func (s *loanAggStub) List(context.Context, int, int, domain.Actor) ([]domain.Loan, int, error) {
	return s.loans, len(s.loans), nil
}

func (s *loanAggStub) ListLoanScheduleAggregates(_ context.Context, asOf time.Time, _ domain.Actor) ([]domain.LoanScheduleAggregate, error) {
	s.gotAsOf = asOf
	return s.aggregates, nil
}

// TestListLoansForOJKMengisiAgregatJadwal menutup rantai adaptor: asOf diteruskan
// ke agregat jadwal, dan hasilnya dipetakan ke LoanRow lewat nomor kredit (tanggal
// angsuran pertama, tunggakan, dan piutang bunga). Kredit tanpa agregat (tidak punya
// jadwal) dibiarkan nil/nol, bukan diberi angka kredit lain.
func TestListLoansForOJKMengisiAgregatJadwal(t *testing.T) {
	asOf := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	pertama := time.Date(2025, time.April, 10, 0, 0, 0, 0, time.UTC)
	repo := &loanAggStub{
		loans: []domain.Loan{
			{LoanNumber: "LN-001", Status: domain.LoanStatusDisbursed},
			{LoanNumber: "LN-002", Status: domain.LoanStatusDisbursed},
		},
		aggregates: []domain.LoanScheduleAggregate{
			{LoanNumber: "LN-001", FirstInstallmentDate: &pertama, OverdueUnpaid: decimal.NewFromInt(150_000), AccruedProfit: decimal.NewFromInt(75_000)},
		},
	}
	src := RepoSource{Loans: repo}

	rows, err := src.ListLoansForOJK(context.Background(), asOf, domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListLoansForOJK: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(rows))
	}
	if !repo.gotAsOf.Equal(asOf) {
		t.Errorf("asOf ke agregat = %s, ingin %s", repo.gotAsOf, asOf)
	}
	if rows[0].FirstInstallmentDate == nil || !rows[0].FirstInstallmentDate.Equal(pertama) {
		t.Errorf("tanggal angsuran pertama LN-001 = %v, ingin %s", rows[0].FirstInstallmentDate, pertama)
	}
	if !rows[0].OverdueUnpaid.Equal(decimal.NewFromInt(150_000)) {
		t.Errorf("tunggakan LN-001 = %s, ingin 150000", rows[0].OverdueUnpaid)
	}
	if !rows[0].AccruedProfit.Equal(decimal.NewFromInt(75_000)) {
		t.Errorf("piutang bunga LN-001 = %s, ingin 75000", rows[0].AccruedProfit)
	}
	if rows[1].FirstInstallmentDate != nil || !rows[1].OverdueUnpaid.IsZero() || !rows[1].AccruedProfit.IsZero() {
		t.Errorf("LN-002 tanpa jadwal harus nil/0, dapat %v/%s/%s",
			rows[1].FirstInstallmentDate, rows[1].OverdueUnpaid, rows[1].AccruedProfit)
	}
}

// TestListLoansForOJKMenolakAktorBukanLintasCabang menjaga kebijakan bank-wide tetap
// berlaku setelah perubahan: aktor non-lintas cabang ditolak sebelum query agregat.
func TestListLoansForOJKMenolakAktorBukanLintasCabang(t *testing.T) {
	repo := &loanAggStub{}
	src := RepoSource{Loans: repo}
	if _, err := src.ListLoansForOJK(context.Background(), time.Now(), domain.Actor{Role: domain.RoleTeller}); err == nil {
		t.Fatal("aktor non-lintas cabang harus ditolak")
	}
}
