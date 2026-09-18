package service_test

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubCountingConfigRepo adalah SystemConfigRepository in-memory yang menghitung
// berapa kali Get dipanggil, untuk memverifikasi cache.
type stubCountingConfigRepo struct {
	values map[string]string
	calls  int
}

func (s *stubCountingConfigRepo) Get(_ context.Context, key string) (string, error) {
	s.calls++
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", errors.New("config key not found: " + key)
}

func (s *stubCountingConfigRepo) Set(_ context.Context, _, _ string, _ uuid.UUID) error { return nil }

func (s *stubCountingConfigRepo) GetAll(_ context.Context) (map[string]string, error) {
	return s.values, nil
}

var _ domain.SystemConfigRepository = (*stubCountingConfigRepo)(nil)

func TestSystemConfigService_GetDecimalValid(t *testing.T) {
	repo := &stubCountingConfigRepo{values: map[string]string{"limit.teller.deposit.per_transaction": "25000000"}}
	svc := service.NewSystemConfigService(repo)

	got := svc.GetDecimal(context.Background(), "limit.teller.deposit.per_transaction", decimal.NewFromInt(1))
	if !got.Equal(decimal.NewFromInt(25_000_000)) {
		t.Fatalf("nilai %s, ingin 25000000", got.String())
	}
}

func TestSystemConfigService_InvalidValueUsesFallback(t *testing.T) {
	repo := &stubCountingConfigRepo{values: map[string]string{"limit.teller.deposit.daily": "bukan-angka"}}
	svc := service.NewSystemConfigService(repo)

	fallback := decimal.NewFromInt(500_000_000)
	got := svc.GetDecimal(context.Background(), "limit.teller.deposit.daily", fallback)
	if !got.Equal(fallback) {
		t.Fatalf("nilai rusak: got %s, ingin fallback %s", got.String(), fallback.String())
	}

	intFallback := 7
	if v := svc.GetInt(context.Background(), "limit.teller.deposit.daily", intFallback); v != intFallback {
		t.Fatalf("GetInt nilai rusak: got %d, ingin fallback %d", v, intFallback)
	}
}

func TestSystemConfigService_CacheAndInvalidate(t *testing.T) {
	repo := &stubCountingConfigRepo{values: map[string]string{"limit.teller.deposit.per_transaction": "1000"}}
	svc := service.NewSystemConfigService(repo)

	key := "limit.teller.deposit.per_transaction"
	for i := 0; i < 2; i++ {
		if got := svc.GetDecimal(context.Background(), key, decimal.Zero); !got.Equal(decimal.NewFromInt(1000)) {
			t.Fatalf("pembacaan ke-%d: got %s, ingin 1000", i+1, got.String())
		}
	}
	if repo.calls != 1 {
		t.Fatalf("repo dipanggil %d kali, ingin 1 (cache berlaku)", repo.calls)
	}

	svc.Invalidate(key)
	repo.values[key] = "2000"
	if got := svc.GetDecimal(context.Background(), key, decimal.Zero); !got.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("setelah Invalidate: got %s, ingin 2000", got.String())
	}
	if repo.calls != 2 {
		t.Fatalf("repo dipanggil %d kali setelah Invalidate, ingin 2", repo.calls)
	}
}

func TestSystemConfigService_MissingKeyUsesFallback(t *testing.T) {
	repo := &stubCountingConfigRepo{values: map[string]string{}}
	svc := service.NewSystemConfigService(repo)

	fallback := decimal.NewFromInt(42)
	if got := svc.GetDecimal(context.Background(), "tidak.ada", fallback); !got.Equal(fallback) {
		t.Fatalf("key hilang: got %s, ingin fallback %s", got.String(), fallback.String())
	}
	if got := svc.GetString(context.Background(), "tidak.ada", "default"); got != "default" {
		t.Fatalf("GetString key hilang: got %q, ingin default", got)
	}
	if got := svc.GetBool(context.Background(), "tidak.ada", true); !got {
		t.Fatalf("GetBool key hilang: got %v, ingin true", got)
	}
}

func TestSystemConfigService_GetBoolValid(t *testing.T) {
	repo := &stubCountingConfigRepo{values: map[string]string{"fitur.aktif": "true", "fitur.mati": "0"}}
	svc := service.NewSystemConfigService(repo)

	if !svc.GetBool(context.Background(), "fitur.aktif", false) {
		t.Fatal("fitur.aktif seharusnya true")
	}
	if svc.GetBool(context.Background(), "fitur.mati", true) {
		t.Fatal("fitur.mati seharusnya false")
	}
}
