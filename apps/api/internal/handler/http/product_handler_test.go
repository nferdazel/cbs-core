package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubProductSvc meniru layanan produk agar handler dan rute dapat diuji tanpa
// database: cukup untuk memetakan error ke status HTTP dan memeriksa masukan.
type stubProductSvc struct {
	product   *domain.BankingProduct
	list      []domain.BankingProduct
	updateErr error
	gotCode   string
	calls     int
}

func (s *stubProductSvc) ListProducts(context.Context) ([]domain.BankingProduct, error) {
	return s.list, nil
}

func (s *stubProductSvc) GetProduct(context.Context, uuid.UUID) (*domain.BankingProduct, error) {
	return nil, domain.ErrProductNotFound
}

func (s *stubProductSvc) GetMapping(context.Context, uuid.UUID, domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return nil, nil
}

func (s *stubProductSvc) UpdateParams(_ context.Context, code string, _ domain.UpdateProductParamsInput, _ domain.Actor) (*domain.BankingProduct, error) {
	s.calls++
	s.gotCode = code
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return s.product, nil
}

var _ domain.ProductService = (*stubProductSvc)(nil)

func productUpdateRequest(t *testing.T, code, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+code, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", code)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.ContextKeyClaims, &domain.JWTClaims{Role: domain.RoleAdmin})
	return req.WithContext(ctx)
}

// Payload yang mencoba mengubah identitas produk (code/family/book) harus ditolak
// sebagai bidang tak dikenal, dan layanan tidak boleh dipanggil sama sekali.
func TestProductHandlerUpdateParams_TolakBidangIdentitas(t *testing.T) {
	for _, body := range []string{
		`{"code":"PMB-LAIN"}`,
		`{"family":"SAVINGS"}`,
		`{"book":"CONVENTIONAL"}`,
		`{"profit_scheme":"INTEREST"}`,
		`{"schedule_method":"FLAT"}`,
		`{"is_active":false}`,
	} {
		t.Run(body, func(t *testing.T) {
			svc := &stubProductSvc{}
			rec := httptest.NewRecorder()
			NewProductHandler(svc).UpdateParams(rec, productUpdateRequest(t, "PMB-MUDHARABAH", body))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, ingin 422", rec.Code)
			}
			if svc.calls != 0 {
				t.Fatalf("layanan dipanggil %d kali untuk payload terlarang", svc.calls)
			}
		})
	}
}

