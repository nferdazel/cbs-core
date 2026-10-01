package middleware

import (
	"net/http"
	"time"
)

// BatchRateLimiter membatasi laju pemanggilan endpoint batch berat (EOD/EOM/EOY)
// per alamat IP. Berbeda dari LoginRateLimiter yang hanya mencatat percobaan GAGAL,
// limiter ini menghitung SETIAP permintaan yang masuk, karena tujuan di sini adalah
// membatasi beban pemrosesan, bukan brute force.
//
// Ambang sengaja longgar (bawaan 30 permintaan/menit per IP) supaya operator tetap
// dapat menjalankan ulang tutup buku saat gagal, sementara pemanggilan tak terkendali
// dari satu sumber tetap dibatasi.
type BatchRateLimiter struct {
	window *failureWindow
	now    func() time.Time
	stop   chan struct{}
	done   chan struct{}
}

// NewBatchRateLimiter membuat limiter dengan max permintaan per window. Nilai max<=0
// atau window<=0 diisi bawaan (30 / 1 menit) agar konfigurasi tidak lengkap tetap aman.
func NewBatchRateLimiter(max int, window time.Duration) *BatchRateLimiter {
	return newBatchRateLimiter(max, window, time.Now)
}

// newBatchRateLimiter menerima jam yang dapat disuntik agar test tidak perlu tidur.
// now tidak boleh nil.
func newBatchRateLimiter(max int, window time.Duration, now func() time.Time) *BatchRateLimiter {
	if max <= 0 {
		max = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	if now == nil {
		now = time.Now
	}
	l := &BatchRateLimiter{
		window: newFailureWindow(max, window),
		now:    now,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go l.sweepLoop()
	return l
}

// Middleware menolak permintaan yang melewati kuota. Setiap permintaan yang diizinkan
// langsung dicatat (bukan menunggu status), karena byte yang sampai ke handler sudah
// membebani sistem.
func (l *BatchRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := l.now()
		ip := clientIP(r)
		if allowed, retryAfter := l.window.allow(ip, now); !allowed {
			writeRateLimited(w, retryAfter)
			return
		}
		l.window.record(ip, now)
		next.ServeHTTP(w, r)
	})
}

// Close menghentikan goroutine sapuan. Aman dipanggil lebih dari sekali.
func (l *BatchRateLimiter) Close() {
	select {
	case <-l.stop:
		return
	default:
		close(l.stop)
		<-l.done
	}
}

func (l *BatchRateLimiter) sweepLoop() {
	defer close(l.done)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case now := <-ticker.C:
			l.window.sweep(now.UTC())
		}
	}
}
