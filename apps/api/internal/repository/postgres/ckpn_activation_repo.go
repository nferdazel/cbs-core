package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// CKPNActivationRepository menyimpan pengaturan aktivasi CKPN sebagai kunci
// system_config. Kuncinya dibatasi daftar domain.CKPNActivationKeys() agar jalur ini
// tidak pernah menyentuh setelan CKPN lanjutan (ckpn.pabl.*, ckpn.individual.*).
// SQL tetap di repository, bukan di service.
type CKPNActivationRepository struct {
	db *sql.DB
}

func NewCKPNActivationRepository(db *sql.DB) *CKPNActivationRepository {
	return &CKPNActivationRepository{db: db}
}

// loadCKPNActivation membaca nilai kunci yang dikelola. Kunci yang belum ada tidak
// dimasukkan ke peta (pemanggil memperlakukannya sebagai kosong).
func loadCKPNActivation(ctx context.Context, q rowsQuerier) (map[string]string, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT key, value FROM system_config WHERE key = ANY($1)`, domain.CKPNActivationKeys())
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

func (r *CKPNActivationRepository) Get(ctx context.Context) (map[string]string, error) {
	return loadCKPNActivation(ctx, r.db)
}

// GetTx membaca di dalam transaksi tulis agar nilai "sebelum" pada audit mencerminkan
// keadaan yang benar-benar ditimpa.
func (r *CKPNActivationRepository) GetTx(ctx context.Context, tx any) (map[string]string, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("ckpn_activation: transaksi tulis tidak sah")
	}
	return loadCKPNActivation(ctx, sqlTx)
}

// SaveTx menyimpan hanya kunci yang berubah (upsert). Bank boleh mengosongkan sebuah
// nilai (mis. menghapus PD yang salah isi); nilai kosong tetap ditulis agar perubahan
// "diisi -> dikosongkan" tercatat.
func (r *CKPNActivationRepository) SaveTx(ctx context.Context, tx any, changes map[string]string, updatedBy uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return fmt.Errorf("ckpn_activation: transaksi tulis tidak sah")
	}
	// Aktor tanpa UUID (mis. sistem) dicatat NULL: kolomnya FK ke staff_users.
	var updatedByArg any
	if updatedBy != uuid.Nil {
		updatedByArg = updatedBy
	}
	// Urutan DETERMINISTIK dan ckpn.parameters.status SELALU terakhir: trigger
	// ckpn_ratification_guard (migrasi 000095) membaca baris bukti dari tabel saat
	// menulis status FINAL, sehingga bukti harus ditulis lebih dulu dalam transaksi
	// yang sama. Iterasi map Go acak; tanpa urutan ini penyetelan FINAL + bukti dalam
	// satu permintaan bisa gagal atau lolos secara acak.
	keys := make([]string, 0, len(changes))
	for key := range changes {
		if key == domain.ConfigKeyCKPNParametersStatus {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if _, ada := changes[domain.ConfigKeyCKPNParametersStatus]; ada {
		keys = append(keys, domain.ConfigKeyCKPNParametersStatus)
	}
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

var _ domain.CKPNActivationStore = (*CKPNActivationRepository)(nil)
