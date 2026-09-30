package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// ModalRepository menyimpan register modal Form 00.06 (migrasi 000128). SQL tetap di
// repository, bukan di service.
type ModalRepository struct {
	db *sql.DB
}

func NewModalRepository(db *sql.DB) *ModalRepository {
	return &ModalRepository{db: db}
}

const modalColumns = `id, jenis_code, tanggal_persetujuan, jenis_modal_code, jumlah, note,
	       created_at, updated_at`

const listModalItemsQuery = `
	SELECT ` + modalColumns + `
	FROM modal_register
	ORDER BY jenis_modal_code, jenis_code, id`

// ListItems membaca seluruh register modal bank-wide untuk UI edit.
func (r *ModalRepository) ListItems(ctx context.Context) ([]domain.ModalItem, error) {
	return r.queryModal(ctx, listModalItemsQuery)
}

// ListModalForOJK membaca baris register untuk Form 00.06, urutan deterministik.
func (r *ModalRepository) ListModalForOJK(ctx context.Context) ([]domain.ModalItem, error) {
	return r.queryModal(ctx, listModalItemsQuery)
}

// queryModal menjalankan query pembacaan dan memetakan baris register.
func (r *ModalRepository) queryModal(ctx context.Context, query string, args ...any) ([]domain.ModalItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.ModalItem
	for rows.Next() {
		var item domain.ModalItem
		var tanggal sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.JenisCode, &tanggal, &item.JenisModalCode, &item.Jumlah,
			&item.Note, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if tanggal.Valid {
			item.TanggalPersetujuan = &tanggal.Time
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id).
func (r *ModalRepository) UpsertItemTx(ctx context.Context, tx any, item domain.ModalItem, actorID uuid.UUID) error {
	sqlTx, err := requireModalTx(tx)
	if err != nil {
		return err
	}
	var tanggal any
	if item.TanggalPersetujuan != nil {
		tanggal = *item.TanggalPersetujuan
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO modal_register (
			id, jenis_code, tanggal_persetujuan, jenis_modal_code, jumlah, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, NOW(), NOW(), $7, $7
		)
		ON CONFLICT (id) DO UPDATE SET
			jenis_code = EXCLUDED.jenis_code,
			tanggal_persetujuan = EXCLUDED.tanggal_persetujuan,
			jenis_modal_code = EXCLUDED.jenis_modal_code,
			jumlah = EXCLUDED.jumlah,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.JenisCode, tanggal, item.JenisModalCode, item.Jumlah, item.Note,
		nullableModalActor(actorID))
	return err
}

// DeleteItemTx menghapus satu baris secara fisik. Form 00.06 tidak menetapkan nomor
// register unik, sehingga DELETE fisik dibenarkan. found=false bila barisnya tidak ada.
func (r *ModalRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireModalTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM modal_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireModalTx memastikan transaksi tulis sah, konsisten dengan pola repository OJK lain.
func requireModalTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register modal: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableModalActor menulis NULL untuk aktor tanpa UUID (mis. sistem).
func nullableModalActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.ModalRepository = (*ModalRepository)(nil)
