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

// CKPNRepository menyediakan data untuk perhitungan CKPN. Semua penulisan jurnal
// tetap melalui posting engine; repo ini hanya membaca snapshot kredit dan menulis
// target CKPN pada baris loans.
type CKPNRepository struct {
	db *sql.DB
}

func NewCKPNRepository(db *sql.DB) *CKPNRepository {
	return &CKPNRepository{db: db}
}

// listActiveLoansCKPNQuery mengambil kredit aktif. required_ppap adalah PPKA yang
// sudah dihitung dan disimpan jalur PPAP — dibaca, bukan dihitung ulang, agar modul
// CKPN tidak menduplikasi logika PPKA. Cast ::text wajib untuk kolom enum/varchar.
const listActiveLoansCKPNQuery = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.outstanding_principal,
		l.collectibility::text,
		l.dpd,
		l.is_restructured,
		l.required_ppap,
		l.required_ckpn
	FROM loans l
	WHERE l.status IN ('DISBURSED', 'DEFAULTED')
		AND l.outstanding_principal > 0
	ORDER BY l.loan_number`

func (r *CKPNRepository) ListActiveLoans(ctx context.Context) ([]domain.CKPNLoanSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, listActiveLoansCKPNQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.CKPNLoanSnapshot
	for rows.Next() {
		var s domain.CKPNLoanSnapshot
		var productID sql.NullString
		var collectibility string

		if err := rows.Scan(
			&s.LoanID, &s.LoanNumber, &productID, &s.Outstanding,
			&collectibility, &s.DPD, &s.IsRestructured,
			&s.RequiredPPAP, &s.RequiredCKPN,
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
		list = append(list, s)
	}
	return list, rows.Err()
}

// ckpnExec memilih handle eksekusi: transaksi pemanggil bila ada, jika tidak pool.
func (r *CKPNRepository) ckpnExec(tx any) (execer, error) {
	if tx == nil {
		return r.db, nil
	}
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("ckpn: transaksi tidak valid")
	}
	return sqlTx, nil
}

// UpdateRequiredCKPN menyimpan target CKPN kredit. Nilai ini adalah target terakhir
// yang diakui per kredit, analog dengan loans.required_ppap pada jalur PPAP; tanpa
// kolom ini selisih target-harapan tidak dapat dihitung idempoten antar run.
func (r *CKPNRepository) UpdateRequiredCKPN(ctx context.Context, tx any, loanID uuid.UUID, target decimal.Decimal) error {
	const q = `UPDATE loans SET required_ckpn=$1, updated_at=NOW() WHERE id=$2`
	exec, err := r.ckpnExec(tx)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, q, target, loanID)
	return err
}

var _ domain.CKPNRepository = (*CKPNRepository)(nil)
