package ojkreport

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// kpmmSvcStub mengimplementasikan domain.KPMMService untuk menguji adaptor RepoSource
// tanpa basis data.
type kpmmSvcStub struct {
	report domain.KPMMReport
	err    error
}

func (s kpmmSvcStub) Hitung(context.Context, time.Time, string, domain.Actor) (domain.KPMMReport, error) {
	return s.report, s.err
}

var _ domain.KPMMService = kpmmSvcStub{}

// TestRepoSourceKPMMModalATMR memastikan adaptor mengembalikan modal/ATMR hanya bila
// keduanya tersedia; komponen yang belum lengkap, galat layanan, atau layanan kosong
// selalu ok=false supaya Form 00.08 menulis "-", bukan nol.
func TestRepoSourceKPMMModalATMR(t *testing.T) {
	asOf := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	actor := domain.Actor{Role: domain.RoleSuperAdmin}

	src := RepoSource{KPMM: kpmmSvcStub{report: domain.KPMMReport{
		TotalModal: domain.KPMMKomponen{Nilai: decimal.NewFromInt(240), Tersedia: true},
		ATMR:       domain.KPMMKomponen{Nilai: decimal.NewFromInt(2000), Tersedia: true},
	}}}
	modal, atmr, ok := src.KPMMModalATMR(context.Background(), asOf, "CONVENTIONAL", actor)
	if !ok || !modal.Equal(decimal.NewFromInt(240)) || !atmr.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("modal/atmr/ok = %s/%s/%v, ingin 240/2000/true", modal, atmr, ok)
	}

	for nama, s := range map[string]RepoSource{
		"komponen belum lengkap": {KPMM: kpmmSvcStub{report: domain.KPMMReport{}}},
		"galat layanan":          {KPMM: kpmmSvcStub{err: errors.New("boom")}},
		"tanpa layanan KPMM":     {},
	} {
		if _, _, ok := s.KPMMModalATMR(context.Background(), asOf, "", actor); ok {
			t.Errorf("%s: ok=true, ingin false", nama)
		}
	}
}

// TestBuilderMengisiKPMMLewatRepoSource menutup rantai: RepoSource (dengan layanan
// KPMM) sebagai Source builder benar-benar mengisi baris KPMM Form 00.08.
func TestBuilderMengisiKPMMLewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		KPMM: kpmmSvcStub{report: domain.KPMMReport{
			TotalModal: domain.KPMMKomponen{Nilai: decimal.NewFromInt(240), Tersedia: true},
			ATMR:       domain.KPMMKomponen{Nilai: decimal.NewFromInt(2000), Tersedia: true},
		}},
	}
	b, err := NewBuilder(src).
		GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	line := findLine(t, b, "00.08", sandiKPMM)
	if line.UnavailableReason != "" {
		t.Fatalf("baris KPMM lewat RepoSource harus terisi: %s", line.UnavailableReason)
	}
	if !line.Amount.Equal(decimal.NewFromInt(12)) {
		t.Fatalf("baris KPMM = %s, ingin 12", line.Amount)
	}
}

// customerBatchStub menangkap pemanggilan GetByIDs agar uji dapat memastikan sandi
// referensi OJK dibaca sebagai SATU query agregat, bukan per kredit.
type customerBatchStub struct {
	domain.CustomerRepository
	records map[uuid.UUID]*domain.CustomerRecord
	gotIDs  []uuid.UUID
	calls   int
}

func (s *customerBatchStub) GetByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]*domain.CustomerRecord, error) {
	s.calls++
	s.gotIDs = ids
	return s.records, nil
}

// TestListLoansForOJKMengisiSandiReferensiDariCustomer menutup rantai adaptor:
// sandi pihak lawan (kolom XVIII) dan sektor ekonomi (kolom XX) per nasabah dimuat
// lewat satu GetByIDs untuk seluruh nasabah berbeda. Nasabah tanpa sandi dibiarkan
// kosong (form menulis "-"), dan nasabah yang sama tidak diduplikasi.
func TestListLoansForOJKMengisiSandiReferensiDariCustomer(t *testing.T) {
	nasabahA := uuid.New()
	nasabahB := uuid.New()
	repo := &loanAggStub{
		loans: []domain.Loan{
			{LoanNumber: "LN-A", Status: domain.LoanStatusDisbursed, CustomerID: nasabahA},
			{LoanNumber: "LN-B", Status: domain.LoanStatusDisbursed, CustomerID: nasabahA}, // nasabah sama
			{LoanNumber: "LN-C", Status: domain.LoanStatusDisbursed, CustomerID: nasabahB},
		},
	}
	customers := &customerBatchStub{records: map[uuid.UUID]*domain.CustomerRecord{
		nasabahA: {ID: nasabahA, OJKPihakLawanCode: "860", OJKSektorEkonomiCode: "G00000"},
		nasabahB: {ID: nasabahB}, // belum diisi
	}}
	src := RepoSource{Loans: repo, Customers: customers}

	rows, err := src.ListLoansForOJK(context.Background(), time.Now().UTC(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListLoansForOJK: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3", len(rows))
	}
	if customers.calls != 1 {
		t.Errorf("GetByIDs dipanggil %d kali, ingin 1 (agregat)", customers.calls)
	}
	if len(customers.gotIDs) != 2 {
		t.Errorf("GetByIDs menerima %d id, ingin 2 unik", len(customers.gotIDs))
	}
	if rows[0].OJKPihakLawanCode != "860" || rows[0].OJKSektorEkonomiCode != "G00000" {
		t.Errorf("sandi nasabah A = %q/%q, ingin 860/G00000", rows[0].OJKPihakLawanCode, rows[0].OJKSektorEkonomiCode)
	}
	if rows[1].OJKPihakLawanCode != "860" {
		t.Errorf("baris kedua nasabah A = %q, ingin 860", rows[1].OJKPihakLawanCode)
	}
	if rows[2].OJKPihakLawanCode != "" || rows[2].OJKSektorEkonomiCode != "" {
		t.Errorf("sandi nasabah B belum diisi, ingin kosong, dapat %q/%q", rows[2].OJKPihakLawanCode, rows[2].OJKSektorEkonomiCode)
	}
}
