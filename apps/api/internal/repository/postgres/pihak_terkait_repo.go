package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// PihakTerkaitRepository menyimpan register pihak terkait lainnya Form 00.05
// (migrasi 000127). SQL tetap di repository, bukan di service.
type PihakTerkaitRepository struct {
	db *sql.DB
}

func NewPihakTerkaitRepository(db *sql.DB) *PihakTerkaitRepository {
	return &PihakTerkaitRepository{db: db}
}

// pihakTerkaitColumns tidak memuat kolom II No. Identitas: kolom itu sengaja tidak
// disimpan (keputusan privasi) dan laporan menulis "-" beserta alasannya.
const pihakTerkaitColumns = `id, nama, alamat, jenis_code, hubungan_code, note,
	       created_at, updated_at`

const listPihakTerkaitItemsQuery = `
	SELECT ` + pihakTerkaitColumns + `
	FROM pihak_terkait_lainnya_register
	ORDER BY jenis_code, nama, id`

// ListItems membaca seluruh register pihak terkait bank-wide untuk UI edit.
func (r *PihakTerkaitRepository) ListItems(ctx context.Context) ([]domain.PihakTerkaitItem, error) {
	return r.queryPihakTerkait(ctx, listPihakTerkaitItemsQuery)
}

// ListPihakTerkaitForOJK membaca baris register untuk Form 00.05, urutan deterministik.
func (r *PihakTerkaitRepository) ListPihakTerkaitForOJK(ctx context.Context) ([]domain.PihakTerkaitItem, error) {
	return r.queryPihakTerkait(ctx, listPihakTerkaitItemsQuery)
}

// queryPihakTerkait menjalankan query pembacaan dan memetakan baris register.
func (r *PihakTerkaitRepository) queryPihakTerkait(ctx context.Context, query string, args ...any) ([]domain.PihakTerkaitItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.PihakTerkaitItem
	for rows.Next() {
		var item domain.PihakTerkaitItem
		if err := rows.Scan(
			&item.ID, &item.Nama, &item.Alamat, &item.JenisCode, &item.HubunganCode,
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

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id).
func (r *PihakTerkaitRepository) UpsertItemTx(ctx context.Context, tx any, item domain.PihakTerkaitItem, actorID uuid.UUID) error {
	sqlTx, err := requirePihakTerkaitTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO pihak_terkait_lainnya_register (
			id, nama, alamat, jenis_code, hubungan_code, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, NOW(), NOW(), $7, $7
		)
		ON CONFLICT (id) DO UPDATE SET
			nama = EXCLUDED.nama,
			alamat = EXCLUDED.alamat,
			jenis_code = EXCLUDED.jenis_code,
			hubungan_code = EXCLUDED.hubungan_code,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.Nama, item.Alamat, item.JenisCode, item.HubunganCode, item.Note,
		nullablePihakTerkaitActor(actorID))
	return err
}

// DeleteItemTx menghapus satu baris secara fisik. Form 00.05 tidak menetapkan nomor
// register unik, sehingga DELETE fisik dibenarkan. found=false bila barisnya tidak ada.
func (r *PihakTerkaitRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requirePihakTerkaitTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM pihak_terkait_lainnya_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requirePihakTerkaitTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requirePihakTerkaitTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register pihak terkait: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullablePihakTerkaitActor menulis NULL untuk aktor tanpa UUID (mis. sistem).
func nullablePihakTerkaitActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.PihakTerkaitRepository = (*PihakTerkaitRepository)(nil)
