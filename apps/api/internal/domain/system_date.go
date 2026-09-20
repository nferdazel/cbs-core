package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrEODAlreadyRunForDate = errors.New("end of day (EOD) process has already been executed for this business date")
	ErrInvalidBusinessDate  = errors.New("business date cannot be set to a past date")
)

type BusinessDateStatus string

const (
	BusinessDateStatusOpen   BusinessDateStatus = "OPEN"
	BusinessDateStatusEOD    BusinessDateStatus = "IN_EOD_PROCESSING"
	BusinessDateStatusClosed BusinessDateStatus = "CLOSED"
)

type SystemBusinessDate struct {
	CurrentDate time.Time          `json:"current_date"` // YYYY-MM-DD
	Status      BusinessDateStatus `json:"status"`
	UpdatedBy   *uuid.UUID         `json:"updated_by,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

type EODSummaryResult struct {
	ExecutedDate               time.Time       `json:"executed_date"`
	NextBusinessDate           time.Time       `json:"next_business_date"`
	TotalPostedJournalsToday   int             `json:"total_posted_journals_today"`
	TotalDepositAmountToday    decimal.Decimal `json:"total_deposit_amount_today"`
	TotalWithdrawalAmountToday decimal.Decimal `json:"total_withdrawal_amount_today"`
	// Pekerjaan harian berikut bersifat best-effort: kegagalannya tidak
	// menggagalkan tutup hari, tetapi selalu tampil di Warnings agar tidak
	// terlihat sukses padahal tidak berjalan.
	DepositsRolledOver    int             `json:"deposits_rolled_over"`
	PPAPProcessed         int             `json:"ppap_processed"`
	LoanPenaltiesAccrued  int             `json:"loan_penalties_accrued"`
	LoanPenaltyAmount     decimal.Decimal `json:"loan_penalty_amount"`
	// Akrual pendapatan bunga kredit berbasis jadwal angsuran (peristiwa EOD kelima).
	LoanInterestAccrued       int             `json:"loan_interest_accrued"`
	LoanInterestAccruedAmount decimal.Decimal `json:"loan_interest_accrued_amount"`
	AccountsMarkedDormant int             `json:"accounts_marked_dormant"`
	Warnings              []string        `json:"warnings,omitempty"`
	ExecutedBy            uuid.UUID       `json:"executed_by"`
	CompletedAt           time.Time       `json:"completed_at"`
}

type EOMSummaryResult struct {
	ExecutedMonth          string          `json:"executed_month"` // YYYY-MM
	TotalAdminFeesDeducted decimal.Decimal `json:"total_admin_fees_deducted"`
	TotalInterestPaid      decimal.Decimal `json:"total_interest_paid"`
	ProcessedAccounts      int             `json:"processed_accounts"`
	FailedAccounts         int             `json:"failed_accounts"`
	CompletedAt            time.Time       `json:"completed_at"`
}

type EOYSummaryResult struct {
	FiscalYear          int             `json:"fiscal_year"`
	TotalRevenueClosed  decimal.Decimal `json:"total_revenue_closed"`
	TotalExpenseClosed  decimal.Decimal `json:"total_expense_closed"`
	NetRetainedEarnings decimal.Decimal `json:"net_retained_earnings"`
	ClosingJournalRef   string          `json:"closing_journal_ref"`
	Books               []EOYBookResult `json:"books"`
	CompletedAt         time.Time       `json:"completed_at"`
}

// --- Interfaces ---

type BusinessDateRepository interface {
	GetCurrentDate(ctx context.Context) (*SystemBusinessDate, error)
	AdvanceDate(ctx context.Context, nextDate time.Time, updatedBy uuid.UUID) error
	// ClaimEOD memindahkan status tanggal bisnis ke EOD dalam SATU statement, dan
	// menolak bila tanggal sudah CLOSED. Hasil false berarti tutup hari tidak boleh
	// dijalankan. Ini menggantikan pola baca-lalu-tulis yang membuat dua permintaan
	// bersamaan dapat sama-sama lolos dan menjalankan pekerjaan harian dua kali.
	ClaimEOD(ctx context.Context) (bool, error)
}

type BatchProcessService interface {
	GetCurrentBusinessDate(ctx context.Context) (*SystemBusinessDate, error)
	RunEOD(ctx context.Context, executedBy uuid.UUID) (*EODSummaryResult, error)
	RunEOM(ctx context.Context, executedBy uuid.UUID) (*EOMSummaryResult, error)
	RunEOY(ctx context.Context, book string, executedBy uuid.UUID) (*EOYSummaryResult, error)
}
