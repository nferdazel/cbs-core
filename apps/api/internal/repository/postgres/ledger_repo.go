package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type LedgerRepository struct {
	db *sql.DB
}

func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

func (r *LedgerRepository) GetDB() *sql.DB {
	return r.db
}

func (r *LedgerRepository) GetCOAList(ctx context.Context) ([]domain.ChartOfAccount, error) {
	query := `SELECT id, code, name, type, normal_balance, is_active FROM chart_of_accounts ORDER BY code ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ChartOfAccount
	for rows.Next() {
		var coa domain.ChartOfAccount
		if err := rows.Scan(&coa.ID, &coa.Code, &coa.Name, &coa.Type, &coa.NormalBalance, &coa.IsActive); err != nil {
			return nil, err
		}
		list = append(list, coa)
	}
	return list, nil
}

func (r *LedgerRepository) GetCOAByCode(ctx context.Context, code string) (*domain.ChartOfAccount, error) {
	query := `SELECT id, code, name, type, normal_balance, is_active FROM chart_of_accounts WHERE code = $1`
	var coa domain.ChartOfAccount
	err := r.db.QueryRowContext(ctx, query, code).Scan(&coa.ID, &coa.Code, &coa.Name, &coa.Type, &coa.NormalBalance, &coa.IsActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("chart of account not found")
		}
		return nil, err
	}
	return &coa, nil
}

func (r *LedgerRepository) PostJournal(ctx context.Context, entry *domain.JournalEntry) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert Journal Entry Header
	entryQuery := `
		INSERT INTO journal_entries (id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = tx.ExecContext(ctx, entryQuery,
		entry.ID, entry.ReferenceNumber, entry.IdempotencyKey, entry.TransactionType, entry.Description, entry.Status, entry.PostedAt, entry.CreatedBy, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert journal entry: %w", err)
	}

	// 2. Insert Journal Lines
	lineQuery := `
		INSERT INTO journal_lines (id, journal_entry_id, account_id, direction, amount, currency, balance_after, sequence, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	for _, line := range entry.Lines {
		_, err = tx.ExecContext(ctx, lineQuery,
			line.ID, line.JournalEntryID, line.AccountID, line.Direction, line.Amount, line.Currency, line.BalanceAfter, line.Sequence, line.Description, line.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert journal line: %w", err)
		}
	}

	return tx.Commit()
}

func (r *LedgerRepository) GetJournalByRef(ctx context.Context, ref string) (*domain.JournalEntry, error) {
	entryQuery := `
		SELECT id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at
		FROM journal_entries WHERE reference_number = $1
	`
	var entry domain.JournalEntry
	err := r.db.QueryRowContext(ctx, entryQuery, ref).Scan(
		&entry.ID, &entry.ReferenceNumber, &entry.IdempotencyKey, &entry.TransactionType, &entry.Description, &entry.Status, &entry.PostedAt, &entry.CreatedBy, &entry.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("journal entry not found")
		}
		return nil, err
	}

	// Load lines
	linesQuery := `
		SELECT jl.id, jl.journal_entry_id, jl.account_id, a.account_number, jl.direction, jl.amount, jl.currency, jl.balance_after, jl.sequence, jl.description, jl.created_at
		FROM journal_lines jl
		JOIN accounts a ON jl.account_id = a.id
		WHERE jl.journal_entry_id = $1
		ORDER BY jl.sequence ASC
	`
	rows, err := r.db.QueryContext(ctx, linesQuery, entry.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var line domain.JournalLine
		if err := rows.Scan(
			&line.ID, &line.JournalEntryID, &line.AccountID, &line.AccountNumber, &line.Direction, &line.Amount, &line.Currency, &line.BalanceAfter, &line.Sequence, &line.Description, &line.CreatedAt,
		); err != nil {
			return nil, err
		}
		entry.Lines = append(entry.Lines, line)
	}

	return &entry, nil
}

func (r *LedgerRepository) ListJournals(ctx context.Context, limit, offset int) ([]domain.JournalEntry, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM journal_entries").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at
		FROM journal_entries
		ORDER BY posted_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.JournalEntry
	for rows.Next() {
		var entry domain.JournalEntry
		if err := rows.Scan(
			&entry.ID, &entry.ReferenceNumber, &entry.IdempotencyKey, &entry.TransactionType, &entry.Description, &entry.Status, &entry.PostedAt, &entry.CreatedBy, &entry.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, entry)
	}
	return list, total, nil
}

func (r *LedgerRepository) ListAccountStatements(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.JournalLine, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM journal_lines WHERE account_id = $1", accountID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT jl.id, jl.journal_entry_id, jl.account_id, a.account_number, jl.direction, jl.amount, jl.currency, jl.balance_after, jl.sequence, jl.description, jl.created_at
		FROM journal_lines jl
		JOIN accounts a ON jl.account_id = a.id
		WHERE jl.account_id = $1
		ORDER BY jl.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var lines []domain.JournalLine
	for rows.Next() {
		var line domain.JournalLine
		if err := rows.Scan(
			&line.ID, &line.JournalEntryID, &line.AccountID, &line.AccountNumber, &line.Direction, &line.Amount, &line.Currency, &line.BalanceAfter, &line.Sequence, &line.Description, &line.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		lines = append(lines, line)
	}
	return lines, total, nil
}
