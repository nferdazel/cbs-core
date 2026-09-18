package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
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
	limits      domain.TransactionLimitService
	approvals   domain.MakerCheckerService
}

func NewLedgerService(
	db *sql.DB,
	ledgerRepo domain.LedgerRepository,
	accountRepo domain.AccountRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	posting domain.PostingService,
	configSvc domain.SystemConfigService,
	limits domain.TransactionLimitService,
	approvals domain.MakerCheckerService,
) domain.LedgerService {
	return &ledgerService{
		db:          db,
		ledgerRepo:  ledgerRepo,
		accountRepo: accountRepo,
		productRepo: productRepo,
		resolver:    resolver,
		posting:     posting,
		configSvc:   configSvc,
		limits:      limits,
		approvals:   approvals,
	}
}

// Types jenis aksi maker-checker untuk transaksi rekening. Dipakai juga sebagai
// kunci pendaftaran eksekutor.
const (
	ActionDeposit  = "DEPOSIT"
	ActionWithdraw = "WITHDRAWAL"
	ActionTransfer = "TRANSFER"
)

// guardLimit menegakkan batas transaksi. Tiga kemungkinan:
//   - lolos,
//   - ditolak karena melebihi batas (dikembalikan sebagai error),
//   - melewati ambang persetujuan, sehingga transaksi TIDAK diposting dan masuk
//     antrean maker-checker; pemanggil menerima PendingApprovalError (HTTP 202).
//
// Aturan ini ditegakkan di service, bukan di handler, karena menyangkut kewenangan
// pejabat bank dan tidak boleh bisa dilewati dengan memanggil service langsung.
func (s *ledgerService) guardLimit(ctx context.Context, actor domain.Actor, action, txType string, amount decimal.Decimal, payload map[string]any) error {
	if s.limits == nil {
		return nil
	}

	err := s.limits.Check(ctx, actor, txType, amount)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrRequiresApproval) {
		return err
	}
	if s.approvals == nil {
		return err
	}

	req, createErr := s.approvals.CreateRequest(ctx, domain.CreateMakerCheckerInput{
		ActionType: action,
		Amount:     amount,
		Payload:    payload,
		Notes:      "Nominal melewati ambang persetujuan pejabat",
	}, actor)
	if createErr != nil {
		return createErr
	}
	return &domain.PendingApprovalError{RequestID: req.ID, ActionType: action}
}

