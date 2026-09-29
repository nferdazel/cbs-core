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

// penyertaanSvcStub mengimplementasikan domain.PenyertaanRegisterService di memori untuk
// uji handler tanpa basis data.
type penyertaanSvcStub struct {
	report      domain.PenyertaanReport
	reportErr   error
	items       []domain.PenyertaanItem
	upsertErr   error
	deleteErr   error
	lastDeleted uuid.UUID
}

func (s *penyertaanSvcStub) PenyertaanReport(context.Context, time.Time, domain.Actor) (domain.PenyertaanReport, error) {
	return s.report, s.reportErr
}

func (s *penyertaanSvcStub) ListItems(context.Context) ([]domain.PenyertaanItem, error) {
	return s.items, nil
}

func (s *penyertaanSvcStub) UpsertItem(_ context.Context, in domain.UpdatePenyertaanItemInput, _ domain.Actor) (*domain.PenyertaanItem, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &domain.PenyertaanItem{ID: uuid.New(), NoRegister: in.NoRegister}, nil
}

func (s *penyertaanSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.lastDeleted = id
	return s.deleteErr
}

var _ domain.PenyertaanRegisterService = (*penyertaanSvcStub)(nil)

func penyertaanUpsertRequest(body string) *http.Request {
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/penyertaan/items", body,
		&domain.JWTClaims{Role: domain.RoleAdmin})
	return req
}

// Daftar baris register penyertaan disajikan apa adanya untuk UI edit.
func TestListPenyertaanItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{penyertaan: &penyertaanSvcStub{items: []domain.PenyertaanItem{
		{ID: uuid.New(), NoRegister: "PM-001", JumlahBulanLaporan: decimal.NewFromInt(900)},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/penyertaan/items", nil)

	h.ListPenyertaanItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"no_register":"PM-001"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Nomor register yang sudah dipakai baris lain (termasuk NONAKTIF) dipetakan ke 422
// dengan pesan penyertaan_no_register_used.
func TestUpsertPenyertaanMeneruskanGalatNomorTerpakai(t *testing.T) {
	svc := &penyertaanSvcStub{upsertErr: domain.ErrPenyertaanNoRegisterUsed}
	h := &OJKReportHandler{penyertaan: svc}
	rec := httptest.NewRecorder()

	h.UpsertPenyertaanItem(rec, penyertaanUpsertRequest(`{"no_register":"PM-001"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tidak boleh dipakai ulang") {
		t.Fatalf("pesan tidak menyebut nomor register terpakai: %s", rec.Body.String())
	}
}

// Baris penyertaan yang hendak dihapus tetapi tidak ada dipetakan ke 404.
func TestDeletePenyertaanBarisTidakAda(t *testing.T) {
	h := &OJKReportHandler{penyertaan: &penyertaanSvcStub{deleteErr: domain.ErrPenyertaanNotFound}}
	id := uuid.New()
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/penyertaan/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.DeletePenyertaanItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Penolakan bank-wide dari layanan tetap dipetakan ke 403 (bukan 422).
func TestExportPenyertaanBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{penyertaan: &penyertaanSvcStub{reportErr: domain.ErrPenyertaanBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/penyertaan?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportPenyertaan(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}
