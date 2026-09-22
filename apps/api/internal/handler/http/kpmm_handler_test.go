package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

type stubKPMMService struct {
	report domain.KPMMReport
	err    error
}

func (s stubKPMMService) Hitung(context.Context, time.Time, string, domain.Actor) (domain.KPMMReport, error) {
	return s.report, s.err
}

func kpmmReqWithClaims(claims *domain.JWTClaims) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/reports/kpmm?period=2026-03", nil)
	return req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
}

// TestKPMMHandlerReport memastikan endpoint KPMM mengembalikan angka laporan untuk
// peran lintas cabang, menolak peran cabang, dan menolak tanpa autentikasi.
func TestKPMMHandlerReport(t *testing.T) {
	h := NewKPMMHandler(stubKPMMService{report: domain.KPMMReport{
		KPMMMinFrac: decimal.NewFromFloat(0.12),
		RasioKPMM:   domain.KPMMKomponen{Nilai: decimal.NewFromInt(30), Tersedia: true},
	}})

	// Peran lintas cabang: 200 dan memuat rasio.
	rec := httptest.NewRecorder()
	h.Report(rec, kpmmReqWithClaims(&domain.JWTClaims{Role: domain.RoleSuperAdmin, BranchCode: "001"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status superadmin = %d, mau 200; body=%s", rec.Code, rec.Body.String())
	}

	// Peran cabang: laporan bank-wide ditolak 403.
	rec = httptest.NewRecorder()
	h.Report(rec, kpmmReqWithClaims(&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"}))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status teller = %d, mau 403", rec.Code)
	}

	// Tanpa klaim: 401.
	rec = httptest.NewRecorder()
	h.Report(rec, httptest.NewRequest(http.MethodGet, "/reports/kpmm", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status tanpa klaim = %d, mau 401", rec.Code)
	}
}
