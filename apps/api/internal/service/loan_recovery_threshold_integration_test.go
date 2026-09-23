package service_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// Uji integrasi ambang recovery (keputusan pemilik sistem): pemulihan kredit hapus
// buku di bawah ambang boleh dicatat langsung oleh pelaksana (TELLER/AO); pada atau
// di atas ambang wajib lewat maker-checker. Ambangnya kunci konfigurasi
// maker_checker.loan_recovery.threshold (nilai awal 10.000.000), bukan angka di kode.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiAmbangRecovery -v

func TestIntegrasiAmbangRecoveryDiBawahAmbangLangsung(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := woffLoanOf(t, e, "W5", 12_000_000)
	woffWriteOff(t, e, loan, actor, 12_000_000)
	woffSetThreshold(t, e, "loan_recovery", "10000000")

	if _, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: decimal.NewFromInt(5_000_000), IdempotencyKey: "bawah-ambang",
	}, actor); err != nil {
		t.Fatalf("pemulihan di bawah ambang harus berjalan langsung: %v", err)
	}
	if n := woffJournalCount(t, e, "RECOV-", loan.LoanNumber); n != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1", n)
	}
}

func TestIntegrasiAmbangRecoveryDiAtasAmbangMasukMakerChecker(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := woffLoanOf(t, e, "W6", 12_000_000)
	woffWriteOff(t, e, loan, actor, 12_000_000)
	woffSetThreshold(t, e, "loan_recovery", "10000000")

	// Nominal di ATAS ambang (10jt) tetapi TIDAK melebihi nilai hapus buku (12jt),
	// sehingga yang diuji murni perilaku ambang, bukan batas pemulihan.
	_, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: decimal.NewFromInt(11_000_000), IdempotencyKey: "atas-ambang",
	}, actor)
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("pemulihan di atas ambang harus menunggu maker-checker, dapat: %v", err)
	}
	if pending.ActionType != service.ActionLoanRecovery {
		t.Fatalf("jenis aksi %s, ingin LOAN_RECOVERY", pending.ActionType)
	}
	if n := woffJournalCount(t, e, "RECOV-", loan.LoanNumber); n != 0 {
		t.Fatalf("jurnal recovery %d, ingin 0 sebelum disetujui", n)
	}
}

// Keputusan panel mengubah bawaan ambang recovery menjadi 0 lewat migrasi 000092,
// tetapi migrasi itu TIDAK boleh menimpa kebijakan bank. Uji di sini menjalankan
// berkas migrasi apa adanya di atas database uji untuk membuktikan tiga sifat:
// nilai bank yang berbeda dibiarkan, nilai bawaan 000085 disetel 0, dan jalan kedua
// tidak mengubah apa pun.
func TestIntegrasiMigrasi000092TidakMenimpaNilaiBank(t *testing.T) {
	e := newBookWriteEnv(t)
	const key = "maker_checker.loan_recovery.threshold"

	baca := func() string {
		t.Helper()
		var value string
		if err := e.db.QueryRowContext(e.ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value); err != nil {
			t.Fatalf("membaca %s: %v", key, err)
		}
		return value
	}
	jalankanMigrasi := func() {
		t.Helper()
		path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", "000092_recovery_threshold_zero.up.sql")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("membaca migrasi 000092: %v", err)
		}
		if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
			t.Fatalf("menjalankan migrasi 000092: %v", err)
		}
		e.configSvc.Invalidate(key)
	}
	// Kembalikan ke bawaan 0 (nilai setelah seluruh migrasi) agar uji lain tak terpengaruh.
	t.Cleanup(func() { woffSetThreshold(t, e, "loan_recovery", "0") })

	// Nilai bank yang berbeda (mis. dilonggarkan) TIDAK boleh ditimpa.
	woffSetThreshold(t, e, "loan_recovery", "5000000")
	jalankanMigrasi()
	if got := baca(); got != "5000000" {
		t.Fatalf("migrasi menimpa nilai bank: %q, ingin tetap 5000000", got)
	}

	// Nilai bawaan 000085 ('10000000') disetel 0.
	woffSetThreshold(t, e, "loan_recovery", "10000000")
	jalankanMigrasi()
	if got := baca(); got != "0" {
		t.Fatalf("nilai bawaan 10000000 = %q, ingin disetel 0", got)
	}

	// Jalan kedua idempoten: tidak mengubah apa pun.
	jalankanMigrasi()
	if got := baca(); got != "0" {
		t.Fatalf("migrasi tidak idempoten: %q, ingin tetap 0", got)
	}
}
