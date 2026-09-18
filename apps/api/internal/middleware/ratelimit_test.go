package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// testClock menggantikan time.Now agar jendela waktu bisa dimajukan tanpa
// time.Sleep. Mutex dipakai karena goroutine sapuan memanggil Now juga.
type testClock struct {
	mu sync.Mutex
	at time.Time
}

func newTestClock() *testClock {
	return &testClock{at: time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	c.at = c.at.Add(d)
	c.mu.Unlock()
}

// testLimiter membuat limiter dengan jam palsu dan memastikan goroutine sapuan
// berhenti di akhir test.
func testLimiter(t *testing.T, cfg LoginRateLimitConfig) (*LoginRateLimiter, *testClock) {
	t.Helper()
	clock := newTestClock()
	l := newLoginRateLimiter(cfg, clock.Now)
	t.Cleanup(l.Close)
	return l, clock
}

// loginAttempt membangun request login dengan body JSON dan RemoteAddr tertentu.
func loginAttempt(username, password, ip string) *http.Request {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(body)))
	req.RemoteAddr = ip + ":43210"
	return req
}

func statusHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLoginRateLimitBlocksAfterRepeatedFailures(t *testing.T) {
	cfg := LoginRateLimitConfig{AccountMax: 3, AccountWindow: 15 * time.Minute, IPMax: 50, IPWindow: 15 * time.Minute}
	l, _ := testLimiter(t, cfg)
	h := l.Middleware(statusHandler(http.StatusUnauthorized))

	for i := 0; i < cfg.AccountMax; i++ {
		if rec := serve(h, loginAttempt("budi", "salah", "10.0.0.1")); rec.Code != http.StatusUnauthorized {
			t.Fatalf("percobaan gagal ke-%d harus 401, dapat %d", i+1, rec.Code)
		}
	}

	rec := serve(h, loginAttempt("budi", "salah", "10.0.0.1"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("setelah ambang harus 429, dapat %d", rec.Code)
	}

	raw := rec.Header().Get("Retry-After")
	if raw == "" {
		t.Fatal("429 harus menyertakan header Retry-After")
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 {
		t.Fatalf("Retry-After harus detik positif, dapat %q", raw)
	}
	if seconds > int((15 * time.Minute).Seconds()) {
		t.Fatalf("Retry-After %d melebihi jendela", seconds)
	}

	if body := rec.Body.String(); strings.Contains(body, "budi") {
		t.Fatalf("pesan 429 tidak boleh membocorkan username: %s", body)
	}
}

func TestLoginRateLimitResetsAccountOnSuccess(t *testing.T) {
	cfg := LoginRateLimitConfig{AccountMax: 3, AccountWindow: time.Minute, IPMax: 50, IPWindow: time.Minute}
	l, _ := testLimiter(t, cfg)
	fail := l.Middleware(statusHandler(http.StatusUnauthorized))
	ok := l.Middleware(statusHandler(http.StatusOK))

	for i := 0; i < cfg.AccountMax-1; i++ {
		if rec := serve(fail, loginAttempt("budi", "salah", "10.0.0.2")); rec.Code != http.StatusUnauthorized {
			t.Fatalf("gagal ke-%d harus 401, dapat %d", i+1, rec.Code)
		}
	}

	if rec := serve(ok, loginAttempt("budi", "benar", "10.0.0.2")); rec.Code != http.StatusOK {
		t.Fatalf("login benar harus 200, dapat %d", rec.Code)
	}

	// Tanpa reset, kegagalan ke-3 sudah terakumulasi dan yang ke-4 diblokir.
	for i := 0; i < cfg.AccountMax-1; i++ {
		if rec := serve(fail, loginAttempt("budi", "salah", "10.0.0.2")); rec.Code != http.StatusUnauthorized {
			t.Fatalf("setelah reset, gagal ke-%d harus 401, dapat %d", i+1, rec.Code)
		}
	}
}

func TestLoginRateLimitSeparatesAccountAndIPKeys(t *testing.T) {
	// Batas akun ketat, batas IP longgar: akun lain dari IP yang sama tetap boleh.
	accountCfg := LoginRateLimitConfig{AccountMax: 2, AccountWindow: time.Minute, IPMax: 50, IPWindow: time.Minute}
	accountLimiter, _ := testLimiter(t, accountCfg)
	accountHandler := accountLimiter.Middleware(statusHandler(http.StatusUnauthorized))

	for i := 0; i < accountCfg.AccountMax; i++ {
		serve(accountHandler, loginAttempt("budi", "salah", "10.0.0.3"))
	}
	if rec := serve(accountHandler, loginAttempt("budi", "salah", "10.0.0.3")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("akun budi harus terkunci, dapat %d", rec.Code)
	}
	if rec := serve(accountHandler, loginAttempt("siti", "salah", "10.0.0.3")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("akun lain dari IP sama tidak boleh ikut terkunci, dapat %d", rec.Code)
	}

	// Batas IP ketat, batas akun longgar: IP lain tidak terpengaruh.
	ipCfg := LoginRateLimitConfig{AccountMax: 100, AccountWindow: time.Minute, IPMax: 2, IPWindow: time.Minute}
	ipLimiter, _ := testLimiter(t, ipCfg)
	ipHandler := ipLimiter.Middleware(statusHandler(http.StatusUnauthorized))

	serve(ipHandler, loginAttempt("u1", "salah", "10.0.0.4"))
	serve(ipHandler, loginAttempt("u2", "salah", "10.0.0.4"))
	if rec := serve(ipHandler, loginAttempt("u3", "salah", "10.0.0.4")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP dengan banyak akun gagal harus diblokir, dapat %d", rec.Code)
	}
	if rec := serve(ipHandler, loginAttempt("u3", "salah", "10.0.0.5")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("IP lain tidak boleh ikut diblokir, dapat %d", rec.Code)
	}
}

func TestLoginRateLimitExpiresEntriesAfterWindow(t *testing.T) {
	cfg := LoginRateLimitConfig{AccountMax: 2, AccountWindow: time.Minute, IPMax: 50, IPWindow: time.Minute}
	l, clock := testLimiter(t, cfg)
	h := l.Middleware(statusHandler(http.StatusUnauthorized))

	serve(h, loginAttempt("budi", "salah", "10.0.0.6"))
	serve(h, loginAttempt("budi", "salah", "10.0.0.6"))
	if rec := serve(h, loginAttempt("budi", "salah", "10.0.0.6")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("harus terkunci sebelum jendela lewat, dapat %d", rec.Code)
	}

	clock.advance(time.Minute + time.Second)
	l.sweep(clock.Now())

	l.account.mu.Lock()
	accountEntries := len(l.account.hits)
	l.account.mu.Unlock()
	if accountEntries != 0 {
		t.Fatalf("entri akun kedaluwarsa harus disapu, tersisa %d", accountEntries)
	}

	if rec := serve(h, loginAttempt("budi", "salah", "10.0.0.6")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("setelah jendela lewat harus boleh mencoba lagi, dapat %d", rec.Code)
	}
}

func TestLoginRateLimitPreservesRequestBody(t *testing.T) {
	cfg := LoginRateLimitConfig{AccountMax: 5, AccountWindow: time.Minute, IPMax: 50, IPWindow: time.Minute}
	l, _ := testLimiter(t, cfg)

	var got struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("handler tidak bisa membaca body setelah middleware: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Body kecil harus utuh.
	if rec := serve(h, loginAttempt("budi", "rahasia", "10.0.0.7")); rec.Code != http.StatusOK {
		t.Fatalf("handler harus 200, dapat %d", rec.Code)
	}
	if got.Username != "budi" || got.Password != "rahasia" {
		t.Fatalf("body kecil berubah: %+v", got)
	}

	// Password lebih besar dari buffer: sisa body tetap harus sampai ke handler.
	bigPassword := strings.Repeat("p", maxLoginBodyBytes*2)
	got = struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{}
	if rec := serve(h, loginAttempt("siti", bigPassword, "10.0.0.8")); rec.Code != http.StatusOK {
		t.Fatalf("handler harus 200 untuk body besar, dapat %d", rec.Code)
	}
	if got.Username != "siti" {
		t.Fatalf("username body besar salah: %q", got.Username)
	}
	if got.Password != bigPassword {
		t.Fatalf("password body besar terpotong: %d byte, mau %d", len(got.Password), len(bigPassword))
	}
}

func TestLoginRateLimitKeysOnProxyAppendedIP(t *testing.T) {
	cfg := LoginRateLimitConfig{AccountMax: 100, AccountWindow: time.Minute, IPMax: 1, IPWindow: time.Minute}
	l, _ := testLimiter(t, cfg)
	h := l.Middleware(statusHandler(http.StatusUnauthorized))

	requestWithXFF := func(xff string) *http.Request {
		req := loginAttempt("korban", "salah", "192.0.2.1")
		req.Header.Set("X-Forwarded-For", xff)
		return req
	}

	// Caddy menambahkan IP asli di kanan; entri kiri dapat dipalsukan.
	if rec := serve(h, requestWithXFF("9.9.9.9, 203.0.113.7")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("percobaan pertama harus 401, dapat %d", rec.Code)
	}
	// Entri kiri diubah, IP asli sama: tetap harus terkunci.
	if rec := serve(h, requestWithXFF("8.8.8.8, 203.0.113.7")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP asli sama harus terkunci walau entri kiri diubah, dapat %d", rec.Code)
	}
	// Entri kiri sama, IP asli berbeda: tidak boleh ikut terkunci.
	if rec := serve(h, requestWithXFF("9.9.9.9, 203.0.113.8")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("IP asli berbeda tidak boleh terkunci, dapat %d", rec.Code)
	}
}

func TestClientIPNormalization(t *testing.T) {
	cases := []struct {
		name string
		set  func(*http.Request)
		want string
	}{
		{"remote addr dengan port", func(r *http.Request) { r.RemoteAddr = "192.0.2.10:1234" }, "192.0.2.10"},
		{"xff entri kanan dipakai", func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.7")
			r.RemoteAddr = "192.0.2.10:1234"
		}, "203.0.113.7"},
		{"x-real-ip fallback", func(r *http.Request) {
			r.Header.Set("X-Real-IP", "203.0.113.9")
			r.RemoteAddr = "192.0.2.10:1234"
		}, "203.0.113.9"},
		{"ipv6 dinormalkan", func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", "2001:0db8::0001")
		}, "2001:db8::1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			tc.set(req)
			if got := clientIP(req); got != tc.want {
				t.Fatalf("clientIP = %q, mau %q", got, tc.want)
			}
		})
	}
}
