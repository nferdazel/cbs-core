package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// Uji integrasi keputusan panel butir 2/3 dan tahap T0 butir 4 terhadap PostgreSQL
// sungguhan (migrasi 000094). Di-skip kecuali CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiPanelT0 -v

const panelT0Migration = "000094_panel_tazir_collateral_ckpn_t0.up.sql"

func jalankanMigrasiPanelT0(t *testing.T, e *moneyEnv) {
	t.Helper()
	path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", panelT0Migration)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", panelT0Migration, err)
	}
	if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
		t.Fatalf("menjalankan migrasi %s: %v", panelT0Migration, err)
	}
}

func bacaConfigPanelT0(t *testing.T, e *moneyEnv, key string) string {
	t.Helper()
	var value string
	if err := e.db.QueryRowContext(e.ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value); err != nil {
		t.Fatalf("membaca konfigurasi %s: %v", key, err)
	}
	return value
}

// Migrasi 000094: bawaan aman, idempoten, dan TIDAK menimpa nilai bank. Bobot agunan
// tetap 100%/mati dan required_ckpn tidak disentuh.
func TestIntegrasiPanelT0MigrasiBawaanAmanDanIdempotent(t *testing.T) {
	e := newMoneyEnv(t)
	jalankanMigrasiPanelT0(t, e)

	bawaan := map[string]string{
		"loan.penalty.syariah.akad_disclosed":            "false",
		"ckpn.individual.enabled":                        "false",
		"ckpn.individual.significance_amount":            "1000000000",
		"ckpn.individual.significance_top_n":             "20",
		"ckpn.individual.method":                         "MAX",
		"ckpn.individual.mandatory_on_macet":             "true",
		"ckpn.individual.mandatory_on_restructured":      "true",
		"ckpn.individual.mandatory_dpd_days":             "90",
		"collateral.weight.activation.shadow_months":     "2",
		"collateral.weight.activation.coverage_min_frac": "0.90",
	}
	for key, want := range bawaan {
		if got := bacaConfigPanelT0(t, e, key); got != want {
			t.Errorf("bawaan %s = %q, mau %q", key, got, want)
		}
	}

	// Bobot agunan TIDAK berubah: semua applied 100% dan saklar mati.
	var menyimpang int
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT count(*) FROM collateral_lampiran_ii_weights
		WHERE applied_weight_frac <> 1.00000 OR enabled`).Scan(&menyimpang); err != nil {
		t.Fatalf("membaca pemetaan bobot: %v", err)
	}
	if menyimpang != 0 {
		t.Fatalf("%d baris bobot bukan-100%%/aktif: ATMR belum boleh berubah", menyimpang)
	}

	// Kolom/tabel T0 ada.
	for _, q := range []string{
		`SELECT count(*) FROM information_schema.columns WHERE table_name='loans' AND column_name='ckpn_method'`,
		`SELECT count(*) FROM information_schema.columns WHERE table_name='loan_collaterals' AND column_name='selling_cost_amount'`,
		`SELECT count(*) FROM information_schema.columns WHERE table_name='collateral_lampiran_ii_weights' AND column_name='direksi_policy_number'`,
		`SELECT count(*) FROM loan_ckpn_individual_assessments WHERE false`,
		`SELECT count(*) FROM loan_cashflow_projections WHERE false`,
	} {
		var n int
		if err := e.db.QueryRowContext(e.ctx, q).Scan(&n); err != nil {
			t.Fatalf("kueri struktur T0 gagal (%s): %v", q, err)
		}
	}

	// Nilai bank tidak ditimpa: ubah lalu jalankan ulang.
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE system_config SET value = '5' WHERE key = 'collateral.weight.activation.shadow_months'`); err != nil {
		t.Fatalf("menyetel nilai bank: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx,
			`UPDATE system_config SET value = '2' WHERE key = 'collateral.weight.activation.shadow_months'`)
	})
	jalankanMigrasiPanelT0(t, e)
	if got := bacaConfigPanelT0(t, e, "collateral.weight.activation.shadow_months"); got != "5" {
		t.Fatalf("nilai bank ditimpa menjadi %q, mau tetap 5", got)
	}
}

