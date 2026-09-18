package domain

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrAccountNotFound = errors.New("rekening tidak ditemukan")

type AccountType string

const (
	AccountTypeSavings    AccountType = "SAVINGS"
	AccountTypeChecking   AccountType = "CHECKING"
	AccountTypeLoan       AccountType = "LOAN"
	AccountTypeInternalGL AccountType = "INTERNAL_GL"
)

type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "ACTIVE"
	AccountStatusDormant AccountStatus = "DORMANT"
	AccountStatusFrozen  AccountStatus = "FROZEN"
	AccountStatusClosed  AccountStatus = "CLOSED"
)

type Account struct {
	ID               uuid.UUID       `json:"id"`
	AccountNumber    string          `json:"account_number"`
	CustomerID       *uuid.UUID      `json:"customer_id,omitempty"`
	CustomerName     string          `json:"customer_name,omitempty"`
	ProductID        *uuid.UUID      `json:"product_id,omitempty"`
	BranchID         *uuid.UUID      `json:"branch_id,omitempty"`
	COAID            uuid.UUID       `json:"coa_id"`
	COACode          string          `json:"coa_code,omitempty"`
	// COABook adalah buku COA rekening (konvensional/syariah). Dipakai untuk
	// menolak transaksi yang mencampur buku, mis. pembiayaan syariah yang
	// mencairkan dana ke rekening konvensional.
	COABook          COABook         `json:"coa_book,omitempty"`
	NormalBalance    BalanceType     `json:"normal_balance"`
	AccountType      AccountType     `json:"account_type"`
	Currency         string          `json:"currency"`
	Balance          decimal.Decimal `json:"balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	HoldBalance      decimal.Decimal `json:"hold_balance"`
	Status           AccountStatus   `json:"status"`
	Version          int             `json:"version"`
	OpenedAt         *time.Time      `json:"opened_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// OpenAccountInput membuka rekening dari produk. COA kewajiban ditentukan produk,
// bukan dipilih teller, supaya pemetaan akuntansi konsisten.
type OpenAccountInput struct {
	CustomerID uuid.UUID `json:"customer_id"`
	ProductID  uuid.UUID `json:"product_id"`
	Currency   string    `json:"currency"`
	BranchCode string    `json:"branch_code"`
}

type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	CreateTx(ctx context.Context, tx *sql.Tx, account *Account) error
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	GetByNumber(ctx context.Context, accountNumber string) (*Account, error)
	GetByNumberForUpdate(ctx context.Context, tx any, accountNumber string) (*Account, error)
	ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]Account, error)
	ListAll(ctx context.Context, limit, offset int) ([]Account, int, error)
	UpdateBalance(ctx context.Context, tx any, accountID uuid.UUID, balance, available decimal.Decimal, version int) error
}

type AccountService interface {
	OpenAccount(ctx context.Context, input OpenAccountInput, actor Actor) (*Account, error)
	GetAccountByNumber(ctx context.Context, accountNumber string) (*Account, error)
	ListAccounts(ctx context.Context, page, pageSize int) ([]Account, int, error)
}
