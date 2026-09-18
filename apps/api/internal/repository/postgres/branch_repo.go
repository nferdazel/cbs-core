package postgres

import (
	"context"
	"database/sql"
	"errors"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type BranchRepository struct {
	db *sql.DB
}

func NewBranchRepository(db *sql.DB) *BranchRepository {
	return &BranchRepository{db: db}
}

const branchColumns = `id, code, name, COALESCE(address, ''), COALESCE(phone, ''), is_head_office, is_active`

func (r *BranchRepository) List(ctx context.Context) ([]domain.Branch, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+branchColumns+` FROM branches ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Branch
	for rows.Next() {
		var b domain.Branch
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Address, &b.Phone, &b.IsHeadOffice, &b.IsActive); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (r *BranchRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Branch, error) {
	return r.scanOne(ctx, `SELECT `+branchColumns+` FROM branches WHERE id = $1`, id)
}

func (r *BranchRepository) GetByCode(ctx context.Context, code string) (*domain.Branch, error) {
	return r.scanOne(ctx, `SELECT `+branchColumns+` FROM branches WHERE code = $1`, code)
}

func (r *BranchRepository) scanOne(ctx context.Context, query string, arg any) (*domain.Branch, error) {
	var b domain.Branch
	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&b.ID, &b.Code, &b.Name, &b.Address, &b.Phone, &b.IsHeadOffice, &b.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrBranchNotFound
		}
		return nil, err
	}
	return &b, nil
}
