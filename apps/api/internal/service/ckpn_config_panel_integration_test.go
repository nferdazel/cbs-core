package service_test

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// Uji integrasi keputusan panel konfigurasi (23 Sep 2026) terhadap PostgreSQL
// sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// ckpn_coa_config_migration_integration_test.go.
//
// Membuktikan:
//   - migrasi 000090 mengisi pemetaan akun CKPN syariah HANYA bila kosong, dan
//     peringatan installation_validation.go hilang setelah kedua akun terisi;
//   - nilai yang sudah disesuaikan bank TIDAK ditimpa;
//   - kunci kebijakan umur taksasi agunan ter-seed = 12;
//   - migrasi 000090 TIDAK mengubah ckpn.shadow_mode.enabled (instalasi lama yang
//     sudah true tetap true; yang sudah false tidak dipaksa menjadi true).

const ckpnPanelMigration = "000090_ckpn_syariah_coa_and_collateral_appraisal_policy.up.sql"

func TestIntegrasiMigrasiCKPNSyariahDefaultMengisiHanyaBilaKosong(t *testing.T) {
	e := newMoneyEnv(t)

	// Kembalikan ke nilai seed 000090 agar uji lain pada DB yang sama tidak terkejut.
	t.Cleanup(func() {
		setConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah", "15901")
		setConfigCKPNCOA(t, e, "ckpn.coa.reserve.syariah", "11950")
	})

	// Tiru keadaan pra-000090: kedua kunci ada tetapi masih kosong (seed 000071).
	setConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah", "")
	setConfigCKPNCOA(t, e, "ckpn.coa.reserve.syariah", "")

	jalankanMigrasiCkpnCOABerkas(t, e, ckpnPanelMigration)

	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah"); got != "15901" {
		t.Fatalf("ckpn.coa.expense.syariah = %q, mau 15901", got)
	}
	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.reserve.syariah"); got != "11950" {
		t.Fatalf("ckpn.coa.reserve.syariah = %q, mau 11950", got)
	}

	// Akun syariah yang dipetakan harus benar-benar ada di bagan akun (seed 000042).
	for _, code := range []string{"15901", "11950"} {
		var n int
		if err := e.db.QueryRowContext(e.ctx,
			`SELECT count(*) FROM chart_of_accounts WHERE code = $1`, code).Scan(&n); err != nil {
			t.Fatalf("membaca bagan akun %s: %v", code, err)
		}
		if n == 0 {
			t.Fatalf("akun syariah %s tidak ada di bagan akun; pemetaan akan gagal", code)
		}
	}

	// Peringatan pemetaan syariah harus HILANG setelah kedua akun terisi. Scope awal
	// produksi adalah DUAL (yang juga memproses pembiayaan syariah).
	e.configSvc.Invalidate("ckpn.coa.expense.syariah")
	e.configSvc.Invalidate("ckpn.coa.reserve.syariah")
	if warnings := service.InstallationValidationWarnings(context.Background(), e.configSvc); len(warnings) != 0 {
		t.Fatalf("peringatan pemetaan syariah masih muncul padahal kedua akun terisi: %v", warnings)
	}

	// Idempotent: jalan kedua tidak mengubah apa pun.
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnPanelMigration)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah"); got != "15901" {
		t.Fatalf("jalan kedua mengubah ckpn.coa.expense.syariah menjadi %q", got)
	}

	// Nilai yang sudah disesuaikan bank TIDAK ditimpa.
	setConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah", "15902")
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnPanelMigration)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah"); got != "15902" {
		t.Fatalf("nilai bank ckpn.coa.expense.syariah ditimpa menjadi %q, mau tetap 15902", got)
	}
}

func TestIntegrasiMigrasiPanelSeedUmurTaksasiDanTidakMengubahModeBayangan(t *testing.T) {
	e := newMoneyEnv(t)

	// Kembalikan mode bayangan ke bawaan baru (true) agar tidak mengganggu uji lain.
	t.Cleanup(func() {
		setConfigCKPNCOA(t, e, "ckpn.shadow_mode.enabled", "true")
	})

	// Kunci kebijakan umur taksasi ter-seed 12 bulan.
	if got := bacaConfigCKPNCOA(t, e, "collateral.appraisal.validity_months"); got != "12" {
		t.Fatalf("collateral.appraisal.validity_months = %q, mau 12", got)
	}

	// Instalasi lama dengan mode bayangan true: tetap true.
	setConfigCKPNCOA(t, e, "ckpn.shadow_mode.enabled", "true")
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnPanelMigration)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.shadow_mode.enabled"); got != "true" {
		t.Fatalf("instalasi lama (true) berubah menjadi %q, mau tetap true", got)
	}

	// Instalasi lama yang operatornya mematikan mode bayangan: TIDAK dipaksa menyala.
	setConfigCKPNCOA(t, e, "ckpn.shadow_mode.enabled", "false")
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnPanelMigration)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.shadow_mode.enabled"); got != "false" {
		t.Fatalf("nilai operator (false) berubah menjadi %q; migrasi tidak boleh membalik saklar", got)
	}

	// ckpn.enabled juga tidak disentuh: instalasi berjalan tetap mati sampai onboarding.
	if got := bacaConfigCKPNCOA(t, e, "ckpn.enabled"); got != "false" {
		t.Fatalf("ckpn.enabled berubah menjadi %q; migrasi tidak boleh menyalakan CKPN otomatis", got)
	}
}

// Uji integrasi peringatan kesiapan CKPN: instalasi yang sudah beroperasi (punya
// tanggal bisnis) dengan ckpn.enabled=false harus memunculkan peringatan saat start,
// dan peringatan itu hilang setelah ckpn.enabled=true. Mekanisme ini tidak menyalakan
// apa pun — ia hanya membuat pelanggaran kewajiban sejak 1 Jan 2025 terlihat.
func TestIntegrasiKesiapanCKPNPeringatanInstalasiBeroperasi(t *testing.T) {
	e := newMoneyEnv(t)

	// Kembalikan ke bawaan mati agar uji lain pada DB yang sama tidak terpengaruh.
	t.Cleanup(func() { setConfigCKPNCOA(t, e, "ckpn.enabled", "false") })

	dateRepo := postgres.NewBusinessDateRepository(e.db)

	setConfigCKPNCOA(t, e, "ckpn.enabled", "false")
	if warnings := service.CKPNReadinessWarnings(context.Background(), e.configSvc, dateRepo); len(warnings) != 1 {
		t.Fatalf("CKPN mati pada instalasi beroperasi harus 1 peringatan, dapat %d: %v", len(warnings), warnings)
	}

	setConfigCKPNCOA(t, e, "ckpn.enabled", "true")
	if warnings := service.CKPNReadinessWarnings(context.Background(), e.configSvc, dateRepo); len(warnings) != 0 {
		t.Fatalf("CKPN sudah menyala tidak boleh ada peringatan, dapat %d: %v", len(warnings), warnings)
	}
}
