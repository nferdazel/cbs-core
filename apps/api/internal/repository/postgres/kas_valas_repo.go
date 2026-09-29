package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// KasValasRegisterRepository menyimpan register kas valuta asing (migrasi 000123):
// rincian per jenis valas untuk Form 03.00. SQL tetap di repository, bukan di service.
type KasValasRegisterRepository struct {
	db *sql.DB
}

func NewKasValasRegisterRepository(db *sql.DB) *KasValasRegisterRepository {
	return &KasValasRegisterRepository{db: db}
}

// kasValasRegisterColumns tidak memuat kolom turunan: kolom V Nilai Rupiah dihitung
// laporan dari III x IV dan tidak disimpan di register.
const kasValasRegisterColumns = `id, jenis_valas_code, nominal, kurs_tengah, as_of,
	       status, note, created_at, updated_at`

const listKasValasItemsQuery = `
	SELECT ` + kasValasRegisterColumns + `
	FROM kas_valas_register
	ORDER BY as_of DESC, jenis_valas_code`

// ListItems membaca seluruh register kas valas bank-wide untuk UI edit.
func (r *KasValasRegisterRepository) ListItems(ctx context.Context) ([]domain.KasValasItem, error) {
	return r.queryKasValas(ctx, listKasValasItemsQuery)
}

const listKasValasForOJKQuery = `
	SELECT ` + kasValasRegisterColumns + `
	FROM kas_valas_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY jenis_valas_code`

// ListKasValasForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 03.00. Satu query, urutan deterministik.
func (r *KasValasRegisterRepository) ListKasValasForOJK(ctx context.Context, asOf time.Time) ([]domain.KasValasItem, error) {
	return r.queryKasValas(ctx, listKasValasForOJKQuery, asOf)
}

// queryKasValas menjalankan query pembacaan dan memetakan baris register.
func (r *KasValasRegisterRepository) queryKasValas(ctx context.Context, query string, args ...any) ([]domain.KasValasItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.KasValasItem
	for rows.Next() {
		var item domain.KasValasItem
		if err := rows.Scan(
			&item.ID, &item.JenisValasCode, &item.Nominal, &item.KursTengah,
			&item.AsOf, &item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt,
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

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id). Form 03.00 tidak
// menetapkan nomor unik, sehingga tidak ada pemetaan pelanggaran keunikan.
func (r *KasValasRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.KasValasItem, actorID uuid.UUID) error {
	sqlTx, err := requireKasValasTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO kas_valas_register (
			id, jenis_valas_code, nominal, kurs_tengah, as_of, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, NOW(), NOW(), $8, $8
		)
		ON CONFLICT (id) DO UPDATE SET
			jenis_valas_code = EXCLUDED.jenis_valas_code,
			nominal = EXCLUDED.nominal,
			kurs_tengah = EXCLUDED.kurs_tengah,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.JenisValasCode, item.Nominal, item.KursTengah,
		item.AsOf, item.Status, item.Note, nullableKasValasActor(actorID))
	return err
}

// DeleteItemTx menghapus satu baris kas valas secara fisik. Form 03.00 tidak menetapkan
// nomor unik (tidak ada aturan no reuse/no recycle), sehingga DELETE fisik dibenarkan.
// found=false bila barisnya tidak ada.
func (r *KasValasRegisterRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireKasValasTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM kas_valas_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireKasValasTx memastikan transaksi tulis sah, konsisten dengan pola repository OJK
// lain.
func requireKasValasTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register kas valuta asing: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableKasValasActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK ke
// staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableKasValasActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.KasValasRegisterRepository = (*KasValasRegisterRepository)(nil)
