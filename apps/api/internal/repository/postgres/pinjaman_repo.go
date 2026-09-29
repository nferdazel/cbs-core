package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// PinjamanRegisterRepository menyimpan register pinjaman yang diterima
// (migrasi 000117): rincian per pinjaman/kreditur untuk Form 00.07. SQL tetap di
// repository, bukan di service.
type PinjamanRegisterRepository struct {
	db *sql.DB
}

func NewPinjamanRegisterRepository(db *sql.DB) *PinjamanRegisterRepository {
	return &PinjamanRegisterRepository{db: db}
}

// pinjamanRegisterColumns tidak memuat kolom XV Baki Debet Neto: kolom itu turunan
// yang dihitung laporan, bukan data yang disimpan.
const pinjamanRegisterColumns = `id, counterparty_id, creditor_group_code, bank_code,
	       location_code, jenis_code, relationship_code, start_date, maturity_date,
	       interest_rate, interest_calc_code, plafon, collateral_type_code,
	       collateral_amount, baki_debet, unamortized_transaction_cost,
	       unamortized_discount, as_of, status, note, created_at, updated_at`

const listPinjamanItemsQuery = `
	SELECT ` + pinjamanRegisterColumns + `
	FROM pinjaman_diterima_register
	ORDER BY as_of DESC, counterparty_id, start_date`

// ListItems membaca seluruh register pinjaman bank-wide untuk UI edit.
func (r *PinjamanRegisterRepository) ListItems(ctx context.Context) ([]domain.PinjamanItem, error) {
	return r.queryPinjaman(ctx, listPinjamanItemsQuery)
}

const listPinjamanForOJKQuery = `
	SELECT ` + pinjamanRegisterColumns + `
	FROM pinjaman_diterima_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY counterparty_id, start_date`

// ListPinjamanForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 00.07. Satu query, urutan deterministik.
func (r *PinjamanRegisterRepository) ListPinjamanForOJK(ctx context.Context, asOf time.Time) ([]domain.PinjamanItem, error) {
	return r.queryPinjaman(ctx, listPinjamanForOJKQuery, asOf)
}

// queryPinjaman menjalankan query pembacaan dan memetakan baris register.
func (r *PinjamanRegisterRepository) queryPinjaman(ctx context.Context, query string, args ...any) ([]domain.PinjamanItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.PinjamanItem
	for rows.Next() {
		var item domain.PinjamanItem
		if err := rows.Scan(
			&item.ID, &item.CounterpartyID, &item.CreditorGroupCode, &item.BankCode,
			&item.LocationCode, &item.JenisCode, &item.RelationshipCode, &item.StartDate,
			&item.MaturityDate, &item.InterestRate, &item.InterestCalcCode, &item.Plafon,
			&item.CollateralTypeCode, &item.CollateralAmount, &item.BakiDebet,
			&item.UnamortizedTransactionCost, &item.UnamortizedDiscount, &item.AsOf,
			&item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt,
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

// UpsertItemTx menyimpan satu pinjaman (idempotensi lewat PRIMARY KEY id). Nilai
// kosong tetap ditulis agar perubahan "diisi -> dikosongkan" tercatat.
func (r *PinjamanRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.PinjamanItem, actorID uuid.UUID) error {
	sqlTx, err := requirePinjamanTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO pinjaman_diterima_register (
			id, counterparty_id, creditor_group_code, bank_code, location_code,
			jenis_code, relationship_code, start_date, maturity_date, interest_rate,
			interest_calc_code, plafon, collateral_type_code, collateral_amount,
			baki_debet, unamortized_transaction_cost, unamortized_discount, as_of,
			status, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, NOW(), NOW(), $21, $21
		)
		ON CONFLICT (id) DO UPDATE SET
			counterparty_id = EXCLUDED.counterparty_id,
			creditor_group_code = EXCLUDED.creditor_group_code,
			bank_code = EXCLUDED.bank_code,
			location_code = EXCLUDED.location_code,
			jenis_code = EXCLUDED.jenis_code,
			relationship_code = EXCLUDED.relationship_code,
			start_date = EXCLUDED.start_date,
			maturity_date = EXCLUDED.maturity_date,
			interest_rate = EXCLUDED.interest_rate,
			interest_calc_code = EXCLUDED.interest_calc_code,
			plafon = EXCLUDED.plafon,
			collateral_type_code = EXCLUDED.collateral_type_code,
			collateral_amount = EXCLUDED.collateral_amount,
			baki_debet = EXCLUDED.baki_debet,
			unamortized_transaction_cost = EXCLUDED.unamortized_transaction_cost,
			unamortized_discount = EXCLUDED.unamortized_discount,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.CounterpartyID, item.CreditorGroupCode, item.BankCode,
		item.LocationCode, item.JenisCode, item.RelationshipCode, item.StartDate,
		item.MaturityDate, item.InterestRate, item.InterestCalcCode, item.Plafon,
		item.CollateralTypeCode, item.CollateralAmount, item.BakiDebet,
		item.UnamortizedTransactionCost, item.UnamortizedDiscount, item.AsOf,
		item.Status, item.Note, nullablePinjamanActor(actorID))
	return err
}

// DeleteItemTx menghapus satu pinjaman; found=false bila barisnya tidak ada.
func (r *PinjamanRegisterRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requirePinjamanTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM pinjaman_diterima_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requirePinjamanTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requirePinjamanTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register pinjaman yang diterima: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullablePinjamanActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK
// ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullablePinjamanActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.PinjamanRegisterRepository = (*PinjamanRegisterRepository)(nil)
