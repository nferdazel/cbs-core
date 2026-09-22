package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// TestRequirePermissionMengutamakanIzinDatabase membuktikan penjaga rute memakai
// izin efektif dari database, bukan pemetaan peran di kode (keputusan pemilik:
// izin dapat diubah tanpa rilis kode). Bila izin sudah dimuat, peran TIDAK lagi
// menjadi sumber kedua.
func TestRequirePermissionMengutamakanIzinDatabase(t *testing.T) {
	cases := []struct {
		name   string
		claims *domain.JWTClaims
		want   int
	}{
		{
			name:   "peran punya tetapi izin DB kosong ditolak",
			claims: &domain.JWTClaims{Role: domain.RoleSuperAdmin, PermissionsLoaded: true, Permissions: []domain.Permission{}},
			want:   http.StatusForbidden,
		},
		{
			name:   "peran tidak punya tetapi izin DB ada diterima",
			claims: &domain.JWTClaims{Role: domain.RoleTeller, PermissionsLoaded: true, Permissions: []domain.Permission{domain.PermSystemConfig}},
			want:   http.StatusOK,
		},
		{
			name:   "izin belum dimuat jatuh ke peran (kompatibilitas klaim tanpa DB)",
			claims: &domain.JWTClaims{Role: domain.RoleSuperAdmin},
			want:   http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			h := RequirePermission(domain.PermSystemConfig)(next)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
			req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, tc.claims))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status %d, mau %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
