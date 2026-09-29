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

// kreditSindikasiSvcStub mengimplementasikan domain.SindikasiRegisterService di memori
// untuk uji handler tanpa basis data.
type kreditSindikasiSvcStub struct {
	report      domain.SindikasiReport
	reportErr   error
	items       []domain.SindikasiItem
	upsertErr   error
	deleteErr   error
	lastDeleted uuid.UUID
}

func (s *kreditSindikasiSvcStub) SindikasiReport(context.Context, time.Time, domain.Actor) (domain.SindikasiReport, error) {
	return s.report, s.reportErr
}

func (s *kreditSindikasiSvcStub) ListItems(context.Context) ([]domain.SindikasiItem, error) {
	return s.items, nil
}

func (s *kreditSindikasiSvcStub) UpsertItem(_ context.Context, in domain.UpdateSindikasiItemInput, _ domain.Actor) (*domain.SindikasiItem, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &domain.SindikasiItem{ID: uuid.New(), NoRekening: in.NoRekening}, nil
}

func (s *kreditSindikasiSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.lastDeleted = id
	return s.deleteErr
}

var _ domain.SindikasiRegisterService = (*kreditSindikasiSvcStub)(nil)

func kreditSindikasiUpsertRequest(body string) *http.Request {
	return ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/kredit-sindikasi/items", body,
		&domain.JWTClaims{Role: domain.RoleAdmin})
}

// Daftar baris register kredit sindikasi disajikan apa adanya untuk UI edit.
func TestListKreditSindikasiItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{kreditSindikasi: &kreditSindikasiSvcStub{items: []domain.SindikasiItem{
		{ID: uuid.New(), NoRekening: "001", NomorPerjanjianInduk: "ABC-1", BagianPendanaan: decimal.NewFromInt(200)},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/kredit-sindikasi/items", nil)

	h.ListKreditSindikasiItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"no_rekening":"001"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Payload tidak sah dipetakan ke 422 lewat layanan.
func TestUpsertKreditSindikasiMeneruskanGalatValidasi(t *testing.T) {
	svc := &kreditSindikasiSvcStub{upsertErr: domain.ErrSindikasiInputInvalid}
	h := &OJKReportHandler{kreditSindikasi: svc}
	rec := httptest.NewRecorder()

	h.UpsertKreditSindikasiItem(rec, kreditSindikasiUpsertRequest(`{"counterparty_id":"CP-1"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Nomor rekening yang sudah dipakai adalah pelanggaran aturan isian bank, tetap 422.
func TestUpsertKreditSindikasiNomorTerpakaiTetap422(t *testing.T) {
	svc := &kreditSindikasiSvcStub{upsertErr: domain.ErrSindikasiNoRekeningUsed}
	h := &OJKReportHandler{kreditSindikasi: svc}
	rec := httptest.NewRecorder()

	h.UpsertKreditSindikasiItem(rec, kreditSindikasiUpsertRequest(`{"no_rekening":"001"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang hendak dinonaktifkan tetapi tidak ada dipetakan ke 404.
func TestDeleteKreditSindikasiBarisTidakAda(t *testing.T) {
	h := &OJKReportHandler{kreditSindikasi: &kreditSindikasiSvcStub{deleteErr: domain.ErrSindikasiNotFound}}
	id := uuid.New()
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/kredit-sindikasi/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.DeleteKreditSindikasiItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Penolakan bank-wide dari layanan tetap dipetakan ke 403 (bukan 422).
func TestExportKreditSindikasiBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{kreditSindikasi: &kreditSindikasiSvcStub{reportErr: domain.ErrSindikasiBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/kredit-sindikasi?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportKreditSindikasi(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}
