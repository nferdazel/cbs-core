package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// maxRequestIDLength membatasi panjang id dari klien agar tidak dipakai untuk
// membanjiri log.
const maxRequestIDLength = 64

// RequestID memakai X-Request-ID dari klien bila formatnya aman, atau membuat UUID.
// Nilai dari klien tidak dipercaya apa adanya: hanya alfanumerik dan tanda hubung.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitizeRequestID(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", id)
		ctx := observability.ContextWithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sanitizeRequestID mengembalikan id yang aman, atau string kosong bila tidak valid.
func sanitizeRequestID(raw string) string {
	if raw == "" || len(raw) > maxRequestIDLength {
		return ""
	}
	for _, c := range raw {
		isAlnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlnum && c != '-' && c != '_' {
			return ""
		}
	}
	return raw
}

// AccessLog mencatat satu entri per permintaan. Query string tidak dicatat karena
// bisa memuat nomor rekening atau NIK; yang dicatat hanya path.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Perkaya logger dengan identitas pelaku bila permintaan sudah terautentikasi.
			var reqLogger *slog.Logger
			if claims, ok := domain.ClaimsFromContext(r.Context()); ok {
				reqLogger = observability.WithRequest(logger,
					observability.RequestIDFromContext(r.Context()),
					claims.UserID.String(), string(claims.Role), claims.BranchCode)
			} else {
				reqLogger = observability.WithRequest(logger,
					observability.RequestIDFromContext(r.Context()), "", "", "")
			}

			ctx := observability.ContextWithLogger(r.Context(), reqLogger)
			next.ServeHTTP(ww, r.WithContext(ctx))

			status := ww.Status()
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"latency_ms", time.Since(start).Milliseconds(),
				"remote_ip", r.RemoteAddr,
			}
			if status >= http.StatusInternalServerError {
				reqLogger.Error("permintaan gagal", attrs...)
				return
			}
			reqLogger.Info("permintaan diproses", attrs...)
		})
	}
}

// Recoverer menangkap panic, mencatat stack trace ke log, dan membalas pesan generik.
// Detail internal tidak pernah dikirim ke klien.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqLogger := observability.FromContext(r.Context())
					reqLogger.Error("panic tertangkap saat menangani permintaan",
						"method", r.Method,
						"path", r.URL.Path,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"success":false,"message":"terjadi kesalahan internal"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
