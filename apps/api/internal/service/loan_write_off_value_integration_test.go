package service_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi (PostgreSQL sungguhan) untuk penyimpanan NILAI HAPUS BUKU dan batas
// pemulihannya (keputusan pemilik sistem: lubang nyata yang harus ditutup).
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiNilaiHapusBuku -v
//
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola berkas integrasi lain.

const wbeWriteOffAmountMigration = "000085_write_off_amount_and_recovery_threshold.up.sql"

// woffLoanOf menyiapkan kredit konvensional sudah dicairkan sebesar amount di cabang uji.
func woffLoanOf(t *testing.T, e *bookWriteEnv, prefix string, amount int64) (*domain.Loan, domain.Actor) {
	t.Helper()
	branchCode := writeOffIntegrationBranch(prefix)
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Nilai Hapus Buku")
	actor := domain.Actor{
		UserID: e.actor.UserID, Username: "admin.ujinilaiwoff",
		Role: domain.RoleAdmin, BranchCode: branchCode, Book: domain.BookConventional,
	}
	cust := e.newCustomer(t, "Nasabah Nilai Hapus Buku", fmt.Sprintf("nilaiwoff-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	return e.disburseAs(t, actor, cust.ID, acc, idr(amount), 6), actor
}

// woffWriteOff menghapus buku kredit uji secara sah (macet + cadangan 100%), memakai
// ambang hapus buku yang dinaikkan agar langsung berjalan.
func woffWriteOff(t *testing.T, e *bookWriteEnv, loan *domain.Loan, actor domain.Actor, requiredPPAP int64) {
	t.Helper()
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), requiredPPAP)
	woffBypassApproval(t, e, "loan_write_off")
	if _, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x + kunjungan",
	}, actor); err != nil {
		t.Fatalf("hapus buku: %v", err)
	}
}

// writtenOffAmount membaca nilai hapus buku yang tersimpan pada baris kredit.
func writtenOffAmount(t *testing.T, e *bookWriteEnv, loanID uuid.UUID) decimal.Decimal {
	t.Helper()
	var amount decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `SELECT written_off_amount FROM loans WHERE id = $1`, loanID).Scan(&amount); err != nil {
		t.Fatalf("membaca written_off_amount: %v", err)
	}
	return amount
}

// Eksekusi hapus buku menyimpan nilai yang dilepas (pokok + bunga + denda), sehingga
// pemulihan setelahnya punya batas yang tidak bergantung pada jejak audit/jurnal.
func TestIntegrasiNilaiHapusBukuTersimpanSaatEksekusi(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := woffLoanOf(t, e, "WZ", 1_000_000)
	woffWriteOff(t, e, loan, actor, 1_000_000)

	if got := writtenOffAmount(t, e, loan.ID); !got.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("written_off_amount = %s, ingin 1000000", got)
	}
}

// Pemulihan sebagian boleh; akumulasi yang melebihi nilai hapus buku ditolak dengan
// galat spesifik dan tidak menulis jurnal baru.
func TestIntegrasiPemulihanMelebihiNilaiHapusBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := woffLoanOf(t, e, "WY", 1_000_000)
	woffWriteOff(t, e, loan, actor, 1_000_000)
	woffBypassApproval(t, e, "loan_recovery")

	if _, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: decimal.NewFromInt(400_000), IdempotencyKey: "sebagian-1",
	}, actor); err != nil {
		t.Fatalf("pemulihan sebagian harus boleh: %v", err)
	}

	_, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: decimal.NewFromInt(700_000), IdempotencyKey: "kelebihan-1",
	}, actor)
	if !errors.Is(err, domain.ErrRecoveryExceedsWriteOff) {
		t.Fatalf("pemulihan melebihi nilai hapus buku harus ditolak, dapat: %v", err)
	}
	if n := woffJournalCount(t, e, "RECOV-", loan.LoanNumber); n != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1 (permintaan yang ditolak tidak boleh terjurnal)", n)
	}
}

// Akumulasi pemulihan yang TEPAT SAMA dengan nilai hapus buku masih boleh.
func TestIntegrasiPemulihanTepatBatasDiizinkan(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := woffLoanOf(t, e, "W1", 1_000_000)
	woffWriteOff(t, e, loan, actor, 1_000_000)
	woffBypassApproval(t, e, "loan_recovery")

	for i, amount := range []int64{600_000, 400_000} {
		if _, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
			LoanID: loan.ID, RecoveryAmount: decimal.NewFromInt(amount),
			IdempotencyKey: fmt.Sprintf("tepat-%d", i),
		}, actor); err != nil {
			t.Fatalf("pemulihan ke-%d (%d) harus boleh: %v", i+1, amount, err)
		}
	}
	if n := woffJournalCount(t, e, "RECOV-", loan.LoanNumber); n != 2 {
		t.Fatalf("jurnal recovery %d, ingin 2", n)
	}
}

