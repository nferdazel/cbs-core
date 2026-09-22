package service_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// UJI KESETARAAN (terpenting untuk W6): menjalankan EOD dari definisi tabel harus
// menghasilkan jurnal dan angka yang SAMA dengan urutan lama.
//
// Metodenya: dua kredit identik dibuat. Kredit A diproses lewat "oracle" urutan lama
// (memanggil layanan yang sama persis dengan urutan hardcode: akrual -> amortisasi ->
// PPAP -> denda), kredit B diproses lewat mesin EOD yang membaca definisi dari
// database. Jurnal per kredit dibandingkan baris demi baris (akun, arah, nominal) dan
// angka kredit kuncinya dibandingkan. Kredit A ikut diproses ulang oleh mesin baru,
// tetapi perbandingan dibatasi pada kunci idempotensi milik kredit B sehingga tidak
// tercampur.
func TestIntegrasiEODDefinisiSetaraUrutanLama(t *testing.T) {
	e := newMoneyEnv(t)
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "true")
	setRestructureLossConfig(t, e, "ppap.collateral.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
	t.Cleanup(func() {
		setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
		setCKPNConfig(t, e, "ckpn.enabled", "false")
		setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
	})

	branchCode := ckpnTestBranchCode("Q")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Kesetaraan EOD")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujisetara", Role: domain.RoleAdmin, BranchCode: branchCode}

	businessDate := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC)

	makeRestructuredLoan := func(label string) *domain.Loan {
		cust := e.newCustomer(t, "Nasabah Setara "+label, fmt.Sprintf("setara-%s-%d@uji.local", label, time.Now().UnixNano()))
		acc := e.newAccountInBranch(t, cust.ID, branchID)
		loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 12)
		if _, err := e.db.ExecContext(e.ctx,
			`UPDATE loans SET dpd=100, collectibility='3_KURANG_LANCAR' WHERE id=$1`, loan.ID); err != nil {
			t.Fatalf("menyiapkan state kredit %s: %v", label, err)
		}
		if _, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
			LoanID:                loan.ID,
			NewTermMonths:         12,
			NewInterestRateAnnual: decimal.NewFromInt(6),
			Reason:                "uji kesetaraan EOD",
		}, actor); err != nil {
			t.Fatalf("RestructureLoan %s: %v", label, err)
		}
		e.setOldestDueDate(t, loan.ID, time.Now().UTC().AddDate(0, 0, -1))
		return loan
	}

	// ── Fase 1: kredit A diproses urutan lama (oracle) ──
	loanA := makeRestructuredLoan("A")
	sysActor := domain.SystemActor(e.actor.UserID)
	if _, err := e.loanSvc.AccrueInterest(e.ctx, businessDate, sysActor); err != nil {
		t.Fatalf("oracle akrual: %v", err)
	}
	if _, err := e.loanSvc.AmortizeRestructureLoss(e.ctx, businessDate, sysActor); err != nil {
		t.Fatalf("oracle amortisasi: %v", err)
	}
	if _, err := e.ppapSvc.RunDaily(e.ctx, businessDate, sysActor); err != nil {
		t.Fatalf("oracle PPAP: %v", err)
	}
	if _, err := e.loanSvc.AccruePenalties(e.ctx, businessDate, sysActor); err != nil {
		t.Fatalf("oracle denda: %v", err)
	}
	linesA := e.loanEODJournalLines(t, loanA.LoanNumber)
	fieldsA := e.loanEODFields(t, loanA.ID)
	if len(linesA) == 0 {
		t.Fatal("oracle urutan lama tidak menghasilkan jurnal; uji tidak bermakna")
	}

	// ── Fase 2: kredit B identik diproses mesin EOD berbasis definisi database ──
	loanB := makeRestructuredLoan("B")
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))
	summary, err := batchSvc.RunEOD(e.ctx, e.actor.UserID)
	if err != nil {
		t.Fatalf("RunEOD (mesin definisi): %v", err)
	}
	// Pastikan mesin benar-benar memakai definisi DB, bukan jalan tanpa langkah.
	if step := eodStepStatus(t, summary, "loan_interest_accrual"); step.Status != domain.EODStepRan {
		t.Fatalf("akrual lewat mesin definisi = %s (%s), mau RAN", step.Status, step.Reason)
	}
	linesB := e.loanEODJournalLines(t, loanB.LoanNumber)
	fieldsB := e.loanEODFields(t, loanB.ID)

	t.Logf("jurnal urutan lama (%d baris): %+v", len(linesA), linesA)
	t.Logf("jurnal mesin tabel (%d baris): %+v", len(linesB), linesB)
	t.Logf("angka urutan lama: %+v", fieldsA)
	t.Logf("angka mesin tabel: %+v", fieldsB)

	if !reflect.DeepEqual(linesA, linesB) {
		t.Fatalf("jurnal kredit TIDAK setara dengan urutan lama.\nurutan lama : %+v\nmesin tabel : %+v", linesA, linesB)
	}
	if !reflect.DeepEqual(fieldsA, fieldsB) {
		t.Fatalf("angka kredit TIDAK setara dengan urutan lama.\nurutan lama : %+v\nmesin tabel : %+v", fieldsA, fieldsB)
	}
}

