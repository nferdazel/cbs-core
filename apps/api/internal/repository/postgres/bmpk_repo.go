package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// BMPKRepository membaca fondasi BMPK (migrasi 000103): penandaan pihak terkait,
// paparan gabungan kredit + penempatan, dan batas per pihak. Seluruh pembacaan
// bank-wide; penyaringan kewenangan dilakukan layanan sebelum repo dipanggil.
type BMPKRepository struct {
	db *sql.DB
}

func NewBMPKRepository(db *sql.DB) *BMPKRepository {
	return &BMPKRepository{db: db}
}

// listBMPKPartyExposuresQuery mengembalikan SATU baris per pihak terkait. Paparan
// kredit dan penempatan digabung di subquery UNION ALL lalu di-GROUP BY customer_id,
// sehingga tidak ada query per pihak (bukan N+1). Subquery hanya memuat kredit ber-
// eksposur berjalan (DISBURSED/DEFAULTED, sama dengan domain.LoanStatus.IsCKPNActive)
// dan penempatan yang sudah ditautkan ke nasabah (customer_id NOT NULL).
const listBMPKPartyExposuresQuery = `
	SELECT rp.customer_id,
	       rp.relationship_type,
	       COALESCE(e.loan_exposure, 0)::numeric      AS loan_exposure,
	       COALESCE(e.placement_exposure, 0)::numeric AS placement_exposure,
	       l.max_amount,
	       l.effective_date,
	       (l.customer_id IS NOT NULL)                AS has_limit
	FROM bmpk_related_parties rp
	LEFT JOIN bmpk_limits l ON l.customer_id = rp.customer_id
	LEFT JOIN (
		SELECT customer_id,
		       SUM(loan_exposure)      AS loan_exposure,
		       SUM(placement_exposure) AS placement_exposure
		FROM (
			SELECT ln.customer_id,
			       ln.outstanding_principal AS loan_exposure,
			       0::numeric               AS placement_exposure
			FROM loans ln
			WHERE ln.status IN ('DISBURSED', 'DEFAULTED')
			UNION ALL
			SELECT p.customer_id,
			       0::numeric     AS loan_exposure,
			       p.outstanding  AS placement_exposure
			FROM lps_placements p
			WHERE p.customer_id IS NOT NULL
		) src
		GROUP BY customer_id
	) e ON e.customer_id = rp.customer_id
	ORDER BY rp.customer_id`

// ListPartyExposures membaca seluruh pihak terkait beserta paparan dan batasnya dalam
// satu query agregat. Nama nasabah tidak diambil di sini (terenkripsi); layanan
// melengkapinya lewat pembacaan nama batch.
func (r *BMPKRepository) ListPartyExposures(ctx context.Context) ([]domain.BMPKPartyExposure, error) {
	rows, err := r.db.QueryContext(ctx, listBMPKPartyExposuresQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.BMPKPartyExposure
	for rows.Next() {
		var (
			e             domain.BMPKPartyExposure
			limitAmount   decimal.NullDecimal
			effectiveDate sql.NullTime
			hasLimit      bool
		)
		if err := rows.Scan(
			&e.CustomerID,
			&e.RelationshipType,
			&e.LoanExposure,
			&e.PlacementExposure,
			&limitAmount,
			&effectiveDate,
			&hasLimit,
		); err != nil {
			return nil, err
		}
		e.HasLimit = hasLimit
		if limitAmount.Valid {
			e.LimitAmount = limitAmount.Decimal
		}
		if effectiveDate.Valid {
			t := effectiveDate.Time
			e.LimitEffectiveDate = &t
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var _ domain.BMPKRepository = (*BMPKRepository)(nil)

const listBMPKRelatedPartiesQuery = `
	SELECT customer_id, relationship_type, COALESCE(note, '')
	FROM bmpk_related_parties
	ORDER BY customer_id`

// ListRelatedParties membaca seluruh penandaan pihak terkait (data mentah UI),
// deterministik menurut customer_id.
func (r *BMPKRepository) ListRelatedParties(ctx context.Context) ([]domain.BMPKRelatedParty, error) {
	rows, err := r.db.QueryContext(ctx, listBMPKRelatedPartiesQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.BMPKRelatedParty
	for rows.Next() {
		var p domain.BMPKRelatedParty
		if err := rows.Scan(&p.CustomerID, &p.RelationshipType, &p.Note); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const listBMPKLimitsQuery = `
	SELECT customer_id, max_amount, effective_date, COALESCE(note, '')
	FROM bmpk_limits
	ORDER BY customer_id`

// ListLimits membaca seluruh batas per nasabah (data mentah UI), deterministik
// menurut customer_id.
func (r *BMPKRepository) ListLimits(ctx context.Context) ([]domain.BMPKLimit, error) {
	rows, err := r.db.QueryContext(ctx, listBMPKLimitsQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.BMPKLimit
	for rows.Next() {
		var (
			l       domain.BMPKLimit
			effDate sql.NullTime
		)
		if err := rows.Scan(&l.CustomerID, &l.MaxAmount, &effDate, &l.Note); err != nil {
			return nil, err
		}
		if effDate.Valid {
			t := effDate.Time
			l.EffectiveDate = &t
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertRelatedPartyTx menimpa baris satu nasabah (invarian satu baris per nasabah).
func (r *BMPKRepository) UpsertRelatedPartyTx(ctx context.Context, tx any, p domain.BMPKRelatedParty) error {
	sqlTx, err := requireBMPKTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO bmpk_related_parties (customer_id, relationship_type, note, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (customer_id) DO UPDATE SET
			relationship_type = EXCLUDED.relationship_type,
			note = EXCLUDED.note,
			updated_at = NOW()`,
		p.CustomerID, p.RelationshipType, p.Note)
	return err
}

// DeleteRelatedPartyTx menghapus penandaan satu nasabah; found=false bila tak ada.
func (r *BMPKRepository) DeleteRelatedPartyTx(ctx context.Context, tx any, customerID uuid.UUID) (bool, error) {
	sqlTx, err := requireBMPKTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM bmpk_related_parties WHERE customer_id = $1`, customerID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpsertLimitTx menimpa batas satu nasabah (invarian satu baris per nasabah).
func (r *BMPKRepository) UpsertLimitTx(ctx context.Context, tx any, l domain.BMPKLimit) error {
	sqlTx, err := requireBMPKTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO bmpk_limits (customer_id, max_amount, effective_date, note, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (customer_id) DO UPDATE SET
			max_amount = EXCLUDED.max_amount,
			effective_date = EXCLUDED.effective_date,
			note = EXCLUDED.note,
			updated_at = NOW()`,
		l.CustomerID, l.MaxAmount, nullableBMPKTime(l.EffectiveDate), l.Note)
	return err
}

// DeleteLimitTx menghapus batas satu nasabah; found=false bila tak ada.
func (r *BMPKRepository) DeleteLimitTx(ctx context.Context, tx any, customerID uuid.UUID) (bool, error) {
	sqlTx, err := requireBMPKTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM bmpk_limits WHERE customer_id = $1`, customerID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireBMPKTx memastikan transaksi tulis sah, konsisten dengan pola repository
// kelembagaan/rekening administratif.
func requireBMPKTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("BMPK: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableBMPKTime menulis NULL untuk tanggal efektif yang belum dicatat.
func nullableBMPKTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
