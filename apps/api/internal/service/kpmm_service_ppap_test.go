package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// kpmmPPAPStubCKPN mengimplementasikan kontrak opsional LastPPAPBusinessDate dan
// merekam tanggal yang dipakai Compare, supaya dapat dibuktikan KPMM memakai
// periode PPAP yang benar (bukan akhir periode kalender).
type kpmmPPAPStubCKPN struct {
	domain.CKPNService
	last    time.Time
	ok      bool
	gotAsOf time.Time
}

func (s *kpmmPPAPStubCKPN) LastPPAPBusinessDate(context.Context) (time.Time, bool, error) {
	return s.last, s.ok, nil
}

func (s *kpmmPPAPStubCKPN) Compare(_ context.Context, asOf time.Time, _ domain.Actor) (domain.CKPNComparisonSummary, error) {
	s.gotAsOf = asOf
	return domain.CKPNComparisonSummary{Enabled: true}, nil
}

// kpmmPPAPStubReport hanya melayani GetBalanceSheet; sisanya ditutup oleh interface
// yang disematkan karena tidak dipanggil Hitung.
type kpmmPPAPStubReport struct {
	domain.ReportService
	bs *domain.BalanceSheet
}

func (s kpmmPPAPStubReport) GetBalanceSheet(context.Context, time.Time, string) (*domain.BalanceSheet, error) {
	return s.bs, nil
}

// kpmmPPAPStubConfig memakai fallback untuk seluruh nilai; cukup untuk menguji
// pemilihan periode, bukan angkanya.
type kpmmPPAPStubConfig struct{}

func (kpmmPPAPStubConfig) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (kpmmPPAPStubConfig) GetInt(context.Context, string, int) int          { return 0 }
func (kpmmPPAPStubConfig) GetString(context.Context, string, string) string { return "" }
func (kpmmPPAPStubConfig) GetBool(context.Context, string, bool) bool       { return false }
func (kpmmPPAPStubConfig) Invalidate(string)                                {}

// KPMM harus membandingkan PPKA-CKPN pada tanggal bisnis run PPAP terakhir, bukan
// pada akhir periode. Bila implementasi lama dipakai (as_of kalender), Compare
// menerima akhir periode dan PPAPBusinessDate kosong sehingga uji ini gagal.
func TestKPMMPakaiTanggalBisnisPPAPTerakhir(t *testing.T) {
	periodEnd := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
	lastPPAP := time.Date(2026, time.September, 24, 0, 0, 0, 0, time.UTC)

	ckpn := &kpmmPPAPStubCKPN{last: lastPPAP, ok: true}
	reports := kpmmPPAPStubReport{bs: &domain.BalanceSheet{
		TotalEquity: decimal.NewFromInt(100_000_000),
		NetIncome:   decimal.Zero,
	}}
	svc := service.NewKPMMService(reports, ckpn, kpmmPPAPStubConfig{})

	report, err := svc.Hitung(context.Background(), periodEnd, "", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}

	if !ckpn.gotAsOf.Equal(lastPPAP) {
		t.Fatalf("Compare dipanggil dengan %s, ingin tanggal run PPAP %s", ckpn.gotAsOf, lastPPAP)
	}
	if report.PPAPBusinessDate != "2026-09-24" {
		t.Fatalf("PPAPBusinessDate = %q, ingin 2026-09-24", report.PPAPBusinessDate)
	}
}
