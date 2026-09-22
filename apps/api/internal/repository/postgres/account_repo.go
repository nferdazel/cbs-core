package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	a.status, a.version, a.opened_at, a.last_activity_at, a.dormant_at,
	a.created_at, a.updated_at,
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

func (r *AccountRepository) ListAll(ctx context.Context, limit, offset int, search string, actor domain.Actor) ([]domain.Account, int, error) {
	// Hanya rekening nasabah. Daftar positif disengaja: akun GL internal adalah
	// akuntansi bank, bukan rekening yang dilayani teller, dan tipe baru yang tidak
	// dikenal tidak boleh otomatis ikut tampil.
	where := "a.account_type IN ('SAVINGS', 'CHECKING', 'LOAN')"
	clause, whereArgs := branchReadClause("a.branch_id", actor)
	if clause != "" {
		where += " AND " + clause
	}
	// Buku rekening mengikuti buku COA-nya (coa.book); query sudah mem-join coa.
	if bclause, bargs := bookReadClause("coa.book", actor, len(whereArgs)+1); bclause != "" {
		whereArgs = append(whereArgs, bargs...)
		where = andCondition(where, bclause)
	}
	if pattern := likePrefixPattern(search); pattern != "" {
		whereArgs = append(whereArgs, pattern)
		where = andCondition(where, fmt.Sprintf("a.account_number ILIKE $%d ESCAPE '\\'", len(whereArgs)))
	}

	countQuery := "SELECT COUNT(*) FROM accounts a JOIN chart_of_accounts coa ON a.coa_id = coa.id"
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

	// last_activity_at dicatat di UPDATE yang sama agar tidak menambah round-trip pada
	// jalur terpanas. Hanya rekening nasabah (customer_id terisi); akun GL internal
	// tidak dianggap punya aktivitas nasabah dan dibiarkan apa adanya.
	query := `
		UPDATE accounts
		SET balance = $1, available_balance = $2, version = version + 1, updated_at = NOW(),
		    last_activity_at = CASE WHEN customer_id IS NOT NULL THEN NOW() ELSE last_activity_at END
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

// ListDormantCandidates mengambil rekening nasabah yang masih ACTIVE. Cakupan
// sengaja hanya SAVINGS/CHECKING milik nasabah: akun GL internal adalah akuntansi
// bank dan rekening kredit bukan rekening transaksional nasabah. actor membatasi
// hasil pada buku yang aktif di instalasi agar batch dormant tidak menyentuh lini
// usaha yang tidak dilayani.
func (r *AccountRepository) ListDormantCandidates(ctx context.Context, actor domain.Actor) ([]domain.Account, error) {
	query := `SELECT ` + accountColumns + `
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE a.status = 'ACTIVE'
			AND a.customer_id IS NOT NULL
			AND a.account_type IN ('SAVINGS', 'CHECKING')`
	if clause, args := bookReadClause("coa.book", actor, 1); clause != "" {
		query += " AND " + clause
		return r.queryAccounts(ctx, query, args...)
	}
	return r.queryAccounts(ctx, query)
}

// MarkDormant menandai satu rekening dormant. Penjaga status = 'ACTIVE' membuat
// operasi idempoten: pemanggilan ulang tidak mengubah apa pun. Tidak ada jurnal
// karena penandaan dormant bukan peristiwa ekonomi dan tidak memindahkan uang.
func (r *AccountRepository) MarkDormant(ctx context.Context, tx any, accountID uuid.UUID) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("invalid transaction context")
	}

	query := `
		UPDATE accounts
		SET status = 'DORMANT', dormant_at = NOW(), updated_at = NOW(), version = version + 1
		WHERE id = $1 AND status = 'ACTIVE'
	`
	res, err := sqlTx.ExecContext(ctx, query, accountID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// Reactivate memulihkan rekening dormant. Reaktivasi dihitung sebagai aktivitas
// rekening, sehingga last_activity_at ikut diperbarui. Penjaga status = 'DORMANT'
// membuat hasil false bila status sudah berubah (mis. balapan dengan aksi lain).
func (r *AccountRepository) Reactivate(ctx context.Context, tx any, accountID uuid.UUID, reactivatedAt time.Time) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("invalid transaction context")
	}

	query := `
		UPDATE accounts
		SET status = 'ACTIVE', dormant_at = NULL, last_activity_at = $1,
		    updated_at = NOW(), version = version + 1
		WHERE id = $2 AND status = 'DORMANT'
	`
	res, err := sqlTx.ExecContext(ctx, query, reactivatedAt, accountID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (r *AccountRepository) queryAccounts(ctx context.Context, query string, args ...any) ([]domain.Account, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
		&a.Status, &a.Version, &a.OpenedAt, &a.LastActivityAt, &a.DormantAt,
		&a.CreatedAt, &a.UpdatedAt,
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