// Backfill nilai hapus buku untuk data lama: dari audit WRITE_OFF_LOAN lebih dulu, dari
// jurnal WOFF- sebagai cadangan, dan yang tidak punya keduanya dibiarkan 0 (tidak pasti).
// Migrasi dijalankan dua kali untuk membuktikan idempotensi.
func TestIntegrasiBackfillNilaiHapusBukuDataLama(t *testing.T) {
	e := newBookWriteEnv(t)
	// Migrasi 000085 yang dijalankan uji ini juga menyetel ambang recovery bawaan
	// (0 -> 10.000.000). Nilai lama dipulihkan saat uji selesai agar kunci itu tidak
	// tercemar ke uji berikutnya, sama seperti helper konfigurasi lain.
	woffSnapshotConfig(t, e, "maker_checker.loan_recovery.threshold")

	// (a) Punya audit WRITE_OFF_LOAN dengan changes.amount.
	loanAudit, actorA := woffLoanOf(t, e, "W2", 1_000_000)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status = 'WRITTEN_OFF', outstanding_principal = 0, penalty_accrued = 0 WHERE id = $1`, loanAudit.ID); err != nil {
		t.Fatalf("menandai kredit audit: %v", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, resource_type, resource_id, changes, created_at)
		VALUES ($1, $2, 'WRITE_OFF_LOAN', 'loan', $3, $4::jsonb, NOW())`,
		actorA.Username, string(actorA.Role), loanAudit.ID.String(),
		`{"amount":"750000.00","principal":"750000.00"}`); err != nil {
		t.Fatalf("menyisipkan audit hapus buku: %v", err)
	}

	// (b) Tidak punya audit, tetapi punya jurnal WOFF- (cadangan). Satu kaki DEBIT.
	loanJournal, _ := woffLoanOf(t, e, "W3", 1_000_000)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status = 'WRITTEN_OFF', outstanding_principal = 0, penalty_accrued = 0 WHERE id = $1`, loanJournal.ID); err != nil {
		t.Fatalf("menandai kredit jurnal: %v", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO journal_entries (reference_number, idempotency_key, transaction_type, description, created_by)
		VALUES ($1, $2, 'ADJUSTMENT', 'uji backfill WOFF', 'SYSTEM')`,
		"REF-BACKFILL-"+loanJournal.LoanNumber, "WOFF-"+loanJournal.LoanNumber); err != nil {
		t.Fatalf("menyisipkan jurnal WOFF: %v", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO journal_lines (journal_entry_id, account_id, direction, amount, balance_after, sequence)
		SELECT je.id, (SELECT id FROM accounts ORDER BY created_at LIMIT 1), 'DEBIT', 300000, 0, 1
		FROM journal_entries je WHERE je.idempotency_key = $1`,
		"WOFF-"+loanJournal.LoanNumber); err != nil {
		t.Fatalf("menyisipkan baris jurnal WOFF: %v", err)
	}

	// (c) Tidak punya audit maupun jurnal: tetap 0 dan DILAPORKAN tidak pasti.
	loanUnknown, _ := woffLoanOf(t, e, "W4", 1_000_000)
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status = 'WRITTEN_OFF', outstanding_principal = 0, penalty_accrued = 0 WHERE id = $1`, loanUnknown.ID); err != nil {
		t.Fatalf("menandai kredit tidak pasti: %v", err)
	}

	jalankan := func() {
		path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", wbeWriteOffAmountMigration)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("membaca migrasi %s: %v", wbeWriteOffAmountMigration, err)
		}
		if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
			t.Fatalf("menjalankan migrasi %s: %v", wbeWriteOffAmountMigration, err)
		}
	}
	periksa := func(tahap string) {
		if got := writtenOffAmount(t, e, loanAudit.ID); !got.Equal(decimal.NewFromInt(750_000)) {
			t.Fatalf("%s: nilai hapus buku dari audit = %s, ingin 750000", tahap, got)
		}
		if got := writtenOffAmount(t, e, loanJournal.ID); !got.Equal(decimal.NewFromInt(300_000)) {
			t.Fatalf("%s: nilai hapus buku dari jurnal = %s, ingin 300000", tahap, got)
		}
		if got := writtenOffAmount(t, e, loanUnknown.ID); !got.IsZero() {
			t.Fatalf("%s: kredit tanpa audit/jurnal = %s, ingin tetap 0 (tidak pasti)", tahap, got)
		}
	}

	jalankan()
	periksa("setelah jalan pertama")
	jalankan()
	periksa("setelah jalan kedua (idempoten)")
}
