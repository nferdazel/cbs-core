package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// LPSPlacementRepository membaca penempatan pada bank lain yang menjadi dasar pengurang
// PPKA Pasal 23 POJK No. 1 Tahun 2024. Modul ini baca-saja; tidak ada penulisan di sini.
type LPSPlacementRepository struct {
	db *sql.DB
}

func NewLPSPlacementRepository(db *sql.DB) *LPSPlacementRepository {
	return &LPSPlacementRepository{db: db}
}

// listLPSPlacementsSelect mengambil penempatan yang tanggal penilaiannya tidak melewati
// asOf. Filter cabang ditambahkan pemanggil memakai branchReadClause; urutan deterministik
// diperlukan agar penolakan plafon dan ringkasan tidak berubah antar pemanggilan.
//
// Cast ::text wajib untuk kolom varchar/enum agar aman dipindai ke string.
const listLPSPlacementsSelect = `
	SELECT
		p.id,
		p.coa_code,
		p.counterparty_bank,
		p.placement_type::text,
		p.outstanding,
		p.lps_guaranteed,
		p.collectibility::text,
		p.as_of,
		COALESCE(b.code, '')
	FROM lps_placements p
	LEFT JOIN branches b ON b.id = p.branch_id`

func (r *LPSPlacementRepository) ListPlacements(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LPSPlacement, error) {
	args := []any{}
	branchFilter := ""
	if clause, branchArgs := branchReadClause("p.branch_id", actor); clause != "" {
		branchFilter = " AND " + clause
		args = append(args, branchArgs...)
	}
	// Placeholder asOf dibuat setelah argumen filter cabang agar penomoran posisi benar
	// saat klausa cabang memakai $1 (branchReadClause) maupun saat tidak ada filter.
	asOfPlaceholder := fmt.Sprintf("$%d", len(args)+1)
	args = append(args, asOf)

	query := listLPSPlacementsSelect +
		" WHERE p.as_of <= " + asOfPlaceholder + branchFilter +
		" ORDER BY p.counterparty_bank, p.id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.LPSPlacement
	for rows.Next() {
		var p domain.LPSPlacement
		var placementType, collectibility string
		if err := rows.Scan(
			&p.ID, &p.COACode, &p.CounterpartyBank, &placementType,
			&p.Outstanding, &p.LPSGuaranteed, &collectibility, &p.AsOf, &p.BranchCode,
		); err != nil {
			return nil, err
		}
		p.PlacementType = domain.LPSPlacementType(placementType)
		p.Collectibility = domain.LPSPlacementCollectibility(collectibility)
		list = append(list, p)
	}
	return list, rows.Err()
}

var _ domain.LPSPlacementRepository = (*LPSPlacementRepository)(nil)
