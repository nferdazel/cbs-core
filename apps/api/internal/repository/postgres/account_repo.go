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

// accountColumns tidak mengambil nama nasabah: nama tersimpan terenkripsi dan hanya
// service yang boleh membukanya. CustomerName diisi oleh service, bukan oleh SQL.
const accountColumns = `a.id, a.account_number, a.customer_id, NULL,
	a.product_id, a.branch_id, a.coa_id, coa.code, coa.book::text, coa.normal_balance,
	a.account_type, a.currency, a.balance, a.available_balance, a.hold_balance,
	a.status, a.version, a.opened_at, a.created_at, a.updated_at,
	COALESCE((SELECT b.code FROM branches b WHERE b.id = a.branch_id), '')`

func (r *AccountRepository) Create(ctx context.Context, a *domain.Account) error {
	return r.executeCreate(ctx, r.db, a)
}

func (r *AccountRepository) CreateTx(ctx context.Context, tx *sql.Tx, a *domain.Account) error {
	return r.executeCreate(ctx, tx, a)
}

func (r *AccountRepository) executeCreate(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, a *domain.Account) error {
	query := `
		INSERT INTO accounts (
			id, account_number, customer_id, product_id, branch_id, coa_id,
			account_type, currency, balance, available_balance, hold_balance,
			status, version, opened_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := exec.ExecContext(ctx, query,
		a.ID, a.AccountNumber, a.CustomerID, a.ProductID, a.BranchID, a.COAID,
		a.AccountType, a.Currency, a.Balance, a.AvailableBalance, a.HoldBalance,
		a.Status, a.Version, a.OpenedAt, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}
	return nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	query := `SELECT ` + accountColumns + `
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.id = $1`
	return scanAccount(r.db.QueryRowContext(ctx, query, id))
}

func (r *AccountRepository) GetByNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	query := `SELECT ` + accountColumns + `
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.account_number = $1`
	return scanAccount(r.db.QueryRowContext(ctx, query, accountNumber))
}

func (r *AccountRepository) GetByNumberForUpdate(ctx context.Context, tx any, accountNumber string) (*domain.Account, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("invalid transaction context")
	}

	query := `SELECT ` + accountColumns + `
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.account_number = $1
		FOR UPDATE OF a`
	return scanAccount(sqlTx.QueryRowContext(ctx, query, accountNumber))
}

func (r *AccountRepository) ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]domain.Account, error) {
	query := `SELECT ` + accountColumns + `
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.customer_id = $1
		ORDER BY a.created_at DESC`
	return r.queryAccounts(ctx, query, customerID)
}

func (r *AccountRepository) ListAll(ctx context.Context, limit, offset int, actor domain.Actor) ([]domain.Account, int, error) {
	where, whereArgs := branchReadClause("a.branch_id", actor)

	countQuery := "SELECT COUNT(*) FROM accounts a"
	if where != "" {
		countQuery += " WHERE " + where
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + accountColumns + `
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id`
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	args := append(append([]any{}, whereArgs...), limit, offset)
	list, err := r.queryAccounts(ctx, query, args...)
	if err != nil {
		return nil, 0, err
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

func (r *AccountRepository) queryAccounts(ctx context.Context, query string, args ...any) ([]domain.Account, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *a)
	}
	return list, rows.Err()
}

func scanAccount(row rowScanner) (*domain.Account, error) {
	var a domain.Account
	var customerName sql.NullString
	err := row.Scan(
		&a.ID, &a.AccountNumber, &a.CustomerID, &customerName,
		&a.ProductID, &a.BranchID, &a.COAID, &a.COACode, &a.COABook, &a.NormalBalance,
		&a.AccountType, &a.Currency, &a.Balance, &a.AvailableBalance, &a.HoldBalance,
		&a.Status, &a.Version, &a.OpenedAt, &a.CreatedAt, &a.UpdatedAt,
		&a.BranchCode,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}
	a.CustomerName = customerName.String
	return &a, nil
}
