package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"cbs-core/apps/core-api/internal/i18n"
)

// CSRFMiddleware menerapkan pola double-submit: nilai cookie non-httpOnly
// (default csrf_token) harus sama persis dengan header X-CSRF-Token pada request
// yang mengubah state.
//
// Cakupannya request berbasis cookie. Integrasi non-browser memakai header
// Authorization: Bearer yang tidak dikirim otomatis oleh browser, sehingga tidak
// rentan CSRF dan dibiarkan lewat. Bila cookie sesi ada, CSRF selalu diwajibkan.
//
// Endpoint login dan refresh sengaja TIDAK memakai middleware ini: keduanya
// adalah pintu masuk sesi sehingga belum ada sesi terautentikasi yang bisa
// disalahgunakan. Login justru menetapkan cookie, dan refresh mengandalkan cookie
// refresh httpOnly + SameSite=Strict yang tidak dapat dikirim penyerang lintas
// situs. Setelah login, seluruh endpoint tulis terautentikasi wajib lolos CSRF.
func CSRFMiddleware(cfg CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			if _, err := r.Cookie(cfg.AccessName); err != nil && hasBearerToken(r) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(cfg.CSRFName)
			if err != nil || cookie.Value == "" {
				writeErrorCode(w, http.StatusForbidden, i18n.MsgCSRFMissing)
				return
			}

			header := r.Header.Get(cfg.CSRFHeader)
			if header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				writeErrorCode(w, http.StatusForbidden, i18n.MsgCSRFInvalid)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func hasBearerToken(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
}
