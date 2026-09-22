package service_test

import (
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// Dua permintaan recovery identik (tanpa kunci pemanggil) harus menghasilkan SATU
// jurnal. Kode lama memakai time.Now().Unix() sehingga tiap permintaan menghasilkan
// kunci berbeda dan posting engine menulis jurnal kedua; bila perilaku itu kembali,
// COUNT(*) menjadi 2 dan test ini gagal.
func TestIntegrasiRecoveryIdempotenSatuJurnal(t *testing.T) {
	e := newMoneyEnv(t)

	branchID := e.ensureBranch(t, "RCV", "Cabang Uji Recovery")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujirecov", Role: domain.RoleAdmin, BranchCode: "RCV"}
	cust := e.newCustomer(t, "Nasabah Recovery", fmt.Sprintf("recov-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, decimal.NewFromInt(1_000_000), 6)

	// recoveryTarget menuntut status WRITTEN_OFF; tandai langsung agar fokus uji pada
	// idempotensi jurnal recovery, bukan alur persetujuan hapus buku. Nilai hapus buku
	// ikut diisi karena pemulihan kini dibatasi padanya (migrasi 000085).
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status = 'WRITTEN_OFF', outstanding_principal = 0, penalty_accrued = 0,
		                 written_off_amount = 1000000
		WHERE id = $1`, loan.ID); err != nil {
		t.Fatalf("menandai kredit hapus buku: %v", err)
	}

	payload := map[string]any{"loan_id": loan.ID.String(), "recovery_amount": "100000"}
	for i := 0; i < 2; i++ {
		// Jeda > 1 detik: kunci lama time.Now().Unix() bernilai beda di detik berbeda,
		// sehingga tanpa perbaikan dua permintaan menulis dua jurnal. Tanpa jeda,
		// retry dalam detik yang sama kebetulan memakai kunci sama dan bug tersamar.
		if i > 0 {
			time.Sleep(1100 * time.Millisecond)
		}
		tx, err := e.db.BeginTx(e.ctx, nil)
		if err != nil {
			t.Fatalf("membuka transaksi ke-%d: %v", i+1, err)
		}
		if err := e.loanSvc.ExecuteApproved(e.ctx, tx, service.ActionLoanRecovery, payload, actor); err != nil {
			_ = tx.Rollback()
			t.Fatalf("ExecuteApproved recovery ke-%d: %v", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit recovery ke-%d: %v", i+1, err)
		}
	}

	var n int
	var key string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*), COALESCE(MAX(idempotency_key), '') FROM journal_entries
		 WHERE idempotency_key LIKE 'RECOV-' || $1 || '-%'`, loan.LoanNumber).
		Scan(&n, &key); err != nil {
		t.Fatalf("menghitung jurnal recovery: %v", err)
	}
	if n != 1 {
		t.Fatalf("dua permintaan recovery identik menghasilkan %d jurnal, ingin 1", n)
	}
	want := fmt.Sprintf("RECOV-%s-100000-%s", loan.LoanNumber, time.Now().UTC().Format("20060102"))
	if key != want {
		t.Fatalf("kunci idempotensi recovery %q, ingin kunci deterministik %q", key, want)
	}
}
