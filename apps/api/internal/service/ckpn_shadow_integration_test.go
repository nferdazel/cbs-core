package service_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi mode bayangan CKPN terhadap PostgreSQL sungguhan. Di-skip kecuali
// CBS_TEST_DB_DSN diisi, mengikuti pola ckpn_integration_test.go.

// setCKPNState menyetel kolektibilitas, tunggakan, dan PPKA satu kredit uji secara
// langsung (mensimulasikan hasil jalur PPAP) agar perhitungan CKPN dapat diuji tanpa
// menjalankan PPAP lebih dulu.
func (e *moneyEnv) setCKPNState(t *testing.T, loanID uuid.UUID, collectibility string, dpd int, requiredPPAP decimal.Decimal) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility = $1, dpd = $2, required_ppap = $3 WHERE id = $4`,
		collectibility, dpd, requiredPPAP, loanID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}
}

// TestIntegrasiCKPNModeBayanganPortofolioCampuran membuktikan mode bayangan MENGHITUNG
// CKPN per kredit (EAD x PD x LGD), bukan membaca required_ckpn yang belum pernah diisi.
// Portofolio campuran: kredit A PPKA > CKPN (menjadi pengurang modal inti), kredit B
// CKPN > PPKA, dan kredit C aset baik (CKPN nol). Angka uji dihitung manual dan harus
// sama persis, termasuk potensi pengurang modal inti per kredit.
func TestIntegrasiCKPNModeBayanganPortofolioCampuran(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd.1", "0.005")
	setCKPNConfig(t, e, "ckpn.pd.2", "0.05")
	setCKPNConfig(t, e, "ckpn.pd.3", "0.10")
	setCKPNConfig(t, e, "ckpn.pd.4", "0.30")
	setCKPNConfig(t, e, "ckpn.pd.5", "0.60")
	setCKPNConfig(t, e, "ckpn.lgd", "0.45")

	branchCode := ckpnTestBranchCode("M")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN Campuran")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujicampuran", Role: domain.RoleAdmin, BranchCode: branchCode}

	newLoan := func(name string, amount decimal.Decimal) *domain.Loan {
		t.Helper()
		cust := e.newCustomer(t, name, fmt.Sprintf("campuran-%d-%s@uji.local", time.Now().UnixNano(), name))
		acc := e.newAccountInBranch(t, cust.ID, branchID)
		return e.disburseAs(t, actor, cust.ID, acc, amount, 12)
	}

	loanA := newLoan("Nasabah Campuran A", idr(100_000_000))
	e.setCKPNState(t, loanA.ID, "2_DPK", 30, idr(5_000_000))
	loanB := newLoan("Nasabah Campuran B", idr(100_000_000))
	e.setCKPNState(t, loanB.ID, "5_MACET", 180, idr(10_000_000))
	loanC := newLoan("Nasabah Campuran C", idr(50_000_000))
	e.setCKPNState(t, loanC.ID, "1_LANCAR", 0, idr(1_000_000))

	beforeJournals := e.countCKPNJournals(t)
	ckpnSvc := newCKPNSvcForTest(e)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := ckpnSvc.Compare(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Compare mode bayangan: %v", err)
	}
	if summary.Enabled || !summary.ShadowMode {
		t.Fatalf("mode bayangan salah tandai: enabled=%v shadow=%v", summary.Enabled, summary.ShadowMode)
	}
	if summary.Processed != 3 || summary.Failed != 0 {
		t.Fatalf("processed=%d failed=%d, mau 3/0 (%+v)", summary.Processed, summary.Failed, summary.Failures)
	}

	// A: 100.000.000 x 5% x 45% = 2.250.000; PPKA 5.000.000 -> PPKA lebih besar.
	itemA := ckpnItem(t, summary, loanA.ID)
	if !itemA.CKPN.Equal(decimal.NewFromInt(2_250_000)) || itemA.Larger != domain.CKPNLargerPPKA {
		t.Fatalf("kredit A: CKPN=%s larger=%s, mau 2250000/PPKA", itemA.CKPN, itemA.Larger)
	}
	// B: 100.000.000 x 60% x 45% = 27.000.000; PPKA 10.000.000 -> CKPN lebih besar.
	itemB := ckpnItem(t, summary, loanB.ID)
	if !itemB.CKPN.Equal(decimal.NewFromInt(27_000_000)) || itemB.Larger != domain.CKPNLargerCKPN {
		t.Fatalf("kredit B: CKPN=%s larger=%s, mau 27000000/CKPN", itemB.CKPN, itemB.Larger)
	}
	// C: aset baik -> nol tanpa memakai PD/LGD.
	itemC := ckpnItem(t, summary, loanC.ID)
	if !itemC.IsAsetBaik || !itemC.CKPN.IsZero() {
		t.Fatalf("kredit C harus aset baik tanpa CKPN: %+v", itemC)
	}

	if !summary.TotalCKPN.Equal(decimal.NewFromInt(29_250_000)) {
		t.Fatalf("total CKPN %s, mau 29250000", summary.TotalCKPN)
	}
	if !summary.TotalPPKA.Equal(decimal.NewFromInt(16_000_000)) {
		t.Fatalf("total PPKA %s, mau 16000000", summary.TotalPPKA)
	}
	if !summary.Difference.Equal(decimal.NewFromInt(-13_250_000)) || summary.Higher != domain.CKPNLargerCKPN {
		t.Fatalf("selisih agregat %s/%s, mau -13250000/CKPN", summary.Difference, summary.Higher)
	}
	// Potensi pengurang modal inti PER KREDIT: max(5.000.000-2.250.000,0) +
	// max(10.000.000-27.000.000,0) + max(1.000.000-0,0) = 3.750.000. BUKAN selisih agregat.
	if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(3_750_000)) {
		t.Fatalf("potensi pengurang modal inti %s, mau 3750000 (Σ per kredit)", summary.ModalIntiDeduction)
	}
	if summary.AsetBaikCount != 1 || !summary.AsetBaikOutstanding.Equal(decimal.NewFromInt(50_000_000)) {
		t.Fatalf("aset baik %d/%s, mau 1/50000000", summary.AsetBaikCount, summary.AsetBaikOutstanding)
	}
	t.Logf("sesudah (campuran): totalPPKA=%s totalCKPN=%s difference=%s deduction=%s asetBaik=%d sisaPokokAsetBaik=%s",
		summary.TotalPPKA, summary.TotalCKPN, summary.Difference, summary.ModalIntiDeduction,
		summary.AsetBaikCount, summary.AsetBaikOutstanding)
	if len(summary.ParameterGaps) != 0 {
		t.Fatalf("parameter lengkap, gap harus kosong, dapat %v", summary.ParameterGaps)
	}

	// Mode bayangan TIDAK menjurnal dan TIDAK menyimpan required_ckpn.
	if after := e.countCKPNJournals(t); after != beforeJournals {
		t.Fatalf("mode bayangan menambah jurnal CKPN: sebelum %d, sesudah %d", beforeJournals, after)
	}
	for _, loan := range []*domain.Loan{loanA, loanB, loanC} {
		if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.IsZero() {
			t.Fatalf("required_ckpn kredit %s = %s, mau tetap nol", loan.LoanNumber, got)
		}
	}
}

// TestIntegrasiCKPNModeBayanganParameterPersenDilaporkan mereproduksi keadaan produksi:
// ckpn.enabled mati, mode bayangan hidup, dan parameter diisi dalam PERSEN (1,5,15,35,60
// dan 45) sementara mesin membaca FRAKSI. Dibuktikan bahwa nilai di luar 0..1
// dilaporkan sebagai KEKURANGAN PARAMETER (bukan angka nol diam-diam), dan bahwa total
// CKPN nol dijelaskan sebagai pengecualian aset baik (bukan model belum dijalankan).
func TestIntegrasiCKPNModeBayanganParameterPersenDilaporkan(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd.1", "1")
	setCKPNConfig(t, e, "ckpn.pd.2", "5")
	setCKPNConfig(t, e, "ckpn.pd.3", "15")
	setCKPNConfig(t, e, "ckpn.pd.4", "35")
	setCKPNConfig(t, e, "ckpn.pd.5", "60")
	setCKPNConfig(t, e, "ckpn.lgd", "45")

	branchCode := ckpnTestBranchCode("P")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN Persen")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujipersen", Role: domain.RoleAdmin, BranchCode: branchCode}

	// 6 kredit lancar tanpa tunggakan: semuanya aset baik -> CKPN nol. PPKA disebar agar
	// totalnya persis 929.407 seperti laporan produksi.
	totalOutstanding := decimal.Zero
	for i := 0; i < 6; i++ {
		cust := e.newCustomer(t, fmt.Sprintf("Nasabah Persen %d", i), fmt.Sprintf("persen-%d-%d@uji.local", time.Now().UnixNano(), i))
		acc := e.newAccountInBranch(t, cust.ID, branchID)
		loan := e.disburseAs(t, actor, cust.ID, acc, idr(30_833_333), 12)
		ppka := idr(154_901)
		if i == 5 {
			ppka = idr(154_902)
		}
		e.setCKPNState(t, loan.ID, "1_LANCAR", 0, ppka)
		totalOutstanding = totalOutstanding.Add(idr(30_833_333))
	}

	beforeJournals := e.countCKPNJournals(t)
	ckpnSvc := newCKPNSvcForTest(e)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := ckpnSvc.Compare(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Compare mode bayangan: %v", err)
	}
	if summary.Processed != 6 || summary.Failed != 0 {
		t.Fatalf("processed=%d failed=%d, mau 6/0", summary.Processed, summary.Failed)
	}
	// Nol karena SELURUH kredit aset baik; bukan karena model belum dijalankan.
	if !summary.TotalCKPN.IsZero() || summary.AsetBaikCount != 6 {
		t.Fatalf("totalCKPN=%s asetBaik=%d, mau 0/6", summary.TotalCKPN, summary.AsetBaikCount)
	}
	if !summary.AsetBaikOutstanding.Equal(totalOutstanding) {
		t.Fatalf("sisa pokok aset baik %s, mau %s", summary.AsetBaikOutstanding, totalOutstanding)
	}
	if !summary.TotalPPKA.Equal(decimal.NewFromInt(929_407)) || !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(929_407)) {
		t.Fatalf("totalPPKA=%s deduction=%s, mau 929407/929407", summary.TotalPPKA, summary.ModalIntiDeduction)
	}
	t.Logf("sesudah (repro persen): totalPPKA=%s totalCKPN=%s difference=%s deduction=%s asetBaik=%d sisaPokokAsetBaik=%s gaps=%v",
		summary.TotalPPKA, summary.TotalCKPN, summary.Difference, summary.ModalIntiDeduction,
		summary.AsetBaikCount, summary.AsetBaikOutstanding, summary.ParameterGaps)
	// Parameter persen di luar 0..1 wajib muncul sebagai kekurangan parameter beserta
	// satuan yang benar, bukan menghasilkan nol tanpa penjelasan.
	joined := strings.Join(summary.ParameterGaps, " ")
	if !strings.Contains(joined, "ckpn.lgd") || !strings.Contains(joined, "ckpn.pd.2") || !strings.Contains(joined, "FRAKSI") {
		t.Fatalf("gap harus menyebut ckpn.lgd/ckpn.pd.2 dan satuan FRAKSI, dapat %v", summary.ParameterGaps)
	}

	if after := e.countCKPNJournals(t); after != beforeJournals {
		t.Fatalf("mode bayangan menambah jurnal CKPN: sebelum %d, sesudah %d", beforeJournals, after)
	}
	var stored int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT count(*) FROM loans WHERE branch_id = $1 AND required_ckpn <> 0`, branchID).Scan(&stored); err != nil {
		t.Fatalf("menghitung required_ckpn tersimpan: %v", err)
	}
	if stored != 0 {
		t.Fatalf("%d kredit menyimpan required_ckpn, mau 0 (mode bayangan tidak menulis state)", stored)
	}
}
