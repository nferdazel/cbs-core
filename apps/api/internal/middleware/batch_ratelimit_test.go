package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func batchRequest(ip string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/eod", nil)
	req.RemoteAddr = ip + ":43210"
	return req
}

func testBatchLimiter(t *testing.T, max int, window time.Duration) (*BatchRateLimiter, *testClock) {
	t.Helper()
	clock := newTestClock()
	l := newBatchRateLimiter(max, window, clock.Now)
	t.Cleanup(l.Close)
	return l, clock
}

// Limiter menghitung SETIAP permintaan (bukan hanya kegagalan), karena tujuannya
// membatasi beban pemrosesan; permintaan ke-(max+1) dari IP yang sama ditolak 429.
func TestBatchRateLimitBlocksAfterMaxRequests(t *testing.T) {
	l, _ := testBatchLimiter(t, 3, time.Minute)
	h := l.Middleware(statusHandler(http.StatusOK))

	for i := 0; i < 3; i++ {
		if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusOK {
			t.Fatalf("permintaan %d harus 200, dapat %d", i+1, rec.Code)
		}
	}
	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("permintaan ke-4 harus 429, dapat %d", rec.Code)
	}
}

// Kuota berlaku per IP: IP lain tidak ikut terblokir.
func TestBatchRateLimitSeparatesIPs(t *testing.T) {
	l, _ := testBatchLimiter(t, 1, time.Minute)
	h := l.Middleware(statusHandler(http.StatusOK))

	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusOK {
		t.Fatalf("IP pertama harus 200, dapat %d", rec.Code)
	}
	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP pertama kedua harus 429, dapat %d", rec.Code)
	}
	if rec := serve(h, batchRequest("192.0.2.2")); rec.Code != http.StatusOK {
		t.Fatalf("IP lain tidak boleh terblokir, dapat %d", rec.Code)
	}
}

// Setelah jendela lewat, kuota terbuka lagi — penting agar operator bisa mengulang
// tutup buku pada menit berikutnya.
func TestBatchRateLimitReopensAfterWindow(t *testing.T) {
	l, clock := testBatchLimiter(t, 1, time.Minute)
	h := l.Middleware(statusHandler(http.StatusOK))

	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusOK {
		t.Fatalf("permintaan pertama harus 200, dapat %d", rec.Code)
	}
	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("permintaan kedua harus 429, dapat %d", rec.Code)
	}
	clock.advance(2 * time.Minute)
	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusOK {
		t.Fatalf("setelah jendela lewat harus 200, dapat %d", rec.Code)
	}
}

// Nilai konfigurasi tidak wajar diisi bawaan, bukan membuat limiter selalu menolak.
func TestBatchRateLimitDefaultsWhenConfigInvalid(t *testing.T) {
	l, _ := testBatchLimiter(t, 0, 0)
	h := l.Middleware(statusHandler(http.StatusOK))
	if rec := serve(h, batchRequest("192.0.2.1")); rec.Code != http.StatusOK {
		t.Fatalf("permintaan pertama harus 200, dapat %d", rec.Code)
	}
}

// Close aman dipanggil lebih dari sekali.
func TestBatchRateLimiterCloseIdempotent(t *testing.T) {
	l, _ := testBatchLimiter(t, 1, time.Minute)
	l.Close()
	l.Close()
}
