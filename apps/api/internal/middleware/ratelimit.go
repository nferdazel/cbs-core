package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Pembatasan percobaan login (anti brute force) untuk endpoint
// POST /api/v1/auth/login.
//
// Dua kunci dihitung terpisah:
//   - per akun  : default 5 percobaan gagal / 15 menit.
//   - per IP    : default 20 percobaan gagal / 15 menit.
//
// Alasan ambang:
//   - 5 gagal per 15 menit per akun memberi ruang bagi pengguna sah yang salah
//     mengetik (umumnya 1-3 kali) sekaligus membatasi tebak-menebak terarah
//     terhadap satu akun menjadi maksimum 20/jam. Ini pelengkap, bukan pengganti,
//     lockout akun yang sudah ada di auth service.
//   - 20 gagal per 15 menit per IP membatasi credential spraying lintas banyak
//     akun dari satu sumber tanpa langsung mengunci kantor yang berbagi satu NAT.
//   - Dua kunci dipisah agar satu IP tidak bisa mengunci semua akun, dan satu
//     akun tidak bisa diserang tanpa batas dari banyak IP (botnet).
//
// Hanya respons 401 (kredensial salah) yang dihitung. Login berhasil mereset
// penghitung akun; status lain (400 body rusak, 403 akun nonaktif, 429 lockout,
// 500) tidak menambah hitungan agar pengguna tidak terkunci karena kesalahan
// klien atau gangguan server. Penghitung IP sengaja TIDAK direset oleh login
// berhasil: bila direset, penyerang yang punya satu akun sah bisa memakai akun
// itu untuk membersihkan kuota IP-nya lalu melanjutkan spraying.
//
// BATASAN PENTING: penyimpanan in-memory per proses. Pada deployment multi
// instance/replika, setiap instance menghitung sendiri sehingga ambang efektif
// berlipat jumlah instance. Untuk mode itu, ganti backend ke penyimpanan
// bersama (mis. Redis) dengan atomic INCR + TTL. Selama API dijalankan sebagai
// satu proses di belakang satu Caddy, in-memory sudah memadai.
type LoginRateLimitConfig struct {
	AccountMax    int
	AccountWindow time.Duration
	IPMax         int
	IPWindow      time.Duration
}

// DefaultLoginRateLimitConfig mengembalikan ambang yang dipakai bila env tidak
// di-set. Nilainya didokumentasikan di komentar paket di atas.
func DefaultLoginRateLimitConfig() LoginRateLimitConfig {
	return LoginRateLimitConfig{
		AccountMax:    5,
		AccountWindow: 15 * time.Minute,
		IPMax:         20,
		IPWindow:      15 * time.Minute,
	}
}

const (
	// maxLoginBodyBytes membatasi jumlah byte body yang dibaca untuk mengambil
	// username. Body yang lebih besar tetap diteruskan utuh ke handler, tetapi
	// hanya bagian ini yang di-buffer sehingga memori tidak bisa dibanjiri.
	maxLoginBodyBytes = 8 << 10 // 8 KiB

	// maxLoginKeys membatasi jumlah kunci yang disimpan per jendela. Tanpa ini,
	// penyerang yang mengirim jutaan username unik dapat menumbuhkan peta tanpa
	// batas meskipun tiap entri kedaluwarsa. Bila penuh, kunci dengan stempel
	// terakhir paling lama dibuang.
	maxLoginKeys = 50_000

	// loginSweepInterval menentukan seberapa sering entri kedaluwarsa disapu
	// dari memori. Pruning juga dilakukan saat akses, jadi sapuan ini hanya
	// mencegah memori bertahan lama tanpa trafik.
	loginSweepInterval = time.Minute
)

