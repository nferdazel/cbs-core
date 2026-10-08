package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newTestRouter memasang middleware pada router chi dengan satu rute ber-parameter,
// meniru pola nyata (/api/v1/loans/{id}), untuk membuktikan label memakai pola rute.
func newTestRouter(reg *Registry, status int) *chi.Mux {
	r := chi.NewRouter()
	r.Use(reg.Middleware)
	r.Get("/api/v1/loans/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
	return r
}

// scrapedBody menjalankan satu permintaan ke path tertentu lalu membaca /metrics.
func scrapedBody(t *testing.T, reg *Registry, router *chi.Mux, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	mrec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return mrec.Body.String()
}

// TestMiddlewareUsesRoutePatternNotRawPath adalah inti kebijakan: label `route` harus
// berupa pola (`/api/v1/loans/{id}`), bukan path berisi id nyata. Kalau id bocor,
// kardinalitas meledak dan (lebih buruk) identifier pengguna terpapar di metrik.
func TestMiddlewareUsesRoutePatternNotRawPath(t *testing.T) {
	reg := New()
	router := newTestRouter(reg, http.StatusOK)

	body := scrapedBody(t, reg, router, "/api/v1/loans/0011223344")

	if !strings.Contains(body, `route="/api/v1/loans/{id}"`) {
		t.Fatalf("metrik harus memakai pola rute; dapat:\n%s", metricLine(body, "cbs_http_requests_total"))
	}
	if strings.Contains(body, "0011223344") {
		t.Fatalf("id nyata bocor ke metrik:\n%s", body)
	}
}

// TestMiddlewareCountsStatus membuktikan kode status tercatat, termasuk 500.
func TestMiddlewareCountsStatus(t *testing.T) {
	reg := New()
	router := newTestRouter(reg, http.StatusInternalServerError)

	body := scrapedBody(t, reg, router, "/api/v1/loans/abc")

	if !strings.Contains(body, `status="500"`) {
		t.Fatalf("status 500 harus tercatat; dapat:\n%s", metricLine(body, "cbs_http_requests_total"))
	}
}

// TestMiddlewareRecordsDuration membuktikan histogram durasi terisi.
func TestMiddlewareRecordsDuration(t *testing.T) {
	reg := New()
	router := newTestRouter(reg, http.StatusOK)

	body := scrapedBody(t, reg, router, "/api/v1/loans/abc")

	if !strings.Contains(body, "cbs_http_request_duration_seconds_bucket") {
		t.Fatalf("histogram durasi harus disajikan; dapat:\n%s", metricLine(body, "cbs_http_request_duration_seconds"))
	}
}

// TestUnmatchedRouteDoesNotLeakPath membuktikan permintaan yang tidak cocok rute
// (404) dikelompokkan sebagai "unmatched", bukan memakai path mentah yang bisa
// memuat apa saja yang diketik pengguna.
func TestUnmatchedRouteDoesNotLeakPath(t *testing.T) {
	reg := New()
	router := newTestRouter(reg, http.StatusOK)

	body := scrapedBody(t, reg, router, "/tidak/ada/halaman/rahasia123")

	if !strings.Contains(body, `route="unmatched"`) {
		t.Fatalf("rute tak cocok harus dilabeli unmatched; dapat:\n%s", metricLine(body, "cbs_http_requests_total"))
	}
	if strings.Contains(body, "rahasia123") {
		t.Fatalf("path rute tak cocok bocor ke metrik:\n%s", body)
	}
}

// metricLine mengembalikan baris-baris metrik yang memuat nama tertentu, untuk pesan uji.
func metricLine(body, name string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
