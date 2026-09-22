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

// Uji integrasi terhadap PostgreSQL sungguhan untuk basis, plafon, dan perlakuan
// syariah pada akrual denda kredit. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti
// pola moneyflow_integration_test.go yang menyediakan newMoneyEnv.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiDenda -v

// newSyariahAccount menyisipkan rekening tabungan syariah (COA 12100 Tabungan Wadiah)
// agar pencairan produk buku SYARIAH lolos pemeriksaan kecocokan buku.
func (e *moneyEnv) newSyariahAccount(t *testing.T, customerID uuid.UUID) uuid.UUID {
	t.Helper()
	accountNumber := fmt.Sprintf("E2E-SYR-%d", time.Now().UnixNano())
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `
		INSERT INTO accounts (account_number, coa_id, account_type, status, balance, available_balance, currency, customer_id, branch_id)
		VALUES ($1, (SELECT id FROM chart_of_accounts WHERE code = '12100'),
		        'SAVINGS', 'ACTIVE', 0, 0, 'IDR', $2, (SELECT id FROM branches WHERE code = '001'))
		RETURNING id`, accountNumber, customerID).Scan(&id); err != nil {
		t.Fatalf("menyiapkan rekening syariah uji: %v", err)
	}
	return id
}

// disburseWithProduct sama dengan disburse, tetapi produknya dipilih agar pembiayaan
// syariah (PMB-MURABAHAH) dapat dicairkan lewat jalur produksi penuh.
func (e *moneyEnv) disburseWithProduct(t *testing.T, productCode string, customerID, accountID uuid.UUID, amount decimal.Decimal, term int) *domain.Loan {
	t.Helper()
	product, err := e.productRepo.GetByCode(e.ctx, productCode)
	if err != nil {
		t.Fatalf("membaca produk kredit %s: %v", productCode, err)
	}
	loan, err := e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
		CustomerID:            customerID,
		ProductID:             product.ID,
		DisbursementAccountID: accountID,
		PrincipalAmount:       amount,
		TermMonths:            term,
		Purpose:               "uji integrasi denda",
	}, e.actor)
	if err != nil {
		t.Fatalf("pengajuan kredit %s: %v", productCode, err)
	}
	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, e.actor); err != nil {
		t.Fatalf("persetujuan kredit %s: %v", productCode, err)
	}
	disbursed, err := e.loanSvc.DisburseLoan(e.ctx, loan.ID, e.actor)
	if err != nil {
		t.Fatalf("pencairan kredit %s: %v", productCode, err)
	}
	return disbursed
}

func findPenaltyItem(t *testing.T, summary domain.LoanPenaltySummary, loanNumber string) domain.LoanPenaltyItem {
	t.Helper()
	for _, it := range summary.Items {
		if it.LoanNumber == loanNumber {
			return it
		}
	}
	t.Fatalf("kredit %s tidak ada di ringkasan denda", loanNumber)
	return domain.LoanPenaltyItem{}
}