// LoginRateLimiter menyimpan penghitung kegagalan dua dimensi. Gunakan
// NewLoginRateLimiter, lalu panggil Middleware di rute login dan Close saat
// server berhenti (atau di akhir test) agar goroutine sapuan tidak bocor.
type LoginRateLimiter struct {
	account *failureWindow
	ip      *failureWindow

	now       func() time.Time
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewLoginRateLimiter membuat limiter dengan ambang dari cfg. Nilai nol pada cfg
// diisi default agar konfigurasi yang tidak lengkap tetap aman.
func NewLoginRateLimiter(cfg LoginRateLimitConfig) *LoginRateLimiter {
	return newLoginRateLimiter(cfg, time.Now)
}

// newLoginRateLimiter menerima jam yang dapat disuntik agar test tidak perlu
// tidur lama. now tidak boleh nil.
func newLoginRateLimiter(cfg LoginRateLimitConfig, now func() time.Time) *LoginRateLimiter {
	defaults := DefaultLoginRateLimitConfig()
	if cfg.AccountMax <= 0 {
		cfg.AccountMax = defaults.AccountMax
	}
	if cfg.AccountWindow <= 0 {
		cfg.AccountWindow = defaults.AccountWindow
	}
	if cfg.IPMax <= 0 {
		cfg.IPMax = defaults.IPMax
	}
	if cfg.IPWindow <= 0 {
		cfg.IPWindow = defaults.IPWindow
	}
	if now == nil {
		now = time.Now
	}

	l := &LoginRateLimiter{
		account: newFailureWindow(cfg.AccountMax, cfg.AccountWindow),
		ip:      newFailureWindow(cfg.IPMax, cfg.IPWindow),
		now:     now,
		stop:    make(chan struct{}),
	}
	l.wg.Add(1)
	go l.sweepLoop()
	return l
}

// Middleware membungkus handler login. Ia memeriksa kuota sebelum handler
// dijalankan, lalu mencatat kegagalan atau mereset penghitung berdasarkan
// status respons.
func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := l.now()

		ip := clientIP(r)
		username := loginUsername(r)
		key := accountKey(username)

		// Periksa akun lebih dulu; bila username tidak terbaca, batas akun
		// dilewati (permintaan seperti itu tidak mungkin lolos autentikasi)
		// dan hanya batas IP yang berlaku.
		if username != "" {
			if allowed, retryAfter := l.account.allow(key, now); !allowed {
				writeRateLimited(w, retryAfter)
				return
			}
		}
		if allowed, retryAfter := l.ip.allow(ip, now); !allowed {
			writeRateLimited(w, retryAfter)
			return
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		now = l.now()
		switch {
		case rec.status >= http.StatusOK && rec.status < http.StatusMultipleChoices:
			// Pengguna sah berhasil masuk: bebaskan kuota akunnya. Kuota IP
			// dibiarkan karena dipakai bersama banyak akun.
			if username != "" {
				l.account.reset(key)
			}
		case rec.status == http.StatusUnauthorized:
			if username != "" {
				l.account.record(key, now)
			}
			l.ip.record(ip, now)
		}
	})
}

// Close menghentikan goroutine sapuan. Aman dipanggil lebih dari sekali.
func (l *LoginRateLimiter) Close() {
	l.closeOnce.Do(func() { close(l.stop) })
	l.wg.Wait()
}

// sweep membuang seluruh entri kedaluwarsa. Dipakai oleh goroutine periodik dan
// boleh dipanggil langsung dari test.
func (l *LoginRateLimiter) sweep(now time.Time) {
	l.account.sweep(now)
	l.ip.sweep(now)
}

func (l *LoginRateLimiter) sweepLoop() {
	defer l.wg.Done()
	ticker := time.NewTicker(loginSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			l.sweep(l.now())
		}
	}
}

// failureWindow adalah sliding window sederhana berisi stempel waktu kegagalan
// per kunci. Memori dibersihkan saat akses dan oleh sapuan berkala.
type failureWindow struct {
	max     int
	window  time.Duration
	mu      sync.Mutex
	hits    map[string][]time.Time
	maxKeys int
}

func newFailureWindow(max int, window time.Duration) *failureWindow {
	return &failureWindow{
		max:     max,
		window:  window,
		hits:    make(map[string][]time.Time),
		maxKeys: maxLoginKeys,
	}
}

// allow melaporkan apakah permintaan boleh lanjut, dan bila tidak, berapa lama
// lagi sampai satu slot bebas. Pemanggil tidak boleh mengubah map.
func (f *failureWindow) allow(key string, now time.Time) (bool, time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	kept := f.pruneLocked(key, now)
	if len(kept) < f.max {
		return true, 0
	}

	// Entri tertua yang menentukan kapan satu slot bebas.
	retryAfter := kept[0].Add(f.window).Sub(now)
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return false, retryAfter
}

// record mencatat satu kegagalan untuk kunci. Bila jendela sudah penuh, hitungan
// tidak ditambah agar blokir tidak diperpanjang oleh percobaan yang sudah ditolak.
func (f *failureWindow) record(key string, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	kept := f.pruneLocked(key, now)
	if len(kept) >= f.max {
		return
	}
	if _, exists := f.hits[key]; !exists {
		f.evictOldestLocked()
	}
	f.hits[key] = append(f.hits[key], now)
}

