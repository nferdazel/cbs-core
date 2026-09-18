package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
)

func TestRequestIDAcceptsSafeHeader(t *testing.T) {
	var got string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = observability.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc-123_XYZ")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got != "abc-123_XYZ" {
		t.Fatalf("request id aman harus dipakai apa adanya, dapat %q", got)
	}
	if rec.Header().Get("X-Request-ID") != got {
		t.Fatal("response harus memuat request id yang sama")
	}
}

// Header dari klien tidak dipercaya: karakter aneh bisa dipakai membanjiri atau
// memalsukan log, jadi harus diganti id baru.
func TestRequestIDRejectsUnsafeHeader(t *testing.T) {
	unsafe := []string{
		"id dengan spasi",
		"id\nnewline",
		"<script>alert(1)</script>",
		"id;drop table",
	}

	for _, raw := range unsafe {
		var got string
		h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = observability.RequestIDFromContext(r.Context())
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", raw)
		h.ServeHTTP(httptest.NewRecorder(), req)

		if got == "" || got == raw {
			t.Fatalf("header tidak aman %q harus diganti id baru, dapat %q", raw, got)
		}
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	var got string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = observability.RequestIDFromContext(r.Context())
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if got == "" {
		t.Fatal("request id harus dibuat bila header tidak ada")
	}
}

func TestSecurityHeadersAreSet(t *testing.T) {
	h := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("X-Content-Type-Options harus nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("X-Frame-Options harus DENY")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("CSP harus dipasang")
	}
}
