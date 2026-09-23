package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// OJKProfileRepository menyimpan identitas Form 00.00 yang tidak muat di
// bank_profile. Nilai disimpan sebagai kunci system_config berawalan ojk.
// SQL tetap di repository, bukan di service.
type OJKProfileRepository struct {
	db *sql.DB
}

func NewOJKProfileRepository(db *sql.DB) *OJKProfileRepository {
	return &OJKProfileRepository{db: db}
}

// rowsQuerier dipenuhi *sql.DB dan *sql.Tx, sehingga pembacaan yang sama dapat
// dilakukan di luar maupun di dalam transaksi tulis.
type rowsQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// loadOJKProfile membaca seluruh kunci ojk.* dan memetakannya ke profil. Kunci yang
// belum ada menjadi nilai kosong, bukan galat.
func loadOJKProfile(ctx context.Context, q rowsQuerier) (*domain.OJKProfile, error) {
	rows, err := q.QueryContext(ctx, `SELECT key, value FROM system_config WHERE key LIKE 'ojk.%'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	values := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		values[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domain.OJKProfileFromValues(values), nil
}

func (r *OJKProfileRepository) Get(ctx context.Context) (*domain.OJKProfile, error) {
	return loadOJKProfile(ctx, r.db)
}

// GetTx membaca di dalam transaksi tulis agar nilai "sebelum" pada audit
// mencerminkan keadaan yang benar-benar ditimpa.
func (r *OJKProfileRepository) GetTx(ctx context.Context, tx any) (*domain.OJKProfile, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("ojk_profile: transaksi tulis tidak sah")
	}
	return loadOJKProfile(ctx, sqlTx)
}

// SaveTx menyimpan seluruh kunci ojk.* (upsert). Bank boleh mengosongkan sebuah
// bidang; nilai kosong tetap ditulis agar perubahan "diisi -> dikosongkan" tercatat.
func (r *OJKProfileRepository) SaveTx(ctx context.Context, tx any, profile *domain.OJKProfile, updatedBy uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return fmt.Errorf("ojk_profile: transaksi tulis tidak sah")
	}
	// Aktor tanpa UUID (mis. sistem) dicatat sebagai NULL: kolomnya FK ke staff_users,
	// sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
	var updatedByArg any
	if updatedBy != uuid.Nil {
		updatedByArg = updatedBy
	}
	for key, value := range profile.KeyValues() {
		_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO system_config (key, value, updated_by, updated_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (key) DO UPDATE
			SET value = $2, updated_by = $3, updated_at = NOW()`,
			key, value, updatedByArg)
		if err != nil {
			return err
		}
	}
	return nil
}

var _ domain.OJKProfileStore = (*OJKProfileRepository)(nil)
