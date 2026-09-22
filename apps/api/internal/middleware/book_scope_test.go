package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// stubConfig hanya melayani GetString; sisanya tidak dipakai middleware cakupan buku.
type stubConfig struct {
	values map[string]string
}

func (s stubConfig) GetString(_ context.Context, key, fallback string) string {
	if v, ok := s.values[key]; ok {
		return v
	}
	return fallback
}
func (s stubConfig) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (s stubConfig) GetInt(context.Context, string, int) int    { return 0 }
func (s stubConfig) GetBool(context.Context, string, bool) bool { return false }
func (s stubConfig) Invalidate(string)                          {}

func claimsRequest(claims *domain.JWTClaims) *http.Request {
	ctx := context.WithValue(context.Background(), domain.ContextKeyClaims, claims)
	return httptest.NewRequest(http.MethodGet, "/api/v1/loans", nil).WithContext(ctx)
}

// Middleware harus mengisi cakupan dari konfigurasi dan, pada instalasi satu buku,
// memaksa buku aktor ke buku aktif agar filter repository ikut berlaku.
func TestBookScopeMiddlewareMenerapkanCakupan(t *testing.T) {
	cases := []struct {
		name      string
		scope     string
		wantScope domain.InstitutionBookScope
		wantBook  domain.COABook
	}{
		{"dual tidak mengubah buku", "DUAL", domain.ScopeDual, domain.BookConventional},
		{"syariah memaksa buku syariah", "SYARIAH", domain.ScopeSyariah, domain.BookSyariah},
		{"konvensional memaksa buku konvensional", "KONVENSIONAL", domain.ScopeConventional, domain.BookConventional},
		{"nilai kosong berarti dual", "", domain.ScopeDual, domain.BookConventional},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := &domain.JWTClaims{Username: "uji", Role: domain.RoleSuperAdmin, Book: domain.BookConventional}
			cfg := stubConfig{values: map[string]string{domain.ConfigKeyInstitutionBookScope: tc.scope}}

			var seenScope domain.InstitutionBookScope
			var seenBook domain.COABook
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, _ := domain.ClaimsFromContext(r.Context())
				seenScope = got.BookScope
				seenBook = got.Book
			})
			BookScopeMiddleware(cfg)(next).ServeHTTP(httptest.NewRecorder(), claimsRequest(claims))

			if seenScope != tc.wantScope {
				t.Fatalf("BookScope=%q, ingin %q", seenScope, tc.wantScope)
			}
			if seenBook != tc.wantBook {
				t.Fatalf("Book=%q, ingin %q", seenBook, tc.wantBook)
			}
		})
	}
}

// Tanpa klaim (rute publik) dan tanpa config service, middleware harus aman/no-op.
func TestBookScopeMiddlewareTanpaKlaimAman(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	BookScopeMiddleware(nil)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, ingin %d", rec.Code, http.StatusNoContent)
	}
}
