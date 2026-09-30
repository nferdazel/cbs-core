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

// hapusBukuSvcStub mengimplementasikan domain.HapusBukuService di memori untuk uji handler.
type hapusBukuSvcStub struct {
	items     []domain.HapusBukuItem
	listErr   error
	upsertErr error
	deleteErr error
	upserted  *domain.UpdateHapusBukuItemInput
	deletedID uuid.UUID
}

func (s *hapusBukuSvcStub) ListHapusBuku(context.Context, domain.Actor) (domain.HapusBukuReport, error) {
	return domain.HapusBukuReport{Items: s.items}, s.listErr
}

func (s *hapusBukuSvcStub) ListItems(context.Context) ([]domain.HapusBukuItem, error) {
	return s.items, s.listErr
}

func (s *hapusBukuSvcStub) UpsertItem(_ context.Context, in domain.UpdateHapusBukuItemInput, _ domain.Actor) (*domain.HapusBukuItem, error) {
	s.upserted = &in
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	item, err := domain.BuildHapusBukuItem(in)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *hapusBukuSvcStub) DeleteItem(_ context.Context, id uuid.UUID, _ domain.Actor) error {
	s.deletedID = id
	return s.deleteErr
}

var _ domain.HapusBukuService = (*hapusBukuSvcStub)(nil)

// Daftar hapus buku Form 15.00 disajikan apa adanya untuk UI pengisian.
func TestListHapusBukuItemsMenyajikanBaris(t *testing.T) {
	h := &OJKReportHandler{hapusBuku: &hapusBukuSvcStub{items: []domain.HapusBukuItem{
		{ID: uuid.New(), JenisAsetCode: domain.HapusBukuJenisAsetKredit,
			HubunganBankCode: domain.HapusBukuHubunganTidakTerkait,
			TanggalHapusBuku: time.Now(), AgunanNilai: decimal.Zero},
	}}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/hapus-buku/items", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.ListHapusBukuItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"jenis_aset_code":"10"`) {
		t.Fatalf("baris tidak disajikan: %s", rec.Body.String())
	}
}

// Upsert tanpa id membuat baris baru.
func TestUpsertHapusBukuItemMembuatBaris(t *testing.T) {
	stub := &hapusBukuSvcStub{}
	h := &OJKReportHandler{hapusBuku: stub}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/hapus-buku/items",
		`{"jenis_aset_code":"10","hubungan_bank_code":"20","tanggal_hapus_buku":"2026-02-10","agunan_nilai":"0"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertHapusBukuItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if stub.upserted == nil || stub.upserted.JenisAsetCode != "10" {
		t.Fatalf("input tidak diteruskan: %+v", stub.upserted)
	}
}

// Sandi di luar baku ditolak 422, bukan diterka.
func TestUpsertHapusBukuSandiSalah422(t *testing.T) {
	h := &OJKReportHandler{hapusBuku: &hapusBukuSvcStub{}}
	rec := httptest.NewRecorder()
	req := ojkRequestWithClaims(http.MethodPut, "/api/v1/reports/ojk/hapus-buku/items",
		`{"jenis_aset_code":"99","hubungan_bank_code":"20","tanggal_hapus_buku":"2026-02-10","agunan_nilai":"0"}`,
		&domain.JWTClaims{Role: domain.RoleAdmin})

	h.UpsertHapusBukuItem(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Baris yang tidak ada saat dihapus dipetakan 404.
func TestDeleteHapusBukuTidakAda404(t *testing.T) {
	h := &OJKReportHandler{hapusBuku: &hapusBukuSvcStub{deleteErr: domain.ErrHapusBukuNotFound}}
	req := hapusBukuDeleteRequest(uuid.New())
	rec := httptest.NewRecorder()

	h.DeleteHapusBukuItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404; body=%s", rec.Code, rec.Body.String())
	}
}

// Ekspor yang ditolak bank-wide dipetakan 403, bukan 500.
func TestExportHapusBukuBankWideKe403(t *testing.T) {
	h := &OJKReportHandler{hapusBuku: &hapusBukuSvcStub{listErr: domain.ErrHapusBukuBankWide}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/hapus-buku?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"})
	rec := httptest.NewRecorder()

	h.ExportHapusBuku(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403; body=%s", rec.Code, rec.Body.String())
	}
}

func hapusBukuDeleteRequest(id uuid.UUID) *http.Request {
	req := ojkRequestWithClaims(http.MethodDelete, "/api/v1/reports/ojk/hapus-buku/items/"+id.String(), "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
