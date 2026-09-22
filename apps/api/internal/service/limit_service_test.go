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

func (s *stubLimitConfig) GetString(_ context.Context, key, fallback string) string {
	// Kembalikan nilai yang tersimpan agar requiredLimit dapat membedakan kunci
	// kosong dari kunci berisi; fallback hanya dikembalikan bila kunci tidak ada.
	if v, ok := s.values[key]; ok {
		return v.String()
	}
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
	// pending adalah nominal pengajuan PENDING per jenis aksi maker-checker, dipakai
	// membuktikan pengajuan yang belum terposting ikut dihitung.
	pending    map[string]decimal.Decimal
	pendingErr error
	// locked mencatat kunci evaluasi yang diminta, agar test dapat membuktikan
	// evaluasi batas saat eksekusi diserialkan.
	lockErr error
}

func (s *stubDailyDebit) SumDebitByCreatedByAndDate(_ context.Context, _ string, _ time.Time) (decimal.Decimal, error) {
	return s.total, s.err
}

func (s *stubDailyDebit) SumPendingDebitByMakerAndAction(_ context.Context, _, actionType string, _ time.Time) (decimal.Decimal, error) {
	if s.pendingErr != nil {
		return decimal.Zero, s.pendingErr
	}
	return s.pending[actionType], nil
}

func (s *stubDailyDebit) LockDailyEvaluation(_ context.Context, _ any, _, _ string, _ time.Time) error {
	return s.lockErr
}

var _ domain.DailyDebitSumReader = (*stubDailyDebit)(nil)

// stubBusinessDate adalah domain.BusinessDateProvider in-memory dengan satu tanggal
// bisnis tetap. Tanggalnya sengaja bukan hari kalender UTC sekarang, agar test
// membuktikan penjaga batas memakai tanggal bisnis, bukan kalender.
type stubBusinessDate struct {
	date time.Time
	err  error
}

func (s stubBusinessDate) CurrentBusinessDate(context.Context) (time.Time, error) {
	if s.err != nil {
		return time.Time{}, s.err
	}
	return s.date, nil
}

func businessDateStub() stubBusinessDate {
	return stubBusinessDate{date: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)}
}

var _ domain.BusinessDateProvider = stubBusinessDate{}

func tellerActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "teller01", Role: domain.RoleTeller}
}

func tellerLimitConfig() *stubLimitConfig {
	// Mengikuti seed produksi (migrasi 000051): 50 juta per transaksi, 100 juta
	// ambang persetujuan. Urutan inilah yang dulu mematikan jalur persetujuan.
	return &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.teller.deposit.per_transaction": decimal.NewFromInt(50_000_000),
		"limit.teller.deposit.daily":           decimal.NewFromInt(500_000_000),
		"limit.teller.deposit.approval_above":  decimal.NewFromInt(100_000_000),
	}}
}

func TestTransactionLimitService_CheckBelowLimits(t *testing.T) {
	svc := service.NewTransactionLimitService(tellerLimitConfig(), &stubDailyDebit{}, businessDateStub())

	if err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(5_000_000)); err != nil {
		t.Fatalf("nominal di bawah batas seharusnya lolos, got %v", err)
	}
}

func TestTransactionLimitService_CheckExceedsPerTransaction(t *testing.T) {
	svc := service.NewTransactionLimitService(tellerLimitConfig(), &stubDailyDebit{}, businessDateStub())

	// 60 juta: di atas per transaksi (50 juta) tetapi belum melewati ambang
	// persetujuan (100 juta). Harus DITOLAK sebagai batas keras, bukan dialihkan
	// ke persetujuan.
	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(60_000_000))
	if !errors.Is(err, domain.ErrLimitPerTransaction) {
		t.Fatalf("got %v, ingin ErrLimitPerTransaction", err)
	}
}

func TestTransactionLimitService_CheckExceedsApprovalThreshold(t *testing.T) {
	svc := service.NewTransactionLimitService(tellerLimitConfig(), &stubDailyDebit{}, businessDateStub())

	// 150 juta melewati ambang persetujuan 100 juta, sehingga harus meminta
	// persetujuan -- bukan ditolak oleh batas per transaksi 50 juta.
	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(150_000_000))
	if !errors.Is(err, domain.ErrRequiresApproval) {
		t.Fatalf("got %v, ingin ErrRequiresApproval", err)
	}
}

