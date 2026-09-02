package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) Create(ctx context.Context, a *domain.Account) error {
	query := `
		INSERT INTO accounts (id, account_number, customer_id, coa_id, account_type, currency, balance, available_balance, hold_balance, status, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(ctx, query,
		a.ID, a.AccountNumber, a.CustomerID, a.COAID, a.AccountType, a.Currency, a.Balance, a.AvailableBalance, a.HoldBalance, a.Status, a.Version, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}
	return nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	query := `
		SELECT a.id, a.account_number, a.customer_id, COALESCE(c.full_name, 'INTERNAL'), a.coa_id, coa.code, a.account_type, a.currency, a.balance, a.available_balance, a.hold_balance, a.status, a.version, a.created_at, a.updated_at
		FROM accounts a
		LEFT JOIN customers c ON a.customer_id = c.id
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.id = $1
	`
	var a domain.Account
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.AccountNumber, &a.CustomerID, &a.CustomerName, &a.COAID, &a.COACode, &a.AccountType, &a.Currency, &a.Balance, &a.AvailableBalance, &a.HoldBalance, &a.Status, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("account not found")
		}
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepository) GetByNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	query := `
		SELECT a.id, a.account_number, a.customer_id, COALESCE(c.full_name, 'INTERNAL'), a.coa_id, coa.code, a.account_type, a.currency, a.balance, a.available_balance, a.hold_balance, a.status, a.version, a.created_at, a.updated_at
		FROM accounts a
		LEFT JOIN customers c ON a.customer_id = c.id
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.account_number = $1
	`
	var a domain.Account
	err := r.db.QueryRowContext(ctx, query, accountNumber).Scan(
		&a.ID, &a.AccountNumber, &a.CustomerID, &a.CustomerName, &a.COAID, &a.COACode, &a.AccountType, &a.Currency, &a.Balance, &a.AvailableBalance, &a.HoldBalance, &a.Status, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("account not found")
		}
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepository) GetByNumberForUpdate(ctx context.Context, tx any, accountNumber string) (*domain.Account, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("invalid transaction context")
	}

	query := `
		SELECT a.id, a.account_number, a.customer_id, COALESCE(c.full_name, 'INTERNAL'), a.coa_id, coa.code, a.account_type, a.currency, a.balance, a.available_balance, a.hold_balance, a.status, a.version, a.created_at, a.updated_at
		FROM accounts a
		LEFT JOIN customers c ON a.customer_id = c.id
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.account_number = $1
		FOR UPDATE
	`
	var a domain.Account
	err := sqlTx.QueryRowContext(ctx, query, accountNumber).Scan(
		&a.ID, &a.AccountNumber, &a.CustomerID, &a.CustomerName, &a.COAID, &a.COACode, &a.AccountType, &a.Currency, &a.Balance, &a.AvailableBalance, &a.HoldBalance, &a.Status, &a.Version, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("account not found")
		}
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepository) ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]domain.Account, error) {
	query := `
		SELECT a.id, a.account_number, a.customer_id, COALESCE(c.full_name, 'INTERNAL'), a.coa_id, coa.code, a.account_type, a.currency, a.balance, a.available_balance, a.hold_balance, a.status, a.version, a.created_at, a.updated_at
		FROM accounts a
		LEFT JOIN customers c ON a.customer_id = c.id
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.customer_id = $1
		ORDER BY a.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Account
	for rows.Next() {
		var a domain.Account
		if err := rows.Scan(
			&a.ID, &a.AccountNumber, &a.CustomerID, &a.CustomerName, &a.COAID, &a.COACode, &a.AccountType, &a.Currency, &a.Balance, &a.AvailableBalance, &a.HoldBalance, &a.Status, &a.Version, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

func (r *AccountRepository) ListAll(ctx context.Context, limit, offset int) ([]domain.Account, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT a.id, a.account_number, a.customer_id, COALESCE(c.full_name, 'INTERNAL'), a.coa_id, coa.code, a.account_type, a.currency, a.balance, a.available_balance, a.hold_balance, a.status, a.version, a.created_at, a.updated_at
		FROM accounts a
		LEFT JOIN customers c ON a.customer_id = c.id
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		ORDER BY a.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.Account
	for rows.Next() {
		var a domain.Account
		if err := rows.Scan(
			&a.ID, &a.AccountNumber, &a.CustomerID, &a.CustomerName, &a.COAID, &a.COACode, &a.AccountType, &a.Currency, &a.Balance, &a.AvailableBalance, &a.HoldBalance, &a.Status, &a.Version, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, nil
}

func (r *AccountRepository) UpdateBalance(ctx context.Context, tx any, accountID uuid.UUID, balance, available decimal.Decimal, version int) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("invalid transaction context")
	}

	query := `
		UPDATE accounts
		SET balance = $1, available_balance = $2, version = version + 1, updated_at = NOW()
		WHERE id = $3 AND version = $4
	`
	res, err := sqlTx.ExecContext(ctx, query, balance, available, accountID, version)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("optimistic lock conflict: account was modified concurrently")
	}
	return nil
}
