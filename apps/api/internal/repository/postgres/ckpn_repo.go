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

// listLoansForCKPNSelect mengambil kredit yang perlu diproses CKPN. required_ppap
// adalah PPKA yang sudah dihitung dan disimpan jalur PPAP — dibaca, bukan dihitung
// ulang, agar modul CKPN tidak menduplikasi logika PPKA. Cast ::text wajib untuk
// kolom enum/varchar. branch_code dipakai untuk mengatribusikan jurnal ke cabang
// KREDIT, bukan cabang aktor.
const listLoansForCKPNSelect = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.outstanding_principal,
		l.collectibility::text,
		l.dpd,
		l.is_restructured,
		l.required_ppap,
		l.required_ckpn,
		l.restructure_loss_balance,
		l.status::text,
		COALESCE((SELECT b.code FROM branches b WHERE b.id = l.branch_id), '')
	FROM loans l`

func (r *CKPNRepository) ListActiveLoans(ctx context.Context, actor domain.Actor) ([]domain.CKPNLoanSnapshot, error) {
	// Kredit aktif dengan eksposur berjalan, DITAMBAH kredit tidak aktif yang masih
	// menyimpan required_ckpn bukan nol. Yang terakhir penting: kredit yang sudah
	// PAID_OFF/WRITTEN_OFF/CANCELLED tidak boleh membiarkan cadangannya menggantung —
	// targetnya dipaksa nol agar pemulihan dilepas lewat jalur yang sudah ada.
	//
	// Celah yang sama pada jalur PPAP BELUM diperbaiki di sini: query PPAP hanya
	// mengambil kredit yang jatuh tempo/aktif, sehingga required_ppap kredit lunas
	// dapat menggantung. Perubahan itu sengaja tidak dilakukan agar cakupan tetap
	// pada CKPN.
	where := `
	WHERE ((l.status IN ('DISBURSED', 'DEFAULTED') AND l.outstanding_principal > 0)
		OR l.required_ckpn <> 0)`
	args := []any{}
	// Filter cabang memakai kolom l.branch_id; aktor lintas cabang tidak difilter.
	if clause, branchArgs := branchReadClause("l.branch_id", actor); clause != "" {
		where += " AND " + clause
		args = append(args, branchArgs...)
	}
	// Filter buku dari produk kredit; aktor lintas buku tidak difilter dan kredit
	// lama tanpa produk (buku NULL) tetap terlihat. Tanpa ini, /ckpn/run dapat
	// membaca rincian per kredit buku lain sekaligus menjurnalnya.
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = l.product_id)"
	if clause, bookArgs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		where += " AND " + clause
		args = append(args, bookArgs...)
	}

	query := listLoansForCKPNSelect + where + " ORDER BY l.loan_number"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.CKPNLoanSnapshot
	for rows.Next() {
		var s domain.CKPNLoanSnapshot
		var productID sql.NullString
		var collectibility, status string

		if err := rows.Scan(
			&s.LoanID, &s.LoanNumber, &productID, &s.Outstanding,
			&collectibility, &s.DPD, &s.IsRestructured,
			&s.RequiredPPAP, &s.RequiredCKPN, &s.RestructureLoss, &status, &s.BranchCode,
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
		s.Status = domain.LoanStatus(status)
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
