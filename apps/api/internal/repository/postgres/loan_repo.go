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

type LoanRepository struct {
	db *sql.DB
}

func NewLoanRepository(db *sql.DB) *LoanRepository {
	return &LoanRepository{db: db}
}

// loanColumns memakai cast ::text untuk kolom bertipe enum kustom (loan_type,
// status). Tanpa cast, nilainya tidak dapat dipindai ke string Go.
const loanColumns = `id, loan_number, customer_id, product_id, branch_id, disbursement_account_id, loan_type::text, status::text,
	principal_amount, acquisition_cost, deferred_margin, interest_rate_annual, margin_amount, profit_sharing_ratio,
	total_payable, term_months, monthly_installment, outstanding_principal, penalty_accrued,
	collectibility, dpd, accrual_status, required_ppap,
	is_restructured, restructured_count, restructured_at, restructuring_reason,
	pre_restructure_collectibility,
	akad_number, akad_date, purpose,
	ao_id, approved_by, approved_at, disbursed_at,
	created_at, updated_at,
	COALESCE((SELECT b.code FROM branches b WHERE b.id = loans.branch_id), ''),
	(SELECT MAX(sf.due_date) FROM loan_schedules sf WHERE sf.loan_id = loans.id)`

func scanLoan(row interface{ Scan(...any) error }) (*domain.Loan, error) {
	var l domain.Loan
	var aoID, approvedBy sql.NullString
	var approvedAt, disbursedAt, restructuredAt, akadDate sql.NullTime
	var restructuringReason, akadNumber, purpose sql.NullString
	var preRestructure sql.NullString
	var finalDue sql.NullTime

	err := row.Scan(
		&l.ID, &l.LoanNumber, &l.CustomerID, &l.ProductID, &l.BranchID, &l.DisbursementAccountID, &l.LoanType, &l.Status,
		&l.PrincipalAmount, &l.AcquisitionCost, &l.DeferredMargin, &l.InterestRateAnnual, &l.MarginAmount, &l.ProfitSharingRatio,
		&l.TotalPayable, &l.TermMonths, &l.MonthlyInstallment, &l.OutstandingPrincipal, &l.PenaltyAccrued,
		&l.Collectibility, &l.DPD, &l.AccrualStatus, &l.RequiredPPAP,
		&l.IsRestructured, &l.RestructuredCount, &restructuredAt, &restructuringReason,
		&preRestructure,
		&akadNumber, &akadDate, &purpose,
		&aoID, &approvedBy, &approvedAt, &disbursedAt,
		&l.CreatedAt, &l.UpdatedAt,
		&l.BranchCode,
		&finalDue,
	)
	if err != nil {
		return nil, err
	}
	if aoID.Valid {
		id, _ := uuid.Parse(aoID.String)
		l.AOID = &id
	}
	if approvedBy.Valid {
		id, _ := uuid.Parse(approvedBy.String)
		l.ApprovedBy = &id
	}
	if approvedAt.Valid {
		l.ApprovedAt = &approvedAt.Time
	}
	if disbursedAt.Valid {
		l.DisbursedAt = &disbursedAt.Time
	}
	if restructuredAt.Valid {
		l.RestructuredAt = &restructuredAt.Time
	}
	if restructuringReason.Valid {
		l.RestructuringReason = restructuringReason.String
	}
	if preRestructure.Valid {
		l.PreRestructureCollectibility = domain.OJKCollectibility(preRestructure.String)
	}
	if akadNumber.Valid {
		l.AkadNumber = akadNumber.String
	}
	if akadDate.Valid {
		l.AkadDate = &akadDate.Time
	}
	if purpose.Valid {
		l.Purpose = purpose.String
	}
	if finalDue.Valid {
		l.FinalDueDate = &finalDue.Time
	}
	return &l, nil
}

