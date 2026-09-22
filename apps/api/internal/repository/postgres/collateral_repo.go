package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
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

// collateralColumns dipakai bersama GetByID dan ListByLoan, jadi keduanya pasti membaca
// kolom yang sama. Kode cabang diambil lewat join karena pemeriksaan akses memakai kode,
// bukan id: tanpa ini pegawai cabang tidak dapat melihat agunan cabangnya sendiri.
const collateralColumns = `lc.id, lc.loan_id, lc.branch_id, lc.collateral_type, lc.description,
	lc.document_number, lc.owner_name, lc.appraisal_value, lc.appraisal_date,
	COALESCE(lc.appraiser, ''), lc.haircut_percent, lc.bound_amount, lc.status, lc.released_at,
	COALESCE(lc.notes, ''), lc.created_by, lc.created_at, COALESCE(lc.updated_by, ''), lc.updated_at,
	COALESCE(b.code, ''),
	lc.appraiser_independent, lc.certified, lc.mortgaged, lc.mortgage_value,
	lc.exists_known, lc.executable, lc.third_party_owner, lc.owner_consent,
	lc.is_cash, lc.cash_account_id, lc.warehouse_receipt_valued_at,
	lc.njop_value, lc.njop_date, lc.njop_source,
	lc.bumn_bumd_criteria_met, COALESCE(lc.bumn_bumd_evidence, '')`

const collateralFrom = `FROM loan_collaterals lc LEFT JOIN branches b ON b.id = lc.branch_id`

