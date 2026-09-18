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

// SavingsInterestRepository membaca rekening simpanan, merekonstruksi saldo harian
// dari jurnal, dan menyimpan penanda idempotensi akrual/biaya administrasi.
type SavingsInterestRepository struct {
	db *sql.DB
}

func NewSavingsInterestRepository(db *sql.DB) *SavingsInterestRepository {
	return &SavingsInterestRepository{db: db}
}

// savingsAccountColumns memakai cast ::text untuk kolom enum kustom agar bisa dipindai
// ke string. Status tidak difilter di SQL supaya rekening dorman/tutup masih bisa
// dilaporkan sebagai dilewati, bukan hilang tanpa jejak.
const savingsAccountColumns = `a.id, a.account_number, coa.code, p.id, p.code,
	p.family::text, p.book::text, p.profit_scheme::text, p.rate_annual,
	p.profit_sharing_ratio, p.admin_fee, a.status::text, a.currency,
	a.balance, a.available_balance`

const savingsAccountFrom = `
	FROM accounts a
	JOIN banking_products p ON p.id = a.product_id
	JOIN chart_of_accounts coa ON coa.id = a.coa_id
	WHERE a.account_type IN ('SAVINGS', 'CHECKING')
	  AND p.family IN ('SAVINGS', 'CURRENT_ACCOUNT')`

func scanSavingsAccount(row interface{ Scan(...any) error }) (*domain.SavingsAccountInfo, error) {
	var info domain.SavingsAccountInfo
	err := row.Scan(
		&info.AccountID, &info.AccountNumber, &info.COACode, &info.ProductID, &info.ProductCode,
		&info.Family, &info.Book, &info.ProfitScheme, &info.RateAnnual,
		&info.ProfitSharingRatio, &info.AdminFee, &info.Status, &info.Currency,
		&info.Balance, &info.AvailableBalance,
	)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *SavingsInterestRepository) ListSavingsAccounts(ctx context.Context) ([]domain.SavingsAccountInfo, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+savingsAccountColumns+savingsAccountFrom+` ORDER BY a.account_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.SavingsAccountInfo
	for rows.Next() {
		info, err := scanSavingsAccount(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *info)
	}
	return list, rows.Err()
}

func (r *SavingsInterestRepository) GetSavingsAccount(ctx context.Context, accountNumber string) (*domain.SavingsAccountInfo, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+savingsAccountColumns+savingsAccountFrom+` AND a.account_number = $1`, accountNumber)
	info, err := scanSavingsAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	return info, err
}

// OpeningBalances menjumlahkan mutasi sebelum `before` untuk setiap rekening simpanan.
// Saldo kewajiban bertambah saat KREDIT dan berkurang saat DEBIT.
func (r *SavingsInterestRepository) OpeningBalances(ctx context.Context, before time.Time) (map[uuid.UUID]decimal.Decimal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jl.account_id,
		       COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE -jl.amount END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		JOIN accounts a ON a.id = jl.account_id
		WHERE je.entry_date < $1::date
		  AND a.account_type IN ('SAVINGS', 'CHECKING')
		GROUP BY jl.account_id`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := make(map[uuid.UUID]decimal.Decimal)
	for rows.Next() {
		var id uuid.UUID
		var bal decimal.Decimal
		if err := rows.Scan(&id, &bal); err != nil {
			return nil, err
		}
		balances[id] = bal
	}
	return balances, rows.Err()
}

// DailyNetChanges mengembalikan perubahan saldo per rekening per tanggal entri dalam
// rentang inklusif. Rekening tanpa mutasi tidak muncul.
func (r *SavingsInterestRepository) DailyNetChanges(ctx context.Context, from, to time.Time) ([]domain.DailyNetChange, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jl.account_id, je.entry_date,
		       COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE -jl.amount END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		JOIN accounts a ON a.id = jl.account_id
		WHERE je.entry_date >= $1::date AND je.entry_date <= $2::date
		  AND a.account_type IN ('SAVINGS', 'CHECKING')
		GROUP BY jl.account_id, je.entry_date
		ORDER BY jl.account_id, je.entry_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []domain.DailyNetChange
	for rows.Next() {
		var c domain.DailyNetChange
		if err := rows.Scan(&c.AccountID, &c.Date, &c.Delta); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

// InsertInterestAccrual menyimpan penanda akrual. ON CONFLICT DO NOTHING membuat
// pemanggil ganda tidak memposting jurnal kedua; nilai balik false berarti sudah ada.
func (r *SavingsInterestRepository) InsertInterestAccrual(ctx context.Context, tx any, rec *domain.InterestAccrualRecord) (bool, error) {
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
		INSERT INTO interest_accruals (
			id, account_id, account_number, period, book, profit_scheme,
			amount, average_balance, expense_coa, payable_coa, journal_entry_id, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
		ON CONFLICT (account_id, period) DO NOTHING`,
		rec.ID, rec.AccountID, rec.AccountNumber, rec.Period, rec.Book, rec.ProfitScheme,
		rec.Amount, rec.AverageBalance, rec.ExpenseCOACode, rec.PayableCOACode, journalID,
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

func (r *SavingsInterestRepository) UpdateInterestAccrualJournal(ctx context.Context, tx any, accountID uuid.UUID, period string, journalID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE interest_accruals SET journal_entry_id = $1
		WHERE account_id = $2 AND period = $3`, journalID, accountID, period)
	return err
}

// InsertAdminFeeCharge menyimpan penanda pemotongan biaya administrasi bulanan.
func (r *SavingsInterestRepository) InsertAdminFeeCharge(ctx context.Context, tx any, rec *domain.AdminFeeChargeRecord) (bool, error) {
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
		INSERT INTO admin_fee_charges (
			id, account_id, account_number, period, amount, revenue_coa, journal_entry_id, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (account_id, period) DO NOTHING`,
		rec.ID, rec.AccountID, rec.AccountNumber, rec.Period, rec.Amount, rec.RevenueCOACode, journalID,
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

func (r *SavingsInterestRepository) UpdateAdminFeeChargeJournal(ctx context.Context, tx any, accountID uuid.UUID, period string, journalID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE admin_fee_charges SET journal_entry_id = $1
		WHERE account_id = $2 AND period = $3`, journalID, accountID, period)
	return err
}

var _ domain.SavingsInterestRepository = (*SavingsInterestRepository)(nil)
