package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Entri X-Forwarded-For paling kiri bisa dikirim klien sendiri, jadi alamat yang
// dipakai untuk audit harus entri paling kanan yang ditambahkan proxy tepercaya.
func TestProxyIP_UsesRightmostForwardedFor(t *testing.T) {
	var got string
	handler := ProxyIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.RemoteAddr
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.9.9.9, 203.0.113.7")
	req.RemoteAddr = "127.0.0.1:54321"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != "203.0.113.7" {
		t.Fatalf("alamat klien %q, ingin entri paling kanan 203.0.113.7", got)
	}
}

// Tanpa header proxy, alamat peer tetap dipakai tetapi tanpa port agar seragam.
func TestProxyIP_StripsPortWithoutHeader(t *testing.T) {
	var got string
	handler := ProxyIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.RemoteAddr
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.4:45678"
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != "198.51.100.4" {
		t.Fatalf("alamat klien %q, ingin 198.51.100.4 tanpa port", got)
	}
}

// Body di atas batas harus gagal dibaca, bukan dipotong diam-diam atau dimuat penuh.
func TestLimitBodySize_RejectsOversizedBody(t *testing.T) {
	var readErr error
	handler := LimitBodySize(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("1234567890"))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("error %v, ingin MaxBytesError", readErr)
	}
}

// Body dalam batas tetap terbaca utuh.
func TestLimitBodySize_AllowsBodyWithinLimit(t *testing.T) {
	var body []byte
	handler := LimitBodySize(16)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{\"a\":1}"))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if string(body) != "{\"a\":1}" {
		t.Fatalf("body terbaca %q, ingin utuh", body)
	}
}
