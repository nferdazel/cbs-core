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

// DocumentCSP menggantikan CSP ketat untuk respons dokumen cetak berbentuk HTML.
// Dokumen memuat gaya inline, sedangkan default-src 'none' memblokir <style> sehingga
// slip tercetak tanpa gaya. Selain style-src, batasannya tetap sama ketat: dokumen
// tidak boleh memuat skrip, gambar, atau koneksi ke mana pun.
//
// Handler dokumen memasang <body onload="window.print()">; atribut itu ikut diblokir
// kebijakan ini karena halaman memuat nama dan nomor rekening nasabah. Cetak otomatis
// harus dipicu pemanggil (tombol cetak), bukan skrip di dalam dokumen. Membuka skrip
// inline di sini hanya aman selama setiap nilai yang disisipkan tetap di-escape.
const DocumentCSP = "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'"
