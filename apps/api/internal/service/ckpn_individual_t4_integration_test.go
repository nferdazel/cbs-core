package service_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// TAHAP T4 CKPN individual: integrasi ke langkah CKPN EOD (rancangan §6.2).
// Angka diuji dengan tangan:
//
//	carrying 150.000 (pokok 150.000, tanpa kerugian restrukturisasi), EIR 10%/bulan,
//	proyeksi 110.000 pada periode 1 → PV = 110.000/1,1 = 100.000 → target DCF 50.000.
//	Segesementasi itu yang harus menjadi required_ckpn — BUKAN hasil kolektif
//	EAD×PD×LGD (yang pada kolektibilitas sama akan bernilai lain).
//	Lantai 12.4.g.1.c: run kedua dengan proyeksi dikecilkan TIDAK boleh menurunkan
//	target di bawah CKPN individual yang sudah dibentuk.
func TestIntegrasiCKPNIndividualT4EODJalurIndividual(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.individual.enabled", "true")
	setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	setCKPNConfig(t, e, "ckpn.individual.method", "MAX")
	setCKPNConfig(t, e, "ckpn.individual.significance_top_n", "0")
	// Jalur TULIS (Run) hanya memposting bila ckpn.enabled=true; bayangan menyala
	// memaksa post=false. Mode resmi dinyalakan agar Run benar-benar menjurnal.
	simpanPulihkanConfigCKPN(t, e)
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
		setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	})

	branchCode := ckpnTestBranchCode("T4")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T4")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit4", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T4", "t4-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)
	// State kredit dipaksa seperti hasil jalur PPAP (pola uji CKPN kolektif).
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET outstanding_principal=$2, restructure_loss_balance=0,
		                  collectibility='4_DIRAGUKAN', dpd=120, required_ppap=$3,
		                  original_eir_monthly=0.10
		WHERE id=$1`, loan.ID, idr(150_000), idr(1_000_000)); err != nil {
		t.Fatalf("menyiapkan state kredit T4: %v", err)
	}
	indSvc := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)
	if err := indSvc.ReplaceProjections(e.ctx, loan.LoanNumber, ckpnTestAsOf(),
		[]domain.CKPNCashflowProjection{{Period: 1, Amount: idr(110_000)}}, actor); err != nil {
		t.Fatalf("menyimpan proyeksi T4: %v", err)
	}
	// Segel individual: keputusan pengelola lewat jalur T3.
	if _, err := indSvc.MarkLoanEntry(e.ctx, loan.LoanNumber, domain.CKPNIndividualMethodMax,
		false, false, false, actor); err != nil {
		t.Fatalf("menandai jalur individual: %v", err)
	}

	ckpnSvc := newCKPNSvcForTest(e)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := ckpnSvc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN T4: %v", err)
	}
	item := ckpnItem(t, summary, loan.ID)
	// Jalur individual TANPA agunan: aturan MAX tidak boleh memakai komponen agunan
	// (Σ NRV kosong = carrying penuh memalsukan "lebih konservatif"). PV =
	// 110.000/1,1 = 100.000 → target = 150.000 − 100.000 = 50.000.
	if !item.CKPN.Equal(idr(50_000)) {
		t.Fatalf("CKPN individual %s, mau 50000 (DCF); kolektif EADxPDxLGD bukan angka ini", item.CKPN)
	}
	if summary.IndividualCount != 1 || !summary.IndividualTotalCKPN.Equal(idr(50_000)) {
		t.Fatalf("ringkasan individual count=%d total=%s, mau 1/50000",
			summary.IndividualCount, summary.IndividualTotalCKPN)
	}
	// required_ckpn tersimpan = target individual (satu sumber kebenaran).
	if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.Equal(idr(50_000)) {
		t.Fatalf("required_ckpn %s, mau 50000", got)
	}
	if got := e.loanDecimal(t, loan.ID, "ckpn_individual_target"); !got.Equal(idr(50_000)) {
		t.Fatalf("ckpn_individual_target jejak %s, mau 50000", got)
	}
	// Jurnal pembentukan 50.000 tercatat dengan kunci idempotensi standar.
	var jmlBentuk int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM journal_entries WHERE idempotency_key = $1`,
		ckpnIdempotencyKey(loan.LoanNumber, asOf, idr(50_000))).Scan(&jmlBentuk); err != nil {
		t.Fatalf("membaca jurnal pembentukan: %v", err)
	}
	if jmlBentuk != 1 {
		t.Fatalf("jurnal pembentukan individual harus 1 baris, dapat %d", jmlBentuk)
	}
	// Jejak audit T4 tertulis.
	var jejak int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM loan_ckpn_individual_assessments WHERE loan_id=$1 AND basis::text LIKE '%eod_t4%'`,
		loan.ID).Scan(&jejak); err != nil {
		t.Fatalf("membaca jejak EOD: %v", err)
	}
	if jejak != 1 {
		t.Fatalf("jejak EOD individual harus 1, dapat %d", jejak)
	}

	// Lantai 12.4.g.1.c: proyeksi dikecilkan → target DCF baru lebih kecil, tetapi
	// required_ckpn TIDAK boleh turun di bawah CKPN individual yang sudah dibentuk.
	if err := indSvc.ReplaceProjections(e.ctx, loan.LoanNumber, ckpnTestAsOf(),
		[]domain.CKPNCashflowProjection{{Period: 1, Amount: idr(120_000)}}, actor); err != nil {
		t.Fatalf("mengganti proyeksi: %v", err)
	}
	summary2, err := ckpnSvc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN T4 kedua: %v", err)
	}
	item2 := ckpnItem(t, summary2, loan.ID)
	// PV baru = 120.000/1,1 = 109.091 → target DCF baru 40.909 < 50.000; lantai menahan 50.000.
	if !item2.CKPN.Equal(idr(50_000)) {
		t.Fatalf("lantai 12.4.g.1.c gagal: CKPN run kedua %s, mau tetap 50000", item2.CKPN)
	}
	// Idempoten: tidak ada jurnal tambahan pada target sama.
	var jmlJurnal int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM journal_entries WHERE idempotency_key = $1`,
		ckpnIdempotencyKey(loan.LoanNumber, asOf, idr(50_000))).Scan(&jmlJurnal); err != nil {
		t.Fatalf("menghitung jurnal: %v", err)
	}
	if jmlJurnal != 1 {
		t.Fatalf("jurnal duplikat: %d baris dengan kunci sama", jmlJurnal)
	}
}