// reset menghapus riwayat kegagalan sebuah kunci (dipakai saat login berhasil).
func (f *failureWindow) reset(key string) {
	f.mu.Lock()
	delete(f.hits, key)
	f.mu.Unlock()
}

// pruneLocked membuang stempel waktu di luar jendela. Pemanggil harus memegang mu.
func (f *failureWindow) pruneLocked(key string, now time.Time) []time.Time {
	cutoff := now.Add(-f.window)
	stamps := f.hits[key]
	kept := stamps[:0]
	for _, t := range stamps {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(f.hits, key)
		return nil
	}
	f.hits[key] = kept
	return kept
}

// evictOldestLocked membuang kunci dengan stempel terakhir paling lama saat map
// mencapai batas. Pemanggil harus memegang mu.
func (f *failureWindow) evictOldestLocked() {
	if f.maxKeys <= 0 || len(f.hits) < f.maxKeys {
		return
	}
	oldestKey := ""
	var oldest time.Time
	for key, stamps := range f.hits {
		if len(stamps) == 0 {
			continue
		}
		last := stamps[len(stamps)-1]
		if oldestKey == "" || last.Before(oldest) {
			oldestKey, oldest = key, last
		}
	}
	if oldestKey != "" {
		delete(f.hits, oldestKey)
	}
}

// sweep membuang entri kedaluwarsa seluruh kunci.
func (f *failureWindow) sweep(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for key := range f.hits {
		f.pruneLocked(key, now)
	}
}

// statusRecorder mencatat kode status respons pertama dari handler.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHeader {
		return
	}
	s.wroteHeader = true
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// writeRateLimited membalas 429 dengan Retry-After (detik). Pesannya sengaja
// netral dan tidak membocorkan apakah username terdaftar.
func writeRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int((retryAfter + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "terlalu banyak percobaan masuk, silakan coba lagi nanti")
}

// loginUsername membaca username dari body JSON tanpa menghabiskan body: isinya
// di-buffer maksimal maxLoginBodyBytes lalu digabung kembali dengan sisa body,
// sehingga handler login tetap menerima payload utuh. Bila body tidak valid,
// string kosong dikembalikan dan batas per akun dilewati.
func loginUsername(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	buffered, err := io.ReadAll(io.LimitReader(r.Body, maxLoginBodyBytes))
	// Pulihkan body apa pun hasilnya: byte yang dibuffer + sisa aslinya.
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buffered), r.Body))
	if err != nil {
		return ""
	}

	var body struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(buffered, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Username)
}

// accountKey menormalkan username ke huruf kecil agar variasi kapitalisasi tidak
// dipakai untuk lolos dari batas per akun. Normalisasi hanya memperketat.
func accountKey(username string) string {
	return strings.ToLower(username)
}

// clientIP menentukan alamat klien di belakang reverse proxy.
//
// Aplikasi berada di belakang Caddy, jadi X-Forwarded-For dihormati. Namun
// header itu dapat disisipi klien, dan Caddy MENAMBAHKAN IP yang benar-benar
// dilihatnya di ujung daftar. Karena itu yang dipakai adalah entri paling KANAN
// yang valid, bukan paling kiri yang bisa dipalsukan. X-Real-IP hanya dipakai
// bila X-Forwarded-For tidak ada; operator sebaiknya mengatur Caddy agar selalu
// menimpanya. Bila tidak ada header proxy, dipakai RemoteAddr.
//
// Catatan: chi RealIP global sudah menimpa RemoteAddr dari entri paling kiri
// (rentan spoof), sehingga fungsi ini sengaja membaca header sendiri dan tidak
// mengandalkan RemoteAddr selama header proxy tersedia.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			if ip := normalizeIP(parts[i]); ip != "" {
				return ip
			}
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := normalizeIP(xri); ip != "" {
			return ip
		}
	}

	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip := normalizeIP(host); ip != "" {
		return ip
	}
	return strings.TrimSpace(host)
}

// normalizeIP mengembalikan bentuk kanonik sebuah alamat, atau string kosong
// bila bukan IP. Port opsional dan kurung IPv6 ikut ditangani.
func normalizeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if h, _, err := net.SplitHostPort(raw); err == nil {
		raw = h
	}
	raw = strings.Trim(raw, "[]")
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}
