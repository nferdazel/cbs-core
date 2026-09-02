package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type SystemConfigRepository struct {
	db *sql.DB
}

func NewSystemConfigRepository(db *sql.DB) *SystemConfigRepository {
	return &SystemConfigRepository{db: db}
}

func (r *SystemConfigRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx,
		"SELECT value FROM system_config WHERE key=$1", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("config key not found: " + key)
	}
	return value, err
}

func (r *SystemConfigRepository) Set(ctx context.Context, key, value string, updatedBy uuid.UUID) error {
	q := `INSERT INTO system_config (key, value, updated_by, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (key) DO UPDATE
		SET value=$2, updated_by=$3, updated_at=$4`
	_, err := r.db.ExecContext(ctx, q, key, value, updatedBy, time.Now().UTC())
	return err
}

func (r *SystemConfigRepository) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT key, value FROM system_config ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cfg := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		cfg[k] = v
	}
	return cfg, nil
}

// Ensure SystemConfigRepository implements the interface
var _ domain.SystemConfigRepository = (*SystemConfigRepository)(nil)
