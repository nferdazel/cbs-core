package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubLimitConfig adalah SystemConfigService in-memory: nilai yang tidak ada jatuh
// ke fallback yang diberikan pemanggil.
type stubLimitConfig struct {
	values map[string]decimal.Decimal
}

func (s *stubLimitConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := s.values[key]; ok {
		return v
	}
	return fallback
}

func (s *stubLimitConfig) GetInt(_ context.Context, _ string, fallback int) int { return fallback }

func (s *stubLimitConfig) GetString(_ context.Context, _ string, fallback string) string {
	return fallback
}

func (s *stubLimitConfig) GetBool(_ context.Context, _ string, fallback bool) bool { return fallback }

func (s *stubLimitConfig) Invalidate(_ string) {}

var _ domain.SystemConfigService = (*stubLimitConfig)(nil)

type stubDailyDebit struct {
	total decimal.Decimal
	err   error
}

func (s *stubDailyDebit) SumDebitByCreatedByAndDate(_ context.Context, _ string, _ time.Time) (decimal.Decimal, error) {
	return s.total, s.err
}

var _ domain.DailyDebitSumReader = (*stubDailyDebit)(nil)

func tellerActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "teller01", Role: domain.RoleTeller}
}

func tellerLimitConfig() *stubLimitConfig {
	return &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.teller.deposit.per_transaction": decimal.NewFromInt(50_000_000),
		"limit.teller.deposit.daily":           decimal.NewFromInt(500_000_000),
		"limit.teller.deposit.approval_above":  decimal.NewFromInt(10_000_000),
	}}
}

func TestTransactionLimitService_CheckBelowLimits(t *testing.T) {
	svc := service.NewTransactionLimitService(tellerLimitConfig(), &stubDailyDebit{})

	if err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(5_000_000)); err != nil {
		t.Fatalf("nominal di bawah batas seharusnya lolos, got %v", err)
	}
}

func TestTransactionLimitService_CheckExceedsPerTransaction(t *testing.T) {
	svc := service.NewTransactionLimitService(tellerLimitConfig(), &stubDailyDebit{})

	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(60_000_000))
	if !errors.Is(err, domain.ErrLimitPerTransaction) {
		t.Fatalf("got %v, ingin ErrLimitPerTransaction", err)
	}
}

func TestTransactionLimitService_CheckExceedsApprovalThreshold(t *testing.T) {
	svc := service.NewTransactionLimitService(tellerLimitConfig(), &stubDailyDebit{})

	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(20_000_000))
	if !errors.Is(err, domain.ErrRequiresApproval) {
		t.Fatalf("got %v, ingin ErrRequiresApproval", err)
	}
}

func TestTransactionLimitService_CheckExceedsDaily(t *testing.T) {
	config := tellerLimitConfig()
	daily := &stubDailyDebit{total: decimal.NewFromInt(500_000_000)}
	svc := service.NewTransactionLimitService(config, daily)

	// Nominal kecil (di bawah per transaksi dan ambang persetujuan) tetap ditolak
	// karena akumulasi hari berjalan sudah menyentuh batas harian.
	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(1_000_000))
	if !errors.Is(err, domain.ErrLimitDaily) {
		t.Fatalf("got %v, ingin ErrLimitDaily", err)
	}
}

func TestTransactionLimitService_ForActorUsesRoleLowercaseKeys(t *testing.T) {
	config := &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.supervisor.transfer.per_transaction": decimal.NewFromInt(250_000_000),
		"limit.supervisor.transfer.daily":           decimal.NewFromInt(1_000_000_000),
		"limit.supervisor.transfer.approval_above":  decimal.NewFromInt(0),
	}}
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{})
	actor := domain.Actor{UserID: uuid.New(), Username: "spv01", Role: domain.RoleSupervisor}

	limit, err := svc.ForActor(context.Background(), actor, "TRANSFER")
	if err != nil {
		t.Fatalf("ForActor error: %v", err)
	}
	if !limit.PerTransaction.Equal(decimal.NewFromInt(250_000_000)) {
		t.Fatalf("per transaksi %s, ingin 250000000", limit.PerTransaction.String())
	}
	if !limit.DailyAmount.Equal(decimal.NewFromInt(1_000_000_000)) {
		t.Fatalf("harian %s, ingin 1000000000", limit.DailyAmount.String())
	}
	if !limit.RequiresApprovalAbove.IsZero() {
		t.Fatalf("approval_above %s, ingin 0 (tanpa ambang)", limit.RequiresApprovalAbove.String())
	}
}

func TestTransactionLimitService_DefaultWhenConfigMissing(t *testing.T) {
	svc := service.NewTransactionLimitService(&stubLimitConfig{}, &stubDailyDebit{})

	limit, err := svc.ForActor(context.Background(), tellerActor(), "WITHDRAWAL")
	if err != nil {
		t.Fatalf("ForActor error: %v", err)
	}
	if !limit.PerTransaction.Equal(decimal.NewFromInt(50_000_000)) {
		t.Fatalf("default per transaksi %s, ingin 50000000", limit.PerTransaction.String())
	}
	if !limit.DailyAmount.Equal(decimal.NewFromInt(500_000_000)) {
		t.Fatalf("default harian %s, ingin 500000000", limit.DailyAmount.String())
	}
	if !limit.RequiresApprovalAbove.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("default approval %s, ingin 10000000", limit.RequiresApprovalAbove.String())
	}
}
