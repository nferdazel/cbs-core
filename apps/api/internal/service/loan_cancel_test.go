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

// cancelLoanRepo adalah LoanRepository minimal untuk pembatalan pencairan. Ia
// mencatat efek samping di memori agar test dapat memastikan langkah mana yang
// benar-benar berjalan dan mana yang tidak setelah transaksi gagal.
type cancelLoanRepo struct {
	domain.LoanRepository

	loan       *domain.Loan
	schedules  []domain.LoanSchedule
	hasPayment bool
	journalRef string
	deleteErr  error

	deletedSchedules bool
	outstanding      decimal.Decimal
	penalty          decimal.Decimal
}

func (r *cancelLoanRepo) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	if r.loan == nil {
		return nil, domain.ErrLoanNotFound
	}
	return r.loan, nil
}

func (r *cancelLoanRepo) GetSchedules(context.Context, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *cancelLoanRepo) HasInstallmentPaymentTx(context.Context, any, uuid.UUID) (bool, error) {
	return r.hasPayment, nil
}

func (r *cancelLoanRepo) GetDisbursementJournalRefTx(context.Context, any, string) (string, error) {
	return r.journalRef, nil
}

func (r *cancelLoanRepo) DeleteSchedulesTx(context.Context, any, uuid.UUID) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletedSchedules = true
	r.schedules = nil
	return nil
}

func (r *cancelLoanRepo) UpdateStatusTx(_ context.Context, _ any, _ uuid.UUID, status domain.LoanStatus, _ *uuid.UUID) error {
	r.loan.Status = status
	return nil
}

func (r *cancelLoanRepo) UpdateOutstandingTx(_ context.Context, _ any, _ uuid.UUID, outstanding, penalty decimal.Decimal) error {
	r.outstanding = outstanding
	r.penalty = penalty
	return nil
}

var _ domain.LoanRepository = (*cancelLoanRepo)(nil)

const cancelDisbursementRef = "TRF-20260920-0001"

// disbursementEntry adalah jurnal pencairan yang dibalik: dana masuk ke rekening
// nasabah (debit) dan pokok kredit tercatat (kredit).
func disbursementEntry() *domain.JournalEntry {
	return &domain.JournalEntry{
		ID:              uuid.New(),
		ReferenceNumber: cancelDisbursementRef,
		TransactionType: domain.TxTypeTransferInternal,
		Status:          domain.JournalStatusPosted,
		Description:     "Pencairan kredit",
		CreatedBy:       "ao.uji",
		BranchCode:      "001",
		Source:          domain.SourceLoan,
		Lines: []domain.JournalLine{
			{ID: uuid.New(), AccountNumber: "1110001234", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(10_000_000)},
			{ID: uuid.New(), AccountNumber: "10301", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(10_000_000)},
		},
	}
}

func cancelFixture() (*cancelLoanRepo, *reversalLedgerRepo, *stubPosting, *stubAuditRepo, *loanService) {
	loanID := uuid.New()
	repo := &cancelLoanRepo{
		loan: &domain.Loan{
			ID:                   loanID,
			LoanNumber:           "KRD-2026-0007",
			Status:               domain.LoanStatusDisbursed,
			BranchCode:           "001",
			OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		},
		journalRef: cancelDisbursementRef,
		schedules: []domain.LoanSchedule{{
			ID:            uuid.New(),
			LoanID:        loanID,
			InstallmentNo: 1,
			Status:        domain.InstallmentStatusPending,
		}},
	}
	ledger := &reversalLedgerRepo{entry: disbursementEntry()}
	posting := &stubPosting{}
	audit := &stubAuditRepo{}
	svc := &loanService{
		loanRepo:   repo,
		ledgerRepo: ledger,
		posting:    posting,
		auditRepo:  audit,
		txRunner:   stubTxRunner{},
		dates:      &reversalDateRepo{date: tanggalBisnisUji},
	}
	return repo, ledger, posting, audit, svc
}

func cancelActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "admin.uji", Role: domain.RoleAdmin, BranchCode: "001"}
}

