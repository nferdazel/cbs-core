package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
)

type stubOJKSource struct{}

func (stubOJKSource) GetBalanceSheet(_ context.Context, _ time.Time, _ string) (*domain.BalanceSheet, error) {
	return &domain.BalanceSheet{}, nil
}

func (stubOJKSource) GetIncomeStatement(_ context.Context, _, _ time.Time, _ string) (*domain.IncomeStatement, error) {
	return &domain.IncomeStatement{}, nil
}

func TestParseOJKPeriod(t *testing.T) {
	if got, err := parseOJKPeriod("2026-03"); err != nil || !got.Equal(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("YYYY-MM: got %s err %v", got, err)
	}
	if got, err := parseOJKPeriod("2026-03-18"); err != nil || !got.Equal(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("YYYY-MM-DD: got %s err %v", got, err)
	}
	if _, err := parseOJKPeriod("2026/03"); err == nil {
		t.Fatal("format tidak valid harus ditolak")
	}

	got, err := parseOJKPeriod("")
	if err != nil {
		t.Fatalf("default tanpa period error: %v", err)
	}
	prev := time.Now().UTC().AddDate(0, -1, 0)
	if got.Day() != 1 || got.Month() != prev.Month() || got.Year() != prev.Year() {
		t.Fatalf("default period = %s, ingin awal bulan lalu", got)
	}
}

func newOJKHandler() *OJKReportHandler {
	return &OJKReportHandler{builder: ojkreport.NewBuilder(stubOJKSource{})}
}

// Aktor yang hanya berwenang atas cabangnya tidak boleh mengekspor laporan
// bank-wide; permintaan harus ditolak 403.
func TestExportMonthlyMenolakAktorNonLintasCabang(t *testing.T) {
	h := newOJKHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/monthly?period=2026-03", nil)
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims,
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"}))
	rec := httptest.NewRecorder()

	h.ExportMonthly(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403", rec.Code)
	}
}

// Aktor lintas cabang boleh mengekspor; keluaran adalah berkas teks lampiran.
func TestExportMonthlyMengizinkanAktorLintasCabang(t *testing.T) {
	h := newOJKHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/monthly?period=2026-03", nil)
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims,
		&domain.JWTClaims{Role: domain.RoleSuperAdmin}))
	rec := httptest.NewRecorder()

	h.ExportMonthly(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "ojk-bulanan-2026-03.txt") {
		t.Fatalf("content-disposition = %q", cd)
	}
	if !strings.Contains(rec.Body.String(), "FORM|SANDI|NAMA POS|JUMLAH") {
		t.Fatal("berkas teks tidak memuat header kolom")
	}
}
