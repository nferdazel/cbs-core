package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// --- Errors ---
var (
	ErrLoanNotFound        = errors.New("loan application not found")
	ErrLoanAlreadyApproved = errors.New("loan has already been approved or rejected")
	ErrLoanNotApproved     = errors.New("loan must be in APPROVED status before disbursement")
	ErrLoanAlreadyDisbursed = errors.New("loan has already been disbursed")
	ErrInvalidLoanAmount   = errors.New("loan principal amount must be positive")
	ErrInvalidLoanTerm     = errors.New("loan term must be at least 1 month")
)

type LoanType string

const (
	LoanTypeFlat      LoanType = "CONVENTIONAL_FLAT"
	LoanTypeAnnuity   LoanType = "CONVENTIONAL_ANNUITY"
	LoanTypeMurabahah LoanType = "SYARIAH_MURABAHAH"
	LoanTypeMudharabah LoanType = "SYARIAH_MUDHARABAH"
)

type LoanStatus string

const (
	LoanStatusPendingApproval LoanStatus = "PENDING_APPROVAL"
	LoanStatusApproved        LoanStatus = "APPROVED"
	LoanStatusDisbursed       LoanStatus = "DISBURSED"
	LoanStatusRejected        LoanStatus = "REJECTED"
	LoanStatusPaidOff         LoanStatus = "PAID_OFF"
	LoanStatusDefaulted       LoanStatus = "DEFAULTED"
	LoanStatusWrittenOff      LoanStatus = "WRITTEN_OFF"
)

type InstallmentStatus string

const (
	InstallmentStatusPending InstallmentStatus = "PENDING"
	InstallmentStatusPaid    InstallmentStatus = "PAID"
	InstallmentStatusOverdue InstallmentStatus = "OVERDUE"
	InstallmentStatusPartial InstallmentStatus = "PARTIAL"
)

type OJKCollectibility string

const (
	CollectibilityKol1 OJKCollectibility = "1_LANCAR"
	CollectibilityKol2 OJKCollectibility = "2_DPK"
	CollectibilityKol3 OJKCollectibility = "3_KURANG_LANCAR"
	CollectibilityKol4 OJKCollectibility = "4_DIRAGUKAN"
	CollectibilityKol5 OJKCollectibility = "5_MACET"
)

type AccrualStatus string

const (
	AccrualStatusAccrual AccrualStatus = "ACCRUAL_PERFORMING" // Kol 1 & Kol 2
	AccrualStatusCash    AccrualStatus = "CASH_BASIS_NPL"     // Kol 3, 4, 5 (Stop Accrual)
)

func CalculateCollectibility(dpd int) (OJKCollectibility, decimal.Decimal, AccrualStatus) {
	switch {
	case dpd <= 0:
		// Kol 1 LANCAR: 0 hari tunggakan (Tepat waktu)
		return CollectibilityKol1, decimal.NewFromFloat(0.005), AccrualStatusAccrual // 0.5% PPAP
	case dpd <= 90:
		// Kol 2 DPK: 1 - 90 hari tunggakan
		return CollectibilityKol2, decimal.NewFromFloat(0.010), AccrualStatusAccrual // 1.0% PPAP
	case dpd <= 120:
		// Kol 3 KURANG LANCAR (NPL): 91 - 120 hari tunggakan
		return CollectibilityKol3, decimal.NewFromFloat(0.150), AccrualStatusCash    // 15% PPAP & Stop Accrual
	case dpd <= 180:
		// Kol 4 DIRAGUKAN (NPL): 121 - 180 hari tunggakan
		return CollectibilityKol4, decimal.NewFromFloat(0.500), AccrualStatusCash    // 50% PPAP & Stop Accrual
	default:
		// Kol 5 MACET (NPL): > 180 hari tunggakan
		return CollectibilityKol5, decimal.NewFromFloat(1.000), AccrualStatusCash    // 100% PPAP & Stop Accrual
	}
}

