package service_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// newBatchSvcForTest merangkai BatchProcessService produksi di atas database uji.
// Layanan yang tidak dibutuhkan jalur EOD ini (ARO deposito, tutup buku tahunan,
// dormant) sengaja nil sehingga langkahnya dilewati; PPAP, akrual/amortisasi kredit,
// dan CKPN memakai layanan produksi yang sama dengan main.go.
func (e *moneyEnv) newBatchSvcForTest(t *testing.T, ckpn domain.CKPNService) domain.BatchProcessService {
	t.Helper()
	ledgerRepo := postgres.NewLedgerRepository(e.db)
	accountRepo := postgres.NewAccountRepository(e.db)
	dateRepo := postgres.NewBusinessDateRepository(e.db)
	referenceGen := postgres.NewReferenceGenerator(e.db)
	postingSvc := service.NewPostingService(e.db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	batchRepo := postgres.NewBatchActivityRepository(e.db)
	return service.NewBatchProcessService(
		dateRepo, batchRepo,
		nil, // savings/ARO: langkah dilewati
		nil, // year end: tidak dipakai EOD
		postingSvc, ledgerRepo, e.configSvc, e.db,
		nil,       // ARO runner
		e.ppapSvc, // PPAP
		e.loanSvc, // akrual denda
		nil,       // dormant runner
		e.loanSvc, // akrual bunga + amortisasi saldo kerugian
		ckpn,      // perbandingan CKPN
		postgres.NewEODStepRepository(e.db),
	)
}

// eodStepIndex mengembalikan posisi satu langkah pada ringkasan EOD.
func eodStepIndex(t *testing.T, summary *domain.EODSummaryResult, name string) int {
	t.Helper()
	for i, step := range summary.Steps {
		if step.Name == name {
			return i
		}
	}
	t.Fatalf("langkah %q tidak ada pada ringkasan EOD: %+v", name, summary.Steps)
	return -1
}

// TestIntegrasiEODAmortisasiSebelumPPAP menjalankan tutup hari sungguhan pada satu
// database: akrual, amortisasi saldo kerugian restrukturisasi, PPAP, lalu CKPN.
// Dibuktikan bahwa amortisasi berjalan lebih dulu, PPAP memakai saldo setelah
// amortisasi, dan penanda run PPAP memungkinkan CKPN berjalan pada tanggal yang sama.
// Selisih required_ppap sebelum vs sesudah amortisasi dilaporkan eksplisit.
func TestIntegrasiEODAmortisasiSebelumPPAP(t *testing.T) {
	e := newMoneyEnv(t)
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "true")
	t.Cleanup(func() { setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false") })
	// Agunan mati agar eksposur sama dengan nilai tercatat, sehingga selisih akibat
	// urutan amortisasi tidak tercampur pengurang agunan.
	setRestructureLossConfig(t, e, "ppap.collateral.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd.3", "0.10")
	setCKPNConfig(t, e, "ckpn.lgd", "0.50")

	branchCode := ckpnTestBranchCode("X")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji EOD")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujieod", Role: domain.RoleAdmin, BranchCode: branchCode}

	cust := e.newCustomer(t, "Nasabah EOD Amortisasi", fmt.Sprintf("eod-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	// Saklar kerugian restrukturisasi hidup SEBELUM pencairan agar EIR orisinal dicatat.
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)

	// NPL sebelum restrukturisasi agar Pasal 31 menahan golongannya di Kurang Lancar
	// (tarif 10%) walau DPD dijadwalkan ulang.
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loans SET dpd=100, collectibility='3_KURANG_LANCAR' WHERE id=$1`, loan.ID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}
	if _, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
		Reason:                "uji integrasi EOD",
	}, actor); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}

	lossBefore := e.loanDecimal(t, loan.ID, "restructure_loss_balance")
	if !lossBefore.IsPositive() {
		t.Fatalf("saldo kerugian restrukturisasi %s, mau positif", lossBefore)
	}
	if eir := e.loanDecimal(t, loan.ID, "original_eir_monthly"); !eir.IsPositive() {
		t.Fatalf("EIR orisinal tidak tersimpan: %s", eir)
	}

	// Angsuran pertama jatuh tempo kemarin agar amortisasi periode ini benar-benar
	// berjalan pada tutup hari.
	e.setOldestDueDate(t, loan.ID, time.Now().UTC().AddDate(0, 0, -1))

	ckpnSvc := newCKPNSvcForTest(e)
	batchSvc := e.newBatchSvcForTest(t, ckpnSvc)

	summary, err := batchSvc.RunEOD(e.ctx, e.actor.UserID)
	if err != nil {
		t.Fatalf("RunEOD: %v", err)
	}

	// Urutan: amortisasi lebih dulu dari PPAP, dan keduanya RAN.
	amortStep := eodStepStatus(t, summary, "restructure_loss_amortization")
	ppapStep := eodStepStatus(t, summary, "ppap")
	ckpnStep := eodStepStatus(t, summary, "ckpn_comparison")
	if amortStep.Status != domain.EODStepRan {
		t.Fatalf("status amortisasi %s (%s), mau RAN", amortStep.Status, amortStep.Reason)
	}
	if ppapStep.Status != domain.EODStepRan {
		t.Fatalf("status PPAP %s (%s), mau RAN", ppapStep.Status, ppapStep.Reason)
	}
	if ckpnStep.Status != domain.EODStepRan {
		t.Fatalf("status CKPN %s (%s), mau RAN", ckpnStep.Status, ckpnStep.Reason)
	}
	if eodStepIndex(t, summary, "restructure_loss_amortization") >= eodStepIndex(t, summary, "ppap") {
		t.Fatalf("amortisasi harus berjalan sebelum PPAP: %+v", summary.Steps)
	}

	// Dampak angka: PPAP harus memakai nilai tercatat SETELAH amortisasi. Eksposur =
	// nilai tercatat (agunan mati), tarif 10% (Kurang Lancar).
	//
	// Amortisasi kredit ini dibaca dari selisih saldo kerugiannya sendiri sebelum vs
	// sesudah EOD, BUKAN dari summary.LoanLossAmortizedAmount yang bank-wide: ringkasan
	// itu menjumlahkan amortisasi seluruh kredit di database, sehingga pada database
	// yang juga memuat kredit uji lain nilainya dapat melebihi saldo kerugian kredit
	// ini dan membuat ekspektasi required_ppap salah.
	outstanding := e.loanDecimal(t, loan.ID, "outstanding_principal")
	lossAfter := e.loanDecimal(t, loan.ID, "restructure_loss_balance")
	amortized := lossBefore.Sub(lossAfter)
	if !amortized.IsPositive() || amortized.GreaterThan(lossBefore) {
		t.Fatalf("amortisasi kredit ini %s tidak wajar terhadap saldo kerugian sebelum %s", amortized, lossBefore)
	}
	rate := decimal.NewFromFloat(0.10)

	// Angka "sebelum" adalah apa yang AKAN dipakai PPAP bila ia berjalan lebih dulu
	// (nilai tercatat dikurangi saldo kerugian pra-amortisasi).
	exposureBefore := outstanding.Sub(lossBefore)
	exposureAfter := outstanding.Sub(lossAfter)
	requiredBefore := domain.RoundToRupiah(exposureBefore.Mul(rate))
	requiredAfter := domain.RoundToRupiah(exposureAfter.Mul(rate))

	got := e.loanDecimal(t, loan.ID, "required_ppap")
	if !got.Equal(requiredAfter) {
		t.Fatalf("required_ppap %s, mau %s (10%% dari nilai tercatat setelah amortisasi %s)",
			got, requiredAfter, exposureAfter)
	}
	if got.Equal(requiredBefore) {
		t.Fatal("required_ppap sama dengan perhitungan pra-amortisasi; urutan tidak berdampak")
	}
	delta := requiredBefore.Sub(requiredAfter)
	t.Logf("dampak urutan amortisasi->PPAP: exposure pra=%s pasca=%s; required_ppap pra=%s pasca=%s; selisih=%s",
		exposureBefore, exposureAfter, requiredBefore, requiredAfter, delta)

	// Penanda run PPAP tercatat pada tanggal bisnis yang ditutup, sehingga CKPN berjalan
	// pada tanggal yang sama (bukan tanggal maju).
	marker := service.NewPPAPRunMarker(postgres.NewSystemConfigRepository(e.db))
	lastRun, ok, err := marker.LastRunBusinessDate(e.ctx)
	if err != nil {
		t.Fatalf("membaca penanda run PPAP: %v", err)
	}
	if !ok {
		t.Fatal("penanda run PPAP tidak tercatat setelah PPAP berhasil")
	}
	if lastRun.Format("2006-01-02") != summary.ExecutedDate.Format("2006-01-02") {
		t.Fatalf("penanda run PPAP %s, mau tanggal bisnis yang ditutup %s",
			lastRun.Format("2006-01-02"), summary.ExecutedDate.Format("2006-01-02"))
	}
}

// TestIntegrasiCKPNMenolakPPAPTanggalLain membuktikan gerbang CKPN pada database
// sungguhan: setelah PPAP berjalan tanggal bisnis lain, perbandingan CKPN ditolak dan
// pesannya menyebut tanggal bisnis PPAP terakhir. Juga dibuktikan bahwa jalur tutup
// hari pada tanggal yang sama tetap berjalan normal.
func TestIntegrasiCKPNMenolakPPAPTanggalLain(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd.3", "0.10")
	setCKPNConfig(t, e, "ckpn.lgd", "0.50")

	// Jalur tutup hari pada tanggal bisnis tertentu berjalan normal.
	ckpnSvc := newCKPNSvcForTest(e)
	batchSvc := e.newBatchSvcForTest(t, ckpnSvc)
	summary, err := batchSvc.RunEOD(e.ctx, e.actor.UserID)
	if err != nil {
		t.Fatalf("RunEOD: %v", err)
	}
	executed := summary.ExecutedDate
	if step := eodStepStatus(t, summary, "ckpn_comparison"); step.Status != domain.EODStepRan {
		t.Fatalf("perbandingan CKPN pada tutup hari %s, mau RAN: %s", executed.Format("2006-01-02"), step.Reason)
	}

	// Panggilan manual pada tanggal bisnis lain harus ditolak: required_ppap masih
	// milik tanggal bisnis yang baru ditutup.
	other := executed.AddDate(0, 0, 1)
	_, err = ckpnSvc.Compare(e.ctx, other, e.actor)
	if err == nil {
		t.Fatal("Compare pada tanggal bisnis lain harus ditolak, bukan memakai PPKA basi")
	}
	if want := executed.Format("2006-01-02"); !strings.Contains(err.Error(), want) {
		t.Fatalf("pesan %q harus menyebut tanggal bisnis PPAP terakhir %s", err, want)
	}
}

// countCKPNJournals menghitung jurnal ber-kunci idempotensi CKPN (prefix "CKPN-").
// Jurnal PPAP/akrual memakai prefix lain sehingga tidak tercampur.
func (e *moneyEnv) countCKPNJournals(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT count(*) FROM journal_entries WHERE idempotency_key LIKE 'CKPN-%'`).Scan(&n); err != nil {
		t.Fatalf("menghitung jurnal CKPN: %v", err)
	}
	return n
}

// TestIntegrasiCKPNModeBayanganTidakMenjurnal menjalankan tutup hari sungguhan dengan
// ckpn.enabled mati dan ckpn.shadow_mode.enabled hidup. Dibuktikan bahwa CKPN dihitung
// dan dilaporkan di bidang bayangan, TETAPI tidak ada jurnal CKPN bertambah dan
// required_ckpn tidak berubah. Ini bukti "nol jurnal" pada database nyata, bukan stub.
func TestIntegrasiCKPNModeBayanganTidakMenjurnal(t *testing.T) {
	e := newMoneyEnv(t)
	setRestructureLossConfig(t, e, "ppap.collateral.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "true")
	// Kembalikan saklar ke bawaan produksi agar uji integrasi lain tidak terpengaruh.
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
		setCKPNConfig(t, e, "ckpn.enabled", "false")
	})
	// Semua PD + LGD diisi agar CKPN dapat dihitung, bukan dilaporkan sebagai gap.
	setCKPNConfig(t, e, "ckpn.pd.1", "0.005")
	setCKPNConfig(t, e, "ckpn.pd.2", "0.05")
	setCKPNConfig(t, e, "ckpn.pd.3", "0.10")
	setCKPNConfig(t, e, "ckpn.pd.4", "0.30")
	setCKPNConfig(t, e, "ckpn.pd.5", "0.50")
	setCKPNConfig(t, e, "ckpn.lgd", "0.50")

	branchCode := ckpnTestBranchCode("S")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Bayangan")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujibayangan", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah Uji Bayangan", fmt.Sprintf("bayangan-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)

	beforeJournals := e.countCKPNJournals(t)
	beforeRequired := e.loanDecimal(t, loan.ID, "required_ckpn")

	ckpnSvc := newCKPNSvcForTest(e)
	batchSvc := e.newBatchSvcForTest(t, ckpnSvc)
	summary, err := batchSvc.RunEOD(e.ctx, e.actor.UserID)
	if err != nil {
		t.Fatalf("RunEOD: %v", err)
	}

	if step := eodStepStatus(t, summary, "ckpn_comparison"); step.Status != domain.EODStepRan {
		t.Fatalf("perbandingan CKPN bayangan = %s, mau RAN: %s", step.Status, step.Reason)
	}
	if !summary.CKPNShadowMode {
		t.Fatal("ringkasan EOD harus menandai mode bayangan")
	}
	if summary.CKPNShadowProcessed == 0 {
		t.Fatal("mode bayangan harus menghitung kredit, tidak ada yang diproses")
	}
	if !strings.Contains(summary.CKPNShadowNote, "MODE BAYANGAN") {
		t.Fatalf("catatan harus melabeli angka bayangan, dapat %q", summary.CKPNShadowNote)
	}

	if after := e.countCKPNJournals(t); after != beforeJournals {
		t.Fatalf("mode bayangan TIDAK boleh menambah jurnal CKPN: sebelum %d, sesudah %d", beforeJournals, after)
	}
	if after := e.loanDecimal(t, loan.ID, "required_ckpn"); !after.Equal(beforeRequired) || !after.IsZero() {
		t.Fatalf("mode bayangan tidak boleh mengubah required_ckpn: sebelum %s, sesudah %s", beforeRequired, after)
	}
	// Field resmi tidak diisi: tidak ada pengurangan modal inti yang diklaim.
	if summary.CKPNCompared != 0 || !summary.CKPNModalIntiDeduction.IsZero() {
		t.Fatalf("bidang resmi tidak boleh terisi oleh mode bayangan: compared=%d deduction=%s",
			summary.CKPNCompared, summary.CKPNModalIntiDeduction)
	}
}
