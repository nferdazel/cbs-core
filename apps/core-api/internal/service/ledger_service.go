package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ledgerService struct {
	ledgerRepo  domain.LedgerRepository
	accountRepo domain.AccountRepository
	db          *sql.DB
}

func NewLedgerService(
	ledgerRepo domain.LedgerRepository,
	accountRepo domain.AccountRepository,
	db *sql.DB,
) domain.LedgerService {
	return &ledgerService{
		ledgerRepo:  ledgerRepo,
		accountRepo: accountRepo,
		db:          db,
	}
}

func (s *ledgerService) Deposit(ctx context.Context, req domain.DepositRequest) (*domain.JournalEntry, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}

	// 1. Check idempotency if key is provided
	if req.IdempotencyKey != "" {
		existing, err := s.ledgerRepo.GetJournalByRef(ctx, req.IdempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 2. Lock and retrieve accounts
	// Vault Account (Debit: Asset increases)
	vaultAcc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, "GL-VAULT-001")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vault account: %w", err)
	}

	// Customer Account (Credit: Liability increases)
	custAcc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, req.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("target customer account not found: %w", err)
	}

	if custAcc.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}

	// 3. Calculate new balances
	newVaultBal := vaultAcc.Balance.Add(req.Amount)
	newCustBal := custAcc.Balance.Add(req.Amount)
	newCustAvail := custAcc.AvailableBalance.Add(req.Amount)

	// 4. Update balances in DB with version check
	if err := s.accountRepo.UpdateBalance(ctx, tx, vaultAcc.ID, newVaultBal, newVaultBal, vaultAcc.Version); err != nil {
		return nil, fmt.Errorf("failed to update vault balance: %w", err)
	}
	if err := s.accountRepo.UpdateBalance(ctx, tx, custAcc.ID, newCustBal, newCustAvail, custAcc.Version); err != nil {
		return nil, fmt.Errorf("failed to update customer balance: %w", err)
	}

	// 5. Construct Double-Entry Journal
	journalID := uuid.New()
	refNumber := fmt.Sprintf("DEP-%s-%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	now := time.Now().UTC()

	var idemKeyPtr *string
	if req.IdempotencyKey != "" {
		idemKeyPtr = &req.IdempotencyKey
	}

	lines := []domain.JournalLine{
		{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      vaultAcc.ID,
			AccountNumber:  vaultAcc.AccountNumber,
			Direction:      domain.DirectionDebit,
			Amount:         req.Amount,
			Currency:       req.Currency,
			BalanceAfter:   newVaultBal,
			Sequence:       1,
			Description:    fmt.Sprintf("Cash Vault Inflow from Deposit to %s", custAcc.AccountNumber),
			CreatedAt:      now,
		},
		{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      custAcc.ID,
			AccountNumber:  custAcc.AccountNumber,
			Direction:      domain.DirectionCredit,
			Amount:         req.Amount,
			Currency:       req.Currency,
			BalanceAfter:   newCustBal,
			Sequence:       2,
			Description:    req.Description,
			CreatedAt:      now,
		},
	}

	if err := domain.ValidateDoubleEntry(lines); err != nil {
		return nil, err
	}

	entry := &domain.JournalEntry{
		ID:              journalID,
		ReferenceNumber: refNumber,
		IdempotencyKey:  idemKeyPtr,
		TransactionType: domain.TxTypeDeposit,
		Description:     req.Description,
		Status:          domain.JournalStatusPosted,
		PostedAt:        now,
		CreatedBy:       req.CreatedBy,
		Lines:           lines,
		CreatedAt:       now,
	}

	// 6. Insert Journal Entry & Lines in same SQL transaction
	entryQuery := `
		INSERT INTO journal_entries (id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.ExecContext(ctx, entryQuery, entry.ID, entry.ReferenceNumber, entry.IdempotencyKey, entry.TransactionType, entry.Description, entry.Status, entry.PostedAt, entry.CreatedBy, entry.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert journal entry: %w", err)
	}

	lineQuery := `
		INSERT INTO journal_lines (id, journal_entry_id, account_id, direction, amount, currency, balance_after, sequence, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, lineQuery, l.ID, l.JournalEntryID, l.AccountID, l.Direction, l.Amount, l.Currency, l.BalanceAfter, l.Sequence, l.Description, l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to insert journal line: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return entry, nil
}

func (s *ledgerService) Withdraw(ctx context.Context, req domain.WithdrawRequest) (*domain.JournalEntry, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Lock customer account first
	custAcc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, req.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("customer account not found: %w", err)
	}

	if custAcc.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}

	if custAcc.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	// 2. Lock vault account
	vaultAcc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, "GL-VAULT-001")
	if err != nil {
		return nil, fmt.Errorf("vault account not found: %w", err)
	}

	// 3. Balances after withdrawal
	newCustBal := custAcc.Balance.Sub(req.Amount)
	newCustAvail := custAcc.AvailableBalance.Sub(req.Amount)
	newVaultBal := vaultAcc.Balance.Sub(req.Amount)

	// 4. Update balances
	if err := s.accountRepo.UpdateBalance(ctx, tx, custAcc.ID, newCustBal, newCustAvail, custAcc.Version); err != nil {
		return nil, fmt.Errorf("failed to update customer balance: %w", err)
	}
	if err := s.accountRepo.UpdateBalance(ctx, tx, vaultAcc.ID, newVaultBal, newVaultBal, vaultAcc.Version); err != nil {
		return nil, fmt.Errorf("failed to update vault balance: %w", err)
	}

	// 5. Construct Double-Entry Journal
	journalID := uuid.New()
	refNumber := fmt.Sprintf("WDL-%s-%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	now := time.Now().UTC()

	var idemKeyPtr *string
	if req.IdempotencyKey != "" {
		idemKeyPtr = &req.IdempotencyKey
	}

	lines := []domain.JournalLine{
		{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      custAcc.ID,
			AccountNumber:  custAcc.AccountNumber,
			Direction:      domain.DirectionDebit,
			Amount:         req.Amount,
			Currency:       req.Currency,
			BalanceAfter:   newCustBal,
			Sequence:       1,
			Description:    req.Description,
			CreatedAt:      now,
		},
		{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      vaultAcc.ID,
			AccountNumber:  vaultAcc.AccountNumber,
			Direction:      domain.DirectionCredit,
			Amount:         req.Amount,
			Currency:       req.Currency,
			BalanceAfter:   newVaultBal,
			Sequence:       2,
			Description:    fmt.Sprintf("Cash Vault Outflow to %s", custAcc.AccountNumber),
			CreatedAt:      now,
		},
	}

	if err := domain.ValidateDoubleEntry(lines); err != nil {
		return nil, err
	}

	entry := &domain.JournalEntry{
		ID:              journalID,
		ReferenceNumber: refNumber,
		IdempotencyKey:  idemKeyPtr,
		TransactionType: domain.TxTypeWithdrawal,
		Description:     req.Description,
		Status:          domain.JournalStatusPosted,
		PostedAt:        now,
		CreatedBy:       req.CreatedBy,
		Lines:           lines,
		CreatedAt:       now,
	}

	// 6. Persist Header & Lines
	entryQuery := `
		INSERT INTO journal_entries (id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.ExecContext(ctx, entryQuery, entry.ID, entry.ReferenceNumber, entry.IdempotencyKey, entry.TransactionType, entry.Description, entry.Status, entry.PostedAt, entry.CreatedBy, entry.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert journal entry: %w", err)
	}

	lineQuery := `
		INSERT INTO journal_lines (id, journal_entry_id, account_id, direction, amount, currency, balance_after, sequence, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, lineQuery, l.ID, l.JournalEntryID, l.AccountID, l.Direction, l.Amount, l.Currency, l.BalanceAfter, l.Sequence, l.Description, l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to insert journal line: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return entry, nil
}

func (s *ledgerService) TransferInternal(ctx context.Context, req domain.TransferRequest) (*domain.JournalEntry, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}
	if req.SourceAccountNumber == req.DestinationAccountNumber {
		return nil, fmt.Errorf("source and destination account cannot be identical")
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Prevent deadlocks by always locking accounts in lexicographical order of account_number
	firstAccNum, secondAccNum := req.SourceAccountNumber, req.DestinationAccountNumber
	if strings.Compare(firstAccNum, secondAccNum) > 0 {
		firstAccNum, secondAccNum = secondAccNum, firstAccNum
	}

	firstAcc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, firstAccNum)
	if err != nil {
		return nil, fmt.Errorf("account %s not found: %w", firstAccNum, err)
	}
	secondAcc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, secondAccNum)
	if err != nil {
		return nil, fmt.Errorf("account %s not found: %w", secondAccNum, err)
	}

	var srcAcc, destAcc *domain.Account
	if firstAcc.AccountNumber == req.SourceAccountNumber {
		srcAcc = firstAcc
		destAcc = secondAcc
	} else {
		srcAcc = secondAcc
		destAcc = firstAcc
	}

	if srcAcc.Status != domain.AccountStatusActive || destAcc.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountInactive
	}

	if srcAcc.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	// Balance calculations
	newSrcBal := srcAcc.Balance.Sub(req.Amount)
	newSrcAvail := srcAcc.AvailableBalance.Sub(req.Amount)
	newDestBal := destAcc.Balance.Add(req.Amount)
	newDestAvail := destAcc.AvailableBalance.Add(req.Amount)

	// Update balances
	if err := s.accountRepo.UpdateBalance(ctx, tx, srcAcc.ID, newSrcBal, newSrcAvail, srcAcc.Version); err != nil {
		return nil, fmt.Errorf("failed to update source balance: %w", err)
	}
	if err := s.accountRepo.UpdateBalance(ctx, tx, destAcc.ID, newDestBal, newDestAvail, destAcc.Version); err != nil {
		return nil, fmt.Errorf("failed to update destination balance: %w", err)
	}

	// Construct Double-Entry Journal
	journalID := uuid.New()
	refNumber := fmt.Sprintf("TRF-%s-%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	now := time.Now().UTC()

	var idemKeyPtr *string
	if req.IdempotencyKey != "" {
		idemKeyPtr = &req.IdempotencyKey
	}

	lines := []domain.JournalLine{
		{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      srcAcc.ID,
			AccountNumber:  srcAcc.AccountNumber,
			Direction:      domain.DirectionDebit,
			Amount:         req.Amount,
			Currency:       req.Currency,
			BalanceAfter:   newSrcBal,
			Sequence:       1,
			Description:    fmt.Sprintf("Transfer to %s: %s", destAcc.AccountNumber, req.Description),
			CreatedAt:      now,
		},
		{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      destAcc.ID,
			AccountNumber:  destAcc.AccountNumber,
			Direction:      domain.DirectionCredit,
			Amount:         req.Amount,
			Currency:       req.Currency,
			BalanceAfter:   newDestBal,
			Sequence:       2,
			Description:    fmt.Sprintf("Transfer received from %s: %s", srcAcc.AccountNumber, req.Description),
			CreatedAt:      now,
		},
	}

	if err := domain.ValidateDoubleEntry(lines); err != nil {
		return nil, err
	}

	entry := &domain.JournalEntry{
		ID:              journalID,
		ReferenceNumber: refNumber,
		IdempotencyKey:  idemKeyPtr,
		TransactionType: domain.TxTypeTransferInternal,
		Description:     req.Description,
		Status:          domain.JournalStatusPosted,
		PostedAt:        now,
		CreatedBy:       req.CreatedBy,
		Lines:           lines,
		CreatedAt:       now,
	}

	entryQuery := `
		INSERT INTO journal_entries (id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.ExecContext(ctx, entryQuery, entry.ID, entry.ReferenceNumber, entry.IdempotencyKey, entry.TransactionType, entry.Description, entry.Status, entry.PostedAt, entry.CreatedBy, entry.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert journal entry: %w", err)
	}

	lineQuery := `
		INSERT INTO journal_lines (id, journal_entry_id, account_id, direction, amount, currency, balance_after, sequence, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, lineQuery, l.ID, l.JournalEntryID, l.AccountID, l.Direction, l.Amount, l.Currency, l.BalanceAfter, l.Sequence, l.Description, l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to insert journal line: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return entry, nil
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
