package postgres

import (
	"context"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// GetByIDs mengambil banyak record nasabah dalam satu query. Dipakai service untuk
// melengkapi nama pada daftar rekening/kredit tanpa N+1 query ke tabel customers.
// Nilai yang dikembalikan masih ciphertext; dekripsi tetap tanggung jawab service.
func (r *CustomerRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*domain.CustomerRecord, error) {
	result := make(map[uuid.UUID]*domain.CustomerRecord, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	query := `SELECT ` + customerColumns + ` FROM customers WHERE id = ANY($1)`
	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil nasabah berdasarkan id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		rec, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		result[rec.ID] = rec
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
