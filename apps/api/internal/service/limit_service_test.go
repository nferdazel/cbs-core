package service_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

// Exists membuat stubLimitConfig memenuhi domain.ConfigKeyExister sehingga List dapat
// membedakan nilai yang ditetapkan dari nilai bawaan.
func (s *stubLimitConfig) Exists(_ context.Context, key string) bool {
	_, ok := s.values[key]
	return ok
}

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

// seedBackedLimitConfig membangun stubLimitConfig dari nilai limit.<peran>.<jenis>.<sufiks>
// yang benar-benar di-seed berkas migrasi. Test berikutnya memakai konfigurasi ini untuk
// membuktikan bahwa nilai efektif datang dari seed, bukan dari bawaan kode.
func seedBackedLimitConfig(t *testing.T) *stubLimitConfig {
	t.Helper()
	seeded := parseSystemConfigSeed(t, filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations"))
	values := map[string]decimal.Decimal{}
	for _, role := range service.TransactionLimitRoles() {
		for _, txType := range service.TransactionLimitTypes() {
			for _, suffix := range []string{"per_transaction", "daily", "approval_above"} {
				key := fmt.Sprintf("limit.%s.%s.%s",
					strings.ToLower(string(role)), strings.ToLower(txType), suffix)
				raw, ok := seeded[key]
				if !ok {
					t.Fatalf("kunci %s tidak di-seed migrasi", key)
				}
				d, err := decimal.NewFromString(raw)
				if err != nil {
					t.Fatalf("nilai seed %s = %q bukan desimal: %v", key, raw, err)
				}
				values[key] = d
			}
		}
	}
	return &stubLimitConfig{values: values}
}

func seededLimitValue(t *testing.T, seeded map[string]string, role domain.StaffRole, txType, suffix string) decimal.Decimal {
	t.Helper()
	key := fmt.Sprintf("limit.%s.%s.%s", strings.ToLower(string(role)), strings.ToLower(txType), suffix)
	raw, ok := seeded[key]
	if !ok {
		t.Fatalf("kunci %s tidak di-seed migrasi", key)
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		t.Fatalf("nilai seed %s = %q bukan desimal: %v", key, raw, err)
	}
	return d
}

// TestEffectiveLimitsComeFromSeedForEveryRoleAndType memenuhi verifikasi wajib: untuk
// SETIAP peran dan SETIAP jenis transaksi, nilai efektif yang dikembalikan penjaga batas
// sama dengan nilai yang di-seed migrasi (sumbernya konfigurasi, bukan bawaan kode).
func TestEffectiveLimitsComeFromSeedForEveryRoleAndType(t *testing.T) {
	config := seedBackedLimitConfig(t)
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{})
	seeded := parseSystemConfigSeed(t, filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations"))

	for _, role := range service.TransactionLimitRoles() {
		for _, txType := range service.TransactionLimitTypes() {
			actor := domain.Actor{UserID: uuid.New(), Role: role}
			limit, err := svc.ForActor(context.Background(), actor, txType)
			if err != nil {
				t.Fatalf("ForActor %s/%s: %v", role, txType, err)
			}
			if want := seededLimitValue(t, seeded, role, txType, "per_transaction"); !limit.PerTransaction.Equal(want) {
				t.Fatalf("%s/%s per transaksi %s, ingin %s (dari seed)", role, txType, limit.PerTransaction, want)
			}
			if want := seededLimitValue(t, seeded, role, txType, "daily"); !limit.DailyAmount.Equal(want) {
				t.Fatalf("%s/%s harian %s, ingin %s (dari seed)", role, txType, limit.DailyAmount, want)
			}
			if want := seededLimitValue(t, seeded, role, txType, "approval_above"); !limit.RequiresApprovalAbove.Equal(want) {
				t.Fatalf("%s/%s approval %s, ingin %s (dari seed)", role, txType, limit.RequiresApprovalAbove, want)
			}
		}
	}
}

// TestEffectiveLimitsNotTakenFromPackageDefaults menaruh nilai yang sengaja berbeda
// dari bawaan (50 juta/500 juta/10 juta) di setiap kunci. Bila penjaga batas mengabaikan
// konfigurasi, nilai bawaan yang akan terlihat dan test gagal.
func TestEffectiveLimitsNotTakenFromPackageDefaults(t *testing.T) {
	config := &stubLimitConfig{values: map[string]decimal.Decimal{}}
	for _, role := range service.TransactionLimitRoles() {
		for _, txType := range service.TransactionLimitTypes() {
			base := fmt.Sprintf("limit.%s.%s", strings.ToLower(string(role)), strings.ToLower(txType))
			config.values[base+".per_transaction"] = decimal.NewFromInt(111)
			config.values[base+".daily"] = decimal.NewFromInt(222)
			config.values[base+".approval_above"] = decimal.NewFromInt(333)
		}
	}
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{})

	for _, role := range service.TransactionLimitRoles() {
		for _, txType := range service.TransactionLimitTypes() {
			limit, err := svc.ForActor(context.Background(), domain.Actor{Role: role}, txType)
			if err != nil {
				t.Fatalf("ForActor %s/%s: %v", role, txType, err)
			}
			if !limit.PerTransaction.Equal(decimal.NewFromInt(111)) ||
				!limit.DailyAmount.Equal(decimal.NewFromInt(222)) ||
				!limit.RequiresApprovalAbove.Equal(decimal.NewFromInt(333)) {
				t.Fatalf("%s/%s memakai bawaan, bukan konfigurasi: %+v", role, txType, limit)
			}
		}
	}
}

