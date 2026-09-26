package postgres

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
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
