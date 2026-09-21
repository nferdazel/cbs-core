package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// disburseLoanRepo membedakan snapshot GetByID dari baris hasil kunci LockLoanTx,
// sekaligus merekam nominal yang dipakai untuk menandai kredit cair.
type disburseLoanRepo struct {
	domain.LoanRepository
	snapshot *domain.Loan
	locked   *domain.Loan

	markedID          uuid.UUID
	markedOutstanding decimal.Decimal
}

func (r *disburseLoanRepo) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	return r.snapshot, nil
}

func (r *disburseLoanRepo) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	src := r.locked
	if src == nil {
		src = r.snapshot
	}
	cp := *src
	return &cp, nil
}

func (r *disburseLoanRepo) MarkDisbursedTx(_ context.Context, _ any, id uuid.UUID, outstanding decimal.Decimal) error {
	r.markedID = id
	r.markedOutstanding = outstanding
	return nil
}

// disburseAccountRepo mengembalikan satu rekening pencairan.
type disburseAccountRepo struct {
	domain.AccountRepository
	account *domain.Account
}

func (r *disburseAccountRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, domain.ErrAccountNotFound
}

// disburseFixture membangun layanan pencairan dengan produk yang punya pemetaan jurnal
// LOAN_DISBURSEMENT lengkap, termasuk akun rekening nasabah (COA 20100) agar override
// dipakai dan jurnal terbentuk.
func disburseFixture(t *testing.T, snapshot, locked *domain.Loan) (*loanService, *disburseLoanRepo, *stubPosting, uuid.UUID) {
	t.Helper()
	productID := snapshot.ProductID
	if productID == nil {
		t.Fatal("fixture memerlukan ProductID")
	}
	product := &domain.BankingProduct{
		ID:   *productID,
		Code: "KRD-FLAT",
		Name: "Kredit Flat",
	}
	products := &stubProductRepo{
		product: product,
		rules: map[domain.PostingEvent][]domain.JournalMappingRule{
			domain.EventLoanDisbursement: {
				{Event: domain.EventLoanDisbursement, Direction: domain.DirectionDebit, COACode: "10301", AmountSource: domain.AmountPrincipal},
				{Event: domain.EventLoanDisbursement, Direction: domain.DirectionCredit, COACode: "20100", AmountSource: domain.AmountPrincipal},
			},
		},
	}
	posting := &stubPosting{}
	repo := &disburseLoanRepo{snapshot: snapshot, locked: locked}
	acc := &domain.Account{
		ID:            snapshot.DisbursementAccountID,
		AccountNumber: "1110001",
		COACode:       "20100",
	}
	accRepo := &disburseAccountRepo{account: acc}
	cfg := &ckpnConfigStub{values: map[string]string{}} // saklar kerugian restrukturisasi mati

	svc := &loanService{
		loanRepo:    repo,
		productRepo: products,
		accountRepo: accRepo,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      cfg,
		txRunner:    stubTxRunner{},
	}
	return svc, repo, posting, *productID
}

// Jurnal pencairan dan penandaan status memakai nilai baris hasil kunci, bukan snapshot
// di luar transaksi.
func TestDisburseLoan_MemakaiBarisHasilKunci(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-DISB-1",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(10_000_000),
		DisbursementAccountID: uuid.New(),
	}
	locked := *snapshot
	locked.PrincipalAmount = decimal.NewFromInt(7_000_000)

	svc, repo, posting, _ := disburseFixture(t, snapshot, &locked)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	returned, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor)
	if err != nil {
		t.Fatalf("DisburseLoan: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(posting.requests))
	}
	if got := posting.requests[0].Lines[0].Amount; !got.Equal(locked.PrincipalAmount) {
		t.Fatalf("jurnal memakai pokok %s, mau pokok hasil kunci %s (snapshot %s)",
			got, locked.PrincipalAmount, snapshot.PrincipalAmount)
	}
	if !repo.markedOutstanding.Equal(locked.PrincipalAmount) {
		t.Fatalf("penandaan memakai pokok %s, mau pokok hasil kunci %s",
			repo.markedOutstanding, locked.PrincipalAmount)
	}
	if !returned.OutstandingPrincipal.Equal(locked.PrincipalAmount) {
		t.Fatalf("kredit kembali %s, mau pokok hasil kunci %s",
			returned.OutstandingPrincipal, locked.PrincipalAmount)
	}
}

// Status otoritatif dibaca dari baris hasil kunci: kredit yang sudah cair tidak boleh
// dicairkan dua kali meskipun snapshot lama masih APPROVED.
func TestDisburseLoan_TolakSudahCairDariBarisHasilKunci(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-DISB-2",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(10_000_000),
		DisbursementAccountID: uuid.New(),
	}
	locked := *snapshot
	locked.Status = domain.LoanStatusDisbursed

	svc, repo, posting, _ := disburseFixture(t, snapshot, &locked)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	_, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor)
	if !errors.Is(err, domain.ErrLoanAlreadyDisbursed) {
		t.Fatalf("mau ErrLoanAlreadyDisbursed, dapat %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("kredit yang sudah cair tidak boleh dijurnal lagi, dapat %d", len(posting.requests))
	}
	if !repo.markedOutstanding.IsZero() {
		t.Fatalf("kredit yang sudah cair tidak boleh ditandai lagi, dapat %s", repo.markedOutstanding)
	}
}

// Status selain APPROVED/DISBURSED juga ditolak dari baris hasil kunci.
func TestDisburseLoan_TolakStatusBukanApprovedDariBarisHasilKunci(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-DISB-3",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(10_000_000),
		DisbursementAccountID: uuid.New(),
	}
	locked := *snapshot
	locked.Status = domain.LoanStatusPendingApproval

	svc, _, posting, _ := disburseFixture(t, snapshot, &locked)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	_, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor)
	if !errors.Is(err, domain.ErrLoanNotApproved) {
		t.Fatalf("mau ErrLoanNotApproved, dapat %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("kredit belum APPROVED tidak boleh dijurnal, dapat %d", len(posting.requests))
	}
}
