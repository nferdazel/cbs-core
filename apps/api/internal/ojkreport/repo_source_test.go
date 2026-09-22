package ojkreport

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
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
