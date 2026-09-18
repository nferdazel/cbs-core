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

// ResolveGLAccount menerjemahkan kode COA menjadi nomor akun kontrol GL yang bisa
// diposting. Konvensinya: akun internal dengan account_number sama dengan kode COA.
// Akun kontrol inilah yang menampung agregat saldo seluruh rekening anak.
// ResolveGLAccount menerjemahkan kode COA menjadi nomor akun GL internal yang bisa
// diposting. Pencarian lewat relasi coa_id, bukan dengan menyamakan nomor akun dan
// kode COA: sebagian akun GL memakai nomor yang berbeda dari kode COA-nya
// (mis. GL-VAULT-001 untuk COA 10100). Operasi ini hanya membaca, jadi tidak
// memerlukan transaksi; tx dipertahankan pada signature agar pemanggil di dalam
// transaksi tetap mendapat snapshot yang sama.
func (r *LedgerRepository) ResolveGLAccount(ctx context.Context, tx any, coaCode string) (string, error) {
	query := `
		SELECT a.account_number
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE coa.code = $1 AND a.account_type = 'INTERNAL_GL'
		ORDER BY a.account_number
		LIMIT 1`

	var accountNumber string
	var err error
	if sqlTx, ok := tx.(*sql.Tx); ok {
		err = sqlTx.QueryRowContext(ctx, query, coaCode).Scan(&accountNumber)
	} else {
		err = r.db.QueryRowContext(ctx, query, coaCode).Scan(&accountNumber)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("akun kontrol GL untuk COA %s belum ada", coaCode)
		}
		return "", err
	}
	return accountNumber, nil
}

// InsertJournal menyimpan header dan seluruh baris jurnal di dalam transaksi yang
// disediakan pemanggil. Posting engine bertanggung jawab atas transaksinya.
func (r *LedgerRepository) InsertJournal(ctx context.Context, tx any, entry *domain.JournalEntry) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("insert journal: transaksi tidak valid")
	}

	entryQuery := `
		INSERT INTO journal_entries (id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := sqlTx.ExecContext(ctx, entryQuery,
		entry.ID, entry.ReferenceNumber, entry.IdempotencyKey, entry.TransactionType, entry.Description, entry.Status, entry.PostedAt, entry.CreatedBy, entry.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert journal entry: %w", err)
	}

	lineQuery := `
		INSERT INTO journal_lines (id, journal_entry_id, account_id, direction, amount, currency, balance_after, sequence, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	for _, line := range entry.Lines {
		if _, err := sqlTx.ExecContext(ctx, lineQuery,
			line.ID, line.JournalEntryID, line.AccountID, line.Direction, line.Amount, line.Currency, line.BalanceAfter, line.Sequence, line.Description, line.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert journal line: %w", err)
		}
	}
	return nil
}

// FindJournalByIdempotencyKey mengembalikan jurnal yang sudah diposting untuk kunci
// idempotency tertentu, atau nil bila belum ada.
func (r *LedgerRepository) FindJournalByIdempotencyKey(ctx context.Context, key string) (*domain.JournalEntry, error) {
	query := `
		SELECT id, reference_number, idempotency_key, transaction_type, description, status, posted_at, created_by, created_at
		FROM journal_entries WHERE idempotency_key = $1
	`
	var entry domain.JournalEntry
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&entry.ID, &entry.ReferenceNumber, &entry.IdempotencyKey, &entry.TransactionType, &entry.Description, &entry.Status, &entry.PostedAt, &entry.CreatedBy, &entry.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	lines, err := r.loadLines(ctx, entry.ID)
	if err != nil {
		return nil, err
	}
	entry.Lines = lines
	return &entry, nil
}

// loadLines memuat baris jurnal beserta nomor akunnya.
func (r *LedgerRepository) loadLines(ctx context.Context, journalID uuid.UUID) ([]domain.JournalLine, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jl.id, jl.journal_entry_id, jl.account_id, a.account_number, jl.direction, jl.amount, jl.currency, jl.balance_after, jl.sequence, jl.description, jl.created_at
		FROM journal_lines jl
		JOIN accounts a ON jl.account_id = a.id
		WHERE jl.journal_entry_id = $1
		ORDER BY jl.sequence ASC`, journalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []domain.JournalLine
	for rows.Next() {
		var line domain.JournalLine
		if err := rows.Scan(
			&line.ID, &line.JournalEntryID, &line.AccountID, &line.AccountNumber, &line.Direction, &line.Amount, &line.Currency, &line.BalanceAfter, &line.Sequence, &line.Description, &line.CreatedAt,
		); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, nil
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
