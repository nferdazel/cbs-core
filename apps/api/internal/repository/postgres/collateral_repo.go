package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CollateralRepository menyimpan agunan kredit. bound_amount tidak pernah ditulis dari
// sini: kolom itu dihitung database, jadi nilai pengurang selalu konsisten dengan taksasi
// dan haircut yang tersimpan.
type CollateralRepository struct {
	db *sql.DB
}

func NewCollateralRepository(db *sql.DB) *CollateralRepository {
	return &CollateralRepository{db: db}
}

const collateralColumns = `id, loan_id, branch_id, collateral_type, description, document_number,
	owner_name, appraisal_value, appraisal_date, COALESCE(appraiser, ''), haircut_percent,
	bound_amount, status, released_at, COALESCE(notes, ''), created_by, created_at,
	COALESCE(updated_by, ''), updated_at`

func scanCollateral(row rowScanner) (*domain.LoanCollateral, error) {
	var c domain.LoanCollateral
	var branchID uuid.NullUUID
	var releasedAt sql.NullTime
	var updatedBy sql.NullString
	if err := row.Scan(
		&c.ID, &c.LoanID, &branchID, &c.CollateralType, &c.Description, &c.DocumentNumber,
		&c.OwnerName, &c.AppraisalValue, &c.AppraisalDate, &c.Appraiser, &c.HaircutPercent,
		&c.BoundAmount, &c.Status, &releasedAt, &c.Notes, &c.CreatedBy, &c.CreatedAt,
		&updatedBy, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if branchID.Valid {
		id := branchID.UUID
		c.BranchID = &id
	}
	if releasedAt.Valid {
		t := releasedAt.Time
		c.ReleasedAt = &t
	}
	if updatedBy.Valid {
		c.UpdatedBy = updatedBy.String
	}
	return &c, nil
}

func (r *CollateralRepository) Create(ctx context.Context, c *domain.LoanCollateral) error {
	query := `
		INSERT INTO loan_collaterals (
			loan_id, branch_id, collateral_type, description, document_number, owner_name,
			appraisal_value, appraisal_date, appraiser, haircut_percent, status, notes,
			created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10, $11, NULLIF($12, ''), $13, NOW(), NOW())
		RETURNING id, bound_amount, created_at, updated_at
	`
	// bound_amount tidak ada pada daftar INSERT karena dihitung database; nilainya
	// dibaca kembali lewat RETURNING, bukan ditaksir di aplikasi.
	err := r.db.QueryRowContext(ctx, query,
		c.LoanID, c.BranchID, c.CollateralType, c.Description, c.DocumentNumber, c.OwnerName,
		c.AppraisalValue, c.AppraisalDate, c.Appraiser, c.HaircutPercent, c.Status, c.Notes,
		c.CreatedBy,
	).Scan(&c.ID, &c.BoundAmount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("menyimpan agunan: %w", err)
	}
	return nil
}

func (r *CollateralRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoanCollateral, error) {
	query := `SELECT ` + collateralColumns + ` FROM loan_collaterals WHERE id = $1`
	c, err := scanCollateral(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCollateralNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mengambil agunan: %w", err)
	}
	return c, nil
}

func (r *CollateralRepository) ListByLoan(ctx context.Context, loanID uuid.UUID) ([]domain.LoanCollateral, error) {
	query := `SELECT ` + collateralColumns + ` FROM loan_collaterals WHERE loan_id = $1
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, loanID)
	if err != nil {
		return nil, fmt.Errorf("mendaftar agunan kredit: %w", err)
	}
	defer rows.Close()

	list := make([]domain.LoanCollateral, 0)
	for rows.Next() {
		c, err := scanCollateral(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *CollateralRepository) Update(ctx context.Context, c *domain.LoanCollateral) error {
	query := `
		UPDATE loan_collaterals SET
			collateral_type = $2, description = $3, document_number = $4, owner_name = $5,
			appraisal_value = $6, appraisal_date = $7, appraiser = NULLIF($8, ''),
			haircut_percent = $9, status = $10, released_at = $11, notes = NULLIF($12, ''),
			updated_by = $13, updated_at = NOW()
		WHERE id = $1
		RETURNING bound_amount, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		c.ID, c.CollateralType, c.Description, c.DocumentNumber, c.OwnerName,
		c.AppraisalValue, c.AppraisalDate, c.Appraiser, c.HaircutPercent, c.Status,
		c.ReleasedAt, c.Notes, c.UpdatedBy,
	).Scan(&c.BoundAmount, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrCollateralNotFound
	}
	if err != nil {
		return fmt.Errorf("mengubah agunan: %w", err)
	}
	return nil
}

func (r *CollateralRepository) SumActiveBoundByLoan(ctx context.Context, loanIDs []uuid.UUID) (map[uuid.UUID]decimal.Decimal, error) {
	total := make(map[uuid.UUID]decimal.Decimal, len(loanIDs))
	if len(loanIDs) == 0 {
		return total, nil
	}

	// Placeholder disusun dari jumlah kredit, bukan memakai tipe array milik driver:
	// nilainya tetap dikirim sebagai parameter dan tidak ada yang dirangkai ke dalam SQL.
	placeholders := make([]string, 0, len(loanIDs))
	args := make([]any, 0, len(loanIDs))
	for i, id := range loanIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	query := `SELECT loan_id, COALESCE(SUM(bound_amount), 0) FROM loan_collaterals
		WHERE status = 'ACTIVE' AND loan_id IN (` + strings.Join(placeholders, ", ") + `)
		GROUP BY loan_id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("menjumlahkan agunan aktif: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var loanID uuid.UUID
		var sum decimal.Decimal
		if err := rows.Scan(&loanID, &sum); err != nil {
			return nil, err
		}
		total[loanID] = sum
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return total, nil
}
