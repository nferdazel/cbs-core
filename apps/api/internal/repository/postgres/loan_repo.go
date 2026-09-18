package postgres

import (
	"context"
	"database/sql"
	"errors"

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
	akad_number, akad_date, purpose,
	ao_id, approved_by, approved_at, disbursed_at,
	created_at, updated_at`

func scanLoan(row interface{ Scan(...any) error }) (*domain.Loan, error) {
	var l domain.Loan
	var aoID, approvedBy sql.NullString
	var approvedAt, disbursedAt, restructuredAt, akadDate sql.NullTime
	var restructuringReason, akadNumber, purpose sql.NullString

	err := row.Scan(
		&l.ID, &l.LoanNumber, &l.CustomerID, &l.ProductID, &l.BranchID, &l.DisbursementAccountID, &l.LoanType, &l.Status,
		&l.PrincipalAmount, &l.AcquisitionCost, &l.DeferredMargin, &l.InterestRateAnnual, &l.MarginAmount, &l.ProfitSharingRatio,
		&l.TotalPayable, &l.TermMonths, &l.MonthlyInstallment, &l.OutstandingPrincipal, &l.PenaltyAccrued,
		&l.Collectibility, &l.DPD, &l.AccrualStatus, &l.RequiredPPAP,
		&l.IsRestructured, &l.RestructuredCount, &restructuredAt, &restructuringReason,
		&akadNumber, &akadDate, &purpose,
		&aoID, &approvedBy, &approvedAt, &disbursedAt,
		&l.CreatedAt, &l.UpdatedAt,
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
	if akadNumber.Valid {
		l.AkadNumber = akadNumber.String
	}
	if akadDate.Valid {
		l.AkadDate = &akadDate.Time
	}
	if purpose.Valid {
		l.Purpose = purpose.String
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

func (r *LoanRepository) List(ctx context.Context, limit, offset int) ([]domain.Loan, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM loans").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `SELECT `+loanColumns+` FROM loans ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
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
		total_installment, paid_principal, paid_profit, profit_type, outstanding_principal, status::text, paid_at, created_at
		FROM loan_schedules WHERE loan_id = $1 ORDER BY installment_no ASC`

	rows, err := r.db.QueryContext(ctx, q, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LoanSchedule
	for rows.Next() {
		var s domain.LoanSchedule
		var paidAt sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.LoanID, &s.InstallmentNo, &s.DueDate, &s.PrincipalAmount,
			&s.ProfitAmount, &s.TotalInstallment, &s.PaidPrincipal, &s.PaidProfit,
			&s.ProfitType, &s.OutstandingPrincipal, &s.Status, &paidAt, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			s.PaidAt = &paidAt.Time
		}
		list = append(list, s)
	}
	return list, nil
}

func (r *LoanRepository) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidProfit decimal.Decimal, status domain.InstallmentStatus) error {
	q := `UPDATE loan_schedules
		SET paid_principal = paid_principal + $1, paid_profit = paid_profit + $2,
		    status = $3, paid_at = NOW()
		WHERE id = $4`
	_, err := r.db.ExecContext(ctx, q, paidPrincipal, paidProfit, status, scheduleID)
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
		updated_at=NOW()
		WHERE id=$13`

	_, err = tx.ExecContext(ctx, q,
		l.TermMonths, l.InterestRateAnnual, l.MarginAmount, l.TotalPayable, l.MonthlyInstallment,
		l.Collectibility, l.AccrualStatus, l.RequiredPPAP,
		l.IsRestructured, l.RestructuredCount, l.RestructuredAt, l.RestructuringReason,
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
	q := `UPDATE loans SET outstanding_principal=$1, penalty_accrued=$2, updated_at=NOW() WHERE id=$3`
	_, err := r.db.ExecContext(ctx, q, outstanding, penalty, id)
	return err
}

var _ domain.LoanRepository = (*LoanRepository)(nil)
