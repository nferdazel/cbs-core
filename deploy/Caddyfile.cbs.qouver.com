# Caddy site configuration untuk cbs.qouver.com & api.qouver.com
# Berkas ini mendokumentasikan konfigurasi yang berjalan di VPS. Sisipkan ke
# /etc/caddy/Caddyfile lalu reload: systemctl reload caddy
#
# Catatan penting soal origin:
#   Halaman web dan API sengaja TIDAK dipisah origin. Cookie sesi bersifat httpOnly
#   dan token CSRF dibaca dari document.cookie, sehingga keduanya hanya bekerja bila
#   permintaan berasal dari origin yang sama dengan halaman. Browser memanggil
#   /api/v1/* pada cbs.qouver.com, lalu Next.js meneruskannya ke container API lewat
#   rewrite di apps/web/next.config.mjs. Karena itu cbs.qouver.com cukup mem-proxy
#   seluruh path ke Next.js dan TIDAK perlu handler API terpisah.
#   api.qouver.com/cbs/* tetap ada untuk konsumen API non-browser (mis. integrasi).

# 1. API gateway untuk konsumen non-browser
api.qouver.com {
	handle /cbs/* {
		uri strip_prefix /cbs
		reverse_proxy 127.0.0.1:8095
	}
}

# 2. Aplikasi backoffice
cbs.qouver.com {
	# Seluruh path, termasuk /api/v1/*, dilayani Next.js yang meneruskan /api/*
	# ke container API di jaringan internal.
	reverse_proxy 127.0.0.1:3005

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
	}

	encode gzip zstd
}
