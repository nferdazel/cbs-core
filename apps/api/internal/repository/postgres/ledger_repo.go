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

// GetCOAList mengembalikan bagan akun yang boleh dibaca aktor. Penyaringan memakai
// bookReadClause yang sama dengan pembacaan akun/jurnal, sehingga aktor terikat satu
// buku tidak melihat bagan akun lini usaha lain (aktor lintas buku tidak difilter).
func (r *LedgerRepository) GetCOAList(ctx context.Context, actor domain.Actor) ([]domain.ChartOfAccount, error) {
	query := `SELECT coa.id, coa.code, coa.name, coa.type, coa.normal_balance, coa.is_active
		FROM chart_of_accounts coa`
	var args []any
	if clause, bookArgs := bookReadClause("coa.book", actor, 1); clause != "" {
		query += " WHERE " + clause
		args = append(args, bookArgs...)
	}
	query += " ORDER BY coa.code ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

	// Cabang diisi lewat subquery di INSERT yang sama agar tidak menambah
	// round-trip di jalur terpanas. Bila BranchCode kosong, subquery menghasilkan
	// NULL — jurnal sistem/batch yang bank-wide memang boleh tanpa cabang.
	entryQuery := `
		INSERT INTO journal_entries (id, reference_number, idempotency_key, transaction_type, description, status, posted_at, entry_date, created_by, created_at, branch_id, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, (SELECT id FROM branches WHERE code = $11), $12)
	`
	if _, err := sqlTx.ExecContext(ctx, entryQuery,
		entry.ID, entry.ReferenceNumber, entry.IdempotencyKey, entry.TransactionType, entry.Description, entry.Status, entry.PostedAt, entry.EntryDate, entry.CreatedBy, entry.CreatedAt, entry.BranchCode, entry.Source,
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
	defer func() { _ = rows.Close() }()

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
	// branch_id diambil lewat join: pemeriksaan cabang di handler bergantung padanya,
	// dan tanpa kolom ini nilai BranchCode kosong membuat pemeriksaan lolos diam-diam.
	entryQuery := `
		SELECT je.id, je.reference_number, je.idempotency_key, je.transaction_type, je.description, je.status, je.posted_at, je.entry_date, je.created_by, je.created_at, je.source,
		       COALESCE(b.code, ''),
		       COALESCE(` + journalBookSubquery + `, ''),
		       ` + journalBookMixedSubquery + `
		FROM journal_entries je
		LEFT JOIN branches b ON b.id = je.branch_id
		WHERE je.reference_number = $1
	`
	var entry domain.JournalEntry
	var book string
	var bookCount int
	err := r.db.QueryRowContext(ctx, entryQuery, ref).Scan(
		&entry.ID, &entry.ReferenceNumber, &entry.IdempotencyKey, &entry.TransactionType, &entry.Description, &entry.Status, &entry.PostedAt, &entry.EntryDate, &entry.CreatedBy, &entry.CreatedAt, &entry.Source,
		&entry.BranchCode,
		&book,
		&bookCount,
	)
	if err == nil {
		entry.Book = domain.COABook(book)
		entry.BookMixed = bookCount > 1
	}
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
	defer func() { _ = rows.Close() }()

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

// MarkJournalReversed menandai jurnal asal sebagai REVERSED. Klausa status = 'POSTED'
// membuat dua pembatalan yang datang bersamaan hanya menghasilkan satu perubahan:
// yang kalah menerima ErrJournalAlreadyReversed, bukan diam-diam menimpa.
func (r *LedgerRepository) MarkJournalReversed(ctx context.Context, tx any, reference string) error {
	var exec execer = r.db
	if tx != nil {
		sqlTx, ok := tx.(*sql.Tx)
		if !ok {
			return errors.New("konteks transaksi pembatalan tidak valid")
		}
		exec = sqlTx
	}

	res, err := exec.ExecContext(ctx,
		`UPDATE journal_entries SET status = 'REVERSED' WHERE reference_number = $1 AND status = 'POSTED'`,
		reference)
	if err != nil {
		return fmt.Errorf("menandai jurnal %s dibatalkan: %w", reference, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrJournalAlreadyReversed
	}
	return nil
}

// journalBookSubquery menurunkan buku COA sebuah jurnal dari akun baris pertamanya:
// journal_lines -> accounts -> chart_of_accounts. Baris tanpa buku dilewati sehingga
// jurnal dengan sebagian akun ber-buku tetap dinilai dari akun ber-buku pertamanya
// (urutan sequence deterministik). NULL berarti buku jurnal tidak dapat ditentukan dan
// oleh bookReadClause tetap disertakan, mengikuti semantik Actor.CanAccessBook untuk
// data lama. Dipakai bersama oleh daftar jurnal dan pembacaan satu jurnal.
const journalBookSubquery = `(
		SELECT c.book::text
		FROM journal_lines jl
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts c ON c.id = a.coa_id
		WHERE jl.journal_entry_id = je.id AND c.book IS NOT NULL
		ORDER BY jl.sequence, jl.id
		LIMIT 1
	)`

// journalBookMixedSubquery menghitung jumlah buku COA berbeda yang muncul pada baris
// sebuah jurnal. Lebih dari satu berarti jurnal lintas buku (lihat Actor.CanAccessJournal).
const journalBookMixedSubquery = `(
		SELECT COUNT(DISTINCT c.book)
		FROM journal_lines jl
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts c ON c.id = a.coa_id
		WHERE jl.journal_entry_id = je.id AND c.book IS NOT NULL
	)`

// buildJournalListQuery menyusun query daftar jurnal beserta klausa filter cabang
// dan buku serta argumennya. Klausa yang sama dipakai untuk COUNT(*) dan SELECT agar
// total_items cocok dengan halaman yang dikembalikan. Dipisah sebagai fungsi murni
// supaya penentuan scope dapat diuji tanpa database; SQL akhirnya sendiri tetap hanya
// terverifikasi saat runtime.
func buildJournalListQuery(actor domain.Actor) (countQuery, listQuery string, whereArgs []any) {
	where, whereArgs := branchReadClause("je.branch_id", actor)
	// Buku jurnal berasal dari gabungan barisnya, bukan satu kolom; klausa khusus
	// (journalBookFilter) mengecualikan jurnal yang menyentuh buku lain.
	if clause, args := journalBookFilter("je", actor, len(whereArgs)+1); clause != "" {
		whereArgs = append(whereArgs, args...)
		where = andCondition(where, clause)
	}

	countQuery = "SELECT COUNT(*) FROM journal_entries je"
	listQuery = `
		SELECT je.id, je.reference_number, je.idempotency_key, je.transaction_type, je.description, je.status, je.posted_at, je.entry_date, je.created_by, je.created_at, je.source,
		       COALESCE(b.code, '')
		FROM journal_entries je
		LEFT JOIN branches b ON b.id = je.branch_id`
	if where != "" {
		countQuery += " WHERE " + where
		listQuery += " WHERE " + where
	}
	listQuery += fmt.Sprintf(" ORDER BY je.posted_at DESC LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	return countQuery, listQuery, whereArgs
}

func (r *LedgerRepository) ListJournals(ctx context.Context, limit, offset int, actor domain.Actor) ([]domain.JournalEntry, int, error) {
	countQuery, listQuery, whereArgs := buildJournalListQuery(actor)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args := append(append([]any{}, whereArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.JournalEntry
	for rows.Next() {
		var entry domain.JournalEntry
		if err := rows.Scan(
			&entry.ID, &entry.ReferenceNumber, &entry.IdempotencyKey, &entry.TransactionType, &entry.Description, &entry.Status, &entry.PostedAt, &entry.EntryDate, &entry.CreatedBy, &entry.CreatedAt, &entry.Source,
			&entry.BranchCode,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, entry)
	}
	return list, total, nil
}

// buildAccountStatementQuery menyusun query mutasi rekening beserta filter cabang
// pada cabang rekeningnya. accountID diletakkan setelah argumen cabang agar
// klausa bersama branchReadClause (yang memakai $1) tetap dapat dipakai apa
// adanya. Posisi placeholder dihitung dari jumlah argumen filter, bukan ditulis
// tetap, supaya pagination benar untuk aktor lintas cabang maupun cabang.
func buildAccountStatementQuery(actor domain.Actor, accountID uuid.UUID) (countQuery, listQuery string, whereArgs []any) {
	where, whereArgs := branchReadClause("a.branch_id", actor)

	accountArg := len(whereArgs) + 1
	countQuery = fmt.Sprintf("SELECT COUNT(*) FROM journal_lines jl JOIN accounts a ON a.id = jl.account_id WHERE jl.account_id = $%d", accountArg)
	listQuery = fmt.Sprintf(`
		SELECT jl.id, jl.journal_entry_id, jl.account_id, a.account_number, jl.direction, jl.amount, jl.currency, jl.balance_after, jl.sequence, jl.description, jl.created_at
		FROM journal_lines jl
		JOIN accounts a ON jl.account_id = a.id
		WHERE jl.account_id = $%d`, accountArg)
	if where != "" {
		countQuery += " AND " + where
		listQuery += " AND " + where
	}
	listQuery += fmt.Sprintf(" ORDER BY jl.created_at DESC LIMIT $%d OFFSET $%d", accountArg+1, accountArg+2)

	whereArgs = append(whereArgs, accountID)
	return countQuery, listQuery, whereArgs
}

func (r *LedgerRepository) ListAccountStatements(ctx context.Context, accountID uuid.UUID, limit, offset int, actor domain.Actor) ([]domain.JournalLine, int, error) {
	countQuery, listQuery, whereArgs := buildAccountStatementQuery(actor, accountID)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args := append(append([]any{}, whereArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

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
