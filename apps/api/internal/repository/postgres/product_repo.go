package postgres

import (
	"context"
	"database/sql"
	"errors"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

const productColumns = `id, code, name, family, book, profit_scheme, schedule_method,
	rate_annual, profit_sharing_ratio, min_amount, max_amount, min_term_months, max_term_months,
	allow_partial_payment, early_withdrawal_penalty_rate, admin_fee, tax_rate, is_active`

func scanProduct(row interface{ Scan(...any) error }) (*domain.BankingProduct, error) {
	var p domain.BankingProduct
	err := row.Scan(
		&p.ID, &p.Code, &p.Name, &p.Family, &p.Book, &p.ProfitScheme, &p.ScheduleMethod,
		&p.RateAnnual, &p.ProfitSharingRatio, &p.MinAmount, &p.MaxAmount, &p.MinTermMonths, &p.MaxTermMonths,
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
	defer rows.Close()

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

func (r *ProductRepository) GetMapping(ctx context.Context, productID uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT event, direction, coa_code, amount_source
		FROM product_journal_mapping
		WHERE product_id = $1 AND event = $2
		ORDER BY direction`, productID, event)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