// eodJournalLine adalah satu baris jurnal yang dinormalkan agar dua kredit berbeda
// dapat dibandingkan: nomor kredit di kunci idempotensi diganti penanda "LOAN".
type eodJournalLine struct {
	Key       string
	Account   string
	Direction string
	Amount    string
}

// loanEODJournalLines mengambil jurnal yang berasal dari langkah-langkah EOD untuk satu
// kredit, berdasarkan awalan kunci idempotensi yang memuat nomor kredit, lalu
// menormalkan nomor itu.
func (e *moneyEnv) loanEODJournalLines(t *testing.T, loanNumber string) []eodJournalLine {
	t.Helper()
	rows, err := e.db.QueryContext(e.ctx, `
		SELECT je.idempotency_key, a.account_number, jl.direction, jl.amount
		FROM journal_entries je
		JOIN journal_lines jl ON jl.journal_entry_id = je.id
		JOIN accounts a ON a.id = jl.account_id
		WHERE je.idempotency_key LIKE 'ACCR-' || $1 || '-%'
		   OR je.idempotency_key LIKE 'LOSSAMORT-' || $1 || '-%'
		   OR je.idempotency_key LIKE 'PPAP-' || $1 || '-%'
		   OR je.idempotency_key LIKE 'PENALTY-' || $1 || '-%'`, loanNumber)
	if err != nil {
		t.Fatalf("membaca jurnal EOD kredit %s: %v", loanNumber, err)
	}
	defer func() { _ = rows.Close() }()

	lines := []eodJournalLine{}
	for rows.Next() {
		var line eodJournalLine
		if err := rows.Scan(&line.Key, &line.Account, &line.Direction, &line.Amount); err != nil {
			t.Fatalf("scan jurnal: %v", err)
		}
		line.Key = normalizeLoanKey(line.Key, loanNumber)
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterasi jurnal: %v", err)
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Key != lines[j].Key {
			return lines[i].Key < lines[j].Key
		}
		if lines[i].Account != lines[j].Account {
			return lines[i].Account < lines[j].Account
		}
		return lines[i].Direction < lines[j].Direction
	})
	return lines
}

// normalizeLoanKey mengganti nomor kredit di kunci idempotensi dengan "LOAN" agar
// kredit A dan B dapat dibandingkan apa adanya.
func normalizeLoanKey(key, loanNumber string) string {
	if loanNumber == "" {
		return key
	}
	return strings.ReplaceAll(key, loanNumber, "LOAN")
}

// loanEODFields merangkum angka kredit yang dipengaruhi langkah EOD.
func (e *moneyEnv) loanEODFields(t *testing.T, loanID uuid.UUID) map[string]string {
	t.Helper()
	return map[string]string{
		"outstanding_principal":    e.loanDecimal(t, loanID, "outstanding_principal").String(),
		"required_ppap":            e.loanDecimal(t, loanID, "required_ppap").String(),
		"restructure_loss_balance": e.loanDecimal(t, loanID, "restructure_loss_balance").String(),
		"penalty_accrued":          e.loanDecimal(t, loanID, "penalty_accrued").String(),
	}
}
