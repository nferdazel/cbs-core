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
	q1 := `INSERT INTO system_config (key, value, description, updated_by, updated_at)
		VALUES ('system.business_date', $1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, description = $2, updated_by = $3, updated_at = NOW()`
	
	if _, err := r.db.ExecContext(ctx, q1, dateStr, updatedBy.String(), updatedBy); err != nil {
		return err
	}

	q2 := `INSERT INTO system_config (key, value, description, updated_by, updated_at)
		VALUES ('system.business_date_status', 'OPEN', 'Operational status', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = 'OPEN', updated_by = $1, updated_at = NOW()`
	
	_, err := r.db.ExecContext(ctx, q2, updatedBy)
	return err
}

func (r *BusinessDateRepository) SetStatus(ctx context.Context, status domain.BusinessDateStatus) error {
	q := `INSERT INTO system_config (key, value, description, updated_at)
		VALUES ('system.business_date_status', $1, 'Operational status', NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()`
	_, err := r.db.ExecContext(ctx, q, string(status))
	return err
}

var _ domain.BusinessDateRepository = (*BusinessDateRepository)(nil)
