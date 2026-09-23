package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// Uji integrasi rute HTTP gerbang bobot agunan terhadap PostgreSQL sungguhan. Di-skip
// kecuali CBS_TEST_DB_DSN diisi (newMoneyEnv). Membuktikan jalur dua-aktor yang aman:
// pengajuan aktivasi hanya mencatat pembuat dari TOKEN (identitas di kueri/body
// diabaikan), kategori belum menyala sampai pemeriksa LAIN menyetujui, pembuat tidak
// dapat menyetujui pengajuannya sendiri, dan jejak audit mencatat identitas yang benar.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiAgunanBobotGerbang -v
func TestIntegrasiAgunanBobotGerbangAPI(t *testing.T) {
	e := newMoneyEnv(t)
	jalankanMigrasiPanelT0(t, e)

	const kategoriUji = "UJI_BOBOT_API_KENDARAAN"
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO collateral_lampiran_ii_weights
			(category_code, lampiran_ii_item, label, collateral_type, binding_type,
			 official_weight_frac, applied_weight_frac, enabled, notes, shadow_started_at)
		VALUES ($1, 98, 'kategori uji API gerbang', 'KENDARAAN', NULL,
		        0.70000, 1.00000, FALSE, 'uji API', NOW() - INTERVAL '3 months')
		ON CONFLICT (category_code) DO UPDATE SET enabled=FALSE, applied_weight_frac=1.00000,
		    shadow_started_at=NOW() - INTERVAL '3 months', direksi_policy_number=NULL,
		    direksi_policy_date=NULL, activated_maker=NULL, activated_by=NULL, activated_at=NULL`,
		kategoriUji); err != nil {
		t.Fatalf("menyiapkan kategori uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM collateral_lampiran_ii_weights WHERE category_code = $1`, kategoriUji)
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM audit_logs WHERE resource_type = 'collateral_weight_category' AND resource_id = $1`, kategoriUji)
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM maker_checker_requests WHERE action_type = $1`, domain.CollateralWeightActivateAction)
	})

	cust := e.newCustomer(t, "Bobot API Kendaraan", "")
	loan := e.disburse(t, cust.ID, e.newAccount(t, cust.ID), idr(10_000_000), 4)
	ikatan := domain.BindingFidusia
	akhirAsuransi := tanggalSaja(time.Now().UTC().AddDate(0, 6, 0))
	berlakuTaksasi := tanggalSaja(time.Now().UTC().AddDate(0, 6, 0))
	col := &domain.LoanCollateral{
		LoanID:              loan.ID,
		CollateralType:      domain.CollateralKendaraan,
		Description:         "kendaraan uji API",
		DocumentNumber:      "DOC-BOBOT-API",
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
	t.Cleanup(func() { _, _ = e.db.ExecContext(e.ctx, `DELETE FROM loan_collaterals WHERE id = $1`, col.ID) })

	auditRepo := postgres.NewAuditRepository(e.db)
	svc := service.NewCollateralWeightService(postgres.NewCollateralWeightRepository(e.db), e.configSvc, auditRepo)
	executors := service.NewExecutorRegistry()
	executors.Register(domain.CollateralWeightActivateAction, svc)
	mcSvc := service.NewMakerCheckerService(e.db, postgres.NewMakerCheckerRepository(e.db), auditRepo,
		e.configSvc, executors, postgres.NewBusinessDateRepository(e.db), postgres.NewBranchRepository(e.db))

	router := chi.NewRouter()
	httpHandler.NewCollateralWeightHandler(svc, mcSvc).RegisterRoutes(router)

	superadmin := &domain.JWTClaims{UserID: e.actor.UserID, Username: e.actor.Username, Role: domain.RoleSuperAdmin}
	auditor := &domain.JWTClaims{UserID: uuid.New(), Username: "auditor.bobot", Role: domain.RoleAuditor}

	// Pemeriksa harus baris staf nyata karena audit menyimpan uuid aktor.
	checkerActor := e.newCheckerStaff(t, "checker.bobot")

	// 1. Penilaian baca-saja: syarat yang gagal disebut, cakupan dan umur bayangan
	// dilaporkan, kategori tetap mati. Auditor hanya punya system:config:read.
	rec := cwRequest(t, router, auditor, http.MethodGet, "/collateral/weights/"+kategoriUji, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("penilaian baca-saja = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}
	env := cwDecode(t, rec)
	if env.Data == nil {
		t.Fatalf("respons penilaian tanpa data: %s", rec.Body.String())
	}
	if env.Data.Enabled {
		t.Fatal("kategori menyala sebelum aktivasi")
	}
	if env.Data.Allowed {
		t.Fatalf("gerbang lolos padahal belum lengkap: %v", env.Data.Failures)
	}
	if !cwFailureContained(env.Data.Failures, domain.CollateralWeightC8) ||
		!cwFailureContained(env.Data.Failures, domain.CollateralWeightC9) {
		t.Fatalf("penilaian tidak menyebut C8/C9 yang gagal: %v", env.Data.Failures)
	}
	if env.Data.ShadowMonthsRequired != 2 || env.Data.ShadowMonthsElapsed < 3 {
		t.Fatalf("status mode bayangan %d/%d, mau minimal 3 dari 2",
			env.Data.ShadowMonthsElapsed, env.Data.ShadowMonthsRequired)
	}
	if !env.Data.CoverageFrac.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("cakupan %s, mau 1 (satu agunan layak dari satu)", env.Data.CoverageFrac)
	}

	// 2. Auditor tanpa izin system:config ditolak 403 pada pengajuan aktivasi.
	rec = cwRequest(t, router, auditor, http.MethodPost, "/collateral/weights/"+kategoriUji+"/activate",
		`{"direksi_policy_number":"SK-1","direksi_policy_date":"2026-01-01"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pengajuan oleh auditor = %d, mau 403", rec.Code)
	}

	// 3. Identitas dari kueri/body DIABAIKAN: pengajuan membawa ?maker=evil&checker=evil
	// dan body maker palsu, tetapi pengaju yang tercatat adalah aktor token.
	rec = cwRequest(t, router, superadmin, http.MethodPost,
		"/collateral/weights/"+kategoriUji+"/activate?maker=evil&checker=evil",
		`{"maker":"evil.body","checker":"evil.body","direksi_policy_number":"SK-DIR-1","direksi_policy_date":"2026-01-01"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("pengajuan aktivasi = %d, mau 202 (%s)", rec.Code, rec.Body.String())
	}
	reqID := cwPendingRequestID(t, rec)

	var makerID, makerUsername string
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT maker_id, COALESCE(payload->>'maker_username','')
		FROM maker_checker_requests WHERE id = $1`, reqID).Scan(&makerID, &makerUsername); err != nil {
		t.Fatalf("membaca pengajuan tersimpan: %v", err)
	}
	if makerID != superadmin.UserID.String() {
		t.Fatalf("maker_id tersimpan %q, mau id token %q (identitas kueri/body tidak boleh dipakai)", makerID, superadmin.UserID)
	}
	if makerUsername != superadmin.Username {
		t.Fatalf("maker_username tersimpan %q, mau %q dari token", makerUsername, superadmin.Username)
	}
	if cwEnabled(t, e, kategoriUji) {
		t.Fatal("kategori menyala saat pengajuan masih PENDING")
	}

	// 4. Pembuat tidak boleh menyetujui pengajuannya sendiri (pemisahan tugas).
	if err := mcSvc.Approve(e.ctx, reqID, e.actor, ""); !errors.Is(err, domain.ErrCannotSelfApprove) {
		t.Fatalf("pembuat menyetujui sendiri = %v, mau ErrCannotSelfApprove", err)
	}
	if cwEnabled(t, e, kategoriUji) {
		t.Fatal("kategori menyala padahal persetujuan sendiri ditolak")
	}

	// 5. Pemeriksa LAIN menyetujui: kategori menyala dan audit mencatat identitas token.
	if err := mcSvc.Approve(e.ctx, reqID, checkerActor, ""); err != nil {
		t.Fatalf("persetujuan oleh pemeriksa lain gagal: %v", err)
	}
	if !cwEnabled(t, e, kategoriUji) {
		t.Fatal("kategori tidak menyala setelah disetujui pemeriksa lain")
	}

	var maker, checker string
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT COALESCE(changes->>'maker',''), COALESCE(changes->>'checker','')
		FROM audit_logs
		WHERE action = 'COLLATERAL_WEIGHT_ACTIVATED' AND resource_id = $1
		ORDER BY created_at DESC, id DESC LIMIT 1`, kategoriUji).Scan(&maker, &checker); err != nil {
		t.Fatalf("membaca audit aktivasi: %v", err)
	}
	if maker != superadmin.Username || checker != checkerActor.Username {
		t.Fatalf("audit maker/checker = %q/%q, mau %s/%s (dari token)",
			maker, checker, superadmin.Username, checkerActor.Username)
	}

	// 6. MATI BAWAAN: kategori lain tetap 100%/mati dan bobot KPMM tidak bergeser.
	var menyimpang int
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT count(*) FROM collateral_lampiran_ii_weights
		WHERE category_code <> $1 AND (enabled OR applied_weight_frac <> 1.00000)`, kategoriUji).Scan(&menyimpang); err != nil {
		t.Fatalf("memeriksa bobot bawaan: %v", err)
	}
	if menyimpang != 0 {
		t.Fatalf("%d kategori lain menyala/berbobot bukan-100%%: ATMR tidak boleh berubah", menyimpang)
	}
	var rwa string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT value FROM system_config WHERE key = 'kpmm.rwa_frac.kredit'`).Scan(&rwa); err != nil {
		t.Fatalf("membaca bobot kredit KPMM: %v", err)
	}
	if rwa != "1.00" {
		t.Fatalf("bobot kredit KPMM %q, mau 1.00", rwa)
	}
}

