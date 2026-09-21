package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubPlacementPoster menangkap PostingMeta jalur penempatan supaya jenis transaksi
// (dan karenanya prefix referensi) dapat diperiksa tanpa database.
type stubPlacementPoster struct {
	metas []PostingMeta
}

func (s *stubPlacementPoster) PostEventTx(ctx context.Context, tx any, product *domain.BankingProduct, event domain.PostingEvent, amounts Amounts, meta PostingMeta) (*domain.JournalEntry, error) {
	s.metas = append(s.metas, meta)
	return &domain.JournalEntry{}, nil
}

type stubPlacementNumbering struct{ number string }

func (s stubPlacementNumbering) NextAccountNumber(ctx context.Context, tx *sql.Tx, product *domain.BankingProduct, branchCode string) (string, error) {
	return s.number, nil
}

type stubPlacementAccountRepo struct{ domain.AccountRepository }

func (s *stubPlacementAccountRepo) CreateTx(ctx context.Context, tx *sql.Tx, account *domain.Account) error {
	return nil
}

// Penempatan deposito baru harus memakai jenis transaksi DEPOSIT_PLACEMENT, bukan
// DEPOSIT milik setoran tunai teller. Perbedaan inilah yang membuat laporan dan
// rekonsiliasi bisa memisahkan keduanya; prefix referensi mengikuti jenisnya (DPL).
//
// Jurnal penempatan LAMA tetap DEPOSIT dan tidak diuji di sini: baris lama memang
// sengaja tidak direklasifikasi.
func TestPenempatanDepositoMemakaiJenisTersendiri(t *testing.T) {
	poster := &stubPlacementPoster{}
	now := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	svc := &depositService{
		numbering:   stubPlacementNumbering{number: "DEP-001-0001"},
		accountRepo: &stubPlacementAccountRepo{},
		depositRepo: &stubDepositRepo{},
		poster:      poster,
	}

	prep := &depositPlacementPrep{
		product: &domain.BankingProduct{
			ID:     uuid.New(),
			Code:   "DEP-CONV",
			Name:   "Deposito Berjangka",
			Family: domain.FamilyTimeDeposit,
		},
		customer:   &domain.CustomerRecord{ID: uuid.New()},
		branch:     &domain.Branch{ID: uuid.New(), Code: "001"},
		coa:        &domain.ChartOfAccount{ID: uuid.New()},
		start:      now,
		maturity:   now.AddDate(0, 3, 0),
		now:        now,
		profitType: domain.ProfitTypeInterest,
		taxRate:    decimal.NewFromInt(20),
	}

	_, err := svc.persistPlacement(context.Background(), nil, domain.PlaceDepositInput{
		CustomerID:      prep.customer.ID,
		ProductID:       prep.product.ID,
		PlacementAmount: decimal.NewFromInt(10_000_000),
		TermMonths:      3,
		Currency:        "IDR",
	}, testActor(), prep)
	if err != nil {
		t.Fatalf("persistPlacement: %v", err)
	}
	if len(poster.metas) != 1 {
		t.Fatalf("jurnal penempatan harus diposting tepat sekali, dapat %d", len(poster.metas))
	}
	if got := poster.metas[0].TransactionType; got != domain.TxTypeDepositPlacement {
		t.Fatalf("jenis transaksi penempatan = %q, mau %q", got, domain.TxTypeDepositPlacement)
	}
	if poster.metas[0].TransactionType == domain.TxTypeDeposit {
		t.Fatal("penempatan deposito tidak boleh lagi memakai jenis setoran tunai DEPOSIT")
	}
}

// Sisi lain kontrak: setoran tunai teller tetap memakai DEPOSIT, sehingga kedua jalur
// benar-benar berbeda setelah perubahan tipe penempatan.
func TestSetoranTunaiTetapMemakaiJenisDeposit(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerForTest(stubLimits{}, &stubApprovals{}, posting)

	_, err := svc.Deposit(context.Background(), domain.DepositRequest{
		AccountNumber: savingsAccount().AccountNumber,
		Amount:        decimal.NewFromInt(1_000_000),
		Actor:         testActor(),
	})
	if err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if posting.last.TransactionType != domain.TxTypeDeposit {
		t.Fatalf("jenis transaksi setoran tunai = %q, mau %q", posting.last.TransactionType, domain.TxTypeDeposit)
	}
	if posting.last.TransactionType == domain.TxTypeDepositPlacement {
		t.Fatal("setoran tunai tidak boleh memakai jenis penempatan deposito")
	}
}