// ExecuteApproved mengeksekusi transaksi rekening yang sudah disetujui maker-checker.
// Dipanggil di dalam transaksi milik maker-checker service; batas transaksi tidak
// diperiksa ulang karena persetujuan itu sendiri yang menjadi kewenangannya.
func (s *ledgerService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	if payload == nil {
		return errors.New("payload persetujuan kosong")
	}
	accountNumber, _ := payload["account_number"].(string)
	amount, err := decimalFromPayload(payload["amount"])
	if err != nil {
		return err
	}
	description, _ := payload["description"].(string)
	idempotencyKey, _ := payload["idempotency_key"].(string)
	currency, _ := payload["currency"].(string)
	if currency == "" {
		currency = "IDR"
	}

	// Jurnal mencatat PEMBUAT transaksi, bukan pemeriksa. Peran pemeriksa tercatat
	// di audit log; mencatatnya sebagai pembuat akan menyesatkan penelusuran.
	createdBy := actor.DisplayName()
	if maker, ok := payload["maker_username"].(string); ok && maker != "" {
		createdBy = maker
	}

	switch normalizeAction(actionType) {
	case ActionDeposit:
		return s.postDepositTx(ctx, tx, accountNumber, amount, currency, description, idempotencyKey, createdBy)
	case ActionWithdraw:
		return s.postWithdrawTx(ctx, tx, accountNumber, amount, currency, description, idempotencyKey, createdBy)
	case ActionTransfer:
		source, _ := payload["source_account_number"].(string)
		destination, _ := payload["destination_account_number"].(string)
		return s.postTransferTx(ctx, tx, source, destination, amount, currency, description, idempotencyKey, createdBy)
	default:
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
}

// decimalFromPayload membaca nominal dari payload JSON, yang bisa berupa string
// (hasil .String()) atau angka.
func decimalFromPayload(raw any) (decimal.Decimal, error) {
	switch v := raw.(type) {
	case string:
		d, err := decimal.NewFromString(v)
		if err != nil {
			return decimal.Zero, fmt.Errorf("nominal tidak valid: %w", err)
		}
		return d, nil
	case float64:
		return decimal.NewFromFloat(v), nil
	case decimal.Decimal:
		return v, nil
	default:
		return decimal.Zero, errors.New("nominal tidak ada pada payload persetujuan")
	}
}

func (s *ledgerService) Deposit(ctx context.Context, req domain.DepositRequest) (*domain.JournalEntry, error) {
	if err := s.validateAmount(req.Amount); err != nil {
		return nil, err
	}

	acc, err := s.loadActiveAccount(ctx, req.AccountNumber)
	if err != nil {
		return nil, err
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

	if err := s.guardLimit(ctx, req.Actor, ActionDeposit, "deposit", req.Amount, map[string]any{
		"account_number":  req.AccountNumber,
		"amount":          req.Amount.String(),
		"currency":        req.Currency,
		"description":     req.Description,
		"idempotency_key": req.IdempotencyKey,
		"maker_username":  req.Actor.DisplayName(),
	}); err != nil {
		return nil, err
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeDeposit,
		Description:     defaultDescription(req.Description, "Setoran tunai "+acc.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       actorName(req.Actor, req.CreatedBy),
		Lines: []domain.PostingLine{
			{AccountNumber: cashAccount, Direction: domain.DirectionDebit, Amount: req.Amount, Description: "Kas masuk"},
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionCredit, Amount: req.Amount, Description: req.Description},
		},
	})
}

func (s *ledgerService) Withdraw(ctx context.Context, req domain.WithdrawRequest) (*domain.JournalEntry, error) {
	if err := s.validateAmount(req.Amount); err != nil {
		return nil, err
	}

	acc, err := s.loadActiveAccount(ctx, req.AccountNumber)
	if err != nil {
		return nil, err
	}
	if acc.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

	if err := s.guardLimit(ctx, req.Actor, ActionWithdraw, "withdrawal", req.Amount, map[string]any{
		"account_number":  req.AccountNumber,
		"amount":          req.Amount.String(),
		"currency":        req.Currency,
		"description":     req.Description,
		"idempotency_key": req.IdempotencyKey,
		"maker_username":  req.Actor.DisplayName(),
	}); err != nil {
		return nil, err
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     defaultDescription(req.Description, "Penarikan tunai "+acc.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       actorName(req.Actor, req.CreatedBy),
		Lines: []domain.PostingLine{
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionDebit, Amount: req.Amount, Description: req.Description},
			{AccountNumber: cashAccount, Direction: domain.DirectionCredit, Amount: req.Amount, Description: "Kas keluar"},
		},
	})
}

func (s *ledgerService) TransferInternal(ctx context.Context, req domain.TransferRequest) (*domain.JournalEntry, error) {
	if err := s.validateAmount(req.Amount); err != nil {
		return nil, err
	}
	if req.SourceAccountNumber == req.DestinationAccountNumber {
		return nil, fmt.Errorf("rekening asal dan tujuan tidak boleh sama")
	}

	src, err := s.loadActiveAccount(ctx, req.SourceAccountNumber)
	if err != nil {
		return nil, err
	}
	dest, err := s.loadActiveAccount(ctx, req.DestinationAccountNumber)
	if err != nil {
		return nil, err
	}
	if src.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	if err := s.guardLimit(ctx, req.Actor, ActionTransfer, "transfer", req.Amount, map[string]any{
		"source_account_number":      req.SourceAccountNumber,
		"destination_account_number": req.DestinationAccountNumber,
		"account_number":             req.SourceAccountNumber,
		"amount":                     req.Amount.String(),
		"currency":                   req.Currency,
		"description":                req.Description,
		"idempotency_key":            req.IdempotencyKey,
		"maker_username":             req.Actor.DisplayName(),
	}); err != nil {
		return nil, err
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeTransferInternal,
		Description:     defaultDescription(req.Description, "Transfer "+src.AccountNumber+" ke "+dest.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       actorName(req.Actor, req.CreatedBy),
		Lines: []domain.PostingLine{
			{AccountNumber: src.AccountNumber, Direction: domain.DirectionDebit, Amount: req.Amount, Description: "Transfer keluar"},
			{AccountNumber: dest.AccountNumber, Direction: domain.DirectionCredit, Amount: req.Amount, Description: "Transfer masuk"},
		},
	})
}

