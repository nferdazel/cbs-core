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

// suratBerhargaSvcStub mengimplementasikan domain.SuratBerhargaRegisterService di memori
// untuk uji handler tanpa basis data.
type suratBerhargaSvcStub struct {
	report      domain.SuratBerhargaReport
	reportErr   error
	items       []domain.SuratBerhargaItem
	upsertErr   error
	deleteErr   error
	lastDeleted uuid.UUID
}

func (s *suratBerhargaSvcStub) SuratBerhargaReport(context.Context, time.Time, domain.Actor) (domain.SuratBerhargaReport, error) {
	return s.report, s.reportErr
}

func (s *suratBerhargaSvcStub) ListItems(context.Context) ([]domain.SuratBerhargaItem, error) {
	return s.items, nil
}

func (s *suratBerhargaSvcStub) UpsertItem(_ context.Context, in domain.UpdateSuratBerhargaItemInput, _ domain.Actor) (*domain.SuratBerhargaItem, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &domain.SuratBerhargaItem{ID: uuid.New(), NomorSuratBerharga: in.NomorSuratBerharga}, nil
}

func (s *suratBerhargaSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.lastDeleted = id
	return s.deleteErr
}

var _ domain.SuratBerhargaRegisterService = (*suratBerhargaSvcStub)(nil)

func suratBerhargaUpsertRequest(body string) *http.Request {
	return ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/surat-berharga/items", body,
		&domain.JWTClaims{Role: domain.RoleAdmin})
}

// Daftar baris register surat berharga disajikan apa adanya untuk UI edit.
func TestListSuratBerhargaItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{suratBerharga: &suratBerhargaSvcStub{items: []domain.SuratBerhargaItem{
		{ID: uuid.New(), NomorSuratBerharga: "ID0000000001", Nominal: decimal.NewFromInt(900)},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/surat-berharga/items", nil)

	h.ListSuratBerhargaItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"nomor_surat_berharga":"ID0000000001"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Payload tidak sah (mis. klasifikasi asing) dipetakan ke 422 lewat layanan.
func TestUpsertSuratBerhargaMeneruskanGalatValidasi(t *testing.T) {
	svc := &suratBerhargaSvcStub{upsertErr: domain.ErrSuratBerhargaInputInvalid}
	h := &OJKReportHandler{suratBerharga: svc}
	rec := httptest.NewRecorder()

	h.UpsertSuratBerhargaItem(rec, suratBerhargaUpsertRequest(`{"nomor_surat_berharga":"ID1"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang hendak dihapus tetapi tidak ada dipetakan ke 404.
func TestDeleteSuratBerhargaBarisTidakAda(t *testing.T) {
	h := &OJKReportHandler{suratBerharga: &suratBerhargaSvcStub{deleteErr: domain.ErrSuratBerhargaNotFound}}
	id := uuid.New()
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/surat-berharga/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.DeleteSuratBerhargaItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Penolakan bank-wide dari layanan tetap dipetakan ke 403 (bukan 422).
func TestExportSuratBerhargaBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{suratBerharga: &suratBerhargaSvcStub{reportErr: domain.ErrSuratBerhargaBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/surat-berharga?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportSuratBerharga(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}
