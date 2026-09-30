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
	"github.com/shopspring/decimal"
)

// modalSvcStub mengimplementasikan domain.ModalService di memori untuk uji handler.
type modalSvcStub struct {
	items     []domain.ModalItem
	listErr   error
	upsertErr error
	deleteErr error
	upserted  *domain.UpdateModalItemInput
	deletedID uuid.UUID
}

func (s *modalSvcStub) ListModal(context.Context, domain.Actor) (domain.ModalReport, error) {
	return domain.ModalReport{Items: s.items}, s.listErr
}

func (s *modalSvcStub) ListItems(context.Context) ([]domain.ModalItem, error) {
	return s.items, s.listErr
}

func (s *modalSvcStub) UpsertItem(_ context.Context, in domain.UpdateModalItemInput, _ domain.Actor) (*domain.ModalItem, error) {
	s.upserted = &in
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	item, err := domain.BuildModalItem(in)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *modalSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.deletedID = id
	return s.deleteErr
}

var _ domain.ModalService = (*modalSvcStub)(nil)

// Daftar modal Form 00.06 disajikan apa adanya untuk UI pengisian.
func TestListModalItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{modal: &modalSvcStub{items: []domain.ModalItem{
		{ID: uuid.New(), JenisCode: domain.ModalJenisDana,
			JenisModalCode: domain.ModalJenisModalDisetor, Jumlah: decimal.NewFromInt(1000)},
	}}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/modal/items", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.ListModalItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"jenis_modal_code":"01"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Upsert tanpa id membuat baris baru.
func TestUpsertModalItemMembuatBaris(t *testing.T) {
	stub := &modalSvcStub{}
	h := &OJKReportHandler{modal: stub}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/modal/items",
		`{"jenis_code":"01","jenis_modal_code":"01","jumlah":"1000000"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertModalItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if stub.upserted == nil || stub.upserted.JenisModalCode != "01" {
		t.Fatalf("input tidak diteruskan: %+v", stub.upserted)
	}
}

// Sandi di luar baku ditolak 422, bukan diterka.
func TestUpsertModalSandiSalah422(t *testing.T) {
	h := &OJKReportHandler{modal: &modalSvcStub{}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/modal/items",
		`{"jenis_code":"99","jenis_modal_code":"01","jumlah":"1"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertModalItem(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang tidak ada saat dihapus dipetakan 404.
func TestDeleteModalTidakAda404(t *testing.T) {
	h := &OJKReportHandler{modal: &modalSvcStub{deleteErr: domain.ErrModalNotFound}}
	req := modalDeleteRequest(uuid.New())
	rec := httptest.NewRecorder()

	h.DeleteModalItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Ekspor yang ditolak bank-wide dipetakan 403, bukan 500.
func TestExportModalBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{modal: &modalSvcStub{listErr: domain.ErrModalBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/modal?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportModal(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}

func modalDeleteRequest(id uuid.UUID) *http.Request {
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/modal/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