func (r *LoanRepository) Create(ctx context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := `INSERT INTO loans
		(id, loan_number, customer_id, product_id, branch_id, disbursement_account_id, loan_type, status,
		 principal_amount, acquisition_cost, deferred_margin, interest_rate_annual, margin_amount, profit_sharing_ratio,
		 total_payable, term_months, monthly_installment, outstanding_principal, penalty_accrued,
		 collectibility, dpd, accrual_status, required_ppap,
		 akad_number, akad_date, purpose, ao_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)`

	_, err = tx.ExecContext(ctx, q,
		l.ID, l.LoanNumber, l.CustomerID, l.ProductID, l.BranchID, l.DisbursementAccountID, l.LoanType, l.Status,
		l.PrincipalAmount, l.AcquisitionCost, l.DeferredMargin, l.InterestRateAnnual, l.MarginAmount, l.ProfitSharingRatio,
		l.TotalPayable, l.TermMonths, l.MonthlyInstallment, l.OutstandingPrincipal, l.PenaltyAccrued,
		l.Collectibility, l.DPD, l.AccrualStatus, l.RequiredPPAP,
		l.AkadNumber, l.AkadDate, l.Purpose, l.AOID, l.CreatedAt, l.UpdatedAt,
	)
	if err != nil {
		return err
	}

	sq := `INSERT INTO loan_schedules
		(id, loan_id, installment_no, due_date, principal_amount, profit_amount, total_installment,
		 paid_principal, paid_profit, profit_type, outstanding_principal, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	for _, s := range schedules {
		if _, err := tx.ExecContext(ctx, sq,
			s.ID, s.LoanID, s.InstallmentNo, s.DueDate, s.PrincipalAmount, s.ProfitAmount, s.TotalInstallment,
			s.PaidPrincipal, s.PaidProfit, s.ProfitType, s.OutstandingPrincipal, s.Status, s.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LoanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Loan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+loanColumns+` FROM loans WHERE id = $1`, id)
	l, err := scanLoan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLoanNotFound
	}
	if err != nil {
		return nil, err
	}

	schedules, err := r.GetSchedules(ctx, l.ID)
	if err == nil {
		l.Schedules = schedules
	}
	return l, nil
}

func (r *LoanRepository) GetByNumber(ctx context.Context, loanNumber string) (*domain.Loan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+loanColumns+` FROM loans WHERE loan_number = $1`, loanNumber)
	l, err := scanLoan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLoanNotFound
	}
	return l, err
}

func (r *LoanRepository) List(ctx context.Context, limit, offset int, actor domain.Actor) ([]domain.Loan, int, error) {
	where, whereArgs := branchReadClause("branch_id", actor)

	countQuery := "SELECT COUNT(*) FROM loans"
	if where != "" {
		countQuery += " WHERE " + where
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + loanColumns + ` FROM loans`
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	args := append(append([]any{}, whereArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.Loan
	for rows.Next() {
		l, err := scanLoan(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *l)
	}
	return list, total, nil
}

func (r *LoanRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	return updateLoanStatus(ctx, r.db, id, status, approvedBy)
}

// UpdateStatusTx menjalankan update status di dalam transaksi pemanggil, agar
// perubahan status dan penulisan audit bisa commit bersama.
func (r *LoanRepository) UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return updateLoanStatus(ctx, sqlTx, id, status, approvedBy)
}

func updateLoanStatus(ctx context.Context, exec execer, id uuid.UUID, status domain.LoanStatus, approvedBy *uuid.UUID) error {
	q := `UPDATE loans SET status=$1, approved_by=$2, approved_at=NOW(), updated_at=NOW() WHERE id=$3`
	_, err := exec.ExecContext(ctx, q, status, approvedBy, id)
	return err
}

func (r *LoanRepository) MarkDisbursed(ctx context.Context, id uuid.UUID, outstanding decimal.Decimal) error {
	return markLoanDisbursed(ctx, r.db, id, outstanding)
}

// MarkDisbursedTx menjalankan penandaan pencairan di dalam transaksi pemanggil.
func (r *LoanRepository) MarkDisbursedTx(ctx context.Context, tx any, id uuid.UUID, outstanding decimal.Decimal) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return markLoanDisbursed(ctx, sqlTx, id, outstanding)
}

func markLoanDisbursed(ctx context.Context, exec execer, id uuid.UUID, outstanding decimal.Decimal) error {
	q := `UPDATE loans SET status=$1, disbursed_at=NOW(), outstanding_principal=$2, updated_at=NOW() WHERE id=$3`
	_, err := exec.ExecContext(ctx, q, domain.LoanStatusDisbursed, outstanding, id)
	return err
}

func (r *LoanRepository) GetSchedules(ctx context.Context, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	q := `SELECT id, loan_id, installment_no, due_date, principal_amount, profit_amount,
		total_installment, paid_principal, paid_profit, profit_type, outstanding_principal, status::text, paid_at, created_at,
		profit_accrued_at, profit_accrued_amount
		FROM loan_schedules WHERE loan_id = $1 ORDER BY installment_no ASC`

	rows, err := r.db.QueryContext(ctx, q, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LoanSchedule
	for rows.Next() {
		var s domain.LoanSchedule
		var paidAt, profitAccruedAt sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.LoanID, &s.InstallmentNo, &s.DueDate, &s.PrincipalAmount,
			&s.ProfitAmount, &s.TotalInstallment, &s.PaidPrincipal, &s.PaidProfit,
			&s.ProfitType, &s.OutstandingPrincipal, &s.Status, &paidAt, &s.CreatedAt,
			&profitAccruedAt, &s.ProfitAccruedAmount,
		); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			s.PaidAt = &paidAt.Time
		}
		if profitAccruedAt.Valid {
			s.ProfitAccruedAt = &profitAccruedAt.Time
		}
		list = append(list, s)
	}
	return list, nil
}

// UpdateSchedulePayment mencatat pembayaran angsuran dan mengurangi sisa akruan bunga
// yang diselesaikan. GREATEST(..., 0) menjaga piutang bunga (10400) tidak pernah
// negatif bila ada pembayaran yang mencoba menyelesaikan lebih dari yang diakru.
func (r *LoanRepository) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	return updateSchedulePayment(ctx, r.db, scheduleID, paidPrincipal, paidProfit, settleAccrued, status)
}

// UpdateSchedulePaymentTx mencatat pembayaran angsuran di dalam transaksi pemanggil,
// agar jurnal dan perubahan jadwal tidak pernah terpisah.
func (r *LoanRepository) UpdateSchedulePaymentTx(ctx context.Context, tx any, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return updateSchedulePayment(ctx, sqlTx, scheduleID, paidPrincipal, paidProfit, settleAccrued, status)
}

func updateSchedulePayment(ctx context.Context, exec execer, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	// paid_at hanya diisi saat angsuran benar-benar lunas; angsuran sebagian tetap
	// menyimpan tanggal pembayaran pertamanya di kolom lain bila diperlukan kelak.
	q := `UPDATE loan_schedules
		SET paid_principal = paid_principal + $1, paid_profit = paid_profit + $2,
		    profit_accrued_amount = GREATEST(profit_accrued_amount - $3, 0),
		    status = $4,
		    paid_at = CASE WHEN $4::installment_status = 'PAID' THEN NOW() ELSE paid_at END
		WHERE id = $5`
	_, err := exec.ExecContext(ctx, q, paidPrincipal, paidProfit, settleAccrued, status, scheduleID)
	return err
}

// UpdateRestructure menyimpan hasil restrukturisasi loan beserta jadwal angsuran baru
// dalam satu transaksi: update parameter dan replace seluruh schedule lama.
func (r *LoanRepository) UpdateRestructure(ctx context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := `UPDATE loans SET
		term_months=$1, interest_rate_annual=$2, margin_amount=$3, total_payable=$4, monthly_installment=$5,
		collectibility=$6, accrual_status=$7, required_ppap=$8,
		is_restructured=$9, restructured_count=$10, restructured_at=$11, restructuring_reason=$12,
		pre_restructure_collectibility=$13,
		updated_at=NOW()
		WHERE id=$14`

	_, err = tx.ExecContext(ctx, q,
		l.TermMonths, l.InterestRateAnnual, l.MarginAmount, l.TotalPayable, l.MonthlyInstallment,
		l.Collectibility, l.AccrualStatus, l.RequiredPPAP,
		l.IsRestructured, l.RestructuredCount, l.RestructuredAt, l.RestructuringReason,
		l.PreRestructureCollectibility,
		l.ID,
	)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM loan_schedules WHERE loan_id = $1`, l.ID); err != nil {
		return err
	}

	sq := `INSERT INTO loan_schedules
		(id, loan_id, installment_no, due_date, principal_amount, profit_amount, total_installment,
		 paid_principal, paid_profit, profit_type, outstanding_principal, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	for _, s := range schedules {
		if _, err := tx.ExecContext(ctx, sq,
			s.ID, s.LoanID, s.InstallmentNo, s.DueDate, s.PrincipalAmount, s.ProfitAmount, s.TotalInstallment,
			s.PaidPrincipal, s.PaidProfit, s.ProfitType, s.OutstandingPrincipal, s.Status, s.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LoanRepository) UpdateCollectibility(ctx context.Context, id uuid.UUID, col domain.OJKCollectibility, dpd int, accrual domain.AccrualStatus, ppap decimal.Decimal) error {
	q := `UPDATE loans SET collectibility=$1, dpd=$2, accrual_status=$3, required_ppap=$4, updated_at=NOW() WHERE id=$5`
	_, err := r.db.ExecContext(ctx, q, col, dpd, accrual, ppap, id)
	return err
}

func (r *LoanRepository) UpdateOutstanding(ctx context.Context, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	return updateOutstanding(ctx, r.db, id, outstanding, penalty)
}

// UpdateOutstandingTx menyimpan sisa pokok dan denda di dalam transaksi pemanggil.
func (r *LoanRepository) UpdateOutstandingTx(ctx context.Context, tx any, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	return updateOutstanding(ctx, sqlTx, id, outstanding, penalty)
}

func updateOutstanding(ctx context.Context, exec execer, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	q := `UPDATE loans SET outstanding_principal=$1, penalty_accrued=$2, updated_at=NOW() WHERE id=$3`
	_, err := exec.ExecContext(ctx, q, outstanding, penalty, id)
	return err
}

// listPenaltyCandidatesQuery mengambil kredit aktif beserta pokok angsuran yang lewat
// jatuh tempo. Dasar denda adalah pokok angsuran yang belum dibayar, bukan seluruh
// sisa pokok: GREATEST(principal_amount - paid_principal, 0) tetap benar untuk
// angsuran sebagian. Cast ::text wajib untuk kolom enum status.
const listPenaltyCandidatesQuery = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.disbursement_account_id,
		l.status::text,
		COALESCE(SUM(GREATEST(s.principal_amount - s.paid_principal, 0)), 0) AS overdue_principal,
		MIN(s.due_date) AS oldest_due_date,
		l.penalty_last_accrued_on
	FROM loans l
	JOIN loan_schedules s
		ON s.loan_id = l.id
		AND s.status <> 'PAID'
		AND s.due_date <= $1
	WHERE l.status IN ('DISBURSED', 'DEFAULTED')
		AND l.outstanding_principal > 0
	GROUP BY l.id
	ORDER BY l.loan_number`

