package config

import (
	"log"
	"os"

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
	}

	if cfg.JWTSecret == "" {
		if cfg.Environment == "production" {
			log.Fatal("JWT_SECRET wajib di-set di environment production; server tidak dijalankan dengan secret default")
		}
		cfg.JWTSecret = "dev-only-insecure-secret"
		log.Println("PERINGATAN: JWT_SECRET tidak di-set, memakai secret pengembangan. Jangan dipakai di production.")
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