type Loan struct {
	ID                     uuid.UUID         `json:"id"`
	LoanNumber             string            `json:"loan_number"`
	CustomerID             uuid.UUID         `json:"customer_id"`
	DisbursementAccountID uuid.UUID         `json:"disbursement_account_id"`
	Type                   LoanType          `json:"loan_type"`
	Status                 LoanStatus        `json:"status"`
	Collectibility         OJKCollectibility `json:"collectibility"`
	DPD                    int               `json:"dpd"`
	AccrualStatus          AccrualStatus     `json:"accrual_status"`
	RequiredPPAP           decimal.Decimal   `json:"required_ppap"`
	IsRestructured         bool              `json:"is_restructured"`
	RestructuredCount      int               `json:"restructured_count"`
	RestructuredAt         *time.Time        `json:"restructured_at,omitempty"`
	RestructuringReason    string            `json:"restructuring_reason,omitempty"`
	PrincipalAmount        decimal.Decimal   `json:"principal_amount"`
	InterestRateAnnual     decimal.Decimal   `json:"interest_rate_annual"`
	MarginAmount           decimal.Decimal   `json:"margin_amount"`
	TotalPayable           decimal.Decimal `json:"total_payable"`
	TermMonths             int             `json:"term_months"`
	MonthlyInstallment     decimal.Decimal `json:"monthly_installment"`
	AOID                   *uuid.UUID      `json:"ao_id,omitempty"`
	ApprovedBy             *uuid.UUID      `json:"approved_by,omitempty"`
	ApprovedAt             *time.Time      `json:"approved_at,omitempty"`
	DisbursedAt            *time.Time      `json:"disbursed_at,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`

	Schedules []LoanSchedule `json:"schedules,omitempty"`
}