// TestIntegrasiAgunanBobotGerbangTanpaPenyetuju menegaskan pengajuan tanpa pemeriksa
// sah tidak pernah menyalakan kategori, sekalipun setiap syarat lain lengkap: gerbang
// tetap menolak dan status pengajuan kembali PENDING.
func TestIntegrasiAgunanBobotGerbangTanpaPenyetuju(t *testing.T) {
	e := newMoneyEnv(t)
	jalankanMigrasiPanelT0(t, e)

	const kategoriUji = "UJI_BOBOT_API_TANPA_CHECKER"
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO collateral_lampiran_ii_weights
			(category_code, lampiran_ii_item, label, official_weight_frac, applied_weight_frac,
			 enabled, notes, shadow_started_at)
		VALUES ($1, 97, 'kategori uji tanpa checker', 0.70000, 1.00000, FALSE, 'uji', NOW() - INTERVAL '3 months')
		ON CONFLICT (category_code) DO UPDATE SET enabled=FALSE, applied_weight_frac=1.00000,
		    shadow_started_at=NOW() - INTERVAL '3 months', activated_maker=NULL, activated_by=NULL, activated_at=NULL`,
		kategoriUji); err != nil {
		t.Fatalf("menyiapkan kategori uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM collateral_lampiran_ii_weights WHERE category_code = $1`, kategoriUji)
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM maker_checker_requests WHERE action_type = $1`, domain.CollateralWeightActivateAction)
	})

	auditRepo := postgres.NewAuditRepository(e.db)
	svc := service.NewCollateralWeightService(postgres.NewCollateralWeightRepository(e.db), e.configSvc, auditRepo)
	executors := service.NewExecutorRegistry()
	executors.Register(domain.CollateralWeightActivateAction, svc)
	mcSvc := service.NewMakerCheckerService(e.db, postgres.NewMakerCheckerRepository(e.db), auditRepo,
		e.configSvc, executors, postgres.NewBusinessDateRepository(e.db), postgres.NewBranchRepository(e.db))

	// Pengajuan tanpa kebijakan Direksi: gerbang C8 akan menolak saat persetujuan.
	req, err := mcSvc.CreateRequest(e.ctx, domain.CreateMakerCheckerInput{
		ActionType: domain.CollateralWeightActivateAction,
		Payload: map[string]any{
			"category_code":         kategoriUji,
			"direksi_policy_number": "",
			"has_config_permission": true,
		},
	}, e.actor)
	if err != nil {
		t.Fatalf("mengajukan aktivasi: %v", err)
	}

	checkerActor := e.newCheckerStaff(t, "checker.tanpa")
	if err := mcSvc.Approve(e.ctx, req.ID, checkerActor, ""); err == nil {
		t.Fatal("persetujuan tanpa kebijakan Direksi harus ditolak gerbang")
	} else if !strings.Contains(err.Error(), domain.CollateralWeightC8) {
		t.Fatalf("penolakan harus menyebut C8, dapat: %v", err)
	}
	if cwEnabled(t, e, kategoriUji) {
		t.Fatal("kategori menyala padahal gerbang menolak")
	}
	// Pengajuan tetap PENDING sehingga masih dapat diperiksa/ditolak.
	var status string
	if err := e.db.QueryRowContext(e.ctx, `SELECT status FROM maker_checker_requests WHERE id = $1`, req.ID).Scan(&status); err != nil {
		t.Fatalf("membaca status pengajuan: %v", err)
	}
	if status != string(domain.MakerCheckerPending) {
		t.Fatalf("status pengajuan %q, mau PENDING setelah persetujuan ditolak", status)
	}
}

// newCheckerStaff menyisipkan (idempoten) baris staf untuk pemeriksa uji, dan
// mengembalikan aktor dengan uuid baris itu. Audit menyimpan uuid aktor, jadi pemeriksa
// harus baris staf nyata.
func (e *moneyEnv) newCheckerStaff(t *testing.T, username string) domain.Actor {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO staff_users (id, employee_id, username, full_name, email, password_hash, role, branch_code, is_active)
		VALUES ($1, $2, $3, 'Pemeriksa Uji', $4, 'x', 'SUPERADMIN', '001', TRUE)
		ON CONFLICT (username) DO NOTHING`,
		uuid.New(), "EMP-UJI-"+username, username, username+"@uji.local"); err != nil {
		t.Fatalf("menyiapkan pemeriksa %s: %v", username, err)
	}
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `SELECT id FROM staff_users WHERE username = $1`, username).Scan(&id); err != nil {
		t.Fatalf("membaca pemeriksa %s: %v", username, err)
	}
	return domain.Actor{UserID: id, Username: username, Role: domain.RoleSuperAdmin, BranchCode: "001"}
}