// (a) Jalur bahagia: status CANCELLED, jadwal terhapus, jurnal kontra ditulis,
// jurnal asal ditandai REVERSED, dan audit tercatat.
func TestCancelDisbursementLoan_MembatalkanPenuh(t *testing.T) {
	repo, ledger, posting, audit, svc := cancelFixture()

	loan, err := svc.CancelDisbursementLoan(context.Background(), domain.CancelLoanInput{
		LoanID: repo.loan.ID, Reason: "salah nasabah",
	}, cancelActor())
	if err != nil {
		t.Fatalf("pembatalan gagal: %v", err)
	}
	if loan == nil || loan.Status != domain.LoanStatusCancelled {
		t.Fatalf("kredit tidak berstatus CANCELLED: %+v", loan)
	}
	if repo.loan.Status != domain.LoanStatusCancelled {
		t.Fatalf("status tersimpan %s, ingin CANCELLED", repo.loan.Status)
	}
	if !repo.deletedSchedules || len(repo.schedules) != 0 {
		t.Fatal("jadwal angsuran tidak dihapus")
	}
	if !repo.outstanding.IsZero() || !repo.penalty.IsZero() {
		t.Fatalf("sisa pokok/denda tidak nol: %s/%s", repo.outstanding, repo.penalty)
	}

	if len(posting.requests) != 1 {
		t.Fatalf("jurnal kontra %d, ingin 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.TransactionType != domain.TxTypeReversal {
		t.Fatalf("jenis transaksi %q, ingin REVERSAL", req.TransactionType)
	}
	if req.IdempotencyKey != "REV-"+cancelDisbursementRef {
		t.Fatalf("kunci idempotency %q, ingin REV-%s", req.IdempotencyKey, cancelDisbursementRef)
	}
	if req.BranchCode != "001" {
		t.Fatalf("cabang kontra %q, ingin mewarisi jurnal asal 001", req.BranchCode)
	}
	if req.Source != domain.SourceLoan {
		t.Fatalf("alur jurnal kontra %q, ingin LOAN", req.Source)
	}
	// Jurnal kontra harus bertanggal bisnis berjalan. Bila memakai tanggal kalender,
	// pembatalan yang dijalankan saat tutup hari tertinggal akan jatuh di periode yang
	// belum dibuka dan tidak ikut terhitung tutup hari.
	if !req.EntryDate.Equal(tanggalBisnisUji) {
		t.Fatalf("tanggal jurnal kontra %s, ingin tanggal bisnis %s", req.EntryDate, tanggalBisnisUji)
	}
	if len(req.Lines) != 2 || req.Lines[0].Direction != domain.DirectionCredit || req.Lines[1].Direction != domain.DirectionDebit {
		t.Fatalf("arah baris kontra tidak ditukar: %+v", req.Lines)
	}
	if !strings.Contains(req.Description, "KRD-2026-0007") || !strings.Contains(req.Description, "salah nasabah") {
		t.Fatalf("deskripsi harus memuat nomor kredit dan alasan: %q", req.Description)
	}

	if len(ledger.markedRef) != 1 || ledger.markedRef[0] != cancelDisbursementRef {
		t.Fatalf("jurnal asal tidak ditandai REVERSED: %+v", ledger.markedRef)
	}

	if len(audit.events) != 1 {
		t.Fatalf("audit %d peristiwa, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "CANCEL_DISBURSEMENT_LOAN" || event.ResourceType != "loan" || event.ResourceID != repo.loan.ID.String() {
		t.Fatalf("audit tidak sesuai: %+v", event)
	}
	if event.Changes["reason"] != "salah nasabah" || event.Changes["reversed_reference"] != cancelDisbursementRef {
		t.Fatalf("alasan/referensi jurnal tidak tercatat: %+v", event.Changes)
	}
}

// (b) Kredit yang bukan DISBURSED tidak boleh dibatalkan.
func TestCancelDisbursementLoan_MenolakStatusBukanDisbursed(t *testing.T) {
	repo, ledger, posting, _, svc := cancelFixture()
	repo.loan.Status = domain.LoanStatusApproved

	_, err := svc.CancelDisbursementLoan(context.Background(), domain.CancelLoanInput{
		LoanID: repo.loan.ID, Reason: "salah nasabah",
	}, cancelActor())
	if !errors.Is(err, domain.ErrLoanNotCancellable) {
		t.Fatalf("kesalahan %v, ingin ErrLoanNotCancellable", err)
	}
	if len(posting.requests) != 0 || len(ledger.markedRef) != 0 {
		t.Fatal("tidak boleh ada jurnal saat status bukan DISBURSED")
	}
	if repo.deletedSchedules {
		t.Fatal("jadwal tidak boleh dihapus saat penolakan")
	}
}

// (c) Kredit yang sudah punya angsuran dibayar tidak boleh dibatalkan.
func TestCancelDisbursementLoan_MenolakAngsuranSudahDibayar(t *testing.T) {
	repo, ledger, posting, _, svc := cancelFixture()
	repo.hasPayment = true

	_, err := svc.CancelDisbursementLoan(context.Background(), domain.CancelLoanInput{
		LoanID: repo.loan.ID, Reason: "salah nasabah",
	}, cancelActor())
	if !errors.Is(err, domain.ErrLoanHasInstallmentPayments) {
		t.Fatalf("kesalahan %v, ingin ErrLoanHasInstallmentPayments", err)
	}
	if len(posting.requests) != 0 || len(ledger.markedRef) != 0 {
		t.Fatal("tidak boleh ada jurnal saat sudah ada angsuran dibayar")
	}
	if repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("status berubah meski ditolak: %s", repo.loan.Status)
	}
}

// (d) Kredit cabang lain disamarkan sebagai tidak ditemukan, bukan dibedakan.
func TestCancelDisbursementLoan_MenyamarkanCabangLain(t *testing.T) {
	repo, ledger, posting, _, svc := cancelFixture()
	repo.loan.BranchCode = "999"

	_, err := svc.CancelDisbursementLoan(context.Background(), domain.CancelLoanInput{
		LoanID: repo.loan.ID, Reason: "salah nasabah",
	}, cancelActor())
	if !errors.Is(err, domain.ErrLoanNotFound) {
		t.Fatalf("kesalahan %v, ingin ErrLoanNotFound", err)
	}
	if len(posting.requests) != 0 || len(ledger.markedRef) != 0 {
		t.Fatal("tidak boleh ada jurnal untuk cabang lain")
	}
}

// (e) Kegagalan di tengah transaksi tidak boleh meninggalkan status berubah.
func TestCancelDisbursementLoan_RollbackTidakMengubahStatus(t *testing.T) {
	repo, _, _, _, svc := cancelFixture()
	repo.deleteErr = errors.New("gagal menghapus jadwal")

	_, err := svc.CancelDisbursementLoan(context.Background(), domain.CancelLoanInput{
		LoanID: repo.loan.ID, Reason: "salah nasabah",
	}, cancelActor())
	if err == nil {
		t.Fatal("kegagalan menghapus jadwal harus diteruskan")
	}
	if repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("status berubah meski transaksi gagal: %s", repo.loan.Status)
	}
	if repo.deletedSchedules {
		t.Fatal("jadwal tidak boleh tercatat terhapus saat transaksi gagal")
	}
	if !repo.outstanding.IsZero() {
		t.Fatal("sisa pokok tidak boleh diubah saat transaksi gagal")
	}
}

