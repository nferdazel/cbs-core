package middleware

import "net/http"

// SecurityHeaders memasang header keamanan dasar untuk API JSON. Respons API tidak
// boleh dimuat sebagai halaman, sehingga CSP paling ketat bisa dipakai.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")

		// HSTS hanya bermakna di atas HTTPS. Di belakang Caddy, TLS ditandai header
		// X-Forwarded-Proto, jadi keduanya diperiksa.
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
