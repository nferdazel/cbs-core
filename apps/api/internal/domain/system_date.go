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
	// ErrEODInProgress menandai tutup hari lain yang sedang berjalan. Berbeda dari
	// ErrEODAlreadyRunForDate yang berarti tanggalnya memang sudah ditutup.
	ErrEODInProgress = errors.New("tutup hari sedang berjalan; tunggu sampai selesai")
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

// EODStepStatus menandai hasil satu langkah tutup hari.
type EODStepStatus string

const (
	// EODStepRan berarti langkah dijalankan dan selesai tanpa galat.
	EODStepRan EODStepStatus = "RAN"
	// EODStepFailed berarti langkah dijalankan tetapi gagal.
	EODStepFailed EODStepStatus = "FAILED"
	// EODStepSkipped berarti langkah tidak dijalankan: prasyaratnya gagal/tidak
	// berjalan pada tanggal bisnis yang sama, atau layanannya tidak dikonfigurasi.
	EODStepSkipped EODStepStatus = "SKIPPED"
)

// EODStepResult menyatakan apa yang terjadi pada satu langkah tutup hari. Tanpa
// daftar ini, langkah yang menolak berjalan karena prasyaratnya gagal tidak
// terlihat, dan ringkasan bisa tampak sah padahal dasarnya hasil run sebelumnya.
type EODStepResult struct {
	Name   string        `json:"name"`
	Status EODStepStatus `json:"status"`
	// Prerequisites adalah nama langkah yang harus RAN lebih dulu.
	Prerequisites []string `json:"prerequisites,omitempty"`
	// Reason diisi pada status FAILED/SKIPPED agar operator tahu mengapa.
	Reason string `json:"reason,omitempty"`
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
	DepositsRolledOver   int             `json:"deposits_rolled_over"`
	PPAPProcessed        int             `json:"ppap_processed"`
	LoanPenaltiesAccrued int             `json:"loan_penalties_accrued"`
	LoanPenaltyAmount    decimal.Decimal `json:"loan_penalty_amount"`
	// Akrual pendapatan bunga kredit berbasis jadwal angsuran (peristiwa EOD kelima).
	LoanInterestAccrued       int             `json:"loan_interest_accrued"`
	LoanInterestAccruedAmount decimal.Decimal `json:"loan_interest_accrued_amount"`
	// Amortisasi saldo kerugian restrukturisasi ke pendapatan bunga (peristiwa EOD
	// keenam, di balik loan.restructure.loss.enabled).
	LoanLossAmortized       int             `json:"loan_loss_amortized"`
	LoanLossAmortizedAmount decimal.Decimal `json:"loan_loss_amortized_amount"`
	AccountsMarkedDormant   int             `json:"accounts_marked_dormant"`
	// Langkah mencatat status setiap pekerjaan harian: RAN, FAILED, atau SKIPPED.
	// Langkah yang bergantung pada langkah lain (mis. perbandingan CKPN terhadap
	// required_ppap hasil PPAP) menolak berjalan bila prasyaratnya tidak RAN pada
	// tanggal bisnis yang sama, dan penolakan itu tampil di sini.
	Steps []EODStepResult `json:"steps,omitempty"`
	// Perbandingan CKPN vs PPKA tanggal bisnis ini. Baca-saja, dihitung hanya bila
	// langkah PPAP berhasil; nilainya nol bila dilewati.
	CKPNCompared           int             `json:"ckpn_compared"`
	CKPNFailed             int             `json:"ckpn_failed"`
	CKPNTotalPPKA          decimal.Decimal `json:"ckpn_total_ppka"`
	CKPNTotalCKPN          decimal.Decimal `json:"ckpn_total_ckpn"`
	CKPNModalIntiDeduction decimal.Decimal `json:"ckpn_modal_inti_deduction"`
	Warnings               []string        `json:"warnings,omitempty"`
	ExecutedBy             uuid.UUID       `json:"executed_by"`
	CompletedAt            time.Time       `json:"completed_at"`
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
	// TryEODLock mengambil kunci eksklusif tutup hari pada sesi database. Selama
	// kunci dipegang, permintaan tutup hari lain ditolak dengan ErrEODInProgress.
	// Klaim status saja tidak cukup: status EOD berarti "sedang berjalan" sekaligus
	// "pernah berhenti di tengah", dan keduanya harus dibedakan. Kunci dilepas
	// otomatis bila koneksi berakhir, sehingga proses yang mati di tengah tidak
	// meninggalkan kunci permanen; fungsi release yang dikembalikan melepasnya lebih
	// awal.
	TryEODLock(ctx context.Context) (func() error, error)
}

type BatchProcessService interface {
	GetCurrentBusinessDate(ctx context.Context) (*SystemBusinessDate, error)
	RunEOD(ctx context.Context, executedBy uuid.UUID) (*EODSummaryResult, error)
	RunEOM(ctx context.Context, executedBy uuid.UUID) (*EOMSummaryResult, error)
	RunEOY(ctx context.Context, book string, executedBy uuid.UUID) (*EOYSummaryResult, error)
}
