package middleware

import (
	"net"
	"net/http"
	"strings"
)

// MaxBodyBytes adalah batas ukuran body permintaan. API ini hanya menerima JSON
// kecil, jadi batas ini menutup pengiriman body raksasa yang menguras memori dan
// koneksi sebelum handler sempat membaca satu field pun.
const MaxBodyBytes int64 = 1 << 20 // 1 MiB

// LimitBodySize membatasi ukuran body permintaan. Pembacaan yang melewati batas
// menghasilkan error dari MaxBytesReader, sehingga handler membalas kegagalan input
// alih-alih memuat body tanpa batas.
func LimitBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ProxyIP mengisi alamat klien dari entri X-Forwarded-For paling KANAN, yaitu alamat
// yang dicatat proxy tepercaya di depan API. Entri paling kiri berasal dari klien dan
// dapat dipalsukan dengan mengirim header sendiri, sehingga tidak boleh dipakai untuk
// jejak audit maupun pembatasan login. Tanpa header itu, alamat peer dipakai apa adanya
// tanpa port agar formatnya seragam dengan hasil dari header.
func ProxyIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := clientIPFromForwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
			r.RemoteAddr = ip
		} else if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			r.RemoteAddr = host
		}
		next.ServeHTTP(w, r)
	})
}

// clientIPFromForwardedFor mengambil entri terakhir yang tidak kosong dari header
// X-Forwarded-For. Proxy yang tepercaya menambahkan alamat peer di ujung daftar, jadi
// entri terakhir adalah satu-satunya yang tidak dapat ditentukan klien.
func clientIPFromForwardedFor(header string) string {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if ip := strings.TrimSpace(parts[i]); ip != "" {
			return ip
		}
	}
	return ""
}