// Gerbang aktivasi bobot agunan: pengajuan ditolak saat syarat kurang dan lolos
// setelah semua syarat dipenuhi lewat persetujuan pemeriksa LAIN; kategori benar-benar
// menyala di database. Pembuat dan penyetuju selalu diambil dari token.
func TestIntegrasiPanelT0GerbangAktivasiBobotAgunan(t *testing.T) {
	e := newMoneyEnv(t)
	jalankanMigrasiPanelT0(t, e)

	const kategoriUji = "UJI_PANEL_T0_MESIN"
	// Kategori uji terisolasi: jenis MESIN_PERALATAN tidak dipakai uji lain, sehingga
	// cakupan (>= 90%) deterministik.
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO collateral_lampiran_ii_weights
			(category_code, lampiran_ii_item, label, collateral_type, binding_type,
			 official_weight_frac, applied_weight_frac, enabled, notes, shadow_started_at)
		VALUES ($1, 99, 'kategori uji panel T0', 'MESIN_PERALATAN', NULL,
		        0.70000, 1.00000, FALSE, 'uji gerbang', NULL)
		ON CONFLICT (category_code) DO UPDATE SET enabled=FALSE, applied_weight_frac=1.00000,
		    shadow_started_at=NULL, direksi_policy_number=NULL, direksi_policy_date=NULL,
		    activated_maker=NULL, activated_by=NULL, activated_at=NULL`, kategoriUji); err != nil {
		t.Fatalf("menyiapkan kategori uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM collateral_lampiran_ii_weights WHERE category_code = $1`, kategoriUji)
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM maker_checker_requests WHERE action_type = $1`, domain.CollateralWeightActivateAction)
	})

	// Agunan lengkap pada satu kredit baru.
	cust := e.newCustomer(t, "Panel T0 Gerbang", "")
	loan := e.disburse(t, cust.ID, e.newAccount(t, cust.ID), idr(10_000_000), 4)
	ikatan := domain.BindingFidusia
	akhirAsuransi := tanggalSaja(time.Now().UTC().AddDate(0, 6, 0))
	berlakuTaksasi := tanggalSaja(time.Now().UTC().AddDate(0, 6, 0))
	col := &domain.LoanCollateral{
		LoanID:              loan.ID,
		CollateralType:      domain.CollateralMesinPeralatan,
		Description:         "mesin uji gerbang",
		DocumentNumber:      "DOC-PANEL-T0-1",
		OwnerName:           "Pemilik Uji",
		AppraisalValue:      idr(5_000_000),
		AppraisalDate:       tanggalSaja(time.Now().UTC().AddDate(0, 0, -5)),
		AppraisalValidUntil: &berlakuTaksasi,
		HaircutPercent:      decimal.Zero,
		Status:              domain.CollateralActive,
		ExistsKnown:         true,
		Executable:          true,
		BindingType:         &ikatan,
		InsuranceExpiryDate: &akhirAsuransi,
	}
	if err := e.collateralRepo.Create(e.ctx, col, "001"); err != nil {
		t.Fatalf("mencatat agunan uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM loan_collaterals WHERE id = $1`, col.ID)
	})

	auditRepo := postgres.NewAuditRepository(e.db)
	svc := service.NewCollateralWeightService(postgres.NewCollateralWeightRepository(e.db), e.configSvc, auditRepo)
	executors := service.NewExecutorRegistry()
	executors.Register(domain.CollateralWeightActivateAction, svc)
	mcSvc := service.NewMakerCheckerService(e.db, postgres.NewMakerCheckerRepository(e.db), auditRepo,
		e.configSvc, executors, postgres.NewBusinessDateRepository(e.db), postgres.NewBranchRepository(e.db))
	checkerActor := e.newCheckerStaff(t, "checker.panel.t0")

	// 1. Ditolak: pengajuan tanpa kebijakan Direksi (C8) tidak lolos saat disetujui,
	// sekalipun pemeriksa sah. Kategori harus tetap mati.
	req, err := mcSvc.CreateRequest(e.ctx, domain.CreateMakerCheckerInput{
		ActionType: domain.CollateralWeightActivateAction,
		Payload: map[string]any{
			"category_code":         kategoriUji,
			"has_config_permission": true,
		},
	}, e.actor)
	if err != nil {
		t.Fatalf("mengajukan aktivasi: %v", err)
	}
	err = mcSvc.Approve(e.ctx, req.ID, checkerActor, "")
	if err == nil {
		t.Fatal("aktivasi tanpa C8 harus ditolak")
	}
	if !strings.Contains(err.Error(), "C8") {
		t.Fatalf("galat harus menyebut syarat yang gagal (C8), dapat: %v", err)
	}
	if enabled := bacaEnabledKategori(t, e, kategoriUji); enabled {
		t.Fatal("kategori tidak boleh menyala saat gerbang menolak")
	}

	// 2. Syarat dipenuhi: mode bayangan 3 bulan, kebijakan Direksi, maker-checker dua
	// orang. Pengajuan baru membawa kebijakan; pemeriksa LAIN yang menyetujui.
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE collateral_lampiran_ii_weights SET shadow_started_at = NOW() - INTERVAL '3 months' WHERE category_code = $1`,
		kategoriUji); err != nil {
		t.Fatalf("menyetel mulai mode bayangan: %v", err)
	}
	req2, err := mcSvc.CreateRequest(e.ctx, domain.CreateMakerCheckerInput{
		ActionType: domain.CollateralWeightActivateAction,
		Payload: map[string]any{
			"category_code":         kategoriUji,
			"direksi_policy_number": "SK-DIR-PANEL-T0",
			"direksi_policy_date":   time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02"),
			"has_config_permission": true,
		},
	}, e.actor)
	if err != nil {
		t.Fatalf("mengajukan aktivasi lengkap: %v", err)
	}
	if err := mcSvc.Approve(e.ctx, req2.ID, checkerActor, ""); err != nil {
		t.Fatalf("persetujuan dengan seluruh syarat harus lolos: %v", err)
	}
	if !bacaEnabledKategori(t, e, kategoriUji) {
		t.Fatal("kategori harus menyala di database setelah gerbang lolos")
	}
	var applied string
	var checker string
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT applied_weight_frac::text, COALESCE(activated_by,'')
		FROM collateral_lampiran_ii_weights WHERE category_code = $1`, kategoriUji).Scan(&applied, &checker); err != nil {
		t.Fatalf("membaca kategori setelah aktivasi: %v", err)
	}
	if applied != "0.70000" {
		t.Fatalf("applied_weight_frac = %q, mau sama dengan official 0.70000 (C7)", applied)
	}
	if checker != checkerActor.Username {
		t.Fatalf("activated_by = %q, mau %s (C9, identitas dari token)", checker, checkerActor.Username)
	}
}

func bacaEnabledKategori(t *testing.T, e *moneyEnv, kode string) bool {
	t.Helper()
	var enabled bool
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT enabled FROM collateral_lampiran_ii_weights WHERE category_code = $1`, kode).Scan(&enabled); err != nil {
		t.Fatalf("membaca enabled kategori %s: %v", kode, err)
	}
	return enabled
}
