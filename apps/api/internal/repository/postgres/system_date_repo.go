package postgres

import (
	"context"
	"database/sql"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type BusinessDateRepository struct {
	db *sql.DB
}

func NewBusinessDateRepository(db *sql.DB) *BusinessDateRepository {
	return &BusinessDateRepository{db: db}
}

func (r *BusinessDateRepository) GetCurrentDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	var dateStr, statusStr string
	var updatedByStr sql.NullString

	err := r.db.QueryRowContext(ctx,
		"SELECT value, description FROM system_config WHERE key = 'system.business_date'").Scan(&dateStr, &updatedByStr)
	if err != nil {
		// Fallback to today UTC if not configured
		dateStr = time.Now().Format("2006-01-02")
	}

	_ = r.db.QueryRowContext(ctx,
		"SELECT value FROM system_config WHERE key = 'system.business_date_status'").Scan(&statusStr)
	if statusStr == "" {
		statusStr = string(domain.BusinessDateStatusOpen)
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		parsedDate = time.Now().UTC()
	}

	var updatedBy *uuid.UUID
	if updatedByStr.Valid {
		id, err := uuid.Parse(updatedByStr.String)
		if err == nil {
			updatedBy = &id
		}
	}

	return &domain.SystemBusinessDate{
		CurrentDate: parsedDate,
		Status:      domain.BusinessDateStatus(statusStr),
		UpdatedBy:   updatedBy,
		UpdatedAt:   time.Now().UTC(),
	}, nil
}

func (r *BusinessDateRepository) AdvanceDate(ctx context.Context, nextDate time.Time, updatedBy uuid.UUID) error {
	dateStr := nextDate.Format("2006-01-02")

	// Kedua penulisan wajib atomik. Bila hanya salah satu yang tersimpan (mis. proses
	// berhenti di antaranya), bank terjebak: tanggal tidak maju sementara statusnya
	// menandakan tutup hari, dan operasional berhenti sampai diperbaiki manual.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q1 := `INSERT INTO system_config (key, value, description, updated_by, updated_at)
		VALUES ('system.business_date', $1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, description = $2, updated_by = $3, updated_at = NOW()`
	if _, err := tx.ExecContext(ctx, q1, dateStr, updatedBy.String(), updatedBy); err != nil {
		return err
	}

	q2 := `INSERT INTO system_config (key, value, description, updated_by, updated_at)
		VALUES ('system.business_date_status', 'OPEN', 'Operational status', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = 'OPEN', updated_by = $1, updated_at = NOW()`
	if _, err := tx.ExecContext(ctx, q2, updatedBy); err != nil {
		return err
	}

	return tx.Commit()
}

// ClaimEOD mengklaim tanggal bisnis untuk tutup hari secara atomik: status hanya
// berpindah ke EOD bila status saat ini bukan CLOSED, dan hasilnya dilaporkan lewat
// jumlah baris yang berubah. Dengan begitu keputusan "boleh lanjut" dan perubahan
// statusnya tidak dapat disela proses lain.
func (r *BusinessDateRepository) ClaimEOD(ctx context.Context) (bool, error) {
	q := `INSERT INTO system_config (key, value, description, updated_at)
		VALUES ('system.business_date_status', $1, 'Operational status', NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
		WHERE system_config.value <> $2`
	res, err := r.db.ExecContext(ctx, q, string(domain.BusinessDateStatusEOD), string(domain.BusinessDateStatusClosed))
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

var _ domain.BusinessDateRepository = (*BusinessDateRepository)(nil)
