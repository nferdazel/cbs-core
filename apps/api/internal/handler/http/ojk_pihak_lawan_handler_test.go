package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// pihakLawanSvcStub mengimplementasikan domain.PihakLawanService di memori untuk uji handler.
type pihakLawanSvcStub struct {
	items     []domain.PihakLawanItem
	listErr   error
	upsertErr error
	deleteErr error
	upserted  *domain.UpdatePihakLawanItemInput
	deletedID uuid.UUID
}

func (s *pihakLawanSvcStub) ListPihakLawan(context.Context, domain.Actor) (domain.PihakLawanReport, error) {
	return domain.PihakLawanReport{Items: s.items}, s.listErr
}

func (s *pihakLawanSvcStub) ListItems(context.Context) ([]domain.PihakLawanItem, error) {
	return s.items, s.listErr
}

func (s *pihakLawanSvcStub) UpsertItem(_ context.Context, in domain.UpdatePihakLawanItemInput, _ domain.Actor) (*domain.PihakLawanItem, error) {
	s.upserted = &in
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	item, err := domain.BuildPihakLawanItem(in)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *pihakLawanSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.deletedID = id
	return s.deleteErr
}

var _ domain.PihakLawanService = (*pihakLawanSvcStub)(nil)

// Daftar pihak lawan Form 00.16 disajikan apa adanya untuk UI pengisian.
func TestListPihakLawanItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{pihakLawan: &pihakLawanSvcStub{items: []domain.PihakLawanItem{
		{ID: uuid.New(), PihakLawanID: "CP-1", Nama: "PT Contoh"},
	}}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/pihak-lawan/items", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.ListPihakLawanItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"nama":"PT Contoh"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Upsert tanpa id membuat baris baru.
func TestUpsertPihakLawanItemMembuatBaris(t *testing.T) {
	stub := &pihakLawanSvcStub{}
	h := &OJKReportHandler{pihakLawan: stub}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/pihak-lawan/items",
		`{"pihak_lawan_id":"CP-1","nama":"PT Contoh","golongan_code":"01"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertPihakLawanItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if stub.upserted == nil || stub.upserted.PihakLawanID != "CP-1" {
		t.Fatalf("input tidak diteruskan: %+v", stub.upserted)
	}
}

// Sandi X di luar baku ditolak 422, bukan diterka.
func TestUpsertPihakLawanSandiSalah422(t *testing.T) {
	h := &OJKReportHandler{pihakLawan: &pihakLawanSvcStub{}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/pihak-lawan/items",
		`{"pihak_lawan_id":"CP-1","nama":"PT Contoh","hubungan_bank_code":"99"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertPihakLawanItem(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang tidak ada saat dihapus dipetakan 404.
func TestDeletePihakLawanTidakAda404(t *testing.T) {
	h := &OJKReportHandler{pihakLawan: &pihakLawanSvcStub{deleteErr: domain.ErrPihakLawanNotFound}}
	req := pihakLawanDeleteRequest(uuid.New())
	rec := httptest.NewRecorder()

	h.DeletePihakLawanItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Ekspor yang ditolak bank-wide dipetakan 403, bukan 500.
func TestExportPihakLawanBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{pihakLawan: &pihakLawanSvcStub{listErr: domain.ErrPihakLawanBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/pihak-lawan?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportPihakLawan(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}

func pihakLawanDeleteRequest(id uuid.UUID) *http.Request {
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/pihak-lawan/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
