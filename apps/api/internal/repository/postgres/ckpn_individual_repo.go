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

// CKPNIndividualRepository menyimpan/membaca proyeksi arus kas manual CKPN individual
// (tahap T1). Ia TIDAK menyentuh loans.required_ckpn: T1 adalah mode bayangan baca-saja
// dan proyeksi ini data operasional bank (tabel loan_cashflow_projections, migrasi 000094).
type CKPNIndividualRepository struct {
	db *sql.DB
}

func NewCKPNIndividualRepository(db *sql.DB) *CKPNIndividualRepository {
	return &CKPNIndividualRepository{db: db}
}

var _ domain.CKPNIndividualRepository = (*CKPNIndividualRepository)(nil)

// ListCashflowProjections membaca proyeksi pada tanggal asOf, terurut periode.
func (r *CKPNIndividualRepository) ListCashflowProjections(ctx context.Context, loanID uuid.UUID, asOf time.Time) ([]domain.CKPNCashflowProjection, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT period, amount
		FROM loan_cashflow_projections
		WHERE loan_id = $1 AND as_of = $2
		ORDER BY period`, loanID, asOf.UTC())
	if err != nil {
		return nil, fmt.Errorf("membaca proyeksi arus kas CKPN individual: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.CKPNCashflowProjection
	for rows.Next() {
		var p domain.CKPNCashflowProjection
		if err := rows.Scan(&p.Period, &p.Amount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplaceCashflowProjections mengganti seluruh proyeksi kredit pada tanggal asOf dalam
// satu transaksi, sehingga tidak pernah ada campuran versi lama dan baru. Validasi sudah
// dilakukan pemanggil; di sini hanya penyimpanan.
func (r *CKPNIndividualRepository) ReplaceCashflowProjections(ctx context.Context, loanID uuid.UUID, asOf time.Time, projs []domain.CKPNCashflowProjection, createdBy string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM loan_cashflow_projections WHERE loan_id = $1 AND as_of = $2`,
		loanID, asOf.UTC()); err != nil {
		return fmt.Errorf("menghapus proyeksi arus kas lama: %w", err)
	}
	for _, p := range projs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO loan_cashflow_projections (loan_id, as_of, period, amount, source, created_by)
			VALUES ($1, $2, $3, $4, 'MANUAL', $5)`,
			loanID, asOf.UTC(), p.Period, p.Amount, createdBy); err != nil {
			return fmt.Errorf("menyimpan proyeksi arus kas periode %d: %w", p.Period, err)
		}
	}
	return tx.Commit()
}

// ListActiveCollaterals membaca agunan AKTIF kredit untuk NRV (T2): taksasi, haircut,
// bound_amount, dan biaya pelepasan. Hanya status ACTIVE — agunan RELEASED/EXECUTED
// tidak menjamin apa pun. BoundAmount dibaca apa adanya karena dihitung basis data.
func (r *CKPNIndividualRepository) ListActiveCollaterals(ctx context.Context, loanID uuid.UUID) ([]domain.CKPNIndividualCollateral, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, collateral_type::text, description,
		       appraisal_value, haircut_percent, bound_amount, disposal_cost_amount
		FROM loan_collaterals
		WHERE loan_id = $1 AND status = 'ACTIVE'
		ORDER BY created_at, id`, loanID)
	if err != nil {
		return nil, fmt.Errorf("membaca agunan aktif untuk CKPN individual: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.CKPNIndividualCollateral{}
	for rows.Next() {
		var c domain.CKPNIndividualCollateral
		var cost sql.NullString
		if err := rows.Scan(&c.ID, &c.CollateralType, &c.Description,
			&c.AppraisalValue, &c.HaircutPercent, &c.BoundAmount, &cost); err != nil {
			return nil, fmt.Errorf("memindai agunan CKPN individual: %w", err)
		}
		if cost.Valid {
			d, err := decimal.NewFromString(cost.String)
			if err != nil {
				return nil, fmt.Errorf("membaca biaya pelepasan agunan %s: %w", c.ID, err)
			}
			c.DisposalCostAmount = &d
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetDisposalCost menyimpan estimasi biaya pelepasan satu agunan. cost nil berarti
// mengosongkan (NRV tanpa pengurangan); nilai tidak boleh negatif (sudah dicegah
// service, dan ditolak sekali lagi oleh CHECK basis data).
func (r *CKPNIndividualRepository) SetDisposalCost(ctx context.Context, collateralID uuid.UUID, cost *decimal.Decimal, updatedBy string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE loan_collaterals
		SET disposal_cost_amount = $2, updated_by = NULLIF($3, '')
		WHERE id = $1`, collateralID, cost, updatedBy)
	if err != nil {
		return fmt.Errorf("menyimpan biaya pelepasan agunan: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("agunan %s tidak ditemukan", collateralID)
	}
	return nil
}

// ListEntryScanCandidates membaca kredit AKTIF untuk pemindaian pintu masuk individual
// (T3), terurut sisa pokok terbesar dahulu sehingga peringkat eksposur dapat dihitung
// pemanggil. Hanya kolom yang dibutuhkan penilaian pintu masuk yang dibaca.
func (r *CKPNIndividualRepository) ListEntryScanCandidates(ctx context.Context, actor domain.Actor) ([]domain.CKPNIndividualScanRow, error) {
	where, args := branchReadClause("loans.branch_id", actor)
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = loans.product_id)"
	if clause, cargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		args = append(args, cargs...)
		where = andCondition(where, clause)
	}
	query := `
		SELECT loans.id, loans.loan_number, COALESCE(b.code, ''),
		       loans.outstanding_principal, loans.collectibility::text, loans.dpd,
		       loans.is_restructured, loans.ckpn_objective_evidence, loans.ckpn_method,
		       EXISTS (SELECT 1 FROM loan_collaterals lc
		               WHERE lc.loan_id = loans.id AND lc.status = 'ACTIVE')
		FROM loans
		LEFT JOIN branches b ON b.id = loans.branch_id`
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY loans.outstanding_principal DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("membaca kandidat pintu masuk CKPN individual: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []domain.CKPNIndividualScanRow{}
	for rows.Next() {
		var row domain.CKPNIndividualScanRow
		var kol string
		if err := rows.Scan(&row.LoanID, &row.LoanNumber, &row.BranchCode,
			&row.Outstanding, &kol, &row.DPD, &row.IsRestructured,
			&row.ObjectiveEvidence, &row.Method, &row.HasCollateral); err != nil {
			return nil, fmt.Errorf("memindai kandidat CKPN individual: %w", err)
		}
		row.Collectibility = domain.OJKCollectibility(kol)
		out = append(out, row)
	}
	return out, rows.Err()
}

// MarkEntry menandai hasil keputusan pintu masuk pada satu kredit (T3): metode,
// penanda signifikansi, dan penanda bukti objektif. required_ckpn TIDAK disentuh.
// updated_by dicatat untuk jejak siapa yang memutuskan.
func (r *CKPNIndividualRepository) MarkEntry(ctx context.Context, loanID uuid.UUID, method domain.CKPNIndividualMethod, significant, objectiveEvidence bool, updatedBy string) error {
	if !method.Valid() {
		return fmt.Errorf("metode CKPN individual %q tidak dikenal", method)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE loans
		SET ckpn_method = $2, ckpn_significant = $3, ckpn_objective_evidence = $4
		WHERE id = $1`, loanID, string(method), significant, objectiveEvidence)
	if err != nil {
		return fmt.Errorf("menandai pintu masuk CKPN individual: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("kredit %s tidak ditemukan", loanID)
	}
	_ = updatedBy // kolom updated_by tidak ada pada loans; jejak memakai tabel assessment.
	return nil
}

// RecordAssessmentTrail menulis jejak audit keputusan pintu masuk/perhitungan ke
// loan_ckpn_individual_assessments (tabel T0). Tidak menyentuh required_ckpn.
func (r *CKPNIndividualRepository) RecordAssessmentTrail(ctx context.Context, loanID uuid.UUID, asOf time.Time, method domain.CKPNIndividualMethod, carrying, pv, nrv, target decimal.Decimal, basis string, decidedBy string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO loan_ckpn_individual_assessments
			(loan_id, as_of, method, carrying_amount, present_value, collateral_nrv,
			 target, basis, decided_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::jsonb, NULLIF($9, ''))`,
		loanID, asOf.UTC(), string(method), carrying, pv, nrv, target, basis, decidedBy)
	if err != nil {
		return fmt.Errorf("menulis jejak penilaian CKPN individual: %w", err)
	}
	return nil
}