// TestSeedPreservesWrittenRoleLimitIntent menjaga niat yang tertulis di seed lama
// (role_limit.<PERAN>) agar benar-benar berlaku di kunci baru.
func TestSeedPreservesWrittenRoleLimitIntent(t *testing.T) {
	config := seedBackedLimitConfig(t)
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{})

	intent := map[domain.StaffRole]int64{
		domain.RoleTeller:     50_000_000,
		domain.RoleAO:         250_000_000,
		domain.RoleSupervisor: 500_000_000,
		domain.RoleAdmin:      0, // 0 = tanpa batas
	}
	for role, want := range intent {
		for _, txType := range service.TransactionLimitTypes() {
			limit, err := svc.ForActor(context.Background(), domain.Actor{Role: role}, txType)
			if err != nil {
				t.Fatalf("ForActor %s/%s: %v", role, txType, err)
			}
			if !limit.PerTransaction.Equal(decimal.NewFromInt(want)) {
				t.Fatalf("%s/%s per transaksi %s, ingin %d (niat seed lama)", role, txType, limit.PerTransaction, want)
			}
		}
	}
}

// TestSeedAlignsApprovalWithMakerCheckerThreshold memastikan ambang persetujuan di
// limit.* tidak lagi menimpa ambang maker_checker.*: nilainya sama per jenis transaksi.
func TestSeedAlignsApprovalWithMakerCheckerThreshold(t *testing.T) {
	seeded := parseSystemConfigSeed(t, filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations"))

	for _, tc := range []struct{ txType, action string }{
		{"DEPOSIT", "deposit"},
		{"WITHDRAWAL", "withdrawal"},
		{"TRANSFER", "transfer"},
	} {
		want, ok := seeded["maker_checker."+tc.action+".threshold"]
		if !ok {
			t.Fatalf("maker_checker.%s.threshold tidak di-seed", tc.action)
		}
		for _, role := range service.TransactionLimitRoles() {
			key := fmt.Sprintf("limit.%s.%s.approval_above", strings.ToLower(string(role)), strings.ToLower(tc.txType))
			got, ok := seeded[key]
			if !ok {
				t.Fatalf("kunci %s tidak di-seed", key)
			}
			if got != want {
				t.Fatalf("%s = %s, ingin sama dengan maker_checker.%s.threshold = %s", key, got, tc.action, want)
			}
		}
	}
}

func TestListReportsConfiguredFromKeyPresence(t *testing.T) {
	seededConfig := seedBackedLimitConfig(t)
	svc := service.NewTransactionLimitService(seededConfig, &stubDailyDebit{})

	views, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if want := len(service.TransactionLimitRoles()) * len(service.TransactionLimitTypes()); len(views) != want {
		t.Fatalf("jumlah baris %d, ingin %d", len(views), want)
	}
	for _, v := range views {
		if !v.Configured {
			t.Fatalf("%s/%s configured=false padahal semua kunci di-seed", v.Role, v.TransactionType)
		}
	}

	// Tanpa kunci di konfigurasi, baris harus ditandai belum ditetapkan.
	empty := service.NewTransactionLimitService(&stubLimitConfig{}, &stubDailyDebit{})
	emptyViews, err := empty.List(context.Background())
	if err != nil {
		t.Fatalf("List tanpa kunci: %v", err)
	}
	for _, v := range emptyViews {
		if v.Configured {
			t.Fatalf("%s/%s configured=true padahal tidak ada kunci", v.Role, v.TransactionType)
		}
	}
}