// Payload kosong dipetakan ke 422 dengan pesan yang jelas.
func TestProductHandlerUpdateParams_PayloadKosong422(t *testing.T) {
	svc := &stubProductSvc{updateErr: domain.ErrProductParamsEmpty}
	rec := httptest.NewRecorder()
	NewProductHandler(svc).UpdateParams(rec, productUpdateRequest(t, "PMB-MUDHARABAH", `{}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tidak ada parameter produk yang diubah") {
		t.Fatalf("body = %s, ingin pesan payload kosong", rec.Body.String())
	}
}

// Produk tidak ditemukan dipetakan ke 404.
func TestProductHandlerUpdateParams_ProdukTidakAda404(t *testing.T) {
	svc := &stubProductSvc{updateErr: domain.ErrProductNotFound}
	rec := httptest.NewRecorder()
	NewProductHandler(svc).UpdateParams(rec, productUpdateRequest(t, "TIDAK-ADA", `{"rate_annual":"10"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, ingin 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "produk tidak ditemukan") {
		t.Fatalf("body = %s, ingin pesan produk tidak ditemukan", rec.Body.String())
	}
}

// Validasi rentang dari layanan diteruskan apa adanya dengan status 422.
func TestProductHandlerUpdateParams_Validasi422(t *testing.T) {
	svc := &stubProductSvc{updateErr: domain.ErrBagiHasilNisbahOutOfRange}
	rec := httptest.NewRecorder()
	NewProductHandler(svc).UpdateParams(rec, productUpdateRequest(t, "PMB-MUDHARABAH", `{"profit_sharing_ratio":"40"}`))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pecahan") {
		t.Fatalf("body = %s, ingin pesan bersatuan", rec.Body.String())
	}
}

// Permintaan yang sah memakai kode dari path dan mengembalikan 200.
func TestProductHandlerUpdateParams_Sukses(t *testing.T) {
	svc := &stubProductSvc{product: &domain.BankingProduct{Code: "PMB-MUDHARABAH", ProjectedRevenueRateAnnual: decimal.NewFromInt(12)}}
	rec := httptest.NewRecorder()
	NewProductHandler(svc).UpdateParams(rec, productUpdateRequest(t, "PMB-MUDHARABAH", `{"projected_revenue_rate_annual":"12"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.gotCode != "PMB-MUDHARABAH" {
		t.Fatalf("kode diteruskan = %q, ingin PMB-MUDHARABAH", svc.gotCode)
	}
}

// Rute produksi: perubahan parameter produk hanya untuk ADMIN dan SUPERADMIN;
// peran pengawas maupun cabang ditolak 403 sebelum handler berjalan.
func TestRuteProdukUpdateParams_Izin(t *testing.T) {
	allowed := []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin}
	denied := []domain.StaffRole{domain.RoleSupervisor, domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor}

	for _, role := range allowed {
		t.Run("izin_"+string(role), func(t *testing.T) {
			svc := &stubProductSvc{product: &domain.BankingProduct{Code: "PMB-MUDHARABAH"}}
			router := newProductTestRouter(role, svc)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/products/PMB-MUDHARABAH", strings.NewReader(`{"rate_annual":"10"}`))
			req.Header.Set("Authorization", "Bearer uji")
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s = %d, ingin 200", role, rec.Code)
			}
		})
	}
	for _, role := range denied {
		t.Run("tolak_"+string(role), func(t *testing.T) {
			svc := &stubProductSvc{product: &domain.BankingProduct{Code: "PMB-MUDHARABAH"}}
			router := newProductTestRouter(role, svc)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/products/PMB-MUDHARABAH", strings.NewReader(`{"rate_annual":"10"}`))
			req.Header.Set("Authorization", "Bearer uji")
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s = %d, ingin 403", role, rec.Code)
			}
			if svc.calls != 0 {
				t.Fatalf("%s: layanan dipanggil walau ditolak izin", role)
			}
		})
	}
}

// Daftar produk harus menyaring lini usaha yang tidak aktif di instalasi, termasuk
// bagi peran lintas buku, agar web tidak menawarkan produk yang akan ditolak.
func TestProductHandlerList_MenyaringLiniTidakAktif(t *testing.T) {
	svc := &stubProductSvc{list: []domain.BankingProduct{
		{ID: uuid.New(), Code: "KRD-FLAT", Book: domain.BookConventional},
		{ID: uuid.New(), Code: "PMB-MURABAHAH", Book: domain.BookSyariah},
	}}
	cases := []struct {
		name      string
		scope     domain.InstitutionBookScope
		wantCodes []string
	}{
		{"SYARIAH", domain.ScopeSyariah, []string{"PMB-MURABAHAH"}},
		{"KONVENSIONAL", domain.ScopeConventional, []string{"KRD-FLAT"}},
		{"DUAL", domain.ScopeDual, []string{"KRD-FLAT", "PMB-MURABAHAH"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
			claims := &domain.JWTClaims{Role: domain.RoleSuperAdmin, BookScope: tc.scope}
			if b := tc.scope.SingleBook(); b != "" {
				claims.Book = b
			}
			req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
			rec := httptest.NewRecorder()

			NewProductHandler(svc).List(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, ingin 200", rec.Code)
			}
			var body struct {
				Data []domain.BankingProduct `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("respons bukan JSON valid: %v", err)
			}
			if len(body.Data) != len(tc.wantCodes) {
				t.Fatalf("produk=%v, ingin kode %v", body.Data, tc.wantCodes)
			}
			for i, p := range body.Data {
				if p.Code != tc.wantCodes[i] {
					t.Fatalf("produk[%d]=%q, ingin %q", i, p.Code, tc.wantCodes[i])
				}
			}
		})
	}
}

// newProductTestRouter merakit router produksi. Handler selain produk cukup struct
// kosong karena rute produk tidak memanggilnya, tetapi tidak boleh nil karena router
// mendaftarkan seluruh rute saat dibangun.
func newProductTestRouter(role domain.StaffRole, svc domain.ProductService) http.Handler {
	return NewRouter(RouterParams{
		CustomerHandler:     &CustomerHandler{},
		AccountHandler:      &AccountHandler{},
		BranchHandler:       &BranchHandler{},
		ProductHandler:      NewProductHandler(svc),
		LedgerHandler:       &LedgerHandler{},
		AuthHandler:         &AuthHandler{},
		StaffHandler:        &StaffHandler{},
		LoanHandler:         &LoanHandler{},
		MakerCheckerHandler: &MakerCheckerHandler{},
		ReportHandler:       &ReportHandler{},
		CollectionHandler:   &CollectionHandler{},
		IntegrationHandler:  &IntegrationHandler{},
		BatchProcessHandler: &BatchProcessHandler{},
		DocumentHandler:     &DocumentHandler{},
		AuthService:         reportAuthStub{role: role},
	})
}
