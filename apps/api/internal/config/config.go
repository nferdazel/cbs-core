package config

import (
	"log"
	"os"
	"strings"

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

	// Enkripsi data pribadi nasabah (envelope encryption).
	EncryptionKeyID       string
	EncryptionMasterKey   string
	EncryptionPreviousKey map[string]string
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

		EncryptionKeyID:       getEnv("ENCRYPTION_KEY_ID", "k1"),
		EncryptionMasterKey:   os.Getenv("ENCRYPTION_MASTER_KEY"),
		EncryptionPreviousKey: parsePreviousKeys(os.Getenv("ENCRYPTION_PREVIOUS_KEYS")),
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