// Peran tanpa batas per transaksi (per_transaction = 0) tidak boleh berubah:
// nominal di bawah ambang persetujuan tetap lolos, di atas ambang tetap minta
// persetujuan, dan batas per transaksi tidak pernah menolak.
func TestTransactionLimitService_CheckUnlimitedPerTransactionUnchanged(t *testing.T) {
	config := &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.admin.deposit.per_transaction": decimal.NewFromInt(0),
		"limit.admin.deposit.daily":           decimal.NewFromInt(0),
		"limit.admin.deposit.approval_above":  decimal.NewFromInt(100_000_000),
	}}
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())
	actor := domain.Actor{UserID: uuid.New(), Username: "admin01", Role: domain.RoleAdmin}

	if err := svc.Check(context.Background(), actor, "DEPOSIT", decimal.NewFromInt(60_000_000)); err != nil {
		t.Fatalf("60 juta (di bawah ambang) pada peran tanpa batas harus lolos, got %v", err)
	}
	if err := svc.Check(context.Background(), actor, "DEPOSIT", decimal.NewFromInt(150_000_000)); !errors.Is(err, domain.ErrRequiresApproval) {
		t.Fatalf("150 juta harus minta persetujuan, got %v", err)
	}
}

func TestTransactionLimitService_CheckExceedsDaily(t *testing.T) {
	config := tellerLimitConfig()
	daily := &stubDailyDebit{total: decimal.NewFromInt(500_000_000)}
	svc := service.NewTransactionLimitService(config, daily, businessDateStub())

	// Nominal kecil (di bawah per transaksi dan ambang persetujuan) tetap ditolak
	// karena akumulasi hari berjalan sudah menyentuh batas harian.
	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(1_000_000))
	if !errors.Is(err, domain.ErrLimitDaily) {
		t.Fatalf("got %v, ingin ErrLimitDaily", err)
	}
}

// P1(a): pengajuan yang masih menunggu persetujuan ikut dihitung sebagai akumulasi
// harian. Tanpa ini, pengajuan kedua yang akan melampaui batas harian (setelah
// digabung dengan pengajuan pertama yang belum terposting) lolos ke antrean.
func TestTransactionLimitService_CheckDailyIncludesPendingApprovals(t *testing.T) {
	config := tellerLimitConfig() // harian teller deposit 500 juta
	// 450 juta tertahan sebagai pengajuan PENDING; belum ada jurnal terposting.
	daily := &stubDailyDebit{pending: map[string]decimal.Decimal{
		service.ActionDeposit: decimal.NewFromInt(450_000_000),
	}}
	svc := service.NewTransactionLimitService(config, daily, businessDateStub())

	// 60 juta sendirian masih di bawah batas harian, tetapi 450 + 60 melewati 500.
	err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(60_000_000))
	if !errors.Is(err, domain.ErrLimitDaily) {
		t.Fatalf("got %v, ingin ErrLimitDaily karena pengajuan PENDING ikut dihitung", err)
	}

	// Nominal kecil yang masih menyisakan ruang di bawah batas tetap lolos.
	if err := svc.Check(context.Background(), tellerActor(), "DEPOSIT", decimal.NewFromInt(10_000_000)); err != nil {
		t.Fatalf("10 juta setelah 450 juta tertahan seharusnya lolos, got %v", err)
	}
}