func scanCollateral(row rowScanner) (*domain.LoanCollateral, error) {
	var c domain.LoanCollateral
	var branchID uuid.NullUUID
	var releasedAt sql.NullTime
	var updatedBy sql.NullString
	var cashAccountID uuid.NullUUID
	var warehouseReceiptValuedAt sql.NullTime
	var njopDate sql.NullTime
	var njopSource sql.NullString
	if err := row.Scan(
		&c.ID, &c.LoanID, &branchID, &c.CollateralType, &c.Description, &c.DocumentNumber,
		&c.OwnerName, &c.AppraisalValue, &c.AppraisalDate, &c.Appraiser, &c.HaircutPercent,
		&c.BoundAmount, &c.Status, &releasedAt, &c.Notes, &c.CreatedBy, &c.CreatedAt,
		&updatedBy, &c.UpdatedAt, &c.BranchCode,
		&c.AppraiserIndependent, &c.Certified, &c.Mortgaged, &c.MortgageValue,
		&c.ExistsKnown, &c.Executable, &c.ThirdPartyOwner, &c.OwnerConsent,
		&c.IsCash, &cashAccountID, &warehouseReceiptValuedAt,
		&c.NJOPValue, &njopDate, &njopSource,
		&c.BumnBumdCriteriaMet, &c.BumnBumdEvidence,
	); err != nil {
		return nil, err
	}
	if branchID.Valid {
		id := branchID.UUID
		c.BranchID = &id
	}
	if cashAccountID.Valid {
		id := cashAccountID.UUID
		c.CashAccountID = &id
	}
	if warehouseReceiptValuedAt.Valid {
		t := warehouseReceiptValuedAt.Time
		c.WarehouseReceiptValuedAt = &t
	}
	if njopDate.Valid {
		t := njopDate.Time
		c.NJOPDate = &t
	}
	if njopSource.Valid {
		s := domain.NJOPSource(njopSource.String)
		c.NJOPSource = &s
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

func (r *CollateralRepository) Create(ctx context.Context, c *domain.LoanCollateral, branchCode string) error {
	query := `
		INSERT INTO loan_collaterals (
			loan_id, branch_id, collateral_type, description, document_number, owner_name,
			appraisal_value, appraisal_date, appraiser, haircut_percent, status, notes,
			certified, mortgaged, mortgage_value, appraiser_independent,
			exists_known, executable, third_party_owner, owner_consent,
			is_cash, cash_account_id, warehouse_receipt_valued_at,
			njop_value, njop_date, njop_source,
			bumn_bumd_criteria_met, bumn_bumd_evidence,
			created_by, created_at, updated_at
		)
		VALUES ($1, (SELECT id FROM branches WHERE code = $2), $3, $4, $5, $6, $7, $8, NULLIF($9, ''),
		        $10, $11, NULLIF($12, ''), $13, $14, $15, $16, $17, $18, $19, $20,
		        $21, $22, $23, $24, $25, $26, $27, NULLIF($28, ''), $29, NOW(), NOW())
		RETURNING id, bound_amount, created_at, updated_at
	`
	// bound_amount tidak ada pada daftar INSERT karena dihitung database; nilainya
	// dibaca kembali lewat RETURNING, bukan ditaksir di aplikasi.
	// Cabang di-resolve dari kode lewat subquery, sama seperti posting jurnal, supaya
	// service tidak perlu bergantung pada repositori cabang hanya untuk satu lookup.
	err := r.db.QueryRowContext(ctx, query,
		c.LoanID, branchCode, c.CollateralType, c.Description, c.DocumentNumber, c.OwnerName,
		c.AppraisalValue, c.AppraisalDate, c.Appraiser, c.HaircutPercent, c.Status, c.Notes,
		c.Certified, c.Mortgaged, c.MortgageValue, c.AppraiserIndependent,
		c.ExistsKnown, c.Executable, c.ThirdPartyOwner, c.OwnerConsent,
		c.IsCash, c.CashAccountID, c.WarehouseReceiptValuedAt,
		c.NJOPValue, c.NJOPDate, njopSourceArg(c.NJOPSource),
		c.BumnBumdCriteriaMet, strings.TrimSpace(c.BumnBumdEvidence),
		c.CreatedBy,
	).Scan(&c.ID, &c.BoundAmount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("menyimpan agunan: %w", err)
	}
	return nil
}

func (r *CollateralRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoanCollateral, error) {
	query := `SELECT ` + collateralColumns + ` ` + collateralFrom + ` WHERE lc.id = $1`
	c, err := scanCollateral(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCollateralNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mengambil agunan: %w", err)
	}
	return c, nil
}

// GetLoanBook membaca buku produk kredit. Dipakai service agunan untuk menegakkan
// batas buku pada jalur yang berkunci loan_id; kredit tanpa produk mengembalikan buku
// kosong yang oleh CanAccessBook diizinkan agar data lama tetap dapat dioperasikan.
func (r *CollateralRepository) GetLoanBook(ctx context.Context, loanID uuid.UUID) (domain.COABook, error) {
	var book string
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE((SELECT p.book::text FROM banking_products p WHERE p.id = l.product_id), '')
		FROM loans l WHERE l.id = $1`, loanID).Scan(&book)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrLoanNotFound
	}
	if err != nil {
		return "", fmt.Errorf("membaca buku kredit %s: %w", loanID, err)
	}
	return domain.COABook(book), nil
}

func (r *CollateralRepository) ListByLoan(ctx context.Context, loanID uuid.UUID) ([]domain.LoanCollateral, error) {
	query := `SELECT ` + collateralColumns + ` ` + collateralFrom + ` WHERE lc.loan_id = $1
		ORDER BY lc.created_at ASC`
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
			certified = $13, mortgaged = $14, mortgage_value = $15, appraiser_independent = $16,
			exists_known = $17, executable = $18, third_party_owner = $19, owner_consent = $20,
			is_cash = $21, cash_account_id = $22, warehouse_receipt_valued_at = $23,
			njop_value = $24, njop_date = $25, njop_source = $26,
			bumn_bumd_criteria_met = $27, bumn_bumd_evidence = NULLIF($28, ''),
			updated_by = $29, updated_at = NOW()
		WHERE id = $1
		RETURNING bound_amount, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		c.ID, c.CollateralType, c.Description, c.DocumentNumber, c.OwnerName,
		c.AppraisalValue, c.AppraisalDate, c.Appraiser, c.HaircutPercent, c.Status,
		c.ReleasedAt, c.Notes, c.Certified, c.Mortgaged, c.MortgageValue, c.AppraiserIndependent,
		c.ExistsKnown, c.Executable, c.ThirdPartyOwner, c.OwnerConsent,
		c.IsCash, c.CashAccountID, c.WarehouseReceiptValuedAt,
		c.NJOPValue, c.NJOPDate, njopSourceArg(c.NJOPSource),
		c.BumnBumdCriteriaMet, strings.TrimSpace(c.BumnBumdEvidence),
		c.UpdatedBy,
	).Scan(&c.BoundAmount, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrCollateralNotFound
	}
	if err != nil {
		return fmt.Errorf("mengubah agunan: %w", err)
	}
	return nil
}

func (r *CollateralRepository) SummaryActive(ctx context.Context, branchCode string) ([]domain.CollateralSummary, error) {
	// Satu kondisi untuk dua keadaan: aktor lintas cabang mengirim kode kosong dan
	// melihat seluruh cabang, aktor cabang mengirim kodenya sendiri. Agunan tanpa cabang
	// (data lama) hanya terhitung bagi aktor lintas cabang.
	query := `SELECT lc.collateral_type, COUNT(*), COALESCE(SUM(lc.appraisal_value), 0),
		       COALESCE(SUM(lc.bound_amount), 0)
		FROM loan_collaterals lc LEFT JOIN branches b ON b.id = lc.branch_id
		WHERE lc.status = 'ACTIVE' AND ($1 = '' OR b.code = $1)
		GROUP BY lc.collateral_type ORDER BY lc.collateral_type`

	rows, err := r.db.QueryContext(ctx, query, branchCode)
	if err != nil {
		return nil, fmt.Errorf("merekap agunan aktif: %w", err)
	}
	defer rows.Close()

	list := make([]domain.CollateralSummary, 0)
	for rows.Next() {
		var item domain.CollateralSummary
		if err := rows.Scan(&item.CollateralType, &item.Count, &item.AppraisalValue, &item.BoundAmount); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// ListActiveByLoans mengambil seluruh agunan ACTIVE untuk sekumpulan kredit dalam SATU
// query. Perhitungan PPAP membutuhkan tiap agunan, bukan hanya totalnya, karena tarif
// Pasal 20(1), syarat Pasal 21(2), dan penurunan waktu Pasal 20(3)/(5) dinilai per agunan.
func (r *CollateralRepository) ListActiveByLoans(ctx context.Context, loanIDs []uuid.UUID) ([]domain.LoanCollateral, error) {
	if len(loanIDs) == 0 {
		return nil, nil
	}

	// Placeholder disusun dari jumlah kredit, bukan memakai tipe array milik driver:
	// nilainya tetap dikirim sebagai parameter dan tidak ada yang dirangkai ke dalam SQL.
	placeholders := make([]string, 0, len(loanIDs))
	args := make([]any, 0, len(loanIDs))
	for i, id := range loanIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	query := `SELECT ` + collateralColumns + ` ` + collateralFrom + `
		WHERE lc.status = 'ACTIVE' AND lc.loan_id IN (` + strings.Join(placeholders, ", ") + `)
		ORDER BY lc.loan_id, lc.created_at`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mendaftar agunan aktif kredit: %w", err)
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

// njopSourceArg mengubah asal nilai NJOP menjadi parameter SQL. Pointer nil berarti
// kolom NULL — asal yang belum diisi operator, bukan asal yang ditebak aplikasi.
func njopSourceArg(s *domain.NJOPSource) any {
	if s == nil {
		return nil
	}
	return string(*s)
}
