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

// asetKeuanganSvcStub mengimplementasikan domain.AsetKeuanganRegisterService di memori
// untuk uji handler tanpa basis data.
type asetKeuanganSvcStub struct {
	report      domain.AsetKeuanganReport
	reportErr   error
	items       []domain.AsetKeuanganItem
	upsertErr   error
	deleteErr   error
	lastDeleted uuid.UUID
}

func (s *asetKeuanganSvcStub) AsetKeuanganReport(context.Context, time.Time, domain.Actor) (domain.AsetKeuanganReport, error) {
	return s.report, s.reportErr
}

func (s *asetKeuanganSvcStub) ListItems(context.Context) ([]domain.AsetKeuanganItem, error) {
	return s.items, nil
}

func (s *asetKeuanganSvcStub) UpsertItem(_ context.Context, in domain.UpdateAsetKeuanganItemInput, _ domain.Actor) (*domain.AsetKeuanganItem, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	return &domain.AsetKeuanganItem{ID: uuid.New(), NoRekening: in.NoRekening}, nil
}

func (s *asetKeuanganSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.lastDeleted = id
	return s.deleteErr
}

var _ domain.AsetKeuanganRegisterService = (*asetKeuanganSvcStub)(nil)

func asetKeuanganUpsertRequest(body string) *http.Request {
	return ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/aset-keuangan/items", body,
		&domain.JWTClaims{Role: domain.RoleAdmin})
}

// Daftar baris register aset keuangan lainnya disajikan apa adanya untuk UI edit.
func TestListAsetKeuanganItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{asetKeuangan: &asetKeuanganSvcStub{items: []domain.AsetKeuanganItem{
		{ID: uuid.New(), NoRekening: "AKL-001", Nominal: decimal.NewFromInt(900)},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/aset-keuangan/items", nil)

	h.ListAsetKeuanganItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"no_rekening":"AKL-001"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Nomor rekening yang sudah dipakai baris lain (termasuk NONAKTIF) dipetakan ke 422
// dengan pesan aset_keuangan_no_rekening_used.
func TestUpsertAsetKeuanganMeneruskanGalatNomorTerpakai(t *testing.T) {
	svc := &asetKeuanganSvcStub{upsertErr: domain.ErrAsetKeuanganNoRekeningUsed}
	h := &OJKReportHandler{asetKeuangan: svc}
	rec := httptest.NewRecorder()

	h.UpsertAsetKeuanganItem(rec, asetKeuanganUpsertRequest(`{"no_rekening":"AKL-001"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tidak boleh dipakai ulang") {
		t.Fatalf("pesan tidak menyebut nomor rekening terpakai: %s", rec.Body.String())
	}
}

// Baris yang hendak dihapus tetapi tidak ada dipetakan ke 404.
func TestDeleteAsetKeuanganBarisTidakAda(t *testing.T) {
	h := &OJKReportHandler{asetKeuangan: &asetKeuanganSvcStub{deleteErr: domain.ErrAsetKeuanganNotFound}}
	id := uuid.New()
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/aset-keuangan/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.DeleteAsetKeuanganItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Penolakan bank-wide dari layanan tetap dipetakan ke 403 (bukan 422).
func TestExportAsetKeuanganBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{asetKeuangan: &asetKeuanganSvcStub{reportErr: domain.ErrAsetKeuanganBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/aset-keuangan?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportAsetKeuangan(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}