// Wewenang: pembatalan hanya untuk peran tertinggi. Supervisor memegang
// loans:approve (hapus buku) tetapi bukan loans:cancel.
func TestCancelDisbursementLoan_HanyaPeranTertinggi(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin} {
		if !role.HasPermission(domain.PermLoansCancel) {
			t.Fatalf("%s harus berwenang membatalkan pencairan", role)
		}
	}
	if domain.RoleSupervisor.HasPermission(domain.PermLoansCancel) {
		t.Fatal("supervisor tidak boleh berwenang membatalkan pencairan")
	}
}

// Tanggal bisnis tidak terbaca: pembatalan ditolak dan tidak ada jurnal yang ditulis.
// Menebak tanggal kalender akan menempatkan kontra di periode yang bisa saja belum
// dibuka, dan itu lebih buruk daripada menolak operasinya.
func TestCancelDisbursementLoan_TanggalBisnisTidakTerbaca(t *testing.T) {
	repo, ledger, posting, _, svc := cancelFixture()
	svc.dates = &reversalDateRepo{err: errors.New("tanggal bisnis tidak terbaca")}

	_, err := svc.CancelDisbursementLoan(context.Background(), domain.CancelLoanInput{
		LoanID: repo.loan.ID, Reason: "salah nasabah",
	}, cancelActor())
	if err == nil {
		t.Fatal("pembatalan seharusnya ditolak saat tanggal bisnis tidak terbaca")
	}
	if len(posting.requests) != 0 || len(ledger.markedRef) != 0 {
		t.Fatalf("tidak boleh ada jurnal saat tanggal bisnis tidak terbaca: %+v", posting.requests)
	}
	if repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("status kredit berubah menjadi %s", repo.loan.Status)
	}
}
