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
		MIN(s.due_date) AS last_due_date
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
		var lastDue sql.NullTime

		if err := rows.Scan(
			&s.LoanID, &s.LoanNumber, &productID, &s.Outstanding,
			&collectibility, &s.DPD, &accrual, &s.RequiredPPAP, &lastDue,
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
func (r *PPAPRepository) UpdateLoanState(ctx context.Context, tx any, u domain.PPAPLoanUpdate) error {
	q := `UPDATE loans
		SET collectibility=$1, dpd=$2, accrual_status=$3, stop_accrual=$4, required_ppap=$5, updated_at=NOW()
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
