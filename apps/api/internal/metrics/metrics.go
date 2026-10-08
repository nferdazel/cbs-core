// Package metrics menyediakan metrik operasional dalam format Prometheus.
//
// Prinsip yang mengikat paket ini, mengikuti kebijakan repo:
//
//  1. Label metrik TIDAK memuat nilai ber-kardinalitas tinggi atau data pribadi.
//     Yang dipakai hanya metode HTTP, POLA rute (mis. `/api/v1/loans/{id}`, bukan
//     `/api/v1/loans/abc123`), dan kode status. Nomor rekening, NIK, id, dan
//     sejenisnya tidak pernah menjadi label.
//  2. Registry milik paket ini sendiri, bukan registry global bawaan Prometheus,
//     supaya dapat diuji tanpa efek samping antar-test.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry adalah kumpulan metrik pengukuran CBS. Dibuat per-instance (bukan global)
// agar test dapat memakai registry bersih.
type Registry struct {
	reg *prometheus.Registry

	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New membuat Registry lengkap dengan metrik HTTP.
func New() *Registry {
	reg := prometheus.NewRegistry()
	r := &Registry{
		reg: reg,
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cbs_http_requests_total",
				Help: "Jumlah permintaan HTTP menurut metode, pola rute, dan status.",
			},
			[]string{"method", "route", "status"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "cbs_http_request_duration_seconds",
				Help: "Durasi permintaan HTTP menurut metode dan pola rute.",
				// Bucket disesuaikan konteks perbankan: dari sub-milidetik sampai
				// beberapa detik, karena ada laporan berat.
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "route"},
		),
	}
	reg.MustRegister(r.requests, r.duration)

	// Metrik proses Go standar (GC, goroutine, memori) — berguna tanpa biaya kardinalitas.
	reg.MustRegister(collectors.NewGoCollector())
	return r
}

// Handler mengembalikan handler HTTP yang menyajikan metrik dalam format exposition
// Prometheus.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{})
}

// routePattern mengambil pola rute chi bila tersedia. Rute yang tidak dikenali pola-nya
// (mis. 404) dikelompokkan sebagai "unmatched" agar kardinalitas tetap terkendali dan
// path pengguna tidak bocor ke label.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if pattern := rc.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}

// Middleware mencatat jumlah dan durasi permintaan HTTP ke metrik. Dipasang setelah
// router menandai pola rute (chi mengisinya saat ServeHTTP), sehingga pola rute —
// bukan path mentah — yang menjadi label.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, req)

		route := routePattern(req)
		status := strconv.Itoa(ww.status)
		r.requests.WithLabelValues(req.Method, route, status).Inc()
		r.duration.WithLabelValues(req.Method, route).Observe(time.Since(start).Seconds())
	})
}

// statusWriter menangkap kode status respons tanpa mengubah perilaku penulisan.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}
