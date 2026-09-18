package service

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kas default per buku. Teller konvensional memakai kas teller; transaksi syariah
// memakai kas syariah supaya arus kas kedua buku tidak tercampur di laporan.
const (
	defaultCashCOAConventional = "10101"
	defaultCashCOASYariah      = "11100"
)

// ledgerService mengorkestrasi transaksi rekening. Penulisan jurnal dan perhitungan
// saldo diserahkan sepenuhnya ke posting engine; service ini hanya menentukan akun
// lawan dan aturan bisnisnya.
type ledgerService struct {
	db          *sql.DB
	ledgerRepo  domain.LedgerRepository
	accountRepo domain.AccountRepository
	productRepo domain.ProductRepository
	resolver    domain.AccountResolver
	posting     domain.PostingService
	configSvc   domain.SystemConfigService
}

func NewLedgerService(
	db *sql.DB,
	ledgerRepo domain.LedgerRepository,
	accountRepo domain.AccountRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	posting domain.PostingService,
	configSvc domain.SystemConfigService,
) domain.LedgerService {
	return &ledgerService{
		db:          db,
		ledgerRepo:  ledgerRepo,
		accountRepo: accountRepo,
		productRepo: productRepo,
		resolver:    resolver,
		posting:     posting,
		configSvc:   configSvc,
	}
}

func (s *ledgerService) Deposit(ctx context.Context, req domain.DepositRequest) (*domain.JournalEntry, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}

	acc, err := s.accountRepo.GetByNumber(ctx, req.AccountNumber)
	if err != nil {
		return nil, err
	}
	if acc.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}

	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

	// Setoran menambah kewajiban bank kepada nasabah: debit kas, kredit rekening.
	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeDeposit,
		Description:     defaultDescription(req.Description, "Setoran tunai "+acc.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       req.CreatedBy,
		Lines: []domain.PostingLine{
			{AccountNumber: cashAccount, Direction: domain.DirectionDebit, Amount: req.Amount, Description: "Kas masuk"},
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionCredit, Amount: req.Amount, Description: req.Description},
		},
	})
}

func (s *ledgerService) Withdraw(ctx context.Context, req domain.WithdrawRequest) (*domain.JournalEntry, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}

	acc, err := s.accountRepo.GetByNumber(ctx, req.AccountNumber)
	if err != nil {
		return nil, err
	}
	if acc.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}
	if acc.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

	// Penarikan mengurangi kewajiban: debit rekening, kredit kas.
	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     defaultDescription(req.Description, "Penarikan tunai "+acc.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       req.CreatedBy,
		Lines: []domain.PostingLine{
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionDebit, Amount: req.Amount, Description: req.Description},
			{AccountNumber: cashAccount, Direction: domain.DirectionCredit, Amount: req.Amount, Description: "Kas keluar"},
		},
	})
}

func (s *ledgerService) TransferInternal(ctx context.Context, req domain.TransferRequest) (*domain.JournalEntry, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}
	if req.SourceAccountNumber == req.DestinationAccountNumber {
		return nil, fmt.Errorf("rekening asal dan tujuan tidak boleh sama")
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}

	src, err := s.accountRepo.GetByNumber(ctx, req.SourceAccountNumber)
	if err != nil {
		return nil, err
	}
	dest, err := s.accountRepo.GetByNumber(ctx, req.DestinationAccountNumber)
	if err != nil {
		return nil, err
	}
	if src.Status != domain.AccountStatusActive || dest.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}
	if src.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	// Transfer antar rekening nasabah: debit rekening asal, kredit rekening tujuan.
	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeTransferInternal,
		Description:     defaultDescription(req.Description, "Transfer "+src.AccountNumber+" ke "+dest.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       req.CreatedBy,
		Lines: []domain.PostingLine{
			{AccountNumber: src.AccountNumber, Direction: domain.DirectionDebit, Amount: req.Amount, Description: "Transfer keluar"},
			{AccountNumber: dest.AccountNumber, Direction: domain.DirectionCredit, Amount: req.Amount, Description: "Transfer masuk"},
		},
	})
}

// PostCompoundJournal memposting jurnal majemuk yang disusun operator. Posting engine
// yang menghitung saldo; service ini tidak lagi menebak sifat saldo dari nomor akun.
func (s *ledgerService) PostCompoundJournal(ctx context.Context, req domain.CustomJournalRequest) (*domain.JournalEntry, error) {
	if len(req.Lines) < 2 {
		return nil, fmt.Errorf("jurnal majemuk memerlukan minimal dua baris")
	}

	lines := make([]domain.PostingLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		if l.Amount.LessThanOrEqual(decimal.Zero) {
			return nil, domain.ErrInvalidAmount
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: l.AccountNumber,
			Direction:     l.Direction,
			Amount:        l.Amount,
			Description:   l.Description,
		})
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: req.TransactionType,
		Description:     req.Description,
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       req.CreatedBy,
		Lines:           lines,
	})
}

// resolveCashAccount menentukan akun kas lawan berdasarkan buku produk rekening.
// Syariah memakai kas syariah agar laporan arus kas kedua buku tetap terpisah.
func (s *ledgerService) resolveCashAccount(ctx context.Context, acc *domain.Account) (string, error) {
	coaCode := defaultCashCOAConventional
	if s.isSyariahAccount(ctx, acc) {
		coaCode = defaultCashCOASYariah
	}
	if s.configSvc != nil {
		key := "cash.coa.conventional"
		if coaCode == defaultCashCOASYariah {
			key = "cash.coa.syariah"
		}
		coaCode = s.configSvc.GetString(ctx, key, coaCode)
	}
	return s.resolver.ResolveGLAccount(ctx, nil, coaCode)
}

// isSyariahAccount menentukan buku rekening dari produknya. Tanpa produk (data lama),
// rekening dianggap konvensional.
func (s *ledgerService) isSyariahAccount(ctx context.Context, acc *domain.Account) bool {
	if acc.ProductID == nil || s.productRepo == nil {
		return false
	}
	product, err := s.productRepo.GetByID(ctx, *acc.ProductID)
	if err != nil {
		return false
	}
	return product.Book == domain.BookSyariah
}

func (s *ledgerService) GetJournalByReference(ctx context.Context, ref string) (*domain.JournalEntry, error) {
	return s.ledgerRepo.GetJournalByRef(ctx, ref)
}

func (s *ledgerService) ListJournals(ctx context.Context, page, pageSize int) ([]domain.JournalEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.ledgerRepo.ListJournals(ctx, pageSize, offset)
}

func (s *ledgerService) GetAccountStatement(ctx context.Context, accountNumber string, page, pageSize int) ([]domain.JournalLine, int, error) {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.ledgerRepo.ListAccountStatements(ctx, acc.ID, pageSize, offset)
}

func (s *ledgerService) GetChartOfAccounts(ctx context.Context) ([]domain.ChartOfAccount, error) {
	return s.ledgerRepo.GetCOAList(ctx)
}

func defaultDescription(desc, fallback string) string {
	if desc != "" {
		return desc
	}
	return fallback
}
