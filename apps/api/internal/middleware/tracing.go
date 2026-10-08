package middleware

import (
	"net/http"

	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing membuka satu span server per permintaan HTTP. Konteks trace diekstrak dari
// header masuk (W3C) agar permintaan yang datang dari layanan lain menyambung menjadi
// satu trace, bukan terputus.
//
// Aturan: atribut span TIDAK memuat data pribadi. Yang dicatat hanya metode, pola rute
// (bukan path berisi id), dan kode status — konsisten dengan kebijakan redaksi
// `observability` dan label metrik.
func Tracing(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Lanjutkan trace dari header masuk bila ada.
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Nama span awal memakai path; diperbarui ke pola rute setelah handler
			// berjalan (saat chi mengisi RoutePattern), agar tidak memuat id.
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			// request_id yang sudah ada ikut sebagai atribut untuk korelasi log<->trace.
			if rid := observability.RequestIDFromContext(ctx); rid != "" {
				span.SetAttributes(attribute.String("cbs.request_id", rid))
			}

			ww := &spanStatusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r.WithContext(ctx))

			// Pola rute chi tersedia setelah handler berjalan; pakai sebagai nama span
			// dan atribut agar aman dan ber-kardinalitas rendah.
			route := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if p := rc.RoutePattern(); p != "" {
					route = p
				}
			}
			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", ww.status),
			)
			if ww.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(ww.status))
			}
		})
	}
}

// spanStatusWriter menangkap kode status tanpa mengubah perilaku penulisan.
type spanStatusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *spanStatusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *spanStatusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}
