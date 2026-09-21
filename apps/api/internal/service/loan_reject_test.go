package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// w2RejectRepo merekam penolakan yang benar-benar disimpan.
type w2RejectRepo struct {
	domain.LoanRepository
	loan        *domain.Loan
	savedReason string
	rejectCalls int
}

func (r *w2RejectRepo) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	return r.loan, nil
}

func (r *w2RejectRepo) RejectLoanTx(_ context.Context, _ any, _ uuid.UUID, reason string) error {
	r.rejectCalls++
	r.savedReason = reason
	r.loan.Status = domain.LoanStatusRejected
	r.loan.RejectionReason = reason
	return nil
}

func w2RejectFixture() (*w2RejectRepo, *stubAuditRepo, *loanService) {
	repo := &w2RejectRepo{loan: &domain.Loan{
		ID:              uuid.New(),
		LoanNumber:      "KRD-20260921-000001",
		Status:          domain.LoanStatusPendingApproval,
		BranchCode:      "001",
		PrincipalAmount: decimal.NewFromInt(5_000_000),
	}}
	audit := &stubAuditRepo{}
	svc := &loanService{loanRepo: repo, auditRepo: audit, txRunner: stubTxRunner{}}
	return repo, audit, svc
}

func w2RejectActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "spv.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}
}

// Penolakan tanpa alasan harus ditolak dan tidak boleh menyentuh database/audit.
func TestRejectLoan_AlasanWajibDiisi(t *testing.T) {
	repo, audit, svc := w2RejectFixture()

	_, err := svc.RejectLoan(context.Background(), repo.loan.ID, "   ", w2RejectActor())
	if !errors.Is(err, domain.ErrLoanRejectionReasonRequired) {
		t.Fatalf("mau ErrLoanRejectionReasonRequired, dapat %v", err)
	}
	if repo.rejectCalls != 0 {
		t.Fatalf("penolakan tanpa alasan tetap menyentuh repo %d kali", repo.rejectCalls)
	}
	if len(audit.events) != 0 {
		t.Fatalf("penolakan tanpa alasan menulis audit %d kali", len(audit.events))
	}
	if repo.loan.Status != domain.LoanStatusPendingApproval {
		t.Fatalf("status berubah menjadi %s", repo.loan.Status)
	}
}

// Alasan yang melebihi batas ditolak agar tidak menyimpan data tak terkendali.
func TestRejectLoan_AlasanTerlaluPanjangDitolak(t *testing.T) {
	repo, _, svc := w2RejectFixture()

	_, err := svc.RejectLoan(context.Background(), repo.loan.ID,
		strings.Repeat("x", domain.MaxLoanRejectionReasonLen+1), w2RejectActor())
	if !errors.Is(err, domain.ErrLoanRejectionReasonTooLong) {
		t.Fatalf("mau ErrLoanRejectionReasonTooLong, dapat %v", err)
	}
	if repo.rejectCalls != 0 {
		t.Fatalf("alasan kepanjangan tetap disimpan %d kali", repo.rejectCalls)
	}
}

// Alasan yang sah disimpan pada baris kredit, dikembalikan pemanggil, dan dicatat audit.
func TestRejectLoan_AlasanTersimpanDanAuditMencatat(t *testing.T) {
	repo, audit, svc := w2RejectFixture()
	reason := "  agunan tidak memenuhi syarat  "

	loan, err := svc.RejectLoan(context.Background(), repo.loan.ID, reason, w2RejectActor())
	if err != nil {
		t.Fatalf("RejectLoan: %v", err)
	}

	trimmed := strings.TrimSpace(reason)
	if repo.savedReason != trimmed {
		t.Fatalf("alasan tersimpan %q, ingin %q", repo.savedReason, trimmed)
	}
	if repo.loan.Status != domain.LoanStatusRejected {
		t.Fatalf("status tersimpan %s, ingin REJECTED", repo.loan.Status)
	}
	if loan.RejectionReason != trimmed {
		t.Fatalf("alasan pada hasil %q, ingin %q", loan.RejectionReason, trimmed)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit %d event, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "REJECT_LOAN" {
		t.Fatalf("aksi audit %q, ingin REJECT_LOAN", event.Action)
	}
	if got, _ := event.Changes["reason"].(string); got != trimmed {
		t.Fatalf("alasan pada audit changes %q, ingin %q", got, trimmed)
	}
	// Pembaca audit lama membaca metadata, bukan changes; alasan harus ada di sana
	// juga agar penolakan tidak tampak tanpa alasan.
	if got, _ := event.Metadata["reason"].(string); got != trimmed {
		t.Fatalf("alasan pada audit metadata %q, ingin %q", got, trimmed)
	}
}
