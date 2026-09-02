package postgres

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

type ReportRepository struct {
	db *sql.DB
}

func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) GetTrialBalance(ctx context.Context) ([]domain.TrialBalanceItem, error) {
	q := `SELECT c.code, c.name, c.account_type,
		COALESCE(SUM(CASE WHEN jl.direction = 'DEBIT' THEN jl.amount ELSE 0 END), 0) AS total_debit,
		COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0) AS total_credit
		FROM chart_of_accounts c
		LEFT JOIN accounts a ON a.coa_id = c.id
		LEFT JOIN journal_lines jl ON jl.account_id = a.id
		GROUP BY c.id, c.code, c.name, c.account_type
		ORDER BY c.code ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.TrialBalanceItem
	for rows.Next() {
		var item domain.TrialBalanceItem
		var debit, credit decimal.Decimal
		if err := rows.Scan(&item.COACode, &item.COAName, &item.AccountType, &debit, &credit); err != nil {
			return nil, err
		}
		item.DebitBalance = debit
		item.CreditBalance = credit
		items = append(items, item)
	}
	return items, nil
}

func (r *ReportRepository) GetCOABalances(ctx context.Context) (map[string]decimal.Decimal, map[string]domain.COAType, map[string]string, error) {
	q := `SELECT c.code, c.name, c.account_type, COALESCE(SUM(a.balance), 0) AS net_balance
		FROM chart_of_accounts c
		LEFT JOIN accounts a ON a.coa_id = c.id
		GROUP BY c.id, c.code, c.name, c.account_type
		ORDER BY c.code ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	balances := make(map[string]decimal.Decimal)
	types := make(map[string]domain.COAType)
	names := make(map[string]string)

	for rows.Next() {
		var code, name string
		var acctype domain.COAType
		var bal decimal.Decimal
		if err := rows.Scan(&code, &name, &acctype, &bal); err != nil {
			return nil, nil, nil, err
		}
		balances[code] = bal
		types[code] = acctype
		names[code] = name
	}

	return balances, types, names, nil
}

var _ domain.ReportRepository = (*ReportRepository)(nil)
