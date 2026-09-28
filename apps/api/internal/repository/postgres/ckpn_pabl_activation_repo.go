package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// CKPNPABLActivationRepository menyimpan pengaturan CKPN PABL (enam kunci ckpn.pabl.*,
// migrasi 000099/000100) sebagai kunci system_config. Kuncinya dibatasi daftar
// domain.CKPNPABLActivationKeys() agar jalur ini tidak pernah menyentuh setelan CKPN
// lain. SQL tetap di repository, bukan di service.
type CKPNPABLActivationRepository struct {
	db *sql.DB
}

func NewCKPNPABLActivationRepository(db *sql.DB) *CKPNPABLActivationRepository {
	return &CKPNPABLActivationRepository{db: db}
}

// loadCKPNPABLActivation membaca nilai kunci yang dikelola. Kunci yang belum ada tidak
// dimasukkan ke peta (pemanggil memperlakukannya sebagai kosong).
func loadCKPNPABLActivation(ctx context.Context, q rowsQuerier) (map[string]string, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT key, value FROM system_config WHERE key = ANY($1)`, domain.CKPNPABLActivationKeys())
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
	return values, nil
}

func (r *CKPNPABLActivationRepository) Get(ctx context.Context) (map[string]string, error) {
	return loadCKPNPABLActivation(ctx, r.db)
}

// GetTx membaca di dalam transaksi tulis agar nilai "sebelum" pada audit mencerminkan
// keadaan yang benar-benar ditimpa.
func (r *CKPNPABLActivationRepository) GetTx(ctx context.Context, tx any) (map[string]string, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("ckpn_pabl_activation: transaksi tulis tidak sah")
	}
	return loadCKPNPABLActivation(ctx, sqlTx)
}

// SaveTx menyimpan hanya kunci yang berubah (upsert). Bank boleh mengosongkan sebuah
// nilai (mis. menghapus PD yang salah isi); nilai kosong tetap ditulis agar perubahan
// "diisi -> dikosongkan" tercatat.
func (r *CKPNPABLActivationRepository) SaveTx(ctx context.Context, tx any, changes map[string]string, updatedBy uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return fmt.Errorf("ckpn_pabl_activation: transaksi tulis tidak sah")
	}
	// Aktor tanpa UUID (mis. sistem) dicatat NULL: kolomnya FK ke staff_users.
	var updatedByArg any
	if updatedBy != uuid.Nil {
		updatedByArg = updatedBy
	}
	// Urutan DETERMINISTIK agar penulisan dapat direproduksi.
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO system_config (key, value, updated_by, updated_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (key) DO UPDATE
			SET value = $2, updated_by = $3, updated_at = NOW()`,
			key, changes[key], updatedByArg)
		if err != nil {
			return err
		}
	}
	return nil
}

var _ domain.CKPNPABLActivationStore = (*CKPNPABLActivationRepository)(nil)
