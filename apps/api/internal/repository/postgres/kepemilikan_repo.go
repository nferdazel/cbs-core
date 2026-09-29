package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// KepemilikanRegisterRepository menyimpan register pemegang saham BPR
// (migrasi 000116): rincian per pemegang saham untuk Form 00.01. SQL tetap di
// repository, bukan di service.
type KepemilikanRegisterRepository struct {
	db *sql.DB
}

func NewKepemilikanRegisterRepository(db *sql.DB) *KepemilikanRegisterRepository {
	return &KepemilikanRegisterRepository{db: db}
}

// kepemilikanRegisterColumns sengaja tidak memuat kolom identitas: kolom IV
// "No. Identitas" tidak pernah disimpan (keputusan privasi).
const kepemilikanRegisterColumns = `id, shareholder_name, shareholder_address,
	       shareholder_type_code, shareholder_status_code, nominal_amount,
	       ownership_percentage, change_status_code, as_of, status, note,
	       created_at, updated_at`

const listKepemilikanItemsQuery = `
	SELECT ` + kepemilikanRegisterColumns + `
	FROM kepemilikan_bpr_register
	ORDER BY as_of DESC, shareholder_name, shareholder_type_code`

// ListItems membaca seluruh register pemegang saham bank-wide untuk UI edit.
func (r *KepemilikanRegisterRepository) ListItems(ctx context.Context) ([]domain.KepemilikanItem, error) {
	return r.queryKepemilikan(ctx, listKepemilikanItemsQuery)
}

const listKepemilikanForOJKQuery = `
	SELECT ` + kepemilikanRegisterColumns + `
	FROM kepemilikan_bpr_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY shareholder_name, shareholder_type_code`

// ListKepemilikanForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf:
// sumber Form 00.01. Satu query, urutan deterministik.
func (r *KepemilikanRegisterRepository) ListKepemilikanForOJK(ctx context.Context, asOf time.Time) ([]domain.KepemilikanItem, error) {
	return r.queryKepemilikan(ctx, listKepemilikanForOJKQuery, asOf)
}

// queryKepemilikan menjalankan query pembacaan dan memetakan baris register.
func (r *KepemilikanRegisterRepository) queryKepemilikan(ctx context.Context, query string, args ...any) ([]domain.KepemilikanItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.KepemilikanItem
	for rows.Next() {
		var item domain.KepemilikanItem
		if err := rows.Scan(
			&item.ID, &item.ShareholderName, &item.ShareholderAddress,
			&item.ShareholderTypeCode, &item.ShareholderStatusCode, &item.NominalAmount,
			&item.OwnershipPercentage, &item.ChangeStatusCode, &item.AsOf, &item.Status,
			&item.Note, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertItemTx menyimpan satu pemegang saham (idempotensi lewat PRIMARY KEY id).
// Nilai kosong tetap ditulis agar perubahan "diisi -> dikosongkan" tercatat.
func (r *KepemilikanRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.KepemilikanItem, actorID uuid.UUID) error {
	sqlTx, err := requireKepemilikanTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO kepemilikan_bpr_register (
			id, shareholder_name, shareholder_address,
			shareholder_type_code, shareholder_status_code, nominal_amount,
			ownership_percentage, change_status_code, as_of, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW(), $12, $12
		)
		ON CONFLICT (id) DO UPDATE SET
			shareholder_name = EXCLUDED.shareholder_name,
			shareholder_address = EXCLUDED.shareholder_address,
			shareholder_type_code = EXCLUDED.shareholder_type_code,
			shareholder_status_code = EXCLUDED.shareholder_status_code,
			nominal_amount = EXCLUDED.nominal_amount,
			ownership_percentage = EXCLUDED.ownership_percentage,
			change_status_code = EXCLUDED.change_status_code,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.ShareholderName, item.ShareholderAddress,
		item.ShareholderTypeCode, item.ShareholderStatusCode, item.NominalAmount,
		item.OwnershipPercentage, item.ChangeStatusCode, item.AsOf, item.Status, item.Note,
		nullableKepemilikanActor(actorID))
	return err
}

// DeleteItemTx menghapus satu pemegang saham; found=false bila barisnya tidak ada.
func (r *KepemilikanRegisterRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireKepemilikanTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM kepemilikan_bpr_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireKepemilikanTx memastikan transaksi tulis sah, konsisten dengan pola
// repository OJK lain.
func requireKepemilikanTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register kepemilikan BPR: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableKepemilikanActor menulis NULL untuk aktor tanpa UUID (mis. sistem):
// kolomnya FK ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang
// tidak ada.
func nullableKepemilikanActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.KepemilikanRegisterRepository = (*KepemilikanRegisterRepository)(nil)