// Basis denda adalah POKOK ANGSURAN YANG TERTUNGGAK (bukan seluruh sisa pokok), dan
// dikenakan per angsuran yang lewat jatuh tempo. Kredit lancar tanpa tunggakan tidak
// diakru. Angka diuji terhadap perhitungan manual: pokok tunggakan x tarif per-mille
// / 1000 x hari akrual.
func TestIntegrasiDendaBasisPokokTunggakan(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setPenaltyRatePerMille(t, 10) // 10‰ = 1% per hari

	cust := e.newCustomer(t, "Denda Basis Tunggakan", "")
	acc := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, acc, idr(10_000_000), 4)
	e.setOldestDueDate(t, loan.ID, asOf.AddDate(0, 0, -2))

	// Kredit lancar tanpa tunggakan: angsuran pertama belum jatuh tempo.
	custLancar := e.newCustomer(t, "Denda Kredit Lancar", "")
	accLancar := e.newAccount(t, custLancar.ID)
	loanLancar := e.disburse(t, custLancar.ID, accLancar, idr(10_000_000), 4)

	summary, err := e.loanSvc.AccruePenalties(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("akrual denda: %v", err)
	}
	item := findPenaltyItem(t, summary, loan.LoanNumber)
	if item.DPD != 2 {
		t.Fatalf("DPD %d, ingin 2", item.DPD)
	}

	// Manual: pokok angsuran tertunggak x 1% x 2 hari.
	pokokTunggakan := e.schedule(t, loan.ID, 1).PrincipalAmount
	want := domain.RoundToRupiah(pokokTunggakan.Mul(decimal.NewFromInt(10)).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromInt(2)))
	if !item.Penalty.Equal(want) {
		t.Fatalf("denda %s, perhitungan manual %s (pokok tunggakan %s x 10‰ x 2)", item.Penalty, want, pokokTunggakan)
	}
	if got := e.loanDecimal(t, loan.ID, "penalty_accrued"); !got.Equal(want) {
		t.Fatalf("penalty_accrued %s, ingin %s", got, want)
	}
	// Basis BUKAN seluruh sisa pokok: 10.000.000 x 1% x 2 = 200.000 akan jauh berbeda.
	basisPenuh := domain.RoundToRupiah(idr(10_000_000).Mul(decimal.NewFromInt(10)).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromInt(2)))
	if item.Penalty.Equal(basisPenuh) {
		t.Fatalf("denda memakai basis seluruh pokok (%s); seharusnya pokok tunggakan", basisPenuh)
	}
	t.Logf("basis: pokok tunggakan %s x 10‰ x 2 hari = %s (bukan %s bila basis seluruh pokok)", pokokTunggakan, want, basisPenuh)

	// Jurnal: debit piutang denda 10305, kredit pendapatan denda 40500.
	key := "PENALTY-" + loan.LoanNumber + "-" + asOf.Format("2006-01-02")
	id, _ := e.journalByKey(t, key)
	lines := e.journalLines(t, id)
	if len(lines) != 2 {
		t.Fatalf("jurnal %d baris, ingin 2: %+v", len(lines), lines)
	}
	if lines[0].AccountNumber != "10305" || lines[0].Direction != "DEBIT" {
		t.Fatalf("kaki debit harus 10305 piutang denda: %+v", lines[0])
	}
	if lines[1].AccountNumber != "40500" || lines[1].Direction != "CREDIT" {
		t.Fatalf("kaki kredit harus 40500 pendapatan denda: %+v", lines[1])
	}

	// Kredit lancar: tidak mengakru dan tidak berjurnal.
	if n := e.countJournalsByKey(t, "PENALTY-"+loanLancar.LoanNumber+"-"+asOf.Format("2006-01-02")); n != 0 {
		t.Fatalf("kredit lancar berjurnal denda %d, ingin 0", n)
	}
	if got := e.loanDecimal(t, loanLancar.ID, "penalty_accrued"); !got.IsZero() {
		t.Fatalf("kredit lancar menambah penalty_accrued %s, ingin 0", got)
	}
}