func TestIntegrasiCKPNIndividualT4AntiDobelHitungDanPengecualian(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanConfigCKPN(t, e)
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.individual.enabled", "true")
	setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "12")
	setCKPNConfig(t, e, "ckpn.individual.significance_top_n", "0")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.enabled", "false")
		setCKPNConfig(t, e, "ckpn.individual.discount_rate_annual_pct", "")
	})

	branchCode := ckpnTestBranchCode("T4B")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T4B")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit4b", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T4B", "t4b-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET outstanding_principal=$2, restructure_loss_balance=0,
		                  collectibility='4_DIRAGUKAN', dpd=120, required_ppap=$3,
		                  original_eir_monthly=0.10
		WHERE id=$1`, loan.ID, idr(150_000), idr(1_000_000)); err != nil {
		t.Fatalf("menyiapkan state kredit T4B: %v", err)
	}
	indSvc := service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)
	if err := indSvc.ReplaceProjections(e.ctx, loan.LoanNumber, ckpnTestAsOf(),
		[]domain.CKPNCashflowProjection{{Period: 1, Amount: idr(110_000)}}, actor); err != nil {
		t.Fatalf("menyimpan proyeksi T4B: %v", err)
	}

	// A. Compare (baca-saja) TANPA segel: kredit macet-akan tetap dihitung KOLEKTIF
	// (segel kosong). Ini memastikan mesin tidak menduga-duga jalur sendiri.
	ckpnSvc := newCKPNSvcForTest(e)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)
	// Jalur kolektif pembanding butuh PD/LGD terisi (parameter kebijakan, bukan kode).
	setCKPNConfig(t, e, "ckpn.pd_frac.gol_4", "0.50")
	setCKPNConfig(t, e, "ckpn.lgd_frac", "0.50")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.pd_frac.gol_4", "")
		setCKPNConfig(t, e, "ckpn.lgd_frac", "")
	})
	comparison, err := ckpnSvc.Compare(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Compare tanpa segel: %v", err)
	}
	item := ckpnItem(t, comparison, loan.ID)
	// 150.000 × PD 0,50 × LGD 0,50 = 37.500.
	if !item.CKPN.Equal(idr(37_500)) {
		t.Fatalf("tanpa segel harus kolektif 37500, dapat %s", item.CKPN)
	}
	if comparison.IndividualCount != 0 {
		t.Fatalf("tanpa segel tidak boleh ada kredit individual, dapat %d", comparison.IndividualCount)
	}

	// B. Segel EXCLUDED_ASET_BAIK: target nol tanpa proyeksi; cadangan yang belum
	// terbentuk memang nol, jadi tidak ada jurnal, tetapi jalurnya sah.
	if _, err := indSvc.MarkLoanEntry(e.ctx, loan.LoanNumber, "", false, false, true, actor); err != nil {
		t.Fatalf("menandai pengecualian: %v", err)
	}
	comparison, err = ckpnSvc.Compare(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Compare pengecualian: %v", err)
	}
	t.Logf("DEBUG pengecualian: total=%d items=%d failures=%+v", comparison.Total, len(comparison.Items), comparison.Failures)
	item = ckpnItem(t, comparison, loan.ID)
	if !item.CKPN.IsZero() || !item.IsAsetBaik {
		t.Fatalf("kredit tersegel pengecualian harus aset baik bernilai nol: %+v", item)
	}

	// C. Segel MAX tanpa proyeksi pada jalur TULIS: GAGAL, bukan jatuh ke kolektif.
	// Ganti segel ke MAX (proyeksi sengaja dihapus dulu).
	if _, err := e.db.ExecContext(e.ctx, `DELETE FROM loan_cashflow_projections WHERE loan_id=$1`, loan.ID); err != nil {
		t.Fatalf("menghapus proyeksi uji: %v", err)
	}
	if _, err := indSvc.MarkLoanEntry(e.ctx, loan.LoanNumber, domain.CKPNIndividualMethodMax,
		true, false, false, actor); err != nil {
		t.Fatalf("menandai MAX: %v", err)
	}
	// Kegagalan per-kredit tidak menggagalkan Run, tetapi kredit itu harus masuk
	// Failures dengan penyebab jelas — bukan jatuh diam-diam ke jalur kolektif.
	summaryC, err := ckpnSvc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run T4C: %v", err)
	}
	if summaryC.Failed == 0 || len(summaryC.Failures) == 0 {
		t.Fatalf("kredit individual MAX tanpa proyeksi harus tercatat gagal, bukan dihitung kolektif: failed=%d", summaryC.Failed)
	}
	// required_ckpn tidak boleh terlanjur ditulis kolektif.
	if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.IsZero() {
		t.Fatalf("kegagalan jalur individual tidak boleh menulis required_ckpn, tertulis %s", got)
	}
}
