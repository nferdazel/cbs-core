package service_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// Uji integrasi T1 CKPN individual (keputusan panel butir 4/5): DCF memakai EIR
// orisinal dan proyeksi arus kas MANUAL, MODE BAYANGAN BACA-SAJA. Di-skip kecuali
// CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiCKPNIndividualT1 -v
//
// Yang dibuktikan: target DCF sama dengan hitungan manual, loans.required_ckpn TIDAK
// disentuh (bayangan), proyeksi tervalidasi dan tersimpan, dan kredit tanpa EIR GAGAL
// (ErrEIRMissing) bukan dihitung nol.
func TestIntegrasiCKPNIndividualT1DCFBacaSaja(t *testing.T) {
	e := newMoneyEnv(t)
	// Override kosong: wajib EIR orisinal. Saklar individual tetap MATI (T1 bayangan).
	setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
	setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	setCKPNConfig(t, e, "ckpn.individual.method", "MAX")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
		setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	})

	branchCode := ckpnTestBranchCode("T1")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN Individual T1")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit1", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T1", "t1-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)

	// Nilai tercatat 150 dan EIR orisinal 10%/bulan agar DCF deterministik:
	// 110 pada periode 1 -> PV 100, target 50.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET outstanding_principal=$2, restructure_loss_balance=0, original_eir_monthly=0.10
		WHERE id=$1`, loan.ID, idr(150)); err != nil {
		t.Fatalf("menyiapkan nilai tercatat/EIR: %v", err)
	}
	before := e.loanDecimal(t, loan.ID, "required_ckpn")

	svc := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)
	asOf := ckpnTestAsOf()
	proyeksi := []domain.CKPNCashflowProjection{{Period: 1, Amount: idr(110)}}

	if err := svc.ReplaceProjections(e.ctx, loan.LoanNumber, asOf, proyeksi, actor); err != nil {
		t.Fatalf("menyimpan proyeksi sah: %v", err)
	}
	hasil, err := svc.Evaluate(e.ctx, loan.LoanNumber, asOf, actor)
	if err != nil {
		t.Fatalf("menilai DCF: %v", err)
	}
	if !hasil.PresentValue.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("PV %s, mau 100", hasil.PresentValue)
	}
	if !hasil.Target.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("target individual %s, mau 50 (150-100)", hasil.Target)
	}
	if hasil.EIRSource != "EIR_ORISINAL" {
		t.Fatalf("sumber EIR %q, mau EIR_ORISINAL", hasil.EIRSource)
	}
	if len(hasil.Projections) != 1 {
		t.Fatalf("proyeksi terbaca %d, mau 1", len(hasil.Projections))
	}

	// Bayangan: required_ckpn TIDAK berubah.
	if after := e.loanDecimal(t, loan.ID, "required_ckpn"); !after.Equal(before) {
		t.Fatalf("required_ckpn berubah dari %s menjadi %s; T1 harus baca-saja", before, after)
	}

	// Validasi: proyeksi kosong ditolak, tidak mengganti yang sah.
	if err := svc.ReplaceProjections(e.ctx, loan.LoanNumber, asOf, nil, actor); !errors.Is(err, domain.ErrCKPNProjectionInvalid) {
		t.Fatalf("proyeksi kosong: err=%v, mau ErrCKPNProjectionInvalid", err)
	}

	// Kredit tanpa EIR dan tanpa override -> ErrEIRMissing, bukan nol.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE loans SET original_eir_monthly=0 WHERE id=$1`, loan.ID); err != nil {
		t.Fatalf("menghapus EIR: %v", err)
	}
	if _, err := svc.Evaluate(e.ctx, loan.LoanNumber, asOf, actor); !errors.Is(err, domain.ErrEIRMissing) {
		t.Fatalf("tanpa EIR: err=%v, mau ErrEIRMissing", err)
	}
}