// Plafon 10% dari pokok tunggakan memotong denda yang melewatinya dan menghentikan
// akrual berikutnya; jumlah kredit terdampak muncul di ringkasan.
func TestIntegrasiDendaPlafonMenghentikanAkrual(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setPenaltyRatePerMille(t, 10) // 1% per hari

	cust := e.newCustomer(t, "Denda Plafon", "")
	acc := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, acc, idr(10_000_000), 4)
	// DPD 15: denda harian 2.500.000 x 1% = 25.000; 15 hari = 375.000, melewati
	// plafon 10% x 2.500.000 = 250.000.
	e.setOldestDueDate(t, loan.ID, asOf.AddDate(0, 0, -15))

	summary, err := e.loanSvc.AccruePenalties(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("akrual denda: %v", err)
	}
	item := findPenaltyItem(t, summary, loan.LoanNumber)
	pokokTunggakan := e.schedule(t, loan.ID, 1).PrincipalAmount
	plafon := domain.RoundToRupiah(pokokTunggakan.Mul(decimal.NewFromInt(10)).Div(decimal.NewFromInt(100)))
	tanpaPlafon := domain.RoundToRupiah(pokokTunggakan.Mul(decimal.NewFromInt(10)).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromInt(15)))
	if !item.Penalty.Equal(plafon) {
		t.Fatalf("denda %s, ingin terpotong plafon %s (tanpa plafon %s)", item.Penalty, plafon, tanpaPlafon)
	}
	if !item.Capped || !item.CapAmount.Equal(plafon) {
		t.Fatalf("item harus Capped dengan plafon %s: capped=%v cap=%s", plafon, item.Capped, item.CapAmount)
	}
	if summary.Capped < 1 {
		t.Fatalf("ringkasan tidak menghitung kredit terdampak plafon: capped=%d", summary.Capped)
	}
	if got := e.loanDecimal(t, loan.ID, "penalty_accrued"); !got.Equal(plafon) {
		t.Fatalf("penalty_accrued %s, ingin plafon %s", got, plafon)
	}
	t.Logf("basis: pokok tunggakan %s; denda 15 hari %s dipotong plafon 10%% = %s", pokokTunggakan, tanpaPlafon, plafon)

	// Hari berikutnya: ruang plafon habis -> akrual dihentikan dan terlihat.
	besok := asOf.AddDate(0, 0, 1)
	summary2, err := e.loanSvc.AccruePenalties(e.ctx, besok, e.actor)
	if err != nil {
		t.Fatalf("akrual denda hari kedua: %v", err)
	}
	item2 := findPenaltyItem(t, summary2, loan.LoanNumber)
	if item2.Status != domain.BatchItemSkipped || !item2.Capped {
		t.Fatalf("hari kedua harus SKIPPED karena plafon: status=%s capped=%v", item2.Status, item2.Capped)
	}
	if summary2.Capped < 1 {
		t.Fatalf("ringkasan hari kedua tidak menghitung plafon: capped=%d", summary2.Capped)
	}
	if !strings.Contains(summary2.Warning, "plafon denda") {
		t.Fatalf("ringkasan harus menyebut plafon, dapat %q", summary2.Warning)
	}
	if got := e.loanDecimal(t, loan.ID, "penalty_accrued"); !got.Equal(plafon) {
		t.Fatalf("penalty_accrued bertambah setelah plafon: %s, ingin tetap %s", got, plafon)
	}
}

// Denda pembiayaan syariah tidak diakui sebagai pendapatan: jurnalnya debit piutang
// denda syariah 11700 dan kredit Dana Kebajikan 12500, bukan akun pendapatan.
// Sikap sementara menunggu keputusan DPS.
func TestIntegrasiDendaSyariahDanaKebajikan(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setPenaltyRatePerMille(t, 10)

	cust := e.newCustomer(t, "Denda Syariah", "")
	acc := e.newSyariahAccount(t, cust.ID)
	loan := e.disburseWithProduct(t, "PMB-MURABAHAH", cust.ID, acc, idr(3_000_000), 4)
	e.setOldestDueDate(t, loan.ID, asOf.AddDate(0, 0, -2))

	summary, err := e.loanSvc.AccruePenalties(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("akrual denda syariah: %v", err)
	}
	item := findPenaltyItem(t, summary, loan.LoanNumber)
	if !item.SyariahSocialFund {
		t.Fatalf("denda syariah harus ditandai dana kebajikan: %+v", item)
	}
	if got := e.loanDecimal(t, loan.ID, "penalty_accrued"); !got.IsPositive() {
		t.Fatalf("denda syariah tidak diakru, penalty_accrued=%s", got)
	}

	key := "PENALTY-" + loan.LoanNumber + "-" + asOf.Format("2006-01-02")
	id, _ := e.journalByKey(t, key)
	lines := e.journalLines(t, id)
	if len(lines) != 2 {
		t.Fatalf("jurnal %d baris, ingin 2: %+v", len(lines), lines)
	}
	if lines[0].AccountNumber != "11700" || lines[0].Direction != "DEBIT" {
		t.Fatalf("kaki debit harus 11700 piutang denda syariah: %+v", lines[0])
	}
	if lines[1].AccountNumber != "12500" || lines[1].Direction != "CREDIT" {
		t.Fatalf("kaki kredit harus 12500 Dana Kebajikan: %+v", lines[1])
	}
	for _, l := range lines {
		if l.AccountNumber == "14600" || l.AccountNumber == "40500" {
			t.Fatalf("denda syariah masuk akun pendapatan %s: %+v", l.AccountNumber, l)
		}
	}
	t.Logf("jurnal denda syariah: %+v (tidak ada pendapatan)", lines)
}
