package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrLedgerUnbalanced        = errors.New("double-entry violation: sum of debits must equal sum of credits")
	ErrInsufficientFunds       = errors.New("insufficient available balance")
	ErrAccountInactive         = errors.New("account is not active")
	ErrInvalidAmount           = errors.New("transaction amount must be strictly positive")
	ErrDuplicateIdempotencyKey = errors.New("transaction already processed or in progress with this idempotency key")
)

type COAType string

const (
	COATypeAsset     COAType = "ASSET"
	COATypeLiability COAType = "LIABILITY"
	COATypeEquity    COAType = "EQUITY"
	COATypeRevenue   COAType = "REVENUE"
	COATypeExpense   COAType = "EXPENSE"
)

type BalanceType string

const (
	BalanceTypeDebit  BalanceType = "DEBIT"
	BalanceTypeCredit BalanceType = "CREDIT"
)

type ChartOfAccount struct {
	ID            uuid.UUID   `json:"id"`
	Code          string      `json:"code"`
	Name          string      `json:"name"`
	Type          COAType     `json:"type"`
	NormalBalance BalanceType `json:"normal_balance"`
	IsActive      bool        `json:"is_active"`
}

type TransactionType string

const (
	TxTypeDeposit          TransactionType = "DEPOSIT"
	TxTypeWithdrawal       TransactionType = "WITHDRAWAL"
	TxTypeTransferInternal TransactionType = "TRANSFER_INTERNAL"
	TxTypeFeeCharge        TransactionType = "FEE_CHARGE"
	TxTypeInterestAccrual  TransactionType = "INTEREST_ACCRUAL"
	TxTypeReversal         TransactionType = "REVERSAL"
	TxTypeAdjustment       TransactionType = "ADJUSTMENT"
)

type JournalStatus string

const (
	JournalStatusPosted          JournalStatus = "POSTED"
	JournalStatusReversed        JournalStatus = "REVERSED"
	JournalStatusFailed          JournalStatus = "FAILED"
	JournalStatusPendingApproval JournalStatus = "PENDING_APPROVAL"
)

type EntryDirection string

const (
	DirectionDebit  EntryDirection = "DEBIT"
	DirectionCredit EntryDirection = "CREDIT"
)

type JournalLine struct {
	ID             uuid.UUID       `json:"id"`
	JournalEntryID uuid.UUID       `json:"journal_entry_id"`
	AccountID      uuid.UUID       `json:"account_id"`
	AccountNumber  string          `json:"account_number,omitempty"`
	Direction      EntryDirection  `json:"direction"`
	Amount         decimal.Decimal `json:"amount"`
	Currency       string          `json:"currency"`
	BalanceAfter   decimal.Decimal `json:"balance_after"`
	Sequence       int             `json:"sequence"`
	Description    string          `json:"description"`
	CreatedAt      time.Time       `json:"created_at"`
}

type JournalEntry struct {
	ID              uuid.UUID       `json:"id"`
	ReferenceNumber string          `json:"reference_number"`
	IdempotencyKey  *string         `json:"idempotency_key,omitempty"`
	TransactionType TransactionType `json:"transaction_type"`
	Description     string          `json:"description"`
	Status          JournalStatus   `json:"status"`
	PostedAt        time.Time       `json:"posted_at"`
	CreatedBy       string          `json:"created_by"`
	Lines           []JournalLine   `json:"lines,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// ValidateDoubleEntry enforces fundamental accounting equation (Sum of Debits == Sum of Credits)
func ValidateDoubleEntry(lines []JournalLine) error {
	if len(lines) < 2 {
		return errors.New("a journal entry must contain at least 2 lines (debit and credit)")
	}

	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, line := range lines {
		if line.Amount.LessThanOrEqual(decimal.Zero) {
			return ErrInvalidAmount
		}
		if line.Direction == DirectionDebit {
			totalDebit = totalDebit.Add(line.Amount)
		} else if line.Direction == DirectionCredit {
			totalCredit = totalCredit.Add(line.Amount)
		} else {
			return errors.New("invalid entry direction")
		}
	}

	if !totalDebit.Equal(totalCredit) {
		return ErrLedgerUnbalanced
	}

	return nil
}

type PostTransactionRequest struct {
	ReferenceNumber string          `json:"reference_number"`
	IdempotencyKey  string          `json:"idempotency_key"`
	TransactionType TransactionType `json:"transaction_type"`
	Description     string          `json:"description"`
	CreatedBy       string          `json:"created_by"`
	Lines           []PostLineInput `json:"lines"`
}

type PostLineInput struct {
	AccountNumber string          `json:"account_number"`
	Direction     EntryDirection  `json:"direction"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Description   string          `json:"description"`
}

// Simplified DTOs for standard banking operations
type DepositRequest struct {
	AccountNumber  string          `json:"account_number"`
	Amount         decimal.Decimal `json:"amount"`
	Currency       string          `json:"currency"`
	Description    string          `json:"description"`
	IdempotencyKey string          `json:"idempotency_key"`
	CreatedBy      string          `json:"created_by"`
}

type WithdrawRequest struct {
	AccountNumber  string          `json:"account_number"`
	Amount         decimal.Decimal `json:"amount"`
	Currency       string          `json:"currency"`
	Description    string          `json:"description"`
	IdempotencyKey string          `json:"idempotency_key"`
	CreatedBy      string          `json:"created_by"`
}

type TransferRequest struct {
	SourceAccountNumber      string          `json:"source_account_number"`
	DestinationAccountNumber string          `json:"destination_account_number"`
	Amount                   decimal.Decimal `json:"amount"`
	Currency                 string          `json:"currency"`
	Description              string          `json:"description"`
	IdempotencyKey           string          `json:"idempotency_key"`
	CreatedBy                string          `json:"created_by"`
}

type CustomJournalLineInput struct {
	AccountNumber string          `json:"account_number"`
	Direction     EntryDirection  `json:"direction"`
	Amount        decimal.Decimal `json:"amount"`
	Description   string          `json:"description"`
}

type CustomJournalRequest struct {
	TransactionType TransactionType          `json:"transaction_type"`
	Description     string                   `json:"description"`
	IdempotencyKey  string                   `json:"idempotency_key"`
	CreatedBy       string                   `json:"created_by"`
	Lines           []CustomJournalLineInput `json:"lines"`
}

type LedgerRepository interface {
	GetCOAList(ctx context.Context) ([]ChartOfAccount, error)
	GetCOAByCode(ctx context.Context, code string) (*ChartOfAccount, error)
	PostJournal(ctx context.Context, entry *JournalEntry) error
	GetJournalByRef(ctx context.Context, ref string) (*JournalEntry, error)
	ListJournals(ctx context.Context, limit, offset int) ([]JournalEntry, int, error)
	ListAccountStatements(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]JournalLine, int, error)
}

type LedgerService interface {
	Deposit(ctx context.Context, req DepositRequest) (*JournalEntry, error)
	Withdraw(ctx context.Context, req WithdrawRequest) (*JournalEntry, error)
	TransferInternal(ctx context.Context, req TransferRequest) (*JournalEntry, error)
	PostCompoundJournal(ctx context.Context, req CustomJournalRequest) (*JournalEntry, error)
	GetJournalByReference(ctx context.Context, ref string) (*JournalEntry, error)
	ListJournals(ctx context.Context, page, pageSize int) ([]JournalEntry, int, error)
	GetAccountStatement(ctx context.Context, accountNumber string, page, pageSize int) ([]JournalLine, int, error)
	GetChartOfAccounts(ctx context.Context) ([]ChartOfAccount, error)
}
