package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// OffBalanceRepository menyimpan register rekening administratif (migrasi 000113):
// pos komitmen/kontinjensi off-balance yang bank catat untuk Form 01.01. SQL tetap di
// repository, bukan di service.
type OffBalanceRepository struct {
	db *sql.DB
}

func NewOffBalanceRepository(db *sql.DB) *OffBalanceRepository {
	return &OffBalanceRepository{db: db}
}

const listOffBalanceItemsQuery = `
	SELECT id, position_code, category, description, amount,
	       counterparty_customer_id, reference, as_of, status, note,
	       created_at, updated_at
	FROM off_balance_items
	ORDER BY as_of DESC, category, position_code, description`

// ListItems membaca seluruh register rekening administratif bank-wide.
func (r *OffBalanceRepository) ListItems(ctx context.Context) ([]domain.OffBalanceItem, error) {
	rows, err := r.db.QueryContext(ctx, listOffBalanceItemsQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.OffBalanceItem
	for rows.Next() {
		var (
			item domain.OffBalanceItem
			cp   uuid.NullUUID
		)
		if err := rows.Scan(
			&item.ID, &item.PositionCode, &item.Category, &item.Description, &item.Amount,
			&cp, &item.Reference, &item.AsOf, &item.Status, &item.Note,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if cp.Valid {
			id := cp.UUID
			item.CounterpartyCustomerID = &id
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const listOffBalanceAggregatesQuery = `
	SELECT category, position_code,
	       CASE WHEN position_code = '' THEN description ELSE '' END AS uraian,
	       SUM(amount), COUNT(*)
	FROM off_balance_items
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	GROUP BY category, position_code,
	         CASE WHEN position_code = '' THEN description ELSE '' END
	ORDER BY category, position_code, uraian`

// ListAggregates menjumlahkan baris AKTIF per kategori/pos yang as_of-nya berada
// pada bulan asOf. Pos tanpa sandi dipisah menurut uraian bank agar tidak tercampur;
// pos yang sudah bersandi digabung tanpa memecahnya per uraian. Satu query, bukan
// satu query per pos.
func (r *OffBalanceRepository) ListAggregates(ctx context.Context, asOf time.Time) ([]domain.OffBalanceAggregate, error) {
	rows, err := r.db.QueryContext(ctx, listOffBalanceAggregatesQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.OffBalanceAggregate
	for rows.Next() {
		var a domain.OffBalanceAggregate
		if err := rows.Scan(&a.Category, &a.PositionCode, &a.Description, &a.TotalAmount, &a.ItemCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertItemTx menyimpan satu pos (idempotensi lewat PRIMARY KEY id). Kolom NULL
// ditulis untuk lawan yang belum diisi; nilai kosong tetap ditulis agar perubahan
// "diisi -> dikosongkan" tercatat.
func (r *OffBalanceRepository) UpsertItemTx(ctx context.Context, tx any, item domain.OffBalanceItem, actorID uuid.UUID) error {
	sqlTx, err := requireOffBalanceTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO off_balance_items (
			id, position_code, category, description, amount,
			counterparty_customer_id, reference, as_of, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW(), $11, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			position_code = EXCLUDED.position_code,
			category = EXCLUDED.category,
			description = EXCLUDED.description,
			amount = EXCLUDED.amount,
			counterparty_customer_id = EXCLUDED.counterparty_customer_id,
			reference = EXCLUDED.reference,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.PositionCode, item.Category, item.Description, item.Amount,
		nullableOffBalanceUUID(item.CounterpartyCustomerID), item.Reference, item.AsOf,
		item.Status, item.Note, nullableOffBalanceActor(actorID))
	return err
}

// DeleteItemTx menghapus satu pos; found=false bila barisnya tidak ada.
func (r *OffBalanceRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireOffBalanceTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM off_balance_items WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireOffBalanceTx memastikan transaksi tulis sah, konsisten dengan pola
// repository kelembagaan/OJK lain.
func requireOffBalanceTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("rekening administratif: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableOffBalanceUUID menulis NULL untuk UUID yang belum diisi.
func nullableOffBalanceUUID(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return *id
}

// nullableOffBalanceActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya
// FK ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableOffBalanceActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.OffBalanceRepository = (*OffBalanceRepository)(nil)
