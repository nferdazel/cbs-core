package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type accountService struct {
	db           *sql.DB
	accountRepo  domain.AccountRepository
	customerRepo domain.CustomerRepository
	productRepo  domain.ProductRepository
	branchRepo   domain.BranchRepository
	numbering    domain.AccountNumberGenerator
}

func NewAccountService(
	db *sql.DB,
	accountRepo domain.AccountRepository,
	customerRepo domain.CustomerRepository,
	productRepo domain.ProductRepository,
	branchRepo domain.BranchRepository,
	numbering domain.AccountNumberGenerator,
) domain.AccountService {
	return &accountService{
		db:           db,
		accountRepo:  accountRepo,
		customerRepo: customerRepo,
		productRepo:  productRepo,
		branchRepo:   branchRepo,
		numbering:    numbering,
	}
}

// liabilityCOAForProduct memetakan produk simpanan ke akun kewajiban di COA.
// Buku (konvensional/syariah) memisahkan akun karena pelaporan keduanya tidak
// boleh dicampur, meski sama-sama kewajiban kepada nasabah.
func liabilityCOAForProduct(p *domain.BankingProduct) (string, error) {
	switch p.Book {
	case domain.BookConventional:
		switch p.Family {
		case domain.FamilySavings:
			return "20100", nil // Tabungan
		case domain.FamilyTimeDeposit:
			return "20200", nil // Deposito Berjangka
		case domain.FamilyCurrentAccount:
			return "20300", nil // Giro
		}
	case domain.BookSyariah:
		switch p.Family {
		case domain.FamilySavings:
			return "12100", nil // Tabungan Wadiah
		case domain.FamilyTimeDeposit:
			return "12300", nil // Deposito Mudharabah
		case domain.FamilyCurrentAccount:
			return "12300", nil // Giro (belum ada akun giro syariah terpisah)
		}
	}
	return "", fmt.Errorf("produk %s tidak untuk pembukaan rekening simpanan", p.Code)
}

func accountTypeForFamily(f domain.ProductFamily) domain.AccountType {
	switch f {
	case domain.FamilySavings:
		return domain.AccountTypeSavings
	case domain.FamilyCurrentAccount:
		return domain.AccountTypeChecking
	case domain.FamilyLoan:
		return domain.AccountTypeLoan
	default:
		return domain.AccountTypeSavings
	}
}

func (s *accountService) OpenAccount(ctx context.Context, input domain.OpenAccountInput, actor domain.Actor) (*domain.Account, error) {
	if input.Currency == "" {
		input.Currency = "IDR"
	}
	branchCode := input.BranchCode
	if branchCode == "" {
		branchCode = actor.BranchCode
	}
	if branchCode == "" {
		return nil, fmt.Errorf("kode cabang wajib diisi")
	}

	customer, err := s.customerRepo.GetByID(ctx, input.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("nasabah tidak valid: %w", err)
	}
	if customer.Status != domain.CustomerStatusActive {
		return nil, domain.ErrAccountInactive
	}

	product, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if !product.IsActive {
		return nil, fmt.Errorf("produk %s sedang tidak aktif", product.Code)
	}

	branch, err := s.branchRepo.GetByCode(ctx, branchCode)
	if err != nil {
		return nil, fmt.Errorf("cabang tidak valid: %w", err)
	}
	if !branch.IsActive {
		return nil, fmt.Errorf("cabang %s sedang tidak aktif", branchCode)
	}

	coaCode, err := liabilityCOAForProduct(product)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	accountNumber, err := s.numbering.NextAccountNumber(ctx, tx, product, branch.Code)
	if err != nil {
		return nil, err
	}

	coaID, err := s.resolveCOAID(ctx, tx, coaCode)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	account := &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    accountNumber,
		CustomerID:       &customer.ID,
		ProductID:        &product.ID,
		BranchID:         &branch.ID,
		COAID:            coaID,
		AccountType:      accountTypeForFamily(product.Family),
		Currency:         input.Currency,
		Balance:          decimal.Zero,
		AvailableBalance: decimal.Zero,
		HoldBalance:      decimal.Zero,
		Status:           domain.AccountStatusActive,
		Version:          1,
		OpenedAt:         &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.accountRepo.CreateTx(ctx, tx, account); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("nomor rekening sudah terpakai, silakan coba lagi")
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return account, nil
}

// resolveCOAID mencari id akun COA berdasarkan kode. Dilakukan di dalam transaksi
// pembukaan rekening agar konsisten dengan insert.
func (s *accountService) resolveCOAID(ctx context.Context, tx *sql.Tx, code string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRowContext(ctx, `SELECT id FROM chart_of_accounts WHERE account_code = $1`, code).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("akun COA %s tidak ditemukan", code)
		}
		return uuid.Nil, err
	}
	return id, nil
}

// isUniqueViolation mendeteksi pelanggaran unique constraint. Driver pgx
// membungkusnya sebagai error SQLSTATE 23505; pengecekan string dipakai agar
// tidak menambah ketergantungan pada tipe driver tertentu.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}

func (s *accountService) GetAccountByNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	return s.accountRepo.GetByNumber(ctx, accountNumber)
}

func (s *accountService) ListAccounts(ctx context.Context, page, pageSize int) ([]domain.Account, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.accountRepo.ListAll(ctx, pageSize, offset)
}