func (r *LoanRepository) ListPenaltyCandidates(ctx context.Context, asOf time.Time) ([]domain.LoanPenaltyCandidate, error) {
	rows, err := r.db.QueryContext(ctx, listPenaltyCandidatesQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LoanPenaltyCandidate
	for rows.Next() {
		var c domain.LoanPenaltyCandidate
		var productID sql.NullString
		var status string
		var oldestDue, lastAccrued sql.NullTime

		if err := rows.Scan(
			&c.LoanID, &c.LoanNumber, &productID, &c.DisbursementAccountID, &status,
			&c.OverduePrincipal, &oldestDue, &lastAccrued,
		); err != nil {
			return nil, err
		}
		if productID.Valid {
			id, err := uuid.Parse(productID.String)
			if err != nil {
				return nil, err
			}
			c.ProductID = &id
		}
		c.Status = domain.LoanStatus(status)
		if oldestDue.Valid {
			t := oldestDue.Time
			c.OldestDueDate = &t
		}
		if lastAccrued.Valid {
			t := lastAccrued.Time
			c.LastAccruedOn = &t
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// AddPenaltyAccruedTx menambah penalty_accrued dan memajukan
// penalty_last_accrued_on, tetapi hanya bila jurnal dengan idempotency_key tersebut
// belum ada. Tanggal terakhir dikunci dengan GREATEST agar tidak pernah mundur bila
// ada eksekusi yang datang tidak berurutan. Penambahan dan jurnalnya berada pada
// transaksi yang sama, sehingga angka di loans dan jurnal tidak pernah terpisah.
// Baris loans terkunci oleh UPDATE, yang menyerialkan dua batch paralel untuk kredit
// yang sama: yang kedua akan melihat jurnal sudah ada dan tidak menambah lagi.
func (r *LoanRepository) AddPenaltyAccruedTx(ctx context.Context, tx any, loanID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedOn time.Time) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `UPDATE loans
		SET penalty_accrued = penalty_accrued + $1,
		    penalty_last_accrued_on = GREATEST(COALESCE(penalty_last_accrued_on, $2), $2),
		    updated_at = NOW()
		WHERE id = $3
		  AND NOT EXISTS (SELECT 1 FROM journal_entries WHERE idempotency_key = $4)`
	res, err := sqlTx.ExecContext(ctx, q, amount, accruedOn, loanID, idempotencyKey)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// listInterestAccrualCandidatesQuery mengambil angsuran yang layak diakru:
// sudah jatuh tempo pada asOf, belum dibayar, porsi bunganya belum habis, dan belum
// pernah diakru. Hanya kredit konvensional (loan_type) yang disertakan; produk
// syariah tidak punya pemetaan INTEREST_ACCRUAL dan tidak boleh diakru. Jatuh tempo
// angsuran tertua yang belum dibayar ikut diambil untuk menghitung DPD/kolektibilitas.
const listInterestAccrualCandidatesQuery = `
	SELECT
		l.id,
		l.loan_number,
		l.product_id,
		l.loan_type::text,
		l.status::text,
		s.id,
		s.installment_no,
		s.due_date,
		GREATEST(s.profit_amount - s.paid_profit, 0) AS outstanding_profit,
		(SELECT MIN(s2.due_date) FROM loan_schedules s2
			WHERE s2.loan_id = l.id AND s2.status <> 'PAID') AS oldest_due_date,
		(SELECT MAX(sf.due_date) FROM loan_schedules sf
			WHERE sf.loan_id = l.id) AS final_due_date
	FROM loans l
	JOIN loan_schedules s
		ON s.loan_id = l.id
		AND s.status <> 'PAID'
		AND s.due_date <= $1
		AND s.profit_accrued_at IS NULL
		AND s.profit_amount - s.paid_profit > 0
	WHERE l.status IN ('DISBURSED', 'DEFAULTED')
		AND l.loan_type::text IN ('CONVENTIONAL_FLAT', 'CONVENTIONAL_ANNUITY')
	ORDER BY l.loan_number, s.installment_no`

func (r *LoanRepository) ListInterestAccrualCandidates(ctx context.Context, asOf time.Time) ([]domain.LoanInterestAccrualCandidate, error) {
	rows, err := r.db.QueryContext(ctx, listInterestAccrualCandidatesQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LoanInterestAccrualCandidate
	for rows.Next() {
		var c domain.LoanInterestAccrualCandidate
		var productID sql.NullString
		var loanType, status string
		var oldestDue, finalDue sql.NullTime

		if err := rows.Scan(
			&c.LoanID, &c.LoanNumber, &productID, &loanType, &status,
			&c.ScheduleID, &c.InstallmentNo, &c.DueDate, &c.OutstandingProfit, &oldestDue,
			&finalDue,
		); err != nil {
			return nil, err
		}
		if productID.Valid {
			id, err := uuid.Parse(productID.String)
			if err != nil {
				return nil, err
			}
			c.ProductID = &id
		}
		c.LoanType = domain.LoanType(loanType)
		c.LoanStatus = domain.LoanStatus(status)
		if oldestDue.Valid {
			t := oldestDue.Time
			c.OldestDueDate = &t
		}
		if finalDue.Valid {
			t := finalDue.Time
			c.FinalDueDate = &t
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// AddScheduleProfitAccruedTx menambah profit_accrued_amount dan mengisi
// profit_accrued_at satu angsuran, tetapi hanya bila jurnal dengan idempotency_key
// tersebut belum ada. UPDATE mengunci baris loan_schedules sehingga dua batch paralel
// untuk angsuran yang sama terserialisasi: yang kedua melihat jurnal sudah ada dan
// tidak menambah lagi. Penambahan dan jurnalnya berada pada transaksi yang sama,
// sehingga angka sisa akruan dan jurnal tidak pernah terpisah.
func (r *LoanRepository) AddScheduleProfitAccruedTx(ctx context.Context, tx any, scheduleID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedAt time.Time) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `UPDATE loan_schedules
		SET profit_accrued_amount = profit_accrued_amount + $1,
		    profit_accrued_at = $2
		WHERE id = $3
		  AND NOT EXISTS (SELECT 1 FROM journal_entries WHERE idempotency_key = $4)`
	res, err := sqlTx.ExecContext(ctx, q, amount, accruedAt, scheduleID, idempotencyKey)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// HasInstallmentPaymentTx mendeteksi angsuran yang sudah dibayar. Satu baris saja
// yang punya paid_principal/paid_profit bukan nol atau status selain PENDING sudah
// cukup: uang dan jadwal sudah berjalan sehingga pencairan tidak boleh dibatalkan.
func (r *LoanRepository) HasInstallmentPaymentTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return false, errors.New("loan: transaksi tidak valid")
	}
	q := `SELECT COUNT(*)
		FROM loan_schedules
		WHERE loan_id = $1
		  AND (paid_principal <> 0 OR paid_profit <> 0 OR status <> 'PENDING')`
	var count int
	if err := sqlTx.QueryRowContext(ctx, q, loanID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteSchedulesTx menghapus seluruh jadwal angsuran satu kredit. Pola yang sama
// dipakai jalur restrukturisasi yang mengganti jadwal lama dengan jadwal baru.
func (r *LoanRepository) DeleteSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("loan: transaksi tidak valid")
	}
	_, err := sqlTx.ExecContext(ctx, `DELETE FROM loan_schedules WHERE loan_id = $1`, loanID)
	return err
}

// GetDisbursementJournalRefTx mengambil nomor referensi jurnal pencairan kredit.
// Jurnal pencairan tidak menyimpan nomor kredit, jadi pencariannya lewat
// idempotency_key yang dibentuk DisburseLoan ("DISB-"+nomor kredit).
func (r *LoanRepository) GetDisbursementJournalRefTx(ctx context.Context, tx any, loanNumber string) (string, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return "", errors.New("loan: transaksi tidak valid")
	}
	var ref string
	err := sqlTx.QueryRowContext(ctx,
		`SELECT reference_number FROM journal_entries WHERE idempotency_key = $1`,
		"DISB-"+loanNumber).Scan(&ref)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("jurnal pencairan kredit %s tidak ditemukan", loanNumber)
	}
	if err != nil {
		return "", err
	}
	return ref, nil
}

var _ domain.LoanRepository = (*LoanRepository)(nil)
