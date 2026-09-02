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

func scanLoan(row interface{ Scan(...any) error }) (*domain.Loan, error) {
	var l domain.Loan
	var aoID, approvedBy sql.NullString
	var approvedAt, disbursedAt sql.NullTime

	err := row.Scan(
		&l.ID, &l.LoanNumber, &l.CustomerID, &l.DisbursementAccountID,
		&l.Type, &l.Status, &l.PrincipalAmount, &l.InterestRateAnnual,
		&l.MarginAmount, &l.TotalPayable, &l.TermMonths, &l.MonthlyInstallment,
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
	return &l, nil
}

func (r *LoanRepository) Create(ctx context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := `INSERT INTO loans
		(id, loan_number, customer_id, disbursement_account_id, loan_type, status,
		 principal_amount, interest_rate_annual, margin_amount, total_payable, term_months,
		 monthly_installment, ao_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`

	_, err = tx.ExecContext(ctx, q,
		l.ID, l.LoanNumber, l.CustomerID, l.DisbursementAccountID, l.Type, l.Status,
		l.PrincipalAmount, l.InterestRateAnnual, l.MarginAmount, l.TotalPayable, l.TermMonths,
		l.MonthlyInstallment, l.AOID, l.CreatedAt, l.UpdatedAt,
	)
	if err != nil {
		return err
	}

	sq := `INSERT INTO loan_schedules
		(id, loan_id, installment_no, due_date, principal_amount, interest_amount,
		 total_installment, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	for _, s := range schedules {
		_, err := tx.ExecContext(ctx, sq,
			s.ID, s.LoanID, s.InstallmentNo, s.DueDate, s.PrincipalAmount, s.InterestAmount,
			s.TotalInstallment, s.Status, s.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LoanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Loan, error) {
	q := `SELECT id, loan_number, customer_id, disbursement_account_id, loan_type, status,
		principal_amount, interest_rate_annual, margin_amount, total_payable, term_months,
		monthly_installment, ao_id, approved_by, approved_at, disbursed_at,
		created_at, updated_at
		FROM loans WHERE id = $1`
	row := r.db.QueryRowContext(ctx, q, id)
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
	q := `SELECT id, loan_number, customer_id, disbursement_account_id, loan_type, status,
		principal_amount, interest_rate_annual, margin_amount, total_payable, term_months,
		monthly_installment, ao_id, approved_by, approved_at, disbursed_at,
		created_at, updated_at
		FROM loans WHERE loan_number = $1`
	row := r.db.QueryRowContext(ctx, q, loanNumber)
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

	q := `SELECT id, loan_number, customer_id, disbursement_account_id, loan_type, status,
		principal_amount, interest_rate_annual, margin_amount, total_payable, term_months,
		monthly_installment, ao_id, approved_by, approved_at, disbursed_at,
		created_at, updated_at
		FROM loans ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
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
	q := `UPDATE loans SET status=$1, approved_by=$2, approved_at=NOW(), updated_at=NOW() WHERE id=$3`
	_, err := r.db.ExecContext(ctx, q, status, approvedBy, id)
	return err
}

func (r *LoanRepository) MarkDisbursed(ctx context.Context, id uuid.UUID) error {
	q := `UPDATE loans SET status=$1, disbursed_at=NOW(), updated_at=NOW() WHERE id=$2`
	_, err := r.db.ExecContext(ctx, q, domain.LoanStatusDisbursed, id)
	return err
}

func (r *LoanRepository) GetSchedules(ctx context.Context, loanID uuid.UUID) ([]domain.LoanSchedule, error) {
	q := `SELECT id, loan_id, installment_no, due_date, principal_amount, interest_amount,
		total_installment, paid_principal, paid_interest, status, paid_at, created_at
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
			&s.InterestAmount, &s.TotalInstallment, &s.PaidPrincipal, &s.PaidInterest,
			&s.Status, &paidAt, &s.CreatedAt,
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

func (r *LoanRepository) UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidInterest decimal.Decimal, status domain.InstallmentStatus) error {
	q := `UPDATE loan_schedules
		SET paid_principal = paid_principal + $1, paid_interest = paid_interest + $2,
		    status = $3, paid_at = NOW()
		WHERE id = $4`
	_, err := r.db.ExecContext(ctx, q, paidPrincipal, paidInterest, status, scheduleID)
	return err
}

var _ domain.LoanRepository = (*LoanRepository)(nil)
