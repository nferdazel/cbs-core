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
	// Pelepasan cadangan kredit cabang S dijalankan aktor cabang lain (e.actor, 001):
	// jurnal harus tetap diatribusikan ke cabang KREDIT, bukan cabang aktor.
	if bc := e.ckpnJournalBranch(t, key); bc != branchCode {
		t.Fatalf("cabang jurnal pelepasan %q, mau cabang kredit %q (bukan cabang aktor %q)",
			bc, branchCode, e.actor.BranchCode)
	}
}

// coaBalanceForLoan menghitung saldo satu akun COA dari jurnal yang kunci idempotensinya
// memuat nomor kredit, LANGSUNG dari journal_lines (bukan state di memori). Dipakai untuk
// membuktikan akun piutang kredit tepat nol setelah lunas, dan negatif tanpa amortisasi.
func coaBalanceForLoan(t *testing.T, e *moneyEnv, coaCode, loanNumber string) decimal.Decimal {
	t.Helper()
	var saldo decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT COALESCE(SUM(CASE WHEN jl.direction = 'DEBIT' THEN jl.amount ELSE -jl.amount END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts coa ON coa.id = a.coa_id
		WHERE coa.code = $1 AND je.idempotency_key LIKE '%' || $2 || '%'`,
		coaCode, loanNumber).Scan(&saldo); err != nil {
		t.Fatalf("membaca saldo %s kredit %s: %v", coaCode, loanNumber, err)
	}
	return saldo
}

// TestIntegrasiAmortisasiSaldoKerugianLunasNol membuktikan cacat piutang negatif benar
// tertutup: restrukturisasi dengan kerugian, amortisasi EIR berjalan sepanjang tenor,
// kredit lunas, lalu saldo akun piutang kredit (10301) tepat nol dan saldo kerugian nol,
// diperiksa langsung lewat jurnal di database.
func TestIntegrasiAmortisasiSaldoKerugianLunasNol(t *testing.T) {
	e := newMoneyEnv(t)
	// Saklar hidup SEBELUM pencairan agar EIR orisinal dicatat saat disbursement.
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "true")
	t.Cleanup(func() { setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false") })

	branchCode := ckpnTestBranchCode("A")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Amortisasi Kerugian")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujiamort", Role: domain.RoleAdmin, BranchCode: branchCode}

	cust := e.newCustomer(t, "Nasabah Amortisasi Kerugian", fmt.Sprintf("amort-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)

	if eir := e.loanDecimal(t, loan.ID, "original_eir_monthly"); !eir.IsPositive() {
		t.Fatalf("EIR orisinal tidak tersimpan: %s", eir)
	}
	if _, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
		Reason:                "uji integrasi amortisasi kerugian",
	}, actor); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	loss := e.loanDecimal(t, loan.ID, "restructure_loss_balance")
	if !loss.IsPositive() {
		t.Fatalf("saldo kerugian restrukturisasi %s, mau positif", loss)
	}

	schedules := e.schedules(t, loan.ID)
	if len(schedules) == 0 {
		t.Fatal("jadwal hasil restrukturisasi kosong")
	}

	// Amortisasi dan pembayaran berjalan sepanjang tenor. Aktor sengaja dari cabang
	// lain (e.actor, 001) agar atribusi cabang jurnal amortisasi ke cabang KREDIT
	// teruji.
	for _, sc := range schedules {
		asOf := sc.DueDate.Add(24 * time.Hour)
		if _, err := e.loanSvc.AmortizeRestructureLoss(e.ctx, asOf, e.actor); err != nil {
			t.Fatalf("amortisasi angsuran ke-%d: %v", sc.InstallmentNo, err)
		}
		afterFirst := e.loanDecimal(t, loan.ID, "restructure_loss_balance")
		if afterFirst.IsNegative() {
			t.Fatalf("saldo kerugian negatif setelah angsuran ke-%d: %s", sc.InstallmentNo, afterFirst)
		}
		// Pengulangan pada tanggal bisnis yang sama tidak menggandakan jurnal maupun
		// pengurangan saldo (idempotensi lewat kunci jurnal di repository).
		if _, err := e.loanSvc.AmortizeRestructureLoss(e.ctx, asOf, e.actor); err != nil {
			t.Fatalf("amortisasi ulang angsuran ke-%d: %v", sc.InstallmentNo, err)
		}
		if again := e.loanDecimal(t, loan.ID, "restructure_loss_balance"); !again.Equal(afterFirst) {
			t.Fatalf("pengulangan tanggal sama mengubah saldo kerugian angsuran ke-%d: %s -> %s",
				sc.InstallmentNo, afterFirst, again)
		}
		if n := countJournals(t, e, fmt.Sprintf("LOSSAMORT-%s-%d", loan.LoanNumber, sc.InstallmentNo)); n > 1 {
			t.Fatalf("jurnal amortisasi angsuran ke-%d terbit %d kali, mau paling banyak 1 (idempoten)", sc.InstallmentNo, n)
		}
		if _, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{
			LoanID: loan.ID, InstallmentNo: sc.InstallmentNo, Method: domain.LoanPaymentCash,
		}, e.actor); err != nil {
			t.Fatalf("membayar angsuran ke-%d: %v", sc.InstallmentNo, err)
		}
	}

	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusPaidOff) {
		t.Fatalf("status kredit %q, mau PAID_OFF", got)
	}
	if got := e.loanDecimal(t, loan.ID, "restructure_loss_balance"); !got.IsZero() {
		t.Fatalf("saldo kerugian setelah lunas %s, mau tepat nol", got)
	}
	// Saldo akun piutang kredit dari jurnal harus tepat nol: debit pencairan + debit
	// amortisasi = kredit kerugian + kredit pelunasan pokok.
	if got := coaBalanceForLoan(t, e, "10301", loan.LoanNumber); !got.IsZero() {
		t.Fatalf("saldo akun piutang 10301 kredit %s = %s, mau tepat nol", loan.LoanNumber, got)
	}

	// Cabang jurnal amortisasi = cabang kredit, bukan cabang aktor.
	firstKey := fmt.Sprintf("LOSSAMORT-%s-1", loan.LoanNumber)
	if bc := e.ckpnJournalBranch(t, firstKey); bc != branchCode {
		t.Fatalf("cabang jurnal amortisasi %q, mau cabang kredit %q (bukan cabang aktor %q)",
			bc, branchCode, e.actor.BranchCode)
	}
}

// TestIntegrasiAmortisasiSaklarMatiPiutangNegatif mendokumentasikan perilaku LAMA saat
// saklar mati: kerugian sudah dijurnal sebagai kredit akun piutang, tetapi amortisasi
// tidak berjalan. Setelah kredit lunas, saldo akun piutang menjadi negatif sebesar
// kerugian. Uji ini pembanding (bukan asersi yang dilonggarkan).
func TestIntegrasiAmortisasiSaklarMatiPiutangNegatif(t *testing.T) {
	e := newMoneyEnv(t)
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "true")
	t.Cleanup(func() { setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false") })

	branchCode := ckpnTestBranchCode("N")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Tanpa Amortisasi")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujitanpa", Role: domain.RoleAdmin, BranchCode: branchCode}

	cust := e.newCustomer(t, "Nasabah Tanpa Amortisasi", fmt.Sprintf("tanpa-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)

	if _, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
		Reason:                "uji integrasi saklar mati",
	}, actor); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	loss := e.loanDecimal(t, loan.ID, "restructure_loss_balance")
	if !loss.IsPositive() {
		t.Fatalf("saldo kerugian %s, mau positif", loss)
	}

	// Matikan amortisasi, lalu bayar seluruh tenor.
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
	for _, sc := range e.schedules(t, loan.ID) {
		if _, err := e.loanSvc.AmortizeRestructureLoss(e.ctx, sc.DueDate.Add(24*time.Hour), actor); err != nil {
			t.Fatalf("amortisasi saklar mati: %v", err)
		}
		if _, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{
			LoanID: loan.ID, InstallmentNo: sc.InstallmentNo, Method: domain.LoanPaymentCash,
		}, actor); err != nil {
			t.Fatalf("membayar angsuran ke-%d: %v", sc.InstallmentNo, err)
		}
	}

	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusPaidOff) {
		t.Fatalf("status kredit %q, mau PAID_OFF", got)
	}
	// Tanpa amortisasi, akun piutang tertinggal negatif sebesar kerugian.
	if got := e.loanDecimal(t, loan.ID, "restructure_loss_balance"); !got.Equal(loss) {
		t.Fatalf("saldo kerugian tanpa amortisasi %s, mau tetap %s", got, loss)
	}
	got := coaBalanceForLoan(t, e, "10301", loan.LoanNumber)
	if !got.Equal(loss.Neg()) {
		t.Fatalf("saldo akun piutang 10301 tanpa amortisasi %s, mau %s (negatif sebesar kerugian)", got, loss.Neg())
	}
}
