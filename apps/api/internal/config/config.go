package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	RedisHost   string
	RedisPort   string
	Environment string
	JWTSecret   string

	// Cookie sesi & CSRF. Nama dapat dikonfigurasi agar selaras dengan domain
	// deployment; Domain opsional untuk cookie lintas subdomain.
	AccessCookieName  string
	RefreshCookieName string
	CSRFCookieName    string
	CSRFHeaderName    string
	CookieDomain      string

	// Enkripsi data pribadi nasabah (envelope encryption).
	EncryptionKeyID       string
	EncryptionMasterKey   string
	EncryptionPreviousKey map[string]string

	// Kunci indeks pencarian (blind index & token nama), terpisah dari kunci
	// enkripsi. Kosong berarti indeks memakai master key enkripsi seperti sebelumnya,
	// sehingga perilaku dan nilai indeks tidak berubah. Lihat crypto.IndexKeyConfig.
	EncryptionIndexKeyID        string
	EncryptionIndexKey          string
	EncryptionPreviousIndexKeys map[string]string

	// Pembatasan percobaan login (anti brute force). Penghitung disimpan
	// in-memory per proses; pada deployment multi instance nilainya tidak
	// dibagi. Nilai default: 5/15 menit per akun, 20/15 menit per IP.
	LoginRateLimitAccountMax    int
	LoginRateLimitAccountWindow time.Duration
	LoginRateLimitIPMax         int
	LoginRateLimitIPWindow      time.Duration
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "cbs_user"),
		DBPassword:  getEnv("DB_PASSWORD", "cbs_password"),
		DBName:      getEnv("DB_NAME", "cbs_db"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
		RedisHost:   getEnv("REDIS_HOST", "localhost"),
		RedisPort:   getEnv("REDIS_PORT", "6379"),
		Environment: getEnv("APP_ENV", "development"),
		JWTSecret:   os.Getenv("JWT_SECRET"),

		AccessCookieName:  getEnv("CBS_ACCESS_COOKIE", "cbs_access_token"),
		RefreshCookieName: getEnv("CBS_REFRESH_COOKIE", "cbs_refresh_token"),
		CSRFCookieName:    getEnv("CBS_CSRF_COOKIE", "csrf_token"),
		CSRFHeaderName:    getEnv("CBS_CSRF_HEADER", "X-CSRF-Token"),
		CookieDomain:      strings.TrimSpace(os.Getenv("CBS_COOKIE_DOMAIN")),

		EncryptionKeyID:       getEnv("ENCRYPTION_KEY_ID", "k1"),
		EncryptionMasterKey:   os.Getenv("ENCRYPTION_MASTER_KEY"),
		EncryptionPreviousKey: parsePreviousKeys(os.Getenv("ENCRYPTION_PREVIOUS_KEYS")),

		// Semua opsional; tanpa ENCRYPTION_INDEX_KEY, kunci indeks = master key.
		EncryptionIndexKeyID:        strings.TrimSpace(os.Getenv("ENCRYPTION_INDEX_KEY_ID")),
		EncryptionIndexKey:          strings.TrimSpace(os.Getenv("ENCRYPTION_INDEX_KEY")),
		EncryptionPreviousIndexKeys: parsePreviousKeys(os.Getenv("ENCRYPTION_PREVIOUS_INDEX_KEYS")),

		LoginRateLimitAccountMax:    getEnvInt("LOGIN_RATE_LIMIT_ACCOUNT_MAX", 5),
		LoginRateLimitAccountWindow: getEnvDuration("LOGIN_RATE_LIMIT_ACCOUNT_WINDOW", 15*time.Minute),
		LoginRateLimitIPMax:         getEnvInt("LOGIN_RATE_LIMIT_IP_MAX", 20),
		LoginRateLimitIPWindow:      getEnvDuration("LOGIN_RATE_LIMIT_IP_WINDOW", 15*time.Minute),
	}

	if cfg.JWTSecret == "" {
		if cfg.Environment == "production" {
			log.Fatal("JWT_SECRET wajib di-set di environment production; server tidak dijalankan dengan secret default")
		}
		cfg.JWTSecret = "dev-only-insecure-secret"
		log.Println("PERINGATAN: JWT_SECRET tidak di-set, memakai secret pengembangan. Jangan dipakai di production.")
	}

	if cfg.EncryptionMasterKey == "" {
		if cfg.Environment == "production" {
			log.Fatal("ENCRYPTION_MASTER_KEY wajib di-set di environment production; data pribadi nasabah tidak boleh disimpan tanpa enkripsi")
		}
		log.Println("PERINGATAN: ENCRYPTION_MASTER_KEY tidak di-set. Endpoint nasabah akan gagal sampai kunci diisi.")
	}

	return cfg
}

// parsePreviousKeys membaca daftar kunci lama untuk dekripsi, format "k0:<base64>,k-1:<base64>".
// Diperlukan saat rotasi master key: data lama tetap bisa dibuka sebelum ditulis ulang.
func parsePreviousKeys(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	keys := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		id, val, ok := strings.Cut(strings.TrimSpace(pair), ":")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(val) == "" {
			continue
		}
		keys[strings.TrimSpace(id)] = strings.TrimSpace(val)
	}
	if len(keys) == 0 {
		return nil
	}
	return keys
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt membaca bilangan bulat positif dari env. Nilai kosong, bukan angka,
// atau <= 0 dianggap tidak valid dan diganti default agar salah ketik tidak
// membuat pembatasan login hilang.
func getEnvInt(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("PERINGATAN: %s=%q tidak valid, memakai default %d", key, raw, defaultVal)
		return defaultVal
	}
	return n
}

// getEnvDuration membaca durasi (mis. "15m") dari env dengan aturan validasi
// yang sama seperti getEnvInt.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("PERINGATAN: %s=%q tidak valid, memakai default %s", key, raw, defaultVal)
		return defaultVal
	}
	return d
}
