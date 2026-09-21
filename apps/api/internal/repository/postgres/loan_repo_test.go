package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// fakeResult memenuhi sql.Result untuk uji murni tanpa database.
type fakeResult struct{ rows int64 }

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.rows, nil }

// fakeExec merekam query dan argumen yang dikirim ke markLoanDisbursed.
type fakeExec struct {
	query  string
	args   []any
	result sql.Result
	err    error
}

func (f *fakeExec) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	return f.result, f.err
}

// Guard status markLoanDisbursed: penandaan hanya sah dari APPROVED, sehingga kredit
// yang sudah cair tidak bisa dicairkan dua kali lewat query ini.
func TestMarkLoanDisbursed_GuardStatus(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()

	t.Run("dari APPROVED berhasil", func(t *testing.T) {
		exec := &fakeExec{result: fakeResult{rows: 1}}
		if err := markLoanDisbursed(ctx, exec, id, decimal.NewFromInt(5_000_000)); err != nil {
			t.Fatalf("markLoanDisbursed: %v", err)
		}
		if !strings.Contains(exec.query, "status=$4") {
			t.Fatalf("query harus memuat guard status: %q", exec.query)
		}
		if len(exec.args) != 4 {
			t.Fatalf("args %v, ingin 4 argumen", exec.args)
		}
		if exec.args[0] != domain.LoanStatusDisbursed || exec.args[3] != domain.LoanStatusApproved {
			t.Fatalf("args %v, ingin status tujuan DISBURSED dan syarat APPROVED", exec.args)
		}
		if exec.args[2] != id {
			t.Fatalf("argumen id %v, ingin %v", exec.args[2], id)
		}
	})

	t.Run("tanpa baris APPROVED ditolak", func(t *testing.T) {
		exec := &fakeExec{result: fakeResult{rows: 0}}
		err := markLoanDisbursed(ctx, exec, id, decimal.NewFromInt(5_000_000))
		if !errors.Is(err, domain.ErrLoanNotApproved) {
			t.Fatalf("mau ErrLoanNotApproved, dapat %v", err)
		}
	})
}

// P3: penolakan kredit dibatasi status PENDING_APPROVAL di dalam query, sehingga
// penolakan yang membaca status basi tidak dapat menimpa kredit yang sudah
// APPROVED/DISBURSED. Tanpa baris yang berubah, galat domain dikembalikan.
func TestRejectLoan_GuardStatus(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()

	t.Run("dari PENDING_APPROVAL berhasil", func(t *testing.T) {
		exec := &fakeExec{result: fakeResult{rows: 1}}
		if err := rejectLoan(ctx, exec, id, "agunan tidak layak"); err != nil {
			t.Fatalf("rejectLoan: %v", err)
		}
		if !strings.Contains(exec.query, "status=$4") {
			t.Fatalf("query harus memuat guard status: %q", exec.query)
		}
		if len(exec.args) != 4 {
			t.Fatalf("args %v, ingin 4 argumen", exec.args)
		}
		if exec.args[0] != domain.LoanStatusRejected || exec.args[3] != domain.LoanStatusPendingApproval {
			t.Fatalf("args %v, ingin status REJECTED dan syarat PENDING_APPROVAL", exec.args)
		}
		if exec.args[2] != id {
			t.Fatalf("argumen id %v, ingin %v", exec.args[2], id)
		}
	})

	t.Run("status basi tidak menimpa", func(t *testing.T) {
		exec := &fakeExec{result: fakeResult{rows: 0}}
		err := rejectLoan(ctx, exec, id, "agunan tidak layak")
		if !errors.Is(err, domain.ErrLoanAlreadyApproved) {
			t.Fatalf("mau ErrLoanAlreadyApproved, dapat %v", err)
		}
	})
}

// P4: penolakan TIDAK boleh mengisi kolom persetujuan; kredit yang ditolak bukan
// kredit yang disetujui. Jejak penolakan ada di rejection_reason, maker-checker, audit.
func TestRejectLoan_TidakMengisiKolomPersetujuan(t *testing.T) {
	exec := &fakeExec{result: fakeResult{rows: 1}}
	if err := rejectLoan(context.Background(), exec, uuid.New(), "ditolak"); err != nil {
		t.Fatalf("rejectLoan: %v", err)
	}
	if strings.Contains(exec.query, "approved_by") || strings.Contains(exec.query, "approved_at") {
		t.Fatalf("penolakan tidak boleh menyentuh kolom persetujuan: %q", exec.query)
	}
	if !strings.Contains(exec.query, "rejection_reason") {
		t.Fatalf("penolakan harus menyimpan alasan: %q", exec.query)
	}
}
