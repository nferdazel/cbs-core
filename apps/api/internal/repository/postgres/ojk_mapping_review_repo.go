package postgres

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/ojkreport"
	"github.com/google/uuid"
)

// OJKMappingReviewRepository menyimpan keputusan bank atas baris pemetaan COA ke pos
// laporan OJK. Keputusan disimpan di basis data (migrasi 000050), bukan di berkas kode,
// supaya bertahan lintas deploy dan dapat diaudit: siapa memutuskan apa dan kapan.
type OJKMappingReviewRepository struct {
	db *sql.DB
}

func NewOJKMappingReviewRepository(db *sql.DB) *OJKMappingReviewRepository {
	return &OJKMappingReviewRepository{db: db}
}

// ListReviews membaca seluruh keputusan. Jumlah baris pemetaan terbatas (±60), jadi
// pembacaan sekaligus tidak perlu paginasi.
func (r *OJKMappingReviewRepository) ListReviews(ctx context.Context) ([]ojkreport.MappingReview, error) {
	const query = `
		SELECT form, coa_code, decision, note, decided_by,
		       COALESCE(decided_by_id::text, ''), decided_at
		FROM ojk_mapping_reviews
		ORDER BY form, coa_code`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ojkreport.MappingReview
	for rows.Next() {
		var rev ojkreport.MappingReview
		if err := rows.Scan(&rev.Form, &rev.COACode, &rev.Decision, &rev.Note,
			&rev.DecidedBy, &rev.DecidedByID, &rev.DecidedAt); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertReview menyimpan keputusan terbaru untuk satu (form, coa_code). Indeks unik pada
// migrasi 000050 membuat pengiriman ulang bersifat idempoten: baris yang sama diperbarui,
// bukan digandakan. decided_at selalu keputusan terakhir sehingga urutan audit tetap jelas.
func (r *OJKMappingReviewRepository) UpsertReview(ctx context.Context, review ojkreport.MappingReview) error {
	// decided_by_id UUID: kosong berarti pelaku tidak punya uuid (mis. pekerjaan uji),
	// bukan nilai nol yang menyesatkan relasi.
	var decidedByID any
	if review.DecidedByID != "" {
		if id, err := uuid.Parse(review.DecidedByID); err == nil {
			decidedByID = id
		}
	}

	const query = `
		INSERT INTO ojk_mapping_reviews
			(form, coa_code, decision, note, decided_by, decided_by_id, decided_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (form, coa_code) DO UPDATE SET
			decision      = EXCLUDED.decision,
			note          = EXCLUDED.note,
			decided_by    = EXCLUDED.decided_by,
			decided_by_id = EXCLUDED.decided_by_id,
			decided_at    = EXCLUDED.decided_at,
			updated_at    = EXCLUDED.updated_at`

	_, err := r.db.ExecContext(ctx, query,
		review.Form, review.COACode, string(review.Decision), review.Note,
		review.DecidedBy, decidedByID, review.DecidedAt)
	return err
}
