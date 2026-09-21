package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PPAPRepository menyediakan data untuk proses PPAP harian. Semua penulisan jurnal
// tetap melalui posting engine; repo ini hanya membaca snapshot kredit dan menulis
// state PPAP pada baris loans.
type PPAPRepository struct {
	db *sql.DB
}

func NewPPAPRepository(db *sql.DB) *PPAPRepository {
	return &PPAPRepository{db: db}
}

// listDueLoansQuery mengambil kredit aktif beserta jatuh tempo angsuran terlama yang
// belum dibayar pada/tanggal sebelum asOf. DPD kemudian dihitung service dari tanggal
// tersebut. Cast ::text wajib untuk kolom enum/varchar agar aman dipindai ke string.
const listDueLoansQuery = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.outstanding_principal,
		l.collectibility::text,
		l.dpd,
		l.accrual_status::text,
		l.required_ppap,
		MIN(s.due_date) AS last_due_date,
		(SELECT MAX(sf.due_date) FROM loan_schedules sf WHERE sf.loan_id = l.id) AS final_due_date,
		l.is_restructured,
		l.macet_at,
		l.pre_restructure_collectibility,
		CASE WHEN l.restructured_at IS NULL THEN 0 ELSE (
			SELECT COUNT(*)
			FROM loan_schedules sc
			WHERE sc.loan_id = l.id
				AND sc.due_date >= l.restructured_at::date
				AND sc.due_date <= $1
				AND sc.status = 'PAID'
				AND sc.paid_at IS NOT NULL
				AND sc.paid_at::date <= sc.due_date
				AND sc.due_date > COALESCE((
					SELECT MAX(sv.due_date)
					FROM loan_schedules sv
					WHERE sv.loan_id = l.id
						AND sv.due_date >= l.restructured_at::date
						AND sv.due_date <= $1
						AND NOT (sv.status = 'PAID' AND sv.paid_at IS NOT NULL AND sv.paid_at::date <= sv.due_date)
				), l.restructured_at::date - 1)
		) END AS clean_periods
	FROM loans l
	LEFT JOIN loan_schedules s
		ON s.loan_id = l.id
		AND s.status <> 'PAID'
		AND s.due_date <= $1
	WHERE l.status IN ('DISBURSED', 'DEFAULTED')
		AND l.outstanding_principal > 0
	GROUP BY l.id
	ORDER BY l.loan_number`

func (r *PPAPRepository) ListDueLoans(ctx context.Context, asOf time.Time) ([]domain.PPAPLoanSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, listDueLoansQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PPAPLoanSnapshot
	for rows.Next() {
		var s domain.PPAPLoanSnapshot
		var productID sql.NullString
		var collectibility, accrual string
		var lastDue, finalDue, macetAt sql.NullTime
		var preRestructure sql.NullString
		var cleanPeriods int

		if err := rows.Scan(
			&s.LoanID, &s.LoanNumber, &productID, &s.Outstanding,
			&collectibility, &s.DPD, &accrual, &s.RequiredPPAP, &lastDue, &finalDue,
			&s.IsRestructured, &macetAt, &preRestructure, &cleanPeriods,
		); err != nil {
			return nil, err
		}
		if productID.Valid {
			id, err := uuid.Parse(productID.String)
			if err != nil {
				return nil, fmt.Errorf("product_id kredit %s tidak valid: %w", s.LoanNumber, err)
			}
			s.ProductID = &id
		}
		s.Collectibility = domain.CollectibilityFromOJK(domain.OJKCollectibility(collectibility))
		s.AccrualStatus = domain.AccrualStatus(accrual)
		if lastDue.Valid {
			t := lastDue.Time
			s.LastDueDate = &t
		}
		if finalDue.Valid {
			t := finalDue.Time
			s.FinalDueDate = &t
		}
		if macetAt.Valid {
			t := macetAt.Time
			s.MacetAt = &t
		}
		if preRestructure.Valid {
			s.PreRestructureCollectibility = domain.CollectibilityFromOJK(domain.OJKCollectibility(preRestructure.String))
		}
		s.CleanPeriods = cleanPeriods
		list = append(list, s)
	}
	return list, rows.Err()
}

// ppapExec memilih handle eksekusi: transaksi pemanggil bila ada, jika tidak pool.
func (r *PPAPRepository) ppapExec(tx any) (execer, error) {
	if tx == nil {
		return r.db, nil
	}
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("ppap: transaksi tidak valid")
	}
	return sqlTx, nil
}

// UpdateCollectibility menyimpan kolektibilitas beserta status akrual turunannya.
// Kredit NPL (golongan 3-5) dihentikan akrualnya (cash basis) sesuai POJK.
func (r *PPAPRepository) UpdateCollectibility(ctx context.Context, tx any, loanID uuid.UUID, c domain.Collectibility) error {
	accrual := domain.AccrualStatusAccrual
	stop := false
	if c.IsNPL() {
		accrual = domain.AccrualStatusCash
		stop = true
	}
	q := `UPDATE loans
		SET collectibility=$1, accrual_status=$2, stop_accrual=$3, updated_at=NOW()
		WHERE id=$4`
	exec, err := r.ppapExec(tx)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, q, c.OJKCode(), accrual, stop, loanID)
	return err
}

// UpdateLoanState menyimpan seluruh state PPAP hasil satu pemrosesan kredit.
// Saat kredit pertama kali digolongkan Macet, macet_at diisi sekali; nilainya tidak
// pernah ditimpa agar penurunan pengurang agunan Pasal 20(3)/(5) tetap dihitung sejak
// saat kredit benar-benar macet, bukan sejak perhitungan terakhir.
func (r *PPAPRepository) UpdateLoanState(ctx context.Context, tx any, u domain.PPAPLoanUpdate) error {
	// $1 dipakai di dua konteks: nilai kolom collectibility VARCHAR(32) dan pembanding
	// CASE. Tanpa cast, PostgreSQL menyimpulkan tipe yang berbeda (varchar vs text) dan
	// menolak seluruh query dengan SQLSTATE 42P08 "inconsistent types deduced for
	// parameter $1". Cast eksplisit ke varchar menyamakan tipe di kedua pemakaian; ini
	// baru diperlukan sejak parameter dipakai lebih dari sekali.
	q := `UPDATE loans
		SET collectibility=$1::varchar, dpd=$2, accrual_status=$3, stop_accrual=$4, required_ppap=$5,
		    macet_at = CASE WHEN $1::varchar = '5_MACET' THEN COALESCE(macet_at, NOW()) ELSE macet_at END,
		    updated_at=NOW()
		WHERE id=$6`
	exec, err := r.ppapExec(tx)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, q, u.Collectibility.OJKCode(), u.DPD, u.AccrualStatus, u.StopAccrual, u.RequiredPPAP, u.LoanID)
	return err
}

// GetPPAPReserveBalance membaca saldo akun cadangan GL. Akun cadangan adalah akun
// contra-asset: bila normal_balance-nya CREDIT, saldo tersimpan sudah mencerminkan
// cadangan positif; bila tercatat DEBIT (mis. seed lama 10900), pengkreditan justru
// menurunkan saldo, sehingga cadangan = -saldo.
func (r *PPAPRepository) GetPPAPReserveBalance(ctx context.Context, tx any, coaCode string) (decimal.Decimal, error) {
	const q = `
		SELECT a.balance, coa.normal_balance::text
		FROM accounts a
		JOIN chart_of_accounts coa ON a.coa_id = coa.id
		WHERE coa.code = $1 AND a.account_type = 'INTERNAL_GL'
		ORDER BY a.account_number
		LIMIT 1`

	var row *sql.Row
	if tx != nil {
		sqlTx, ok := tx.(*sql.Tx)
		if !ok {
			return decimal.Zero, errors.New("ppap: transaksi tidak valid")
		}
		row = sqlTx.QueryRowContext(ctx, q, coaCode)
	} else {
		row = r.db.QueryRowContext(ctx, q, coaCode)
	}

	var balance decimal.Decimal
	var normal string
	if err := row.Scan(&balance, &normal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, fmt.Errorf("%w: %s", domain.ErrPPAPReserveNotFound, coaCode)
		}
		return decimal.Zero, err
	}
	if domain.BalanceType(normal) == domain.BalanceTypeDebit {
		return balance.Neg(), nil
	}
	return balance, nil
}

var _ domain.PPAPRepository = (*PPAPRepository)(nil)
