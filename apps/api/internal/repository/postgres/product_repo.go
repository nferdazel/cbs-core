package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// productColumns memakai cast ::text untuk kolom bertipe enum kustom. Tanpa cast,
// driver tidak bisa memindai enum PostgreSQL ke string Go dan query gagal saat runtime.
const productColumns = `id, code, name, family::text, book::text, profit_scheme::text,
	schedule_method::text, rate_annual, profit_sharing_ratio, projected_revenue_rate_annual,
	min_amount, max_amount,
	min_term_months, max_term_months, allow_partial_payment, early_withdrawal_penalty_rate,
	admin_fee, tax_rate, is_active`

func scanProduct(row interface{ Scan(...any) error }) (*domain.BankingProduct, error) {
	var p domain.BankingProduct
	err := row.Scan(
		&p.ID, &p.Code, &p.Name, &p.Family, &p.Book, &p.ProfitScheme, &p.ScheduleMethod,
		&p.RateAnnual, &p.ProfitSharingRatio, &p.ProjectedRevenueRateAnnual, &p.MinAmount, &p.MaxAmount, &p.MinTermMonths, &p.MaxTermMonths,
		&p.AllowPartialPayment, &p.EarlyWithdrawalPenaltyRate, &p.AdminFee, &p.TaxRate, &p.IsActive,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepository) List(ctx context.Context) ([]domain.BankingProduct, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+productColumns+` FROM banking_products WHERE is_active = TRUE ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.BankingProduct
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.BankingProduct, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+productColumns+` FROM banking_products WHERE id = $1`, id)
	p, err := scanProduct(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrProductNotFound
	}
	return p, err
}

func (r *ProductRepository) GetByCode(ctx context.Context, code string) (*domain.BankingProduct, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+productColumns+` FROM banking_products WHERE code = $1`, code)
	p, err := scanProduct(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrProductNotFound
	}
	return p, err
}

// queryRower menyatukan *sql.DB dan *sql.Tx agar pembacaan produk bisa mengikuti
// transaksi penulisan.
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// GetByCodeTx membaca produk DI DALAM transaksi penulisan dengan row lock
// (SELECT ... FOR UPDATE). Nilai yang dikembalikan adalah keadaan baris yang
// benar-benar ditimpa, sehingga perbandingan before->after pada audit tidak basi
// saat dua permintaan datang bersamaan.
func (r *ProductRepository) GetByCodeTx(ctx context.Context, tx any, code string) (*domain.BankingProduct, error) {
	var q queryRower = r.db
	if tx != nil {
		sqlTx, ok := tx.(*sql.Tx)
		if !ok {
			return nil, fmt.Errorf("konteks transaksi produk tidak valid")
		}
		q = sqlTx
	}
	row := q.QueryRowContext(ctx, `SELECT `+productColumns+` FROM banking_products WHERE code = $1 FOR UPDATE`, code)
	p, err := scanProduct(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrProductNotFound
	}
	return p, err
}

// UpdateParamsTx menyimpan parameter produk yang boleh diubah. Kolom identitas
// (code, name, family, book, profit_scheme, schedule_method) dan is_active sengaja
// tidak ikut di-SET sehingga tidak ada jalur yang mengubahnya dari sini.
//
// Bila tx diberikan, UPDATE ikut transaksi bisnis yang dibuka service, sehingga
// perubahan dan audit log-nya berhasil atau gagal bersama-sama.
func (r *ProductRepository) UpdateParamsTx(ctx context.Context, tx any, p *domain.BankingProduct) error {
	var exec execer = r.db
	if tx != nil {
		sqlTx, ok := tx.(*sql.Tx)
		if !ok {
			return fmt.Errorf("konteks transaksi produk tidak valid")
		}
		exec = sqlTx
	}

	res, err := exec.ExecContext(ctx, `
		UPDATE banking_products SET
			rate_annual = $1,
			profit_sharing_ratio = $2,
			projected_revenue_rate_annual = $3,
			min_amount = $4,
			max_amount = $5,
			min_term_months = $6,
			max_term_months = $7,
			admin_fee = $8,
			tax_rate = $9,
			early_withdrawal_penalty_rate = $10,
			updated_at = NOW()
		WHERE code = $11`,
		p.RateAnnual,
		p.ProfitSharingRatio,
		p.ProjectedRevenueRateAnnual,
		p.MinAmount,
		p.MaxAmount,
		p.MinTermMonths,
		p.MaxTermMonths,
		p.AdminFee,
		p.TaxRate,
		p.EarlyWithdrawalPenaltyRate,
		p.Code,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func (r *ProductRepository) GetMapping(ctx context.Context, productID uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT event::text, direction::text, coa_code, amount_source::text
		FROM product_journal_mapping
		WHERE product_id = $1 AND event = $2
		ORDER BY direction`, productID, event)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rules []domain.JournalMappingRule
	for rows.Next() {
		var rule domain.JournalMappingRule
		if err := rows.Scan(&rule.Event, &rule.Direction, &rule.COACode, &rule.AmountSource); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

var _ domain.ProductRepository = (*ProductRepository)(nil)
