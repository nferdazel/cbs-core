package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// DueRepository menyediakan bacaan daftar jatuh tempo: angsuran kredit yang belum
// dibayar dan deposito berjangka yang mendekati/sudah jatuh tempo. Satu query per
// sumber kewajiban; penggabungan dan pengurutan dilakukan service agar query tetap
// ringkas dan mudah dibaca. Filter cabang mengikuti konvensi branchReadClause.
type DueRepository struct {
	db *sql.DB
}

func NewDueRepository(db *sql.DB) *DueRepository {
	return &DueRepository{db: db}
}

// dueHorizonLimit membatasi jumlah baris agar satu permintaan tidak menarik seluruh
// portofolio. Daftar ini untuk pemeriksaan harian teller, bukan laporan lengkap.
const dueHorizonLimit = 200

// ListDueLoanInstallments mengambil angsuran kredit yang belum lunas dengan jatuh
// tempo sampai `until`. Kredit yang sudah PAID_OFF/REJECTED/WRITTEN_OFF/CANCELLED
// tidak punya tagihan berjalan sehingga tidak diikutkan.
func (r *DueRepository) ListDueLoanInstallments(ctx context.Context, asOf, until time.Time, actor domain.Actor) ([]domain.DueObligation, error) {
	where, args := branchReadClause("l.branch_id", actor)
	// Buku kredit dari produknya; kredit lama tanpa produk tetap terlihat.
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = l.product_id)"
	if clause, bargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		args = append(args, bargs...)
		where = andCondition(where, clause)
	}
	untilIdx := len(args) + 1
	args = append(args, until)

	q := fmt.Sprintf(`SELECT l.loan_number, l.customer_id, s.installment_no, s.due_date,
		GREATEST(s.total_installment - s.paid_principal - s.paid_profit, 0) AS outstanding,
		s.status::text
		FROM loan_schedules s
		JOIN loans l ON l.id = s.loan_id
		WHERE s.status <> 'PAID'
		AND s.due_date <= $%d
		AND l.status IN ('DISBURSED', 'DEFAULTED')`, untilIdx)
	if where != "" {
		q += " AND " + where
	}
	q += fmt.Sprintf(" ORDER BY s.due_date ASC LIMIT %d", dueHorizonLimit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.DueObligation
	for rows.Next() {
		var item domain.DueObligation
		item.Kind = domain.DueObligationLoanInstallment
		if err := rows.Scan(
			&item.Reference, &item.CustomerID, &item.InstallmentNo, &item.DueDate,
			&item.Amount, &item.Status,
		); err != nil {
			return nil, err
		}
		applyDueTiming(&item, asOf)
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListDueDeposits mengambil deposito berjangka aktif dengan jatuh tempo sampai
// `until`. Nilai kewajiban yang ditampilkan adalah proyeksi hak nasabah (pokok +
// imbal hasil terakru - pajak), memakai fungsi perhitungan domain yang sama dengan
// pencairan agar angka tidak berbeda dari yang akan dibayarkan.
func (r *DueRepository) ListDueDeposits(ctx context.Context, asOf, until time.Time, actor domain.Actor) ([]domain.DueObligation, error) {
	where, args := branchReadClause("d.branch_id", actor)
	// Buku deposito dari produknya; deposito lama tanpa produk tetap terlihat.
	bookColumn := "(SELECT p.book FROM banking_products p WHERE p.id = d.product_id)"
	if clause, bargs := bookReadClause(bookColumn, actor, len(args)+1); clause != "" {
		args = append(args, bargs...)
		where = andCondition(where, clause)
	}
	untilIdx := len(args) + 1
	args = append(args, until)

	q := fmt.Sprintf(`SELECT d.account_number, d.customer_id, d.maturity_date,
		d.placement_amount, d.accrued_profit, d.accrued_tax, d.status::text
		FROM deposits d
		WHERE d.status IN ('PLACED', 'MATURED')
		AND d.maturity_date <= $%d`, untilIdx)
	if where != "" {
		q += " AND " + where
	}
	q += fmt.Sprintf(" ORDER BY d.maturity_date ASC LIMIT %d", dueHorizonLimit)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.DueObligation
	for rows.Next() {
		var item domain.DueObligation
		item.Kind = domain.DueObligationDepositMaturity
		var placement, accruedProfit, accruedTax decimal.Decimal
		if err := rows.Scan(
			&item.Reference, &item.CustomerID, &item.DueDate,
			&placement, &accruedProfit, &accruedTax, &item.Status,
		); err != nil {
			return nil, err
		}
		item.Amount = domain.DepositMaturityProceeds(placement, accruedProfit, accruedTax)
		applyDueTiming(&item, asOf)
		items = append(items, item)
	}
	return items, rows.Err()
}

// applyDueTiming menandai apakah kewajiban sudah lewat dan menyisipkan selisih
// hari dibanding tanggal acuan. Perbandingan memakai tanggal (tanpa jam) agar
// kewajiban hari ini tidak dianggap lewat.
func applyDueTiming(item *domain.DueObligation, asOf time.Time) {
	dueDay := time.Date(item.DueDate.Year(), item.DueDate.Month(), item.DueDate.Day(), 0, 0, 0, 0, time.UTC)
	baseDay := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	item.DaysRemaining = int(dueDay.Sub(baseDay).Hours() / 24)
	item.Overdue = item.DaysRemaining < 0
}
