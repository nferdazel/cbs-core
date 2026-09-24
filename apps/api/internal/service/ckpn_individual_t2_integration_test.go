package service_test

import (
	"net/http"
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

// TAHAP T2 CKPN individual: nilai realisasi bersih agunan (NRV = max(0, bound_amount −
// biaya pelepasan)) dan aturan metode MAX. Semuanya masih BAYANGAN baca-saja: required_ckpn
// tidak pernah disentuh dan tidak ada jurnal. Dibuktikan dengan angka yang dapat
// dihitung ulang dengan tangan:
//
//	carrying 150.000, EIR 10%/bulan, proyeksi 110.000 (periode 1)
//	→ PV 100.000, target DCF 50.000
//	agunan: taksasi 200.000, haircut 50% → bound 100.000
//	tanpa biaya pelepasan → NRV 100.000, target agunan 50.000 → final 50.000
//	biaya pelepasan 60.000 → NRV 40.000, target agunan 110.000 → final 110.000 (agunan menang)
func TestIntegrasiCKPNIndividualT2NRVAgunan(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
	setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	setCKPNConfig(t, e, "ckpn.individual.method", "MAX")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
		setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	})

	branchCode := ckpnTestBranchCode("T2")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T2")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit2", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T2", "t2-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET outstanding_principal=$2, restructure_loss_balance=0, original_eir_monthly=0.10
		WHERE id=$1`, loan.ID, idr(150_000)); err != nil {
		t.Fatalf("menyiapkan nilai tercatat/EIR: %v", err)
	}

	col := &domain.LoanCollateral{
		LoanID:         loan.ID,
		CollateralType: domain.CollateralKendaraan,
		Description:    "kendaraan uji T2",
		DocumentNumber: "DOC-T2",
		OwnerName:      "Pemilik T2",
		AppraisalValue: idr(200_000),
		AppraisalDate:  tanggalSaja(time.Now().UTC().AddDate(0, 0, -5)),
		HaircutPercent: decimal.NewFromInt(50),
		Status:         domain.CollateralActive,
		ExistsKnown:    true,
		Executable:     true,
	}
	if err := e.collateralRepo.Create(e.ctx, col, branchCode); err != nil {
		t.Fatalf("mencatat agunan uji T2: %v", err)
	}
	t.Cleanup(func() { _, _ = e.db.ExecContext(e.ctx, `DELETE FROM loan_collaterals WHERE id = $1`, col.ID) })

	if err := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc).
		ReplaceProjections(e.ctx, loan.LoanNumber, ckpnTestAsOf(), []domain.CKPNCashflowProjection{{Period: 1, Amount: idr(110_000)}}, actor); err != nil {
		t.Fatalf("menyimpan proyeksi: %v", err)
	}

	svc := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)

	// 1. Tanpa biaya pelepasan: NRV = bound_amount penuh, ditandai belum diisi.
	hasil, err := svc.Evaluate(e.ctx, loan.LoanNumber, ckpnTestAsOf(), actor)
	if err != nil {
		t.Fatalf("menilai T2 tanpa biaya pelepasan: %v", err)
	}
	if len(hasil.Collaterals) != 1 {
		t.Fatalf("agunan terbaca %d, mau 1", len(hasil.Collaterals))
	}
	if !hasil.TotalNRV.Equal(idr(100_000)) {
		t.Fatalf("total NRV %s, mau 100000 (bound tanpa pengurangan)", hasil.TotalNRV)
	}
	if hasil.MissingDisposalCostCount != 1 {
		t.Fatalf("agunan tanpa biaya pelepasan = %d, mau 1 (wajib terbuka)", hasil.MissingDisposalCostCount)
	}
	if !hasil.CollateralTarget.Equal(idr(50_000)) || !hasil.FinalTarget.Equal(idr(50_000)) {
		t.Fatalf("tanpa biaya: target agunan %s, final %s; mau 50000/50000", hasil.CollateralTarget, hasil.FinalTarget)
	}

	// 2. Biaya pelepasan 60.000 → NRV 40.000 → target agunan 110.000 MENANG atas DCF 50.000.
	biaya := idr(60_000)
	if err := svc.SetDisposalCost(e.ctx, loan.LoanNumber, col.ID, &biaya, actor); err != nil {
		t.Fatalf("menyimpan biaya pelepasan: %v", err)
	}
	hasil, err = svc.Evaluate(e.ctx, loan.LoanNumber, ckpnTestAsOf(), actor)
	if err != nil {
		t.Fatalf("menilai T2 dengan biaya pelepasan: %v", err)
	}
	if !hasil.TotalNRV.Equal(idr(40_000)) {
		t.Fatalf("total NRV %s, mau 40000 (100000-60000)", hasil.TotalNRV)
	}
	if !hasil.CollateralTarget.Equal(idr(110_000)) {
		t.Fatalf("target agunan %s, mau 110000 (150000-40000)", hasil.CollateralTarget)
	}
	if !hasil.FinalTarget.Equal(idr(110_000)) {
		t.Fatalf("final %s, mau 110000: aturan MAX harus memilih agunan atas DCF 50000", hasil.FinalTarget)
	}
	if hasil.MissingDisposalCostCount != 0 {
		t.Fatalf("agunan tanpa biaya = %d, mau 0 setelah diisi", hasil.MissingDisposalCostCount)
	}

	// 3. Bayangan: required_ckpn TIDAK berubah oleh seluruh alur T2.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE loans SET required_ckpn = 55555 WHERE id = $1`, loan.ID); err != nil {
		t.Fatalf("menandai required_ckpn pembanding: %v", err)
	}
	if _, err := svc.Evaluate(e.ctx, loan.LoanNumber, ckpnTestAsOf(), actor); err != nil {
		t.Fatalf("menilai ulang: %v", err)
	}
	if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.Equal(idr(55555)) {
		t.Fatalf("required_ckpn berubah menjadi %s; T2 wajib baca-saja", got)
	}
	_, _ = e.db.ExecContext(e.ctx, `UPDATE loans SET required_ckpn = 0 WHERE id = $1`, loan.ID)

	// 4. Biaya negatif ditolak; agunan milik kredit lain ditolak.
	neg := idr(-1)
	if err := svc.SetDisposalCost(e.ctx, loan.LoanNumber, col.ID, &neg, actor); err == nil {
		t.Fatal("biaya negatif harus ditolak")
	}
	loanLain := e.disburseAs(t, actor, cust.ID, acc, idr(1_000_000), 6)
	if err := svc.SetDisposalCost(e.ctx, loanLain.LoanNumber, col.ID, &biaya, actor); err == nil {
		t.Fatal("agunan milik kredit lain harus ditolak")
	}
}

