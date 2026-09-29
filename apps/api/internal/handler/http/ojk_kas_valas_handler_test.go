package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// kasValasSvcStub mengimplementasikan domain.KasValasRegisterService di memori untuk uji
// handler tanpa basis data.
type kasValasSvcStub struct {
	report      domain.KasValasReport
	reportErr   error
	items       []domain.KasValasItem
	upsertErr   error
	deleteErr   error
	lastDeleted uuid.UUID
}

func (s *kasValasSvcStub) KasValasReport(context.Context, time.Time, domain.Actor) (domain.KasValasReport, error) {
	return s.report, s.reportErr
}

func (s *kasValasSvcStub) ListItems(context.Context) ([]domain.KasValasItem, error) {
	return s.items, nil
}

func (s *kasValasSvcStub) UpsertItem(_ context.Context, in domain.UpdateKasValasItemInput, _ domain.Actor) (*domain.KasValasItem, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &domain.KasValasItem{ID: uuid.New(), JenisValasCode: in.JenisValasCode}, nil
}

func (s *kasValasSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.lastDeleted = id
	return s.deleteErr
}

var _ domain.KasValasRegisterService = (*kasValasSvcStub)(nil)

func kasValasUpsertRequest(body string) *http.Request {
	return ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/kas-valas/items", body,
		&domain.JWTClaims{Role: domain.RoleAdmin})
}

// Daftar baris register kas valas disajikan apa adanya untuk UI edit.
func TestListKasValasItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{kasValas: &kasValasSvcStub{items: []domain.KasValasItem{
		{ID: uuid.New(), JenisValasCode: "USD", Nominal: decimal.NewFromInt(900)},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/kas-valas/items", nil)

	h.ListKasValasItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"jenis_valas_code":"USD"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Payload tidak sah dipetakan ke 422 lewat layanan.
func TestUpsertKasValasMeneruskanGalatValidasi(t *testing.T) {
	svc := &kasValasSvcStub{upsertErr: domain.ErrKasValasInputInvalid}
	h := &OJKReportHandler{kasValas: svc}
	rec := httptest.NewRecorder()

	h.UpsertKasValasItem(rec, kasValasUpsertRequest(`{"jenis_valas_code":"USD"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang hendak dihapus tetapi tidak ada dipetakan ke 404.
func TestDeleteKasValasBarisTidakAda(t *testing.T) {
	h := &OJKReportHandler{kasValas: &kasValasSvcStub{deleteErr: domain.ErrKasValasNotFound}}
	id := uuid.New()
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/kas-valas/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.DeleteKasValasItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Penolakan bank-wide dari layanan tetap dipetakan ke 403 (bukan 422).
func TestExportKasValasBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{kasValas: &kasValasSvcStub{reportErr: domain.ErrKasValasBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/kas-valas?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportKasValas(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}
