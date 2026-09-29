package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// AgunanRepository membaca agunan untuk Form 06.01 dan menyimpan kolom Form 06.01. Ini
// bukan tabel baru: sumbernya loan_collaterals (migrasi 000036) yang kolom Form 06.01-nya
// ditambahkan migrasi 000125.
type AgunanRepository struct {
	db *sql.DB
}

func NewAgunanRepository(db *sql.DB) *AgunanRepository {
	return &AgunanRepository{db: db}
}

// listAgunanForOJKQuery membaca hanya agunan AKTIF beserta nomor rekening fasilitas.
// Kolom Form 06.01 yang belum diisi tetap NULL dan dipetakan ke nilai netral di pemanggil.
const listAgunanForOJKQuery = `
	SELECT lc.id, l.loan_number,
	       COALESCE(lc.ojk_register_number, ''),
	       COALESCE(lc.ojk_collateral_type_code, ''),
	       COALESCE(lc.ojk_collateral_address, ''),
	       COALESCE(lc.ojk_bound_value, 0),
	       lc.appraisal_value, lc.appraisal_date,
	       COALESCE(lc.ojk_appraiser_code, ''),
	       COALESCE(lc.ojk_ppka_amount, 0),
	       lc.status
	FROM loan_collaterals lc
	JOIN loans l ON l.id = lc.loan_id
	WHERE lc.status = 'ACTIVE'
	ORDER BY l.loan_number, lc.ojk_register_number NULLS LAST, lc.id`

// ListAgunanForOJK membaca agunan AKTIF untuk Form 06.01.
func (r *AgunanRepository) ListAgunanForOJK(ctx context.Context) ([]domain.AgunanRow, error) {
	rows, err := r.db.QueryContext(ctx, listAgunanForOJKQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.AgunanRow
	for rows.Next() {
		var item domain.AgunanRow
		if err := rows.Scan(
			&item.ID, &item.LoanNumber, &item.KodeRegister, &item.JenisAgunanCode,
			&item.AlamatAgunan, &item.NilaiDiagunkan, &item.NilaiAgunan, &item.TanggalPenilaian,
			&item.PenilaiCode, &item.PPKAAmount, &item.Status,
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

// UpdateOjkColumnsTx menyimpan kolom Form 06.01 satu agunan AKTIF. Pelanggaran UNIQUE
// kode register dipetakan ke ErrAgunanRegisterUsed (PDF #171 no reuse/no recycle).
// found=false bila agunan tidak ada atau tidak AKTIF.
func (r *AgunanRepository) UpdateOjkColumnsTx(ctx context.Context, tx any, id uuid.UUID, in domain.UpdateAgunanInput) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return false, fmt.Errorf("agunan Form 06.01: transaksi tulis tidak sah")
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE loan_collaterals SET
			ojk_register_number = $2,
			ojk_collateral_type_code = $3,
			ojk_collateral_address = $4,
			ojk_bound_value = $5,
			appraisal_value = $6,
			ojk_appraiser_code = $7,
			appraisal_date = $8,
			ojk_ppka_amount = $9,
			updated_at = NOW()
		WHERE id = $1 AND status = 'ACTIVE'`,
		id, in.KodeRegister, in.JenisAgunanCode, in.AlamatAgunan, in.NilaiDiagunkan,
		in.NilaiAgunan, in.PenilaiCode, mustParseAgunanDate(in.TanggalPenilaian), in.PPKAAmount)
	if err != nil {
		if isAgunanRegisterViolation(err) {
			return false, domain.ErrAgunanRegisterUsed
		}
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// mustParseAgunanDate mengurai tanggal yang sudah divalidasi BuildAgunanUpdate; pada jalur
// ini format dijamin YYYY-MM-DD sehingga kegagalan tidak mungkin terjadi.
func mustParseAgunanDate(raw string) time.Time {
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

// isAgunanRegisterViolation mengenali pelanggaran UNIQUE kode register agunan
// (SQLSTATE 23505) dengan menyaring nama indeks agar galat unique lain tidak salah dipetakan.
func isAgunanRegisterViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "uq_loan_collaterals_ojk_register_number"
}

var _ domain.AgunanRepository = (*AgunanRepository)(nil)
