package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	COAID            uuid.UUID       `json:"coa_id"`
	COACode          string          `json:"coa_code,omitempty"`
	AccountType      AccountType     `json:"account_type"`
	Currency         string          `json:"currency"`
	Balance          decimal.Decimal `json:"balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	HoldBalance      decimal.Decimal `json:"hold_balance"`
	Status           AccountStatus   `json:"status"`
	Version          int             `json:"version"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type OpenAccountInput struct {
	CustomerID  uuid.UUID   `json:"customer_id"`
	AccountType AccountType `json:"account_type"`
	Currency    string      `json:"currency"`
	COAID       uuid.UUID   `json:"coa_id"`
}

type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	GetByNumber(ctx context.Context, accountNumber string) (*Account, error)
	GetByNumberForUpdate(ctx context.Context, tx any, accountNumber string) (*Account, error)
	ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]Account, error)
	ListAll(ctx context.Context, limit, offset int) ([]Account, int, error)
	UpdateBalance(ctx context.Context, tx any, accountID uuid.UUID, balance, available decimal.Decimal, version int) error
}

type AccountService interface {
	OpenAccount(ctx context.Context, input OpenAccountInput) (*Account, error)
	GetAccountByNumber(ctx context.Context, accountNumber string) (*Account, error)
	ListAccounts(ctx context.Context, page, pageSize int) ([]Account, int, error)
}
