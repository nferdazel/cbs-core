package service_test

import (
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Uji integrasi kerugian restrukturisasi terhadap PostgreSQL sungguhan. Mengikuti pola
// moneyflow_integration_test.go: di-skip kecuali CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55432/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiRestrukturisasi -v

// setRestructureLossConfig menyetel konfigurasi modul kerugian restrukturisasi dan
// membuang cache-nya agar langsung terbaca.
func setRestructureLossConfig(t *testing.T, e *moneyEnv, key, value string) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi kerugian restrukturisasi')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

// TestIntegrasiRestrukturisasiKerugian menjalankan restrukturisasi penuh pada database
// sungguhan: EIR tersimpan saat pencairan, kerugian dihitung dari EIR orisinal dan
// dijurnal (diperiksa langsung di database), lalu PPKA dihitung atas saldo setelah
// kerugian.
func TestIntegrasiRestrukturisasiKerugian(t *testing.T) {
	e := newMoneyEnv(t)
	// Saklar hidup SEBELUM pencairan agar EIR dicatat saat disbursement.
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "true")
	t.Cleanup(func() { setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false") })

	branchCode := ckpnTestBranchCode("R")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Kerugian Restrukturisasi")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujirugi", Role: domain.RoleAdmin, BranchCode: branchCode}

	cust := e.newCustomer(t, "Nasabah Rugi Restrukturisasi", fmt.Sprintf("rugi-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)

	// EIR orisinal tersimpan saat pencairan, beserta dasar auditnya.
	eir := e.loanDecimal(t, loan.ID, "original_eir_monthly")
	if !eir.IsPositive() {
		t.Fatalf("EIR orisinal tidak tersimpan: %s", eir)
	}
	if method := e.loanString(t, loan.ID, "original_eir_method"); method != domain.RestructureLossEIRMethod {
		t.Fatalf("metode EIR %q, mau %q", method, domain.RestructureLossEIRMethod)
	}
	if basis := e.loanString(t, loan.ID, "original_eir_basis"); basis == "" {
		t.Fatal("dasar audit EIR (original_eir_basis) kosong")
	}

	// Kredit dibuat NPL sebelum restrukturisasi agar Pasal 31 menahan kualitasnya di
	// Kurang Lancar, sehingga PPKA pasca-kerugian dapat diuji dengan tarif 10%.
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loans SET dpd=100, collectibility='3_KURANG_LANCAR' WHERE id=$1`, loan.ID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}

	// Turunkan suku bunga: nilai kini arus kas baru < nilai tercatat -> kerugian.
	if _, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
		Reason:                "uji integrasi kerugian restrukturisasi",
	}, actor); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}

	loss := e.loanDecimal(t, loan.ID, "restructure_loss_balance")
	if !loss.IsPositive() {
		t.Fatalf("saldo kerugian restrukturisasi tersimpan %s, mau positif", loss)
	}
	// Jurnal kerugian: RESTRUCT-LOSS-<nomor>-<count pertama>; Db 50401 / Kr 10301.
	key := fmt.Sprintf("RESTRUCT-LOSS-%s-1", loan.LoanNumber)
	assertCKPNJournal(t, e, key, "50401", "DEBIT", "10301", "CREDIT", loss)
	if bc := e.ckpnJournalBranch(t, key); bc != branchCode {
		t.Fatalf("cabang jurnal kerugian %q, mau cabang kredit %q (bukan cabang aktor)", bc, branchCode)
	}

	// PPKA kini dihitung atas saldo setelah kerugian (10% Kurang Lancar).
	if _, err := e.ppapSvc.RunDaily(e.ctx, time.Now().UTC(), actor); err != nil {
		t.Fatalf("RunDaily PPAP: %v", err)
	}
	outstanding := e.loanDecimal(t, loan.ID, "outstanding_principal")
	carrying := outstanding.Sub(loss)
	wantPPAP := domain.RoundToRupiah(carrying.Mul(decimal.NewFromFloat(0.10)))
	gotPPAP := e.loanDecimal(t, loan.ID, "required_ppap")
	if !gotPPAP.Equal(wantPPAP) {
		t.Fatalf("required_ppap %s, mau %s (10%% dari nilai tercatat %s)", gotPPAP, wantPPAP, carrying)
	}
	// Bukti negatif: dasar pokok bruto akan menghasilkan angka yang berbeda.
	if gotPPAP.Equal(domain.RoundToRupiah(outstanding.Mul(decimal.NewFromFloat(0.10)))) {
		t.Fatal("PPKA terlihat masih dihitung atas pokok bruto, bukan saldo setelah kerugian")
	}
}

// TestIntegrasiRestrukturisasiPelepasanCadanganSaatLunas membuktikan celah cadangan PPAP
// menggantung tertutup: kredit PAID_OFF yang masih menyimpan required_ppap ikut
// diproses dengan target nol dan melepas cadangannya lewat jalur PPAP yang ada.
func TestIntegrasiRestrukturisasiPelepasanCadanganSaatLunas(t *testing.T) {
	e := newMoneyEnv(t)

	branchCode := ckpnTestBranchCode("S")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Pelepasan Cadangan")
	cust := e.newCustomer(t, "Nasabah Cadangan Menggantung", fmt.Sprintf("gantung-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, e.actor, cust.ID, acc, decimal.NewFromInt(5_000_000), 12)

	// Kredit lunas, tetapi cadangan belum dilepas dan masih tersimpan.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status='PAID_OFF', outstanding_principal=0, dpd=0,
			collectibility='1_LANCAR', required_ppap=700000
		WHERE id=$1`, loan.ID); err != nil {
		t.Fatalf("menyetel kredit lunas: %v", err)
	}

	asOf := time.Now().UTC()
	if _, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.actor); err != nil {
		t.Fatalf("RunDaily PPAP: %v", err)
	}
	if got := e.loanDecimal(t, loan.ID, "required_ppap"); !got.IsZero() {
		t.Fatalf("required_ppap kredit lunas %s, mau nol setelah pelepasan", got)
	}
	key := fmt.Sprintf("PPAP-%s-%s-%s", loan.LoanNumber, asOf.Format("2006-01-02"), decimal.NewFromInt(-700_000).String())
	assertCKPNJournal(t, e, key, "10900", "DEBIT", "50200", "CREDIT", decimal.NewFromInt(700_000))
}