type LoanSchedule struct {
	ID               uuid.UUID         `json:"id"`
	LoanID           uuid.UUID         `json:"loan_id"`
	InstallmentNo    int               `json:"installment_no"`
	DueDate          time.Time         `json:"due_date"`
	PrincipalAmount  decimal.Decimal   `json:"principal_amount"`
	InterestAmount   decimal.Decimal   `json:"interest_amount"`
	TotalInstallment decimal.Decimal   `json:"total_installment"`
	PaidPrincipal    decimal.Decimal   `json:"paid_principal"`
	PaidInterest     decimal.Decimal   `json:"paid_interest"`
	Status           InstallmentStatus `json:"status"`
	PaidAt           *time.Time        `json:"paid_at,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

// --- DTOs ---

type ApplyLoanInput struct {
	CustomerID             uuid.UUID       `json:"customer_id"`
	DisbursementAccountID uuid.UUID       `json:"disbursement_account_id"`
	Type                   LoanType        `json:"loan_type"`
	PrincipalAmount        decimal.Decimal `json:"principal_amount"`
	InterestRateAnnual     decimal.Decimal `json:"interest_rate_annual"` // e.g. 12.0 for 12% p.a.
	MarginAmount           decimal.Decimal `json:"margin_amount"`        // for Murabahah
	TermMonths             int             `json:"term_months"`
}

type PayInstallmentInput struct {
	LoanID        uuid.UUID       `json:"loan_id"`
	InstallmentNo int             `json:"installment_no"`
	Amount        decimal.Decimal `json:"amount"`
}

// --- Repayment Schedule Calculation Engine ---

func GenerateFlatSchedule(loanID uuid.UUID, principal decimal.Decimal, annualRate decimal.Decimal, termMonths int, startDate time.Time) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	if termMonths <= 0 {
		termMonths = 12
	}

	termDec := decimal.NewFromInt(int64(termMonths))

	// Total Interest = Principal * (Rate / 100) * (TermMonths / 12)
	rateFraction := annualRate.Div(decimal.NewFromInt(100))
	totalInterest := principal.Mul(rateFraction).Mul(termDec.Div(decimal.NewFromInt(12)))
	totalPayable := principal.Add(totalInterest)

	basePrincipal := principal.DivRound(termDec, 4)
	baseInterest := totalInterest.DivRound(termDec, 4)

	var schedules []LoanSchedule
	accPrincipal := decimal.Zero
	accInterest := decimal.Zero

	for i := 1; i <= termMonths; i++ {
		dueDate := startDate.AddDate(0, i, 0)
		var pAmt, iAmt decimal.Decimal

		if i == termMonths {
			// Final installment absorbs any rounding remainder
			pAmt = principal.Sub(accPrincipal)
			iAmt = totalInterest.Sub(accInterest)
		} else {
			pAmt = basePrincipal
			iAmt = baseInterest
			accPrincipal = accPrincipal.Add(pAmt)
			accInterest = accInterest.Add(iAmt)
		}

		tAmt := pAmt.Add(iAmt)

		schedules = append(schedules, LoanSchedule{
			ID:               uuid.New(),
			LoanID:           loanID,
			InstallmentNo:    i,
			DueDate:          dueDate,
			PrincipalAmount:  pAmt,
			InterestAmount:   iAmt,
			TotalInstallment: tAmt,
			Status:           InstallmentStatusPending,
			CreatedAt:        time.Now().UTC(),
		})
	}

	firstTotal := schedules[0].TotalInstallment
	return schedules, totalPayable, firstTotal
}

func GenerateMurabahahSchedule(loanID uuid.UUID, principal decimal.Decimal, margin decimal.Decimal, termMonths int, startDate time.Time) ([]LoanSchedule, decimal.Decimal, decimal.Decimal) {
	if termMonths <= 0 {
		termMonths = 12
	}

	termDec := decimal.NewFromInt(int64(termMonths))
	totalPayable := principal.Add(margin)

	basePrincipal := principal.DivRound(termDec, 4)
	baseMargin := margin.DivRound(termDec, 4)

	var schedules []LoanSchedule
	accPrincipal := decimal.Zero
	accMargin := decimal.Zero

	for i := 1; i <= termMonths; i++ {
		dueDate := startDate.AddDate(0, i, 0)
		var pAmt, mAmt decimal.Decimal

		if i == termMonths {
			// Final installment absorbs any rounding remainder
			pAmt = principal.Sub(accPrincipal)
			mAmt = margin.Sub(accMargin)
		} else {
			pAmt = basePrincipal
			mAmt = baseMargin
			accPrincipal = accPrincipal.Add(pAmt)
			accMargin = accMargin.Add(mAmt)
		}

		tAmt := pAmt.Add(mAmt)

		schedules = append(schedules, LoanSchedule{
			ID:               uuid.New(),
			LoanID:           loanID,
			InstallmentNo:    i,
			DueDate:          dueDate,
			PrincipalAmount:  pAmt,
			InterestAmount:   mAmt,
			TotalInstallment: tAmt,
			Status:           InstallmentStatusPending,
			CreatedAt:        time.Now().UTC(),
		})
	}

	firstTotal := schedules[0].TotalInstallment
	return schedules, totalPayable, firstTotal
}

// --- Interfaces ---

type LoanRepository interface {
	Create(ctx context.Context, loan *Loan, schedules []LoanSchedule) error
	GetByID(ctx context.Context, id uuid.UUID) (*Loan, error)
	GetByNumber(ctx context.Context, loanNumber string) (*Loan, error)
	List(ctx context.Context, limit, offset int) ([]Loan, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status LoanStatus, approvedBy *uuid.UUID) error
	MarkDisbursed(ctx context.Context, id uuid.UUID) error
	GetSchedules(ctx context.Context, loanID uuid.UUID) ([]LoanSchedule, error)
	UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidInterest decimal.Decimal, status InstallmentStatus) error
}

type RestructureLoanInput struct {
	LoanID                uuid.UUID       `json:"loan_id"`
	NewTermMonths         int             `json:"new_term_months"`
	NewInterestRateAnnual decimal.Decimal `json:"new_interest_rate_annual"`
	NewMarginAmount       decimal.Decimal `json:"new_margin_amount"`
	Reason                string          `json:"reason"`
}

type WriteOffLoanInput struct {
	LoanID uuid.UUID `json:"loan_id"`
	Reason string    `json:"reason"`
}

type RecoverWrittenOffLoanInput struct {
	LoanID         uuid.UUID       `json:"loan_id"`
	RecoveryAmount decimal.Decimal `json:"recovery_amount"`
}

type LoanService interface {
	ApplyLoan(ctx context.Context, input ApplyLoanInput, aoID uuid.UUID) (*Loan, error)
	ApproveLoan(ctx context.Context, loanID uuid.UUID, supervisorID uuid.UUID) (*Loan, error)
	RejectLoan(ctx context.Context, loanID uuid.UUID, supervisorID uuid.UUID) (*Loan, error)
	DisburseLoan(ctx context.Context, loanID uuid.UUID, disburserID uuid.UUID) (*Loan, error)
	GetLoan(ctx context.Context, id uuid.UUID) (*Loan, error)
	ListLoans(ctx context.Context, page, pageSize int) ([]Loan, int, error)
	PayInstallment(ctx context.Context, input PayInstallmentInput, tellerID uuid.UUID) (*LoanSchedule, error)
	RestructureLoan(ctx context.Context, input RestructureLoanInput, supervisorID uuid.UUID) (*Loan, error)
	WriteOffLoan(ctx context.Context, input WriteOffLoanInput, supervisorID uuid.UUID) (*Loan, error)
	RecoverWrittenOffLoan(ctx context.Context, input RecoverWrittenOffLoanInput, tellerID uuid.UUID) (*Loan, error)
}