// P1(b): saat pengajuan dieksekusi, hanya jurnal yang sudah terposting yang dihitung
// ulang bersama nominal pengajuan. Nominal di atas ambang persetujuan tetap lolos
// selama batas harian belum terlampaui.
func TestTransactionLimitService_CheckDailyAtExecution(t *testing.T) {
	config := tellerLimitConfig() // harian teller deposit 500 juta, ambang 100 juta

	t.Run("di bawah batas harian lolos meski di atas ambang", func(t *testing.T) {
		svc := service.NewTransactionLimitService(config, &stubDailyDebit{total: decimal.NewFromInt(100_000_000)}, businessDateStub())
		// 150 juta > ambang 100 juta, tetapi 100 + 150 < 500.
		if err := svc.CheckDailyAtExecution(context.Background(), nil, tellerActor(), "DEPOSIT", decimal.NewFromInt(150_000_000)); err != nil {
			t.Fatalf("eksekusi seharusnya lolos, got %v", err)
		}
	})

	t.Run("melampaui batas harian gagal", func(t *testing.T) {
		svc := service.NewTransactionLimitService(config, &stubDailyDebit{total: decimal.NewFromInt(400_000_000)}, businessDateStub())
		err := svc.CheckDailyAtExecution(context.Background(), nil, tellerActor(), "DEPOSIT", decimal.NewFromInt(150_000_000))
		if !errors.Is(err, domain.ErrLimitDaily) {
			t.Fatalf("got %v, ingin ErrLimitDaily", err)
		}
	})

	t.Run("peran tanpa batas harian tidak terpengaruh", func(t *testing.T) {
		unlimited := &stubLimitConfig{values: map[string]decimal.Decimal{
			"limit.admin.deposit.daily": decimal.NewFromInt(0),
		}}
		svc := service.NewTransactionLimitService(unlimited, &stubDailyDebit{total: decimal.NewFromInt(999_000_000)}, businessDateStub())
		actor := domain.Actor{UserID: uuid.New(), Username: "admin01", Role: domain.RoleAdmin}
		if err := svc.CheckDailyAtExecution(context.Background(), nil, actor, "DEPOSIT", decimal.NewFromInt(150_000_000)); err != nil {
			t.Fatalf("peran tanpa batas harian seharusnya lolos, got %v", err)
		}
	})
}

func TestTransactionLimitService_ForActorUsesRoleLowercaseKeys(t *testing.T) {
	config := &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.supervisor.transfer.per_transaction": decimal.NewFromInt(250_000_000),
		"limit.supervisor.transfer.daily":           decimal.NewFromInt(1_000_000_000),
		"limit.supervisor.transfer.approval_above":  decimal.NewFromInt(0),
	}}
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())
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

// Konfigurasi batas yang hilang TIDAK BOLEH jatuh ke angka bawaan kode. Bila kode
// kembali memakai fallback (mis. 50 juta/500 juta/10 juta), test ini gagal: satu-
// satunya perilaku yang benar adalah penolakan dengan galat jelas.
func TestTransactionLimitService_MissingConfigFailsLoudly(t *testing.T) {
	svc := service.NewTransactionLimitService(&stubLimitConfig{}, &stubDailyDebit{}, businessDateStub())

	_, err := svc.ForActor(context.Background(), tellerActor(), "WITHDRAWAL")
	if err == nil {
		t.Fatal("kunci batas yang kosong harus ditolak, bukan memakai angka bawaan")
	}
	if !strings.Contains(err.Error(), "limit.teller.withdrawal.per_transaction") {
		t.Fatalf("galat harus menyebut kunci yang hilang, dapat: %v", err)
	}
}

// Satu kunci yang hilang saja sudah cukup menolak: kode lama akan diam-diam memakai
// bawaan 10 juta untuk approval_above padahal seed bank 50 juta.
func TestTransactionLimitService_MissingApprovalKeyFails(t *testing.T) {
	config := &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.teller.withdrawal.per_transaction": decimal.NewFromInt(50_000_000),
		"limit.teller.withdrawal.daily":           decimal.NewFromInt(500_000_000),
	}}
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())

	if _, err := svc.ForActor(context.Background(), tellerActor(), "WITHDRAWAL"); err == nil {
		t.Fatal("approval_above yang hilang harus ditolak")
	}
	if err := svc.Check(context.Background(), tellerActor(), "WITHDRAWAL", decimal.NewFromInt(5_000_000)); err == nil {
		t.Fatal("Check harus gagal saat kunci batas hilang, bukan memakai angka bawaan")
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
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())
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
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())

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
	svc := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())

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
	svc := service.NewTransactionLimitService(seededConfig, &stubDailyDebit{}, businessDateStub())

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

	// Tanpa kunci di konfigurasi, List menolak dengan galat jelas alih-alih
	// menyajikan batas bawaan yang bukan kebijakan bank.
	empty := service.NewTransactionLimitService(&stubLimitConfig{}, &stubDailyDebit{}, businessDateStub())
	if _, err := empty.List(context.Background()); err == nil {
		t.Fatal("List tanpa kunci harus ditolak, bukan menyajikan batas bawaan")
	}
}
