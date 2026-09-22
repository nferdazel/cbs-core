package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// appInfoStub mengembalikan identitas tetap tanpa menyentuh database.
type appInfoStub struct{ info domain.AppInfo }

func (s appInfoStub) Get(context.Context) domain.AppInfo { return s.info }

// appInfoConfigStub melayani cakupan buku untuk BookScopeMiddleware; hanya kunci
// institution.book_scope yang diberi nilai, sisanya jatuh ke fallback.
type appInfoConfigStub struct{ scope string }

func (s appInfoConfigStub) GetString(_ context.Context, key, fallback string) string {
	if key == domain.ConfigKeyInstitutionBookScope {
		return s.scope
	}
	return fallback
}
func (appInfoConfigStub) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (appInfoConfigStub) GetInt(context.Context, string, int) int    { return 0 }
func (appInfoConfigStub) GetBool(context.Context, string, bool) bool { return false }
func (appInfoConfigStub) Invalidate(string)                          {}

// newAppInfoTestRouter merakit router produksi lengkap. Handler selain identitas
// cukup struct kosong: rutenya tidak memanggilnya, tetapi nilainya tidak boleh nil
// karena router mendaftarkan seluruh rute saat dibangun.
func newAppInfoTestRouter(scope string, info domain.AppInfo) http.Handler {
	return NewRouter(RouterParams{
		CustomerHandler:     &CustomerHandler{},
		AccountHandler:      &AccountHandler{},
		BranchHandler:       &BranchHandler{},
		ProductHandler:      &ProductHandler{},
		LedgerHandler:       &LedgerHandler{},
		AuthHandler:         &AuthHandler{},
		StaffHandler:        &StaffHandler{},
		LoanHandler:         &LoanHandler{},
		MakerCheckerHandler: &MakerCheckerHandler{},
		ReportHandler:       NewReportHandler(reportSvcStub{}),
		CollectionHandler:   &CollectionHandler{},
		IntegrationHandler:  &IntegrationHandler{},
		BatchProcessHandler: &BatchProcessHandler{},
		DocumentHandler:     &DocumentHandler{},
		AppInfoHandler:      NewAppInfoHandler(appInfoStub{info: info}),
		AuthService:         reportAuthStub{role: domain.RoleSuperAdmin},
		ConfigService:       appInfoConfigStub{scope: scope},
	})
}

// TestRuteAppInfoPublikTanpaLoginTanpaDataInternal membuktikan GET /api/v1/app-info
// dapat diakses tanpa autentikasi, hanya memuat identitas publik, dan tetap benar
// pada cakupan buku selain DUAL (cakupan mengatur data, bukan identitas instalasi).
func TestRuteAppInfoPublikTanpaLoginTanpaDataInternal(t *testing.T) {
	want := domain.AppInfo{
		CompanyName: "BPR Contoh Sejahtera",
		DisplayName: "Aplikasi Bank Contoh",
		ShortName:   "ABC",
		Description: "Deskripsi Bank Contoh",
		LogoURL:     "https://contoh.local/logo.png",
	}

	for _, scope := range []string{"DUAL", "SYARIAH", "KONVENSIONAL"} {
		t.Run(scope, func(t *testing.T) {
			router := newAppInfoTestRouter(scope, want)
			// Sengaja TANPA header Authorization: halaman login belum punya sesi.
			req := httptest.NewRequest(http.MethodGet, "/api/v1/app-info", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, mau 200 (body=%s)", rec.Code, rec.Body.String())
			}
			if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "max-age=60") {
				t.Errorf("Cache-Control = %q, mau memuat max-age=60", cache)
			}

			var body struct {
				Success bool           `json:"success"`
				Data    map[string]any `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("respons bukan JSON valid: %v (%s)", err, rec.Body.String())
			}
			if !body.Success {
				t.Fatalf("success = false (%s)", rec.Body.String())
			}

			// Hanya identitas publik yang boleh muncul; tidak ada kunci internal.
			gotKeys := make([]string, 0, len(body.Data))
			for k := range body.Data {
				gotKeys = append(gotKeys, k)
			}
			sort.Strings(gotKeys)
			wantKeys := []string{"company_name", "description", "display_name", "logo_url", "short_name"}
			if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
				t.Fatalf("kunci data = %v, mau %v", gotKeys, wantKeys)
			}

			if body.Data["company_name"] != want.CompanyName ||
				body.Data["display_name"] != want.DisplayName ||
				body.Data["short_name"] != want.ShortName ||
				body.Data["description"] != want.Description ||
				body.Data["logo_url"] != want.LogoURL {
				t.Fatalf("identitas = %v, mau %+v", body.Data, want)
			}
		})
	}
}
