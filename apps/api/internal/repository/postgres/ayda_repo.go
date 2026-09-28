package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// AYDARegisterRepository menyimpan register AYDA (migrasi 000115): rincian per kasus
// agunan yang diambil alih untuk Form 07.00. SQL tetap di repository, bukan di service.
type AYDARegisterRepository struct {
	db *sql.DB
}

func NewAYDARegisterRepository(db *sql.DB) *AYDARegisterRepository {
	return &AYDARegisterRepository{db: db}
}

const aydaRegisterColumns = `id, collateral_type_code, collateral_address, acquisition_date,
	       initial_recognition_value, accumulated_impairment, net_realizable_value,
	       as_of, status, note, created_at, updated_at`

const listAYDAItemsQuery = `
	SELECT ` + aydaRegisterColumns + `
	FROM ayda_register
	ORDER BY as_of DESC, acquisition_date DESC, collateral_address`

// ListItems membaca seluruh register AYDA bank-wide untuk UI edit.
func (r *AYDARegisterRepository) ListItems(ctx context.Context) ([]domain.AYDAItem, error) {
	return r.queryAYDA(ctx, listAYDAItemsQuery)
}

const listAYDAForOJKQuery = `
	SELECT ` + aydaRegisterColumns + `
	FROM ayda_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY acquisition_date, collateral_address`

// ListAYDAForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 07.00. Satu query, urutan deterministik.
func (r *AYDARegisterRepository) ListAYDAForOJK(ctx context.Context, asOf time.Time) ([]domain.AYDAItem, error) {
	return r.queryAYDA(ctx, listAYDAForOJKQuery, asOf)
}

// queryAYDA menjalankan query pembacaan dan memetakan baris register.
func (r *AYDARegisterRepository) queryAYDA(ctx context.Context, query string, args ...any) ([]domain.AYDAItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.AYDAItem
	for rows.Next() {
		var item domain.AYDAItem
		if err := rows.Scan(
			&item.ID, &item.CollateralTypeCode, &item.CollateralAddress, &item.AcquisitionDate,
			&item.InitialRecognitionValue, &item.AccumulatedImpairment, &item.NetRealizableValue,
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

// UpsertItemTx menyimpan satu AYDA (idempotensi lewat PRIMARY KEY id). Nilai kosong
// tetap ditulis agar perubahan "diisi -> dikosongkan" tercatat.
func (r *AYDARegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.AYDAItem, actorID uuid.UUID) error {
	sqlTx, err := requireAYDATx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO ayda_register (
			id, collateral_type_code, collateral_address, acquisition_date,
			initial_recognition_value, accumulated_impairment, net_realizable_value,
			as_of, status, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW(), $11, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			collateral_type_code = EXCLUDED.collateral_type_code,
			collateral_address = EXCLUDED.collateral_address,
			acquisition_date = EXCLUDED.acquisition_date,
			initial_recognition_value = EXCLUDED.initial_recognition_value,
			accumulated_impairment = EXCLUDED.accumulated_impairment,
			net_realizable_value = EXCLUDED.net_realizable_value,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.CollateralTypeCode, item.CollateralAddress, item.AcquisitionDate,
		item.InitialRecognitionValue, item.AccumulatedImpairment, item.NetRealizableValue,
		item.AsOf, item.Status, item.Note, nullableAYDAActor(actorID))
	return err
}

// DeleteItemTx menghapus satu AYDA; found=false bila barisnya tidak ada.
func (r *AYDARegisterRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireAYDATx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM ayda_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireAYDATx memastikan transaksi tulis sah, konsisten dengan pola repository OJK lain.
func requireAYDATx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register AYDA: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableAYDAActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK ke
// staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableAYDAActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.AYDARegisterRepository = (*AYDARegisterRepository)(nil)