// cwRequest menjalankan permintaan HTTP nyata lewat router dengan claims di context,
// sehingga middleware izin dan penguraian URL param ikut teruji.
func cwRequest(t *testing.T, router http.Handler, claims *domain.JWTClaims, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// cwPendingRequestID membaca id pengajuan dari balasan 202 pengajuan aktivasi.
func cwPendingRequestID(t *testing.T, rec *httptest.ResponseRecorder) uuid.UUID {
	t.Helper()
	var env struct {
		Data *struct {
			RequestID uuid.UUID `json:"request_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Data == nil {
		t.Fatalf("mengurai id pengajuan (%s): %v", rec.Body.String(), err)
	}
	return env.Data.RequestID
}

type cwEnvelope struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Data    *struct {
		CategoryCode         string          `json:"category_code"`
		Enabled              bool            `json:"enabled"`
		Allowed              bool            `json:"allowed"`
		Failures             []string        `json:"failures"`
		CoverageFrac         decimal.Decimal `json:"coverage_frac"`
		EligibleValue        decimal.Decimal `json:"eligible_value"`
		TotalValue           decimal.Decimal `json:"total_value"`
		ShadowMonthsElapsed  int             `json:"shadow_months_elapsed"`
		ShadowMonthsRequired int             `json:"shadow_months_required"`
	} `json:"data"`
}

func cwDecode(t *testing.T, rec *httptest.ResponseRecorder) cwEnvelope {
	t.Helper()
	var env cwEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("mengurai respons (%s): %v", rec.Body.String(), err)
	}
	return env
}

func cwFailureContained(failures []string, kode string) bool {
	for _, f := range failures {
		if strings.HasPrefix(f, kode+":") {
			return true
		}
	}
	return false
}

func cwEnabled(t *testing.T, e *moneyEnv, kode string) bool {
	t.Helper()
	var enabled bool
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT enabled FROM collateral_lampiran_ii_weights WHERE category_code = $1`, kode).Scan(&enabled); err != nil {
		t.Fatalf("membaca enabled kategori %s: %v", kode, err)
	}
	return enabled
}
