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

type DepositRepository struct {
	db *sql.DB
}

func NewDepositRepository(db *sql.DB) *DepositRepository {
	return &DepositRepository{db: db}
}

// depositColumns memakai cast ::text untuk kolom enum deposit_status. Tanpa cast,
// driver tidak bisa memindai enum PostgreSQL ke string Go. profit_type dan
// aro_instruction bertipe varchar sehingga tidak perlu cast.
const depositColumns = `id, account_number, customer_id, product_id, branch_id,
	placement_amount, currency, term_months, start_date, maturity_date,
	profit_rate, yield_rate, profit_type, tax_rate, aro, aro_instruction,
	status::text, accrued_profit, accrued_tax, paid_profit, paid_tax,
	maturity_proceeds, last_accrual_date, closed_at, created_at, updated_at`

func scanDeposit(row interface{ Scan(...any) error }) (*domain.Deposit, error) {
	var d domain.Deposit
	var lastAccrual, closedAt sql.NullTime
	err := row.Scan(
		&d.ID, &d.AccountNumber, &d.CustomerID, &d.ProductID, &d.BranchID,
		&d.PlacementAmount, &d.Currency, &d.TermMonths, &d.StartDate, &d.MaturityDate,
		&d.ProfitRate, &d.YieldRate, &d.ProfitType, &d.TaxRate, &d.ARO, &d.AROInstruction,
		&d.Status, &d.AccruedProfit, &d.AccruedTax, &d.PaidProfit, &d.PaidTax,
		&d.MaturityProceeds, &lastAccrual, &closedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDepositNotFound
		}
		return nil, err
	}
	if lastAccrual.Valid {
		t := lastAccrual.Time
		d.LastAccrualDate = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		d.ClosedAt = &t
	}
	return &d, nil
}

func (r *DepositRepository) Create(ctx context.Context, tx any, d *domain.Deposit) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("insert deposit: transaksi tidak valid")
	}

	query := `
		INSERT INTO deposits (
			id, account_number, customer_id, product_id, branch_id,
			placement_amount, currency, term_months, start_date, maturity_date,
			profit_rate, yield_rate, profit_type, tax_rate, aro, aro_instruction,
			status, accrued_profit, accrued_tax, paid_profit, paid_tax,
			maturity_proceeds, last_accrual_date, closed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17::deposit_status, $18, $19, $20, $21, $22, $23, $24, $25, $26)
	`
	_, err := sqlTx.ExecContext(ctx, query,
		d.ID, d.AccountNumber, d.CustomerID, d.ProductID, d.BranchID,
		d.PlacementAmount, d.Currency, d.TermMonths, d.StartDate, d.MaturityDate,
		d.ProfitRate, d.YieldRate, d.ProfitType, d.TaxRate, d.ARO, d.AROInstruction,
		d.Status, d.AccruedProfit, d.AccruedTax, d.PaidProfit, d.PaidTax,
		d.MaturityProceeds, d.LastAccrualDate, d.ClosedAt, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert deposit: %w", err)
	}
	return nil
}

func (r *DepositRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Deposit, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+depositColumns+` FROM deposits WHERE id = $1`, id)
	return scanDeposit(row)
}

func (r *DepositRepository) GetByIDForUpdate(ctx context.Context, tx any, id uuid.UUID) (*domain.Deposit, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("lock deposit: transaksi tidak valid")
	}
	row := sqlTx.QueryRowContext(ctx, `SELECT `+depositColumns+` FROM deposits WHERE id = $1 FOR UPDATE`, id)
	return scanDeposit(row)
}

func (r *DepositRepository) List(ctx context.Context, limit, offset int) ([]domain.Deposit, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deposits`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `SELECT `+depositColumns+`
		FROM deposits ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.Deposit
	for rows.Next() {
		d, err := scanDeposit(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *d)
	}
	return list, total, rows.Err()
}

// ListMaturedARO mengambil deposito ber-ARO yang sudah melewati jatuh tempo dan
// belum ditutup, untuk diperpanjang otomatis.
func (r *DepositRepository) ListMaturedARO(ctx context.Context, asOf time.Time) ([]domain.Deposit, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+depositColumns+`
		FROM deposits
		WHERE aro = TRUE AND status IN ('PLACED', 'MATURED') AND maturity_date <= $1
		ORDER BY maturity_date`, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Deposit
	for rows.Next() {
		d, err := scanDeposit(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *d)
	}
	return list, rows.Err()
}

func (r *DepositRepository) AddAccrual(ctx context.Context, tx any, id uuid.UUID, profit, tax decimal.Decimal, asOf time.Time) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("akrual deposit: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE deposits
		SET accrued_profit = accrued_profit + $2,
		    accrued_tax = accrued_tax + $3,
		    last_accrual_date = $4,
		    updated_at = NOW()
		WHERE id = $1`, id, profit, tax, asOf)
	if err != nil {
		return fmt.Errorf("update akrual deposit: %w", err)
	}
	return nil
}

func (r *DepositRepository) UpdateStatus(ctx context.Context, tx any, id uuid.UUID, status domain.DepositStatus, proceeds, paidProfit, paidTax decimal.Decimal) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("update status deposit: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE deposits
		SET status = $2::deposit_status,
		    maturity_proceeds = $3,
		    paid_profit = $4,
		    paid_tax = $5,
		    closed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`, id, status, proceeds, paidProfit, paidTax)
	if err != nil {
		return fmt.Errorf("update status deposit: %w", err)
	}
	return nil
}

// Rollover memperpanjang kontrak. resetAccrual true dipakai saat imbal hasil
// dikapitalisasi ke pokok sehingga akrual lama dianggap selesai dibayar.
func (r *DepositRepository) Rollover(ctx context.Context, tx any, id uuid.UUID, newPrincipal, paidProfit, paidTax decimal.Decimal, newStart, newMaturity time.Time, resetAccrual bool) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("rollover deposit: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE deposits
		SET placement_amount = $2,
		    paid_profit = paid_profit + $3,
		    paid_tax = paid_tax + $4,
		    start_date = $5,
		    maturity_date = $6,
		    status = 'PLACED',
		    accrued_profit = CASE WHEN $7 THEN 0 ELSE accrued_profit END,
		    accrued_tax = CASE WHEN $7 THEN 0 ELSE accrued_tax END,
		    last_accrual_date = CASE WHEN $7 THEN NULL ELSE last_accrual_date END,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1`, id, newPrincipal, paidProfit, paidTax, newStart, newMaturity, resetAccrual)
	if err != nil {
		return fmt.Errorf("rollover deposit: %w", err)
	}
	return nil
}

var _ domain.DepositRepository = (*DepositRepository)(nil)
