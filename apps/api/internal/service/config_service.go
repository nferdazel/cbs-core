package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// configCacheTTL adalah masa berlaku cache konfigurasi. Limit dibaca pada setiap
// transaksi; cache menghindari satu query tambahan per transaksi. Perubahan
// konfigurasi dapat langsung berlaku lewat Invalidate.
const configCacheTTL = 60 * time.Second

type cachedConfigValue struct {
	value     string
	expiresAt time.Time
}

type systemConfigService struct {
	repo  domain.SystemConfigRepository
	mu    sync.Mutex
	cache map[string]cachedConfigValue
}

func NewSystemConfigService(repo domain.SystemConfigRepository) domain.SystemConfigService {
	return &systemConfigService{
		repo:  repo,
		cache: make(map[string]cachedConfigValue),
	}
}

// get membaca satu key dari cache bila masih berlaku, jika tidak dari repository.
// Key yang gagal dibaca tidak di-cache agar konfigurasi yang baru ditambahkan
// tidak tertahan oleh nilai kosong.
func (s *systemConfigService) get(ctx context.Context, key string) (string, bool) {
	s.mu.Lock()
	if cv, ok := s.cache[key]; ok && time.Now().Before(cv.expiresAt) {
		s.mu.Unlock()
		return cv.value, true
	}
	s.mu.Unlock()

	value, err := s.repo.Get(ctx, key)
	if err != nil {
		return "", false
	}

	s.mu.Lock()
	s.cache[key] = cachedConfigValue{value: value, expiresAt: time.Now().Add(configCacheTTL)}
	s.mu.Unlock()
	return value, true
}

func (s *systemConfigService) GetString(ctx context.Context, key string, fallback string) string {
	if v, ok := s.get(ctx, key); ok {
		return v
	}
	return fallback
}

func (s *systemConfigService) GetDecimal(ctx context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	v, ok := s.get(ctx, key)
	if !ok {
		return fallback
	}
	d, err := decimal.NewFromString(strings.TrimSpace(v))
	if err != nil {
		slog.WarnContext(ctx, "nilai konfigurasi bukan angka desimal; memakai fallback",
			"key", key, "value", v, "fallback", fallback.String())
		return fallback
	}
	return d
}

func (s *systemConfigService) GetInt(ctx context.Context, key string, fallback int) int {
	v, ok := s.get(ctx, key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		slog.WarnContext(ctx, "nilai konfigurasi bukan bilangan bulat; memakai fallback",
			"key", key, "value", v, "fallback", fallback)
		return fallback
	}
	return n
}

func (s *systemConfigService) GetBool(ctx context.Context, key string, fallback bool) bool {
	v, ok := s.get(ctx, key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		slog.WarnContext(ctx, "nilai konfigurasi bukan boolean; memakai fallback",
			"key", key, "value", v, "fallback", fallback)
		return fallback
	}
	return b
}

func (s *systemConfigService) Invalidate(key string) {
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
}
