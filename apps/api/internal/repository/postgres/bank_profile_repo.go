package postgres

import (
	"context"
	"database/sql"
	"errors"

	"cbs-core/apps/core-api/internal/domain"
)

// BankProfileRepository membaca identitas bank dari tabel bank_profile. SQL tetap
// di repository, bukan di service.
type BankProfileRepository struct {
	db *sql.DB
}

func NewBankProfileRepository(db *sql.DB) *BankProfileRepository {
	return &BankProfileRepository{db: db}
}

func (r *BankProfileRepository) Get(ctx context.Context) (*domain.BankProfile, error) {
	const query = `
		SELECT bank_name, address, city, phone, npwp, updated_at
		FROM bank_profile
		WHERE id = 1`

	var p domain.BankProfile
	err := r.db.QueryRowContext(ctx, query).Scan(
		&p.Name, &p.Address, &p.City, &p.Phone, &p.NPWP, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

var _ domain.BankProfileRepository = (*BankProfileRepository)(nil)
