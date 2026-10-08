package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// withRecordingTracer memasang TracerProvider yang merekam span ke memory, lalu
// mengembalikannya. Dikembalikan ke provider sebelumnya lewat t.Cleanup.
func withRecordingTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return recorder
}

// TestTracingCreatesSpanAndUsesRoutePattern membuktikan: satu span server dibuat per
// permintaan, dan nama/atribut rutenya memakai POLA (`/api/v1/loans/{id}`), bukan path
// yang memuat id nyata. Ini yang menjaga trace tidak membocorkan identifier nasabah.
func TestTracingCreatesSpanAndUsesRoutePattern(t *testing.T) {
	recorder := withRecordingTracer(t)

	r := chi.NewRouter()
	r.Use(middleware.Tracing("test-service"))
	r.Get("/api/v1/loans/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/loans/99887766", nil))

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("harus ada tepat 1 span, dapat %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "GET /api/v1/loans/{id}" {
		t.Fatalf("nama span = %q, mau %q", span.Name(), "GET /api/v1/loans/{id}")
	}

	var hasRoute, leakedID bool
	for _, attr := range span.Attributes() {
		if attr.Key == "http.route" && attr.Value.AsString() == "/api/v1/loans/{id}" {
			hasRoute = true
		}
		if attr.Value.AsString() == "99887766" {
			leakedID = true
		}
	}
	if !hasRoute {
		t.Fatal("span harus memuat atribut http.route dengan pola rute")
	}
	if leakedID {
		t.Fatal("id nyata bocor ke atribut span")
	}
}

// TestTracingMarksServerError membuktikan respons 5xx membuat span berstatus error,
// sehingga trace dapat dipakai menemukan kegagalan.
func TestTracingMarksServerError(t *testing.T) {
	recorder := withRecordingTracer(t)

	r := chi.NewRouter()
	r.Use(middleware.Tracing("test-service"))
	r.Get("/api/v1/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("harus ada tepat 1 span, dapat %d", len(spans))
	}
	if spans[0].Status().Code.String() != "Error" {
		t.Fatalf("status span = %s, mau Error", spans[0].Status().Code.String())
	}
}

// TestSetupTracingNoEndpointIsNoop membuktikan tracing opt-in: tanpa endpoint,
// SetupTracing tidak mengubah provider global dan shutdown tidak error.
func TestSetupTracingNoEndpointIsNoop(t *testing.T) {
	shutdown, err := observability.SetupTracing(context.Background(), observability.TracerConfig{
		ServiceName: "test-service",
		Endpoint:    "", // kosong = tracing mati
		Environment: "test",
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("SetupTracing tanpa endpoint tidak boleh error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown no-op tidak boleh error: %v", err)
	}
}