// postDepositTx, postWithdrawTx, dan postTransferTx dipakai eksekusi persetujuan:
// jurnal ditulis pada transaksi pemanggil, bukan membuka transaksi baru.
func (s *ledgerService) postDepositTx(ctx context.Context, tx any, accountNumber string, amount decimal.Decimal, currency, description, idempotencyKey, createdBy string) error {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return err
	}
	if acc.Status != domain.AccountStatusActive {
		return domain.ErrAccountInactive
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return err
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeDeposit,
		Description:     defaultDescription(description, "Setoran tunai "+acc.AccountNumber),
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       createdBy,
		Lines: []domain.PostingLine{
			{AccountNumber: cashAccount, Direction: domain.DirectionDebit, Amount: amount, Description: "Kas masuk"},
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionCredit, Amount: amount, Description: description},
		},
	})
	return err
}

func (s *ledgerService) postWithdrawTx(ctx context.Context, tx any, accountNumber string, amount decimal.Decimal, currency, description, idempotencyKey, createdBy string) error {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return err
	}
	if acc.Status != domain.AccountStatusActive {
		return domain.ErrAccountInactive
	}
	if acc.AvailableBalance.LessThan(amount) {
		return domain.ErrInsufficientFunds
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return err
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     defaultDescription(description, "Penarikan tunai "+acc.AccountNumber),
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       createdBy,
		Lines: []domain.PostingLine{
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionDebit, Amount: amount, Description: description},
			{AccountNumber: cashAccount, Direction: domain.DirectionCredit, Amount: amount, Description: "Kas keluar"},
		},
	})
	return err
}

func (s *ledgerService) postTransferTx(ctx context.Context, tx any, source, destination string, amount decimal.Decimal, currency, description, idempotencyKey, createdBy string) error {
	if source == destination {
		return fmt.Errorf("rekening asal dan tujuan tidak boleh sama")
	}
	src, err := s.accountRepo.GetByNumber(ctx, source)
	if err != nil {
		return err
	}
	dest, err := s.accountRepo.GetByNumber(ctx, destination)
	if err != nil {
		return err
	}
	if src.Status != domain.AccountStatusActive || dest.Status != domain.AccountStatusActive {
		return domain.ErrAccountInactive
	}
	if src.AvailableBalance.LessThan(amount) {
		return domain.ErrInsufficientFunds
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeTransferInternal,
		Description:     defaultDescription(description, "Transfer "+src.AccountNumber+" ke "+dest.AccountNumber),
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       createdBy,
		Lines: []domain.PostingLine{
			{AccountNumber: src.AccountNumber, Direction: domain.DirectionDebit, Amount: amount, Description: "Transfer keluar"},
			{AccountNumber: dest.AccountNumber, Direction: domain.DirectionCredit, Amount: amount, Description: "Transfer masuk"},
		},
	})
	return err
}

// validateAmount memastikan nominal positif dan mata uang terisi.
func (s *ledgerService) validateAmount(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.ErrInvalidAmount
	}
	return nil
}

// loadActiveAccount mengambil rekening dan memastikan statusnya aktif.
func (s *ledgerService) loadActiveAccount(ctx context.Context, accountNumber string) (*domain.Account, error) {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	if acc.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}
	return acc, nil
}

// actorName memakai identitas dari JWT; CreatedBy hanya dipertahankan sebagai
// kompatibilitas pemanggil lama dan tidak boleh menjadi sumber identitas.
func actorName(actor domain.Actor, fallback string) string {
	if actor.UserID != uuid.Nil || actor.Username != "" {
		return actor.DisplayName()
	}
	return fallback
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