// E2E HTTP T1 CKPN individual lewat ROUTER SUNGGUHAN (bukan hanya service): membuktikan
// pemasangan rute, middleware izin, penguraian {loanNumber}, dan pemetaan status. Tanpa
// ini, T1 hanya terbukti benar di tingkat service dan kesalahan pemasangan rute tidak
// akan terlihat sampai pengguna mencobanya.
func TestIntegrasiCKPNIndividualT1LewatRouter(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
	setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	setCKPNConfig(t, e, "ckpn.individual.method", "MAX")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
		setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	})

	branchCode := ckpnTestBranchCode("R1")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T1 Router")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujirouter", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T1 Router", "t1r-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET outstanding_principal=$2, restructure_loss_balance=0, original_eir_monthly=0.10
		WHERE id=$1`, loan.ID, idr(150)); err != nil {
		t.Fatalf("menyiapkan nilai tercatat/EIR: %v", err)
	}

	svc := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)
	router := chi.NewRouter()
	h := httpHandler.NewCKPNHandler(nil, e.configSvc)
	h.Individual = svc
	h.RegisterRoutes(router)

	asOf := ckpnTestAsOf().Format("2006-01-02")
	base := "/ckpn/individual/" + loan.LoanNumber

	// Sebelum ada proyeksi, penilaian menolak dengan 422 dan pesan yang jelas -
	// bukan menghitung DCF dari input yang belum ada. Ini urutan pakai yang benar:
	// isi proyeksi lebih dulu, baru nilai.
	reader := &domain.JWTClaims{UserID: uuid.New(), Username: "reader.t1", Role: domain.RoleAuditor}
	rec := cwRequest(t, router, reader, http.MethodGet, base+"/assessment?as_of="+asOf, "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("penilaian tanpa proyeksi = %d, mau 422 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "proyeksi arus kas kosong") {
		t.Fatalf("pesan tanpa proyeksi tidak menjelaskan sebabnya: %s", rec.Body.String())
	}

	// Izin baca tidak cukup untuk MENULIS proyeksi (butuh wewenang approve kredit).
	if recGet := cwRequest(t, router, reader, http.MethodPut, base+"/projections?as_of="+asOf,
		`{"projections":[{"period":1,"amount":"110"}]}`); recGet.Code != http.StatusForbidden {
		t.Fatalf("penyimpanan proyeksi oleh pembaca = %d, mau 403", recGet.Code)
	}

	// Izin approve: penyimpanan proyeksi boleh.
	approver := &domain.JWTClaims{UserID: e.actor.UserID, Username: e.actor.Username, Role: domain.RoleSuperAdmin}
	jurnalSebelum := hitungJurnalTotal(t, e)
	if recPut := cwRequest(t, router, approver, http.MethodPut, base+"/projections?as_of="+asOf,
		`{"projections":[{"period":1,"amount":"110"}]}`); recPut.Code != http.StatusOK {
		t.Fatalf("menyimpan proyeksi = %d, mau 200 (%s)", recPut.Code, recPut.Body.String())
	}

	// Proyeksi sah kini terbaca pada penilaian: PV 100, target 50.
	rec = cwRequest(t, router, reader, http.MethodGet, base+"/assessment?as_of="+asOf, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("penilaian setelah proyeksi = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"eir_source":"EIR_ORISINAL"`) {
		t.Fatalf("respons penilaian tidak memuat sumber EIR: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"present_value":"100"`) ||
		!strings.Contains(rec.Body.String(), `"target":"50"`) {
		t.Fatalf("penilaian setelah proyeksi tidak sesuai: %s", rec.Body.String())
	}

	// Baca-saja: tidak ada jurnal baru dan required_ckpn tidak bergerak.
	if after := hitungJurnalTotal(t, e); after != jurnalSebelum {
		t.Fatalf("jurnal bertambah dari %d ke %d; T1 harus baca-saja", jurnalSebelum, after)
	}

	// Kredit tanpa EIR: 422 dengan galat jelas, BUKAN 500 dan BUKAN dihitung nol.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE loans SET original_eir_monthly=0 WHERE id=$1`, loan.ID); err != nil {
		t.Fatalf("menghapus EIR: %v", err)
	}
	rec = cwRequest(t, router, reader, http.MethodGet, base+"/assessment?as_of="+asOf, "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("kredit tanpa EIR = %d, mau 422 (%s)", rec.Code, rec.Body.String())
	}

	// Kredit tidak ada: 404, bukan 500.
	rec = cwRequest(t, router, reader, http.MethodGet,
		"/ckpn/individual/TIDAK-ADA-999/assessment?as_of="+asOf, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("kredit tidak ada = %d, mau 404 (%s)", rec.Code, rec.Body.String())
	}

	// Saklar tetap seperti semula sesudah seluruh permintaan.
	if got := e.configSvc.GetString(e.ctx, "ckpn.individual.enabled", ""); got != "false" {
		t.Fatalf("ckpn.individual.enabled = %q, mau tetap false", got)
	}
}

// hitungJurnalTotal menghitung seluruh baris jurnal. Dipakai membuktikan jalur T1
// baca-saja tidak menambah jurnal apa pun (bukan sekadar tidak menambah lewat kunci
// idempotensi tertentu).
func hitungJurnalTotal(t *testing.T, e *moneyEnv) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx, `SELECT count(*) FROM journal_entries`).Scan(&n); err != nil {
		t.Fatalf("menghitung jurnal: %v", err)
	}
	return n
}
