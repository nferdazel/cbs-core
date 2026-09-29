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

// pihakTerkaitSvcStub mengimplementasikan domain.PihakTerkaitService di memori untuk uji
// handler.
type pihakTerkaitSvcStub struct {
	items     []domain.PihakTerkaitItem
	listErr   error
	upsertErr error
	deleteErr error
	upserted  *domain.UpdatePihakTerkaitItemInput
	deletedID uuid.UUID
}

func (s *pihakTerkaitSvcStub) ListPihakTerkait(context.Context, domain.Actor) (domain.PihakTerkaitReport, error) {
	return domain.PihakTerkaitReport{Items: s.items}, s.listErr
}

func (s *pihakTerkaitSvcStub) ListItems(context.Context) ([]domain.PihakTerkaitItem, error) {
	return s.items, s.listErr
}

func (s *pihakTerkaitSvcStub) UpsertItem(_ context.Context, in domain.UpdatePihakTerkaitItemInput, _ domain.Actor) (*domain.PihakTerkaitItem, error) {
	s.upserted = &in
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	item, err := domain.BuildPihakTerkaitItem(in)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *pihakTerkaitSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.deletedID = id
	return s.deleteErr
}

var _ domain.PihakTerkaitService = (*pihakTerkaitSvcStub)(nil)

// Daftar pihak terkait Form 00.05 disajikan apa adanya untuk UI pengisian.
func TestListPihakTerkaitItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{pihakTerkait: &pihakTerkaitSvcStub{items: []domain.PihakTerkaitItem{
		{ID: uuid.New(), Nama: "Budi", JenisCode: domain.PihakTerkaitJenisPerorangan,
			HubunganCode: domain.PihakTerkaitHubunganPengendaliKeluarga},
	}}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/pihak-terkait/items", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.ListPihakTerkaitItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"nama":"Budi"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Upsert tanpa id membuat baris baru dan mengembalikan id-nya.
func TestUpsertPihakTerkaitItemMembuatBaris(t *testing.T) {
	stub := &pihakTerkaitSvcStub{}
	h := &OJKReportHandler{pihakTerkait: stub}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/pihak-terkait/items",
		`{"nama":"Budi","jenis_code":"01","hubungan_code":"01"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertPihakTerkaitItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if stub.upserted == nil || stub.upserted.Nama != "Budi" {
		t.Fatalf("input tidak diteruskan: %+v", stub.upserted)
	}
}

// Sandi IV/V di luar baku ditolak 422, bukan diterka.
func TestUpsertPihakTerkaitSandiSalah422(t *testing.T) {
	h := &OJKReportHandler{pihakTerkait: &pihakTerkaitSvcStub{}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/pihak-terkait/items",
		`{"nama":"Budi","jenis_code":"99","hubungan_code":"01"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertPihakTerkaitItem(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang tidak ada saat dihapus dipetakan 404.
func TestDeletePihakTerkaitTidakAda404(t *testing.T) {
	h := &OJKReportHandler{pihakTerkait: &pihakTerkaitSvcStub{deleteErr: domain.ErrPihakTerkaitNotFound}}
	id := uuid.New()
	req := pihakTerkaitDeleteRequest(id)
	rec := httptest.NewRecorder()

	h.DeletePihakTerkaitItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Ekspor yang ditolak bank-wide dipetakan 403, bukan 500.
func TestExportPihakTerkaitBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{pihakTerkait: &pihakTerkaitSvcStub{listErr: domain.ErrPihakTerkaitBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/pihak-terkait?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportPihakTerkait(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}

func pihakTerkaitDeleteRequest(id uuid.UUID) *http.Request {
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/pihak-terkait/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
