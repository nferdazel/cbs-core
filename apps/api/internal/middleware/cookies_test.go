package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/middleware"
)

func testCookieConfig() middleware.CookieConfig {
	return middleware.CookieConfig{
		AccessName:  "cbs_access_token",
		RefreshName: "cbs_refresh_token",
		CSRFName:    "csrf_token",
		CSRFHeader:  "X-CSRF-Token",
	}
}

func TestSessionCookiesAreHttpOnlyAndStrict(t *testing.T) {
	cfg := testCookieConfig()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	cfg.SetSessionCookies(rec, req, "access", "refresh", 60, 3600)

	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}
	for _, c := range cookies {
		if !c.HttpOnly {
			t.Fatalf("cookie %s harus HttpOnly", c.Name)
		}
		if !c.Secure {
			t.Fatalf("cookie %s harus Secure di HTTPS", c.Name)
		}
		if c.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie %s harus SameSite=Strict", c.Name)
		}
		if c.Path != "/" {
			t.Fatalf("cookie %s harus Path=/", c.Name)
		}
	}
}

func TestCSRFCookieIsReadableByJS(t *testing.T) {
	cfg := testCookieConfig()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)

	cfg.SetCSRFCookie(rec, req, "token-value", 3600)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].HttpOnly {
		t.Fatal("cookie CSRF tidak boleh HttpOnly agar bisa dibaca JavaScript")
	}
}

func TestAccessTokenPrefersCookieOverHeader(t *testing.T) {
	cfg := testCookieConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.AccessName, Value: "dari-cookie"})
	req.Header.Set("Authorization", "Bearer dari-header")

	if got := cfg.AccessTokenFromRequest(req); got != "dari-cookie" {
		t.Fatalf("cookie harus diprioritaskan, dapat %q", got)
	}
}

func TestAccessTokenFallsBackToBearer(t *testing.T) {
	cfg := testCookieConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer integrasi-token")

	if got := cfg.AccessTokenFromRequest(req); got != "integrasi-token" {
		t.Fatalf("fallback Bearer gagal, dapat %q", got)
	}
}

func TestCSRFMiddlewareAllowsSafeMethod(t *testing.T) {
	cfg := testCookieConfig()
	h := middleware.CSRFMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET harus lolos tanpa CSRF, dapat %d", rec.Code)
	}
}

func TestCSRFMiddlewareAcceptsMatchingToken(t *testing.T) {
	cfg := testCookieConfig()
	h := middleware.CSRFMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers", nil)
	req.AddCookie(&http.Cookie{Name: cfg.CSRFName, Value: "abc123"})
	req.Header.Set(cfg.CSRFHeader, "abc123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("token cocok harus lolos, dapat %d", rec.Code)
	}
}

func TestCSRFMiddlewareRejectsMissingOrMismatched(t *testing.T) {
	cfg := testCookieConfig()
	h := middleware.CSRFMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		cookie string
		header string
	}{
		{"tanpa cookie", "", "abc123"},
		{"tanpa header", "abc123", ""},
		{"tidak cocok", "abc123", "abc124"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/customers", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: cfg.CSRFName, Value: tc.cookie})
			}
			if tc.header != "" {
				req.Header.Set(cfg.CSRFHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("harus 403, dapat %d", rec.Code)
			}
		})
	}
}

// Integrasi non-browser memakai Bearer tanpa cookie sesi; tidak ada risiko CSRF
// karena browser tidak mengirim header Authorization secara otomatis.
func TestCSRFMiddlewareAllowsBearerWithoutSessionCookie(t *testing.T) {
	cfg := testCookieConfig()
	h := middleware.CSRFMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers", nil)
	req.Header.Set("Authorization", "Bearer integrasi-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("Bearer tanpa cookie harus lolos CSRF, dapat %d", rec.Code)
	}
}

// Bila cookie sesi ikut terkirim, CSRF tetap wajib walau ada Bearer.
func TestCSRFMiddlewareEnforcesWhenSessionCookiePresent(t *testing.T) {
	cfg := testCookieConfig()
	h := middleware.CSRFMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers", nil)
	req.AddCookie(&http.Cookie{Name: cfg.AccessName, Value: "access"})
	req.Header.Set("Authorization", "Bearer integrasi-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cookie sesi hadir harus tetap wajib CSRF, dapat %d", rec.Code)
	}
}

func TestGenerateCSRFTokenIsRandomAndNonEmpty(t *testing.T) {
	a, err := middleware.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("generate gagal: %v", err)
	}
	b, err := middleware.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("generate gagal: %v", err)
	}
	if a == "" || len(a) < 32 {
		t.Fatalf("token terlalu pendek: %q", a)
	}
	if a == b {
		t.Fatal("dua token berturut-turut tidak boleh sama")
	}
}
