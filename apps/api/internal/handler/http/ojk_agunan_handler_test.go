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

// agunanSvcStub mengimplementasikan domain.AgunanService di memori untuk uji handler.
type agunanSvcStub struct {
	items     []domain.AgunanRow
	listErr   error
	updateErr error
	updatedID uuid.UUID
}

func (s *agunanSvcStub) ListAgunan(context.Context, domain.Actor) ([]domain.AgunanRow, error) {
	return s.items, s.listErr
}

func (s *agunanSvcStub) UpdateAgunan(_ context.Context, id uuid.UUID, _ domain.UpdateAgunanInput, _ domain.Actor) error {
	s.updatedID = id
	return s.updateErr
}

var _ domain.AgunanService = (*agunanSvcStub)(nil)

// Daftar agunan Form 06.01 disajikan apa adanya untuk UI pengisian.
func TestListAgunanItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{agunan: &agunanSvcStub{items: []domain.AgunanRow{
		{ID: uuid.New(), KodeRegister: "AG-1", LoanNumber: "LN-001"},
	}}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/agunan/items", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.ListAgunanItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"kode_register":"AG-1"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Kode register yang sudah dipakai adalah pelanggaran aturan isian bank, dipetakan 422.
func TestUpdateAgunanRegisterTerpakaiTetap422(t *testing.T) {
	h := &OJKReportHandler{agunan: &agunanSvcStub{updateErr: domain.ErrAgunanRegisterUsed}}
	id := uuid.New()
	req := agunanUpdateRequest(id, `{"kode_register":"AG-1"}`)
	rec := httptest.NewRecorder()

	h.UpdateAgunanItem(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Agunan yang tidak ada dipetakan 404.
func TestUpdateAgunanTidakAda404(t *testing.T) {
	h := &OJKReportHandler{agunan: &agunanSvcStub{updateErr: domain.ErrAgunanNotFound}}
	req := agunanUpdateRequest(uuid.New(), `{"kode_register":"AG-1"}`)
	rec := httptest.NewRecorder()

	h.UpdateAgunanItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Ekspor yang ditolak bank-wide dipetakan 403, bukan 500.
func TestExportAgunanBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{agunan: &agunanSvcStub{listErr: domain.ErrAgunanBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/agunan?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportAgunan(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}

func agunanUpdateRequest(id uuid.UUID, body string) *http.Request {
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/agunan/items/"+id.String(), body,
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
