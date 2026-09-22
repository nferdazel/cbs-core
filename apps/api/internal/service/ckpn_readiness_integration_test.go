package service_test

import (
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Uji kesiapan rilis CKPN terhadap PostgreSQL sungguhan. Di-skip kecuali
// CBS_TEST_DB_DSN diisi, mengikuti pola ckpn_integration_test.go.
//
//	TESTSKIP_DB CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run KesiapanRilisCKPN -v
//
// Yang dibuktikan: angka CKPN portofolio campuran (kredit lancar/aset baik, NPL,
// restrukturisasi, hapus buku/pelepasan cadangan, dan syariah) sama persis dengan
// hitungan manual, jalur penjurnalan ckpn.enabled=true memakai akun per buku, dan
// menjalankan proses dua kali pada tanggal bisnis yang sama TIDAK menambah jurnal.
func TestKesiapanRilisCKPNPortofolioCampuranDIHitungManual(t *testing.T) {
	e := newMoneyEnv(t)

	// Parameter kebijakan: FRAKSI 0..1, bukan persen.
	const (
		pdGol3 = "0.15"
		lgd    = "0.40"
	)
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
	setCKPNConfig(t, e, "ckpn.pd_frac.gol_3", pdGol3)
	setCKPNConfig(t, e, "ckpn.lgd_frac", lgd)
	// Akun CKPN per buku: kunci syariah diisi agar pembiayaan TIDAK jatuh ke akun
	// konvensional. Kunci global sudah diisi 50301/10950 oleh migrasi 000069.
	setCKPNConfig(t, e, "ckpn.coa.expense", "50301")
	setCKPNConfig(t, e, "ckpn.coa.reserve", "10950")
	setCKPNConfig(t, e, "ckpn.coa.expense.syariah", "15901")
	setCKPNConfig(t, e, "ckpn.coa.reserve.syariah", "11950")

	// Cabang uji tersendiri supaya kredit dari uji lain tidak masuk ringkasan.
	branchCode := ckpnTestBranchCode("R")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Kesiapan CKPN")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujikesiapan", Role: domain.RoleAdmin, BranchCode: branchCode}

	newConv := func(name string, amount int64) *domain.Loan {
		t.Helper()
		cust := e.newCustomer(t, name, fmt.Sprintf("kesiapan-%d-%s@uji.local", time.Now().UnixNano(), name))
		acc := e.newAccountInBranch(t, cust.ID, branchID)
		return e.disburseAs(t, actor, cust.ID, acc, idr(amount), 12)
	}

	// L — Lancar tanpa tunggakan: memenuhi kriteria aset baik (butir 12.3.a.1.c) dan
	// dikecualikan sebagai pilihan kebijakan (butir 12.3.a.2.a), maka CKPN-nya nol
	// TANPA memerlukan PD/LGD.
	loanL := newConv("Nasabah Kesiapan L", 10_000_000)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility='1_LANCAR', dpd=0, required_ppap=0, required_ckpn=0
		WHERE id=$1`, loanL.ID); err != nil {
		t.Fatalf("menyiapkan kredit L: %v", err)
	}

	// N — Kurang Lancar (NPL), EAD = 20.000.000, PPKA 2.000.000.
	// CKPN = 20.000.000 x 15% x 40% = 1.200.000.
	loanN := newConv("Nasabah Kesiapan N", 20_000_000)
	e.setCKPNState(t, loanN.ID, "3_KURANG_LANCAR", 100, idr(2_000_000))

	// R — direstrukturisasi dengan saldo kerugian restrukturisasi 6.000.000.
	// EAD = 30.000.000 - 6.000.000 = 24.000.000 (nilai tercatat, sama dengan dasar PPKA).
	// CKPN = 24.000.000 x 15% x 40% = 1.440.000. Kalau EAD keliru memakai pokok bruto,
	// hasilnya 1.800.000 dan uji ini gagal.
	loanR := newConv("Nasabah Kesiapan R", 30_000_000)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility='3_KURANG_LANCAR', dpd=100, required_ppap=$2,
			is_restructured=TRUE, pre_restructure_collectibility='1_LANCAR',
			restructure_loss_balance=$3, required_ckpn=0
		WHERE id=$1`, loanR.ID, idr(3_000_000), idr(6_000_000)); err != nil {
		t.Fatalf("menyiapkan kredit R: %v", err)
	}

	// W — sudah hapus buku: tidak punya eksposur, tetapi masih menyimpan cadangan
	// 500.000. Targetnya WAJIB nol dan cadangannya dilepas lewat jurnal pemulihan.
	loanW := newConv("Nasabah Kesiapan W", 5_000_000)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status='WRITTEN_OFF', outstanding_principal=0,
			collectibility='5_MACET', dpd=400, required_ppap=0, required_ckpn=$2
		WHERE id=$1`, loanW.ID, idr(500_000)); err != nil {
		t.Fatalf("menyiapkan kredit W: %v", err)
	}

	// S — pembiayaan syariah (PMB-MURABAHAH), EAD = 40.000.000, PPKA 4.000.000.
	// CKPN = 40.000.000 x 15% x 40% = 2.400.000, dijurnal ke akun syariah 15901/11950.
	custS := e.newCustomer(t, "Nasabah Kesiapan S", fmt.Sprintf("kesiapan-sy-%d@uji.local", time.Now().UnixNano()))
	accS := e.newSyariahAccountInBranch(t, custS.ID, branchID)
	loanS := e.disburseWithProductAs(t, actor, "PMB-MURABAHAH", custS.ID, accS, idr(40_000_000), 12)
	e.setCKPNState(t, loanS.ID, "3_KURANG_LANCAR", 100, idr(4_000_000))

	beforeJournals := e.countCKPNJournals(t)
	ckpnSvc := newCKPNSvcForTest(e)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := ckpnSvc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}
	if summary.Processed != 5 || summary.Failed != 0 {
		t.Fatalf("processed=%d failed=%d, mau 5/0 (%+v)", summary.Processed, summary.Failed, summary.Failures)
	}

	// --- Hitungan manual per kredit (dibandingkan persis, bukan "lulus saja") ---
	const (
		ckpnL = int64(0)         // aset baik
		ckpnN = int64(1_200_000) // 20jt x 0,15 x 0,40
		ckpnR = int64(1_440_000) // (30jt-6jt) x 0,15 x 0,40
		ckpnW = int64(0)         // WRITTEN_OFF: dilepas
		ckpnS = int64(2_400_000) // 40jt x 0,15 x 0,40
	)
	wantTotalCKPN := decimal.NewFromInt(ckpnL + ckpnN + ckpnR + ckpnW + ckpnS)
	wantTotalPPKA := decimal.NewFromInt(0 + 2_000_000 + 3_000_000 + 0 + 4_000_000)
	// Potensi pengurang modal inti = jumlah selisih positif PPKA > CKPN per kredit:
	// (2jt-1,2jt) + (3jt-1,44jt) + (4jt-2,4jt) = 3.960.000.
	wantDeduction := decimal.NewFromInt(800_000 + 1_560_000 + 1_600_000)

	if !summary.TotalCKPN.Equal(wantTotalCKPN) {
		t.Fatalf("total CKPN %s, mau %s (manual)", summary.TotalCKPN, wantTotalCKPN)
	}
	if !summary.TotalPPKA.Equal(wantTotalPPKA) {
		t.Fatalf("total PPKA %s, mau %s (manual)", summary.TotalPPKA, wantTotalPPKA)
	}
	if !summary.ModalIntiDeduction.Equal(wantDeduction) {
		t.Fatalf("pengurang modal inti %s, mau %s (manual, Σ per kredit)", summary.ModalIntiDeduction, wantDeduction)
	}
	if summary.AsetBaikCount != 1 || !summary.AsetBaikOutstanding.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("aset baik %d/%s, mau 1/10000000", summary.AsetBaikCount, summary.AsetBaikOutstanding)
	}

	perLoan := []struct {
		loan  *domain.Loan
		want  int64
		label string
	}{
		{loanL, ckpnL, "L (aset baik)"},
		{loanN, ckpnN, "N (NPL)"},
		{loanR, ckpnR, "R (restrukturisasi)"},
		{loanW, ckpnW, "W (hapus buku)"},
		{loanS, ckpnS, "S (syariah)"},
	}
	for _, tc := range perLoan {
		item := ckpnItem(t, summary, tc.loan.ID)
		if !item.CKPN.Equal(decimal.NewFromInt(tc.want)) {
			t.Fatalf("CKPN %s = %s, mau %d (manual)", tc.label, item.CKPN, tc.want)
		}
	}

	// --- Jurnal per kredit, akun per buku ---
	assertCKPNJournal(t, e, ckpnIdempotencyKey(loanN.LoanNumber, asOf, decimal.NewFromInt(ckpnN)),
		"50301", "DEBIT", "10950", "CREDIT", decimal.NewFromInt(ckpnN))
	assertCKPNJournal(t, e, ckpnIdempotencyKey(loanR.LoanNumber, asOf, decimal.NewFromInt(ckpnR)),
		"50301", "DEBIT", "10950", "CREDIT", decimal.NewFromInt(ckpnR))
	assertCKPNJournal(t, e, ckpnIdempotencyKey(loanW.LoanNumber, asOf, decimal.NewFromInt(-500_000)),
		"10950", "DEBIT", "50301", "CREDIT", decimal.NewFromInt(500_000))
	assertCKPNJournal(t, e, ckpnIdempotencyKey(loanS.LoanNumber, asOf, decimal.NewFromInt(ckpnS)),
		"15901", "DEBIT", "11950", "CREDIT", decimal.NewFromInt(ckpnS))

	// Target tersimpan sama dengan hitungan manual.
	for _, tc := range perLoan {
		if got := e.loanDecimal(t, tc.loan.ID, "required_ckpn"); !got.Equal(decimal.NewFromInt(tc.want)) {
			t.Fatalf("required_ckpn %s = %s, mau %d", tc.label, got, tc.want)
		}
	}

	// --- Idempotensi: run ulang pada tanggal bisnis yang sama tidak menambah jurnal ---
	if got := e.countCKPNJournals(t) - beforeJournals; got != 4 {
		t.Fatalf("jurnal CKPN pertama %d, mau 4 (N, R, W pemulihan, S)", got)
	}
	if _, err := ckpnSvc.Run(e.ctx, asOf, actor); err != nil {
		t.Fatalf("Run CKPN kedua: %v", err)
	}
	if got := e.countCKPNJournals(t) - beforeJournals; got != 4 {
		t.Fatalf("run ulang menambah jurnal: total %d, mau tetap 4", got)
	}
}
