package service_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// setLimitConfig menyetel satu kunci limit.* di database sungguhan dan membuang cache
// konfigurasi agar perubahan langsung berlaku. Nilai lama dipulihkan lewat t.Cleanup
// saat pertama kunci disentuh.
func setLimitConfig(t *testing.T, e *moneyEnv, key, value string) {
	t.Helper()
	e.simpanPulihkanConfigKunci(t, key)
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi batas transaksi')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

// TestIntegrasiBatasTransaksiDariKonfigurasi membuktikan nilai batas yang tersimpan di
// database benar-benar dipakai penjaga batas, bukan bawaan kode. per_transaction di-set
// ke 123456 (jauh di bawah bawaan 50 juta), sehingga nominal 200.000 PASTI ditolak bila
// konfigurasi dibaca dan PASTI lolos bila bawaan yang dipakai.
func TestIntegrasiBatasTransaksiDariKonfigurasi(t *testing.T) {
	e := newMoneyEnv(t)

	perKey := "limit.teller.deposit.per_transaction"
	dailyKey := "limit.teller.deposit.daily"
	approvalKey := "limit.teller.deposit.approval_above"

	setLimitConfig(t, e, perKey, "123456")
	setLimitConfig(t, e, dailyKey, "0")    // 0 = tanpa batas harian
	setLimitConfig(t, e, approvalKey, "0") // 0 = tanpa ambang persetujuan

	limitSvc := service.NewTransactionLimitService(e.configSvc, postgres.NewLedgerRepository(e.db), postgres.NewBusinessDateRepository(e.db))
	actor := domain.Actor{
		UserID:     e.actor.UserID,
		Username:   "teller.ujibatas",
		Role:       domain.RoleTeller,
		BranchCode: e.actor.BranchCode,
	}

	if err := limitSvc.Check(e.ctx, actor, "deposit", decimal.NewFromInt(200_000)); !errors.Is(err, domain.ErrLimitPerTransaction) {
		t.Fatalf("nominal di atas batas konfigurasi: got %v, ingin ErrLimitPerTransaction", err)
	}

	if err := limitSvc.Check(e.ctx, actor, "deposit", decimal.NewFromInt(100_000)); err != nil {
		t.Fatalf("nominal di bawah batas konfigurasi seharusnya lolos, got %v", err)
	}

	// Pulihkan nilai seed agar test lain di container yang sama tidak terpengaruh.
	t.Cleanup(func() {
		setLimitConfig(t, e, perKey, "50000000")
		setLimitConfig(t, e, dailyKey, "500000000")
		setLimitConfig(t, e, approvalKey, "100000000")
	})
}
