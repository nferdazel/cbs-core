package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// writeOffFixture menyiapkan satu kredit aktif beserta pemetaan jurnal hapus buku.
// Ambang persetujuan disetel 0 agar seluruh hapus buku wajib disetujui pejabat kedua.
func writeOffFixture() (interestFixture, *loanService) {
	f := newInterestFixture()
	f.repo.loan = &domain.Loan{
		ID:                    f.loanID,
		LoanNumber:            "KRD-2026-0007",
		Status:                domain.LoanStatusDisbursed,
		ProductID:             &f.productID,
		DisbursementAccountID: f.accountID,
		OutstandingPrincipal:  decimal.NewFromInt(2_500_000),
	}
	f.products.rules[domain.EventLoanWriteOff] = []domain.JournalMappingRule{
		{Direction: domain.DirectionDebit, COACode: "50100", AmountSource: domain.AmountPrincipal},
		{Direction: domain.DirectionCredit, COACode: "10301", AmountSource: domain.AmountPrincipal},
	}
	f.products.rules[domain.EventLoanRecovery] = []domain.JournalMappingRule{
		{Direction: domain.DirectionDebit, COACode: "20100", AmountSource: domain.AmountPrincipal},
		{Direction: domain.DirectionCredit, COACode: "50100", AmountSource: domain.AmountPrincipal},
	}
	svc := newInterestTestService(f)
	svc.approvals = &stubApprovals{threshold: map[string]decimal.Decimal{
		ActionLoanWriteOff: decimal.Zero,
		ActionLoanRecovery: decimal.Zero,
	}}
	return f, svc
}

func writeOffActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "ao.uji", Role: domain.RoleAO, BranchCode: "001"}
}

// Hapus buku tidak langsung berefek: ia melepas tagihan dari neraca dan tidak dapat
// dibatalkan, jadi harus menunggu persetujuan pejabat kedua.
func TestWriteOffLoan_RequiresApprovalBeforePosting(t *testing.T) {
	f, svc := writeOffFixture()
	actor := writeOffActor()

	loan, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit",
	}, actor)
	if loan != nil {
		t.Fatal("hapus buku belum boleh menghasilkan kredit sebelum disetujui")
	}
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("hapus buku harus menunggu persetujuan, dapat: %v", err)
	}
	if pending.ActionType != ActionLoanWriteOff {
		t.Fatalf("jenis aksi %s, ingin %s", pending.ActionType, ActionLoanWriteOff)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0 sebelum disetujui", len(f.posting.requests))
	}
	if f.repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("status kredit berubah sebelum disetujui: %s", f.repo.loan.Status)
	}

	approvals, ok := svc.approvals.(*stubApprovals)
	if !ok || len(approvals.created) != 1 {
		t.Fatalf("permintaan persetujuan %d, ingin 1", len(approvals.created))
	}
	created := approvals.created[0]
	if !created.Amount.Equal(decimal.NewFromInt(2_500_000)) {
		t.Fatalf("nominal permintaan %s, ingin 2500000", created.Amount)
	}
	if created.Payload["loan_id"] != f.loanID.String() {
		t.Fatalf("payload tanpa id kredit: %+v", created.Payload)
	}
	if created.Payload["maker_username"] != actor.Username {
		t.Fatalf("payload tanpa identitas pembuat: %+v", created.Payload)
	}
}

// Setelah disetujui, eksekutor menjalankan hapus buku: jurnal terposting, status
// berubah, dan jejak audit mencatat PEMBUAT keputusan, bukan pemeriksanya.
func TestWriteOffLoan_ExecuteApprovedPostsAndMarksLoan(t *testing.T) {
	f, svc := writeOffFixture()
	maker := writeOffActor()
	checker := domain.Actor{UserID: uuid.New(), Username: "kabag.uji", Role: domain.RoleSupervisor, BranchCode: "001"}
	audit := &stubAuditRepo{}
	svc.auditRepo = audit

	payload := map[string]any{
		"loan_id":        f.loanID.String(),
		"loan_number":    f.repo.loan.LoanNumber,
		"reason":         "debitor pailit",
		"maker_id":       maker.UserID.String(),
		"maker_username": maker.Username,
		"maker_role":     string(maker.Role),
		"maker_branch":   maker.BranchCode,
	}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionLoanWriteOff, payload, checker); err != nil {
		t.Fatalf("ExecuteApproved: %v", err)
	}

	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(f.posting.requests))
	}
	if got := f.posting.requests[0].IdempotencyKey; got != "WOFF-"+f.repo.loan.LoanNumber {
		t.Fatalf("kunci idempotensi %q, ingin WOFF-%s", got, f.repo.loan.LoanNumber)
	}
	if f.repo.loan.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("status kredit %s, ingin WRITTEN_OFF", f.repo.loan.Status)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "WRITE_OFF_LOAN" {
		t.Fatalf("audit hapus buku tidak tercatat: %+v", audit.events)
	}
	if audit.events[0].ActorUsername != maker.Username {
		t.Fatalf("audit mencatat %q, ingin pembuat %q", audit.events[0].ActorUsername, maker.Username)
	}
}

// Kredit yang statusnya berubah antara pengajuan dan persetujuan harus ditolak saat
// eksekusi: jadwal bisa saja sudah lunas, sehingga hapus buku tidak lagi berlaku.
func TestWriteOffLoan_ExecuteApprovedRejectsChangedLoan(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusPaidOff

	payload := map[string]any{"loan_id": f.loanID.String(), "reason": "debitor pailit"}
	err := svc.ExecuteApproved(context.Background(), nil, ActionLoanWriteOff, payload, writeOffActor())
	if err == nil {
		t.Fatal("hapus buku atas kredit yang tidak lagi aktif harus ditolak")
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// Recovery hapus buku melewati ambang yang sama: menunggu persetujuan, lalu
// dieksekusi dengan payload yang diajukan.
func TestRecoverWrittenOffLoan_RequiresApprovalThenExecutes(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	maker := writeOffActor()

	loan, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000),
	}, maker)
	var pending *domain.PendingApprovalError
	if loan != nil || !errors.As(err, &pending) {
		t.Fatalf("recovery harus menunggu persetujuan, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0 sebelum disetujui", len(f.posting.requests))
	}

	approvals := svc.approvals.(*stubApprovals)
	payload := approvals.created[0].Payload
	payload["maker_id"] = maker.UserID.String()
	payload["maker_username"] = maker.Username
	payload["maker_role"] = string(maker.Role)
	payload["maker_branch"] = maker.BranchCode

	checker := domain.Actor{UserID: uuid.New(), Username: "kabag.uji", Role: domain.RoleSupervisor, BranchCode: "001"}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionLoanRecovery, payload, checker); err != nil {
		t.Fatalf("ExecuteApproved recovery: %v", err)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1", len(f.posting.requests))
	}
	if f.repo.loan.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("status kredit berubah oleh recovery: %s", f.repo.loan.Status)
	}
}

// Ambang positif mengizinkan operasi di bawahnya berjalan langsung tanpa persetujuan.
func TestWriteOffLoan_BelowThresholdExecutesDirectly(t *testing.T) {
	f, svc := writeOffFixture()
	svc.approvals = &stubApprovals{threshold: map[string]decimal.Decimal{
		ActionLoanWriteOff: decimal.NewFromInt(10_000_000),
	}}
	svc.txRunner = stubTxRunner{}

	loan, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit",
	}, writeOffActor())
	if err != nil {
		t.Fatalf("hapus buku di bawah ambang: %v", err)
	}
	if loan == nil || loan.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("kredit tidak ditandai hapus buku: %+v", loan)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(f.posting.requests))
	}
}