// E2E HTTP T2 lewat router sungguhan: simpan biaya pelepasan, baca penilaian yang
// memuat NRV per agunan; izin baca TIDAK cukup untuk menulis.
func TestIntegrasiCKPNIndividualT2LewatRouter(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
	setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	setCKPNConfig(t, e, "ckpn.individual.method", "MAX")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
		setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	})

	branchCode := ckpnTestBranchCode("R2")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T2 Router")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujir2", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T2R", "t2r-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET outstanding_principal=$2, restructure_loss_balance=0, original_eir_monthly=0.10
		WHERE id=$1`, loan.ID, idr(150_000)); err != nil {
		t.Fatalf("menyiapkan nilai tercatat/EIR: %v", err)
	}

	col := &domain.LoanCollateral{
		LoanID:         loan.ID,
		CollateralType: domain.CollateralTanahBangunan,
		Description:    "tanah uji T2R",
		DocumentNumber: "DOC-T2R",
		OwnerName:      "Pemilik T2R",
		AppraisalValue: idr(200_000),
		AppraisalDate:  tanggalSaja(time.Now().UTC().AddDate(0, 0, -5)),
		HaircutPercent: decimal.NewFromInt(50),
		Status:         domain.CollateralActive,
		ExistsKnown:    true,
		Executable:     true,
	}
	if err := e.collateralRepo.Create(e.ctx, col, branchCode); err != nil {
		t.Fatalf("mencatat agunan uji T2R: %v", err)
	}
	t.Cleanup(func() { _, _ = e.db.ExecContext(e.ctx, `DELETE FROM loan_collaterals WHERE id = $1`, col.ID) })

	svc := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)
	router := chi.NewRouter()
	h := httpHandler.NewCKPNHandler(nil, e.configSvc)
	h.Individual = svc
	h.RegisterRoutes(router)

	asOf := ckpnTestAsOf().Format("2006-01-02")
	base := "/ckpn/individual/" + loan.LoanNumber

	// Penyimpanan biaya oleh pembaca TIDAK boleh (butuh approve kredit).
	reader := &domain.JWTClaims{UserID: uuid.New(), Username: "reader.t2", Role: domain.RoleAuditor}
	if rec := cwRequest(t, router, reader, http.MethodPut, base+"/collaterals/"+col.ID.String()+"/disposal-cost",
		`{"disposal_cost":"60000"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("simpan biaya oleh pembaca = %d, mau 403", rec.Code)
	}

	approver := &domain.JWTClaims{UserID: e.actor.UserID, Username: e.actor.Username, Role: domain.RoleSuperAdmin}
	if rec := cwRequest(t, router, approver, http.MethodPut, base+"/collaterals/"+col.ID.String()+"/disposal-cost",
		`{"disposal_cost":"60000"}`); rec.Code != http.StatusOK {
		t.Fatalf("simpan biaya = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}

	// Payload tidak sah: bidang tak dikenal ditolak, biaya negatif ditolak, UUID salah ditolak.
	if rec := cwRequest(t, router, approver, http.MethodPut, base+"/collaterals/"+col.ID.String()+"/disposal-cost",
		`{"biaya":"60000"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bidang tak dikenal = %d, mau 422", rec.Code)
	}
	if rec := cwRequest(t, router, approver, http.MethodPut, base+"/collaterals/"+col.ID.String()+"/disposal-cost",
		`{"disposal_cost":"-5"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("biaya negatif = %d, mau 422", rec.Code)
	}
	if rec := cwRequest(t, router, approver, http.MethodPut, base+"/collaterals/bukan-uuid/disposal-cost",
		`{"disposal_cost":"5"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("UUID salah = %d, mau 422", rec.Code)
	}

	// Proyeksi arus kas diisi lebih dulu (urutan pakai T1/T2): tanpa itu penilaian
	// menolak 422 karena tidak ada dasar perhitungan.
	if rec := cwRequest(t, router, approver, http.MethodPut, base+"/projections?as_of="+asOf,
		`{"projections":[{"period":1,"amount":"110000"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("menyimpan proyeksi = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}

	// Penilaian memuat rincian agunan dengan NRV 40000 dan final 110000.
	rec := cwRequest(t, router, reader, http.MethodGet, base+"/assessment?as_of="+asOf, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("penilaian T2 = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, harus := range []string{
		`"total_nrv":"40000"`,
		`"collateral_target":"110000"`,
		`"final_target":"110000"`,
		`"missing_disposal_cost_count":0`,
	} {
		if !strings.Contains(body, harus) {
			t.Fatalf("penilaian tidak memuat %s: %s", harus, body)
		}
	}
}
