package service_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
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
