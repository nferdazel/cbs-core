package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// YearEndRepository menyediakan saldo akun nominal dari jurnal untuk penutupan tahun
// buku dan penanda idempotensi tutup buku. Terpisah dari repo laporan agar perubahan
// di sini tidak menyentuh pekerjaan laporan.
type YearEndRepository struct {
	db *sql.DB
}

func NewYearEndRepository(db *sql.DB) *YearEndRepository {
	return &YearEndRepository{db: db}
}

// NominalBalances mengagregasi mutasi akun REVENUE/EXPENSE satu buku pada periode
// fiskal langsung dari journal_lines (sumber kebenaran), bukan accounts.balance.
// Nominal disajikan menurut sifat alami akun agar pendapatan dan beban positif.
func (r *YearEndRepository) NominalBalances(ctx context.Context, from, to time.Time, book domain.COABook) ([]domain.NominalAccountBalance, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.code, c.name, c.type::text, c.book::text,
		       COALESCE(SUM(CASE WHEN jl.direction = 'DEBIT'  THEN jl.amount ELSE 0 END), 0) AS total_debit,
		       COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0) AS total_credit
		FROM chart_of_accounts c
		JOIN accounts a ON a.coa_id = c.id
		JOIN journal_lines jl ON jl.account_id = a.id
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE c.is_header = FALSE
		  AND c.type IN ('REVENUE', 'EXPENSE')
		  AND je.entry_date >= $1::date AND je.entry_date <= $2::date
		  AND c.book = $3
		GROUP BY c.code, c.name, c.type, c.book
		ORDER BY c.code`, from, to, book)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.NominalAccountBalance
	for rows.Next() {
		var b domain.NominalAccountBalance
		var totalDebit, totalCredit decimal.Decimal
		if err := rows.Scan(&b.COACode, &b.COAName, &b.AccountType, &b.Book, &totalDebit, &totalCredit); err != nil {
			return nil, err
		}
		b.Amount = domain.ClosingBalance(domain.NatureOf(b.AccountType), decimal.Zero, totalDebit, totalCredit)
		if b.Amount.IsZero() {
			continue
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (r *YearEndRepository) GetYearEndClosing(ctx context.Context, fiscalYear int, book domain.COABook) (*domain.YearEndClosingRecord, error) {
	var rec domain.YearEndClosingRecord
	var journalReference sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT y.id, y.fiscal_year, y.book::text, y.total_revenue, y.total_expense, y.net_income,
		       y.retained_earnings_coa, y.journal_entry_id, je.reference_number
		FROM year_end_closings y
		LEFT JOIN journal_entries je ON je.id = y.journal_entry_id
		WHERE y.fiscal_year = $1 AND y.book = $2`, fiscalYear, book).Scan(
		&rec.ID, &rec.FiscalYear, &rec.Book, &rec.TotalRevenue, &rec.TotalExpense, &rec.NetIncome,
		&rec.RetainedEarningsCOA, &rec.JournalEntryID, &journalReference,
	)
	rec.JournalReference = journalReference.String
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *YearEndRepository) InsertYearEndClosing(ctx context.Context, tx any, rec *domain.YearEndClosingRecord) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("transaksi tidak valid")
	}
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	var journalID any
	if rec.JournalEntryID != nil {
		journalID = *rec.JournalEntryID
	}
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO year_end_closings (
			id, fiscal_year, book, total_revenue, total_expense, net_income,
			retained_earnings_coa, journal_entry_id, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (fiscal_year, book) DO NOTHING`,
		rec.ID, rec.FiscalYear, rec.Book, rec.TotalRevenue, rec.TotalExpense, rec.NetIncome,
		rec.RetainedEarningsCOA, journalID,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *YearEndRepository) UpdateYearEndClosingJournal(ctx context.Context, tx any, fiscalYear int, book domain.COABook, journalID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE year_end_closings SET journal_entry_id = $1
		WHERE fiscal_year = $2 AND book = $3`, journalID, fiscalYear, book)
	return err
}

var _ domain.YearEndRepository = (*YearEndRepository)(nil)
