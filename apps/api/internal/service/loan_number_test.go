package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// w2AccountRepo mengembalikan satu rekening pencairan untuk pengajuan kredit.
type w2AccountRepo struct {
	domain.AccountRepository
	account *domain.Account
}

func (r *w2AccountRepo) GetByID(context.Context, uuid.UUID) (*domain.Account, error) {
	return r.account, nil
}

// w2LoanRepo menangkap kredit yang dibuat tanpa menyentuh database.
type w2LoanRepo struct {
	domain.LoanRepository
	created *domain.Loan
}

func (r *w2LoanRepo) Create(_ context.Context, l *domain.Loan, _ []domain.LoanSchedule) error {
	r.created = l
	return nil
}

// w2RefGen mencatat dari pembangkit mana nomor diambil. Ini yang membuktikan
// pengajuan kredit tidak lagi menyerap urutan referensi transaksi.
type w2RefGen struct {
	journalTxTypes []domain.TransactionType
	loanProducts   []*domain.BankingProduct
	// ctxSeen merekam konteks yang diteruskan ke NextLoanNumber untuk membuktikan
	// konteks pemanggil benar-benar menyebar (P2).
	ctxSeen context.Context
}

func (g *w2RefGen) Next(txType domain.TransactionType, _ time.Time) (string, error) {
	g.journalTxTypes = append(g.journalTxTypes, txType)
	return "TRF-20260921-999999", nil
}

func (g *w2RefGen) NextTx(_ context.Context, _ any, txType domain.TransactionType, _ time.Time) (string, error) {
	g.journalTxTypes = append(g.journalTxTypes, txType)
	return "TRF-20260921-999999", nil
}

func (g *w2RefGen) NextLoanNumber(ctx context.Context, product *domain.BankingProduct, _ time.Time) (string, error) {
	g.ctxSeen = ctx
	g.loanProducts = append(g.loanProducts, product)
	return domain.LoanNumberPrefix(product) + "-20260921-000001", nil
}

// P2: nomor kredit dibangkitkan dengan konteks pemanggil, bukan context.Background(),
// sehingga pembatalan/timeout permintaan ikut membatalkan pembacaan sequence.
func TestApplyLoan_MeneruskanKonteksKeNomorKredit(t *testing.T) {
	type ctxKey struct{}
	customerID := uuid.New()
	productID := uuid.New()
	acc := &domain.Account{
		ID:            uuid.New(),
		AccountNumber: "1110001",
		COACode:       "20100",
		COABook:       domain.BookConventional,
		CustomerID:    &customerID,
		BranchCode:    "001",
	}
	product := &domain.BankingProduct{
		ID:             productID,
		Code:           "KRD-FLAT",
		Family:         domain.FamilyLoan,
		Book:           domain.BookConventional,
		MinAmount:      decimal.NewFromInt(100_000),
		RateAnnual:     decimal.NewFromInt(12),
		ScheduleMethod: domain.ScheduleFlat,
		ProfitScheme:   domain.SchemeInterest,
	}
	ref := &w2RefGen{}
	svc := &loanService{
		productRepo: &stubProductRepo{product: product},
		accountRepo: &w2AccountRepo{account: acc},
		loanRepo:    &w2LoanRepo{},
		references:  ref,
	}

	ctx := context.WithValue(context.Background(), ctxKey{}, "penanda")
	_, err := svc.ApplyLoan(ctx, domain.ApplyLoanInput{
		CustomerID:            customerID,
		ProductID:             productID,
		DisbursementAccountID: acc.ID,
		PrincipalAmount:       decimal.NewFromInt(5_000_000),
		TermMonths:            12,
	}, domain.Actor{UserID: uuid.New(), Role: domain.RoleSuperAdmin, BranchCode: "001"})
	if err != nil {
		t.Fatalf("ApplyLoan: %v", err)
	}
	if ref.ctxSeen == nil {
		t.Fatal("NextLoanNumber tidak menerima konteks")
	}
	if got := ref.ctxSeen.Value(ctxKey{}); got != "penanda" {
		t.Fatalf("konteks pemanggil tidak diteruskan ke NextLoanNumber, nilai = %v", got)
	}
}

// Nomor kredit dibangkitkan pembangkit tersendiri (KRD konvensional, PMB syariah) dan
// TIDAK memanggil pembangkit referensi transaksi, sehingga urutan jurnal tidak tergeser
// oleh pengajuan kredit.
func TestApplyLoan_MemakaiNomorKreditTersendiri(t *testing.T) {
	for _, tc := range []struct {
		name       string
		book       domain.COABook
		wantPrefix string
	}{
		{"konvensional", domain.BookConventional, "KRD"},
		{"syariah", domain.BookSyariah, "PMB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			customerID := uuid.New()
			productID := uuid.New()
			acc := &domain.Account{
				ID:            uuid.New(),
				AccountNumber: "1110001",
				COACode:       "20100",
				COABook:       tc.book,
				CustomerID:    &customerID,
				BranchCode:    "001",
			}
			product := &domain.BankingProduct{
				ID:             productID,
				Code:           "KRD-FLAT",
				Family:         domain.FamilyLoan,
				Book:           tc.book,
				MinAmount:      decimal.NewFromInt(100_000),
				RateAnnual:     decimal.NewFromInt(12),
				ScheduleMethod: domain.ScheduleFlat,
				ProfitScheme:   domain.SchemeInterest,
			}
			ref := &w2RefGen{}
			repo := &w2LoanRepo{}
			svc := &loanService{
				productRepo: &stubProductRepo{product: product},
				accountRepo: &w2AccountRepo{account: acc},
				loanRepo:    repo,
				references:  ref,
			}

			loan, err := svc.ApplyLoan(context.Background(), domain.ApplyLoanInput{
				CustomerID:            customerID,
				ProductID:             productID,
				DisbursementAccountID: acc.ID,
				PrincipalAmount:       decimal.NewFromInt(5_000_000),
				TermMonths:            12,
			}, domain.Actor{UserID: uuid.New(), Role: domain.RoleSuperAdmin, BranchCode: "001"})
			if err != nil {
				t.Fatalf("ApplyLoan: %v", err)
			}

			want := tc.wantPrefix + "-20260921-000001"
			if loan.LoanNumber != want {
				t.Fatalf("nomor kredit %q, ingin %q", loan.LoanNumber, want)
			}
			if repo.created == nil || repo.created.LoanNumber != want {
				t.Fatalf("nomor yang disimpan tidak sesuai: %+v", repo.created)
			}
			if len(ref.loanProducts) != 1 {
				t.Fatalf("NextLoanNumber dipanggil %d kali, ingin 1", len(ref.loanProducts))
			}
			if len(ref.journalTxTypes) != 0 {
				t.Fatalf("pengajuan kredit memakai pembangkit referensi transaksi: %v", ref.journalTxTypes)
			}
		})
	}
}
