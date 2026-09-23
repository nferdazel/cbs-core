package service_test

import (
	"errors"
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
