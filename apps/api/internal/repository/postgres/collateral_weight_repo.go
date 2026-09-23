package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CollateralWeightRepository membaca/menyimpan gerbang aktivasi bobot risiko agunan
// (Lampiran II SEOJK No. 2/SEOJK.03/2025). Ia TIDAK dipakai perhitungan ATMR: bobot
// tetap 100% sampai sebuah kategori lolos gerbang dan dinyalakan lewat service.
type CollateralWeightRepository struct {
	db *sql.DB
}

func NewCollateralWeightRepository(db *sql.DB) *CollateralWeightRepository {
	return &CollateralWeightRepository{db: db}
}

var _ domain.CollateralWeightRepository = (*CollateralWeightRepository)(nil)

// GetCategory membaca satu kategori beserta penanda persetujuannya. Kategori tak
// ditemukan dikembalikan sebagai galat, bukan kategori kosong diam-diam.
func (r *CollateralWeightRepository) GetCategory(ctx context.Context, categoryCode string) (*domain.CollateralWeightCategory, error) {
	var (
		c              domain.CollateralWeightCategory
		collateralType sql.NullString
		bindingType    sql.NullString
		policyNumber   sql.NullString
		policyDate     sql.NullTime
		shadowStarted  sql.NullTime
		maker          sql.NullString
		activatedBy    sql.NullString
		activatedAt    sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT category_code, lampiran_ii_item, official_weight_frac, applied_weight_frac,
		       enabled, collateral_type::text, binding_type::text,
		       direksi_policy_number, direksi_policy_date, shadow_started_at,
		       activated_maker, activated_by, activated_at
		FROM collateral_lampiran_ii_weights
		WHERE category_code = $1`, categoryCode).Scan(
		&c.CategoryCode, &c.LampiranIIItem, &c.OfficialWeightFrac, &c.AppliedWeightFrac,
		&c.Enabled, &collateralType, &bindingType,
		&policyNumber, &policyDate, &shadowStarted,
		&maker, &activatedBy, &activatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("kategori bobot agunan %q tidak ditemukan", categoryCode)
	}
	if err != nil {
		return nil, fmt.Errorf("membaca kategori bobot agunan %q: %w", categoryCode, err)
	}
	if collateralType.Valid {
		t := domain.CollateralType(collateralType.String)
		c.CollateralType = &t
	}
	if bindingType.Valid {
		b := domain.CollateralBinding(bindingType.String)
		c.BindingType = &b
	}
	if policyNumber.Valid {
		c.DireksiPolicyNumber = policyNumber.String
	}
	if policyDate.Valid {
		t := policyDate.Time
		c.DireksiPolicyDate = &t
	}
	if shadowStarted.Valid {
		t := shadowStarted.Time
		c.ShadowStartedAt = &t
	}
	if maker.Valid {
		c.ActivatedMaker = maker.String
	}
	if activatedBy.Valid {
		c.ActivatedBy = activatedBy.String
	}
	if activatedAt.Valid {
		t := activatedAt.Time
		c.ActivatedAt = &t
	}
	return &c, nil
}

// ListActiveCollateralsWithExposure mengembalikan seluruh agunan AKTIF beserta sisa
// pokok kreditnya. Hanya ACTIVE yang dihitung: agunan lepas/eksekusi tidak menjamin
// apa pun. Eksposur dipakai gerbang C6.
func (r *CollateralWeightRepository) ListActiveCollateralsWithExposure(ctx context.Context) ([]domain.CollateralWeightCollateral, map[uuid.UUID]decimal.Decimal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT lc.loan_id, lc.collateral_type::text, lc.binding_type::text, lc.disputed,
		       lc.insurance_expiry_date, lc.appraisal_date, lc.appraisal_valid_until,
		       lc.appraisal_value, lc.bound_amount, lc.status::text, l.outstanding_principal
		FROM loan_collaterals lc
		JOIN loans l ON l.id = lc.loan_id
		WHERE lc.status = 'ACTIVE'`)
	if err != nil {
		return nil, nil, fmt.Errorf("membaca agunan aktif untuk gerbang bobot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []domain.CollateralWeightCollateral
	exposure := map[uuid.UUID]decimal.Decimal{}
	for rows.Next() {
		var (
			c           domain.CollateralWeightCollateral
			binding     sql.NullString
			insurance   sql.NullTime
			validUntil  sql.NullTime
			status      string
			outstanding decimal.Decimal
		)
		if err := rows.Scan(
			&c.LoanID, &c.CollateralType, &binding, &c.Disputed,
			&insurance, &c.AppraisalDate, &validUntil,
			&c.AppraisalValue, &c.BoundAmount, &status, &outstanding,
		); err != nil {
			return nil, nil, err
		}
		if binding.Valid {
			b := domain.CollateralBinding(binding.String)
			c.BindingType = &b
		}
		if insurance.Valid {
			t := insurance.Time
			c.InsuranceExpiryDate = &t
		}
		if validUntil.Valid {
			t := validUntil.Time
			c.AppraisalValidUntil = &t
		}
		c.Status = domain.CollateralStatus(status)
		list = append(list, c)
		exposure[c.LoanID] = outstanding
	}
	return list, exposure, rows.Err()
}

// EnableCategory menyalakan kategori dan menyimpan bukti persetujuannya. Pemanggil
// WAJIB memastikan gerbang lolos; fungsi ini tidak menilai ulang. Nilai applied
// dipaksa sama dengan bobot resmi oleh service (C7), bukan diterima dari pengguna.
func (r *CollateralWeightRepository) EnableCategory(ctx context.Context, categoryCode string, approval domain.CollateralWeightApproval) error {
	return enableCategory(ctx, r.db, categoryCode, approval)
}

// EnableCategoryTx menyalakan kategori memakai transaksi pemanggil, sehingga penulisan
// bukti aktivasi dan audit persetujuan maker-checker commit bersama.
func (r *CollateralWeightRepository) EnableCategoryTx(ctx context.Context, tx any, categoryCode string, approval domain.CollateralWeightApproval) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("aktivasi bobot agunan: transaksi tidak valid")
	}
	return enableCategory(ctx, sqlTx, categoryCode, approval)
}

// dbExecer adalah irisan *sql.DB dan *sql.Tx yang cukup untuk satu UPDATE.
type dbExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func enableCategory(ctx context.Context, exec dbExecer, categoryCode string, approval domain.CollateralWeightApproval) error {
	res, err := exec.ExecContext(ctx, `
		UPDATE collateral_lampiran_ii_weights
		SET applied_weight_frac = $2,
		    enabled = TRUE,
		    direksi_policy_number = $3,
		    direksi_policy_date = $4,
		    activated_maker = $5,
		    activated_by = $6,
		    activated_at = $7
		WHERE category_code = $1`,
		categoryCode, approval.AppliedWeightFrac,
		approval.DireksiPolicyNumber, approval.DireksiPolicyDate,
		approval.Maker, approval.Checker, approval.ActivatedAt.UTC())
	if err != nil {
		return fmt.Errorf("mengaktifkan kategori bobot agunan %q: %w", categoryCode, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("kategori bobot agunan %q tidak ditemukan saat aktivasi", categoryCode)
	}
	return nil
}
