package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// LoanInterestAccrualCandidate adalah satu angsuran kredit konvensional yang sudah
// jatuh tempo, belum dibayar, dan belum diakru bunganya. Satu baris mewakili satu
// angsuran; proses batch mengelompokkannya per kredit karena transaksi dan jurnalnya
// per kredit.
type LoanInterestAccrualCandidate struct {
	LoanID     uuid.UUID
	LoanNumber string
	ProductID  *uuid.UUID
	// LoanType dipakai untuk memastikan hanya kredit konvensional yang diakru.
	LoanType   LoanType
	LoanStatus LoanStatus

	ScheduleID    uuid.UUID
	InstallmentNo int
	DueDate       time.Time
	// OutstandingProfit adalah porsi bunga angsuran yang belum dibayar
	// (profit_amount - paid_profit). Nilainya persis porsi bunga jadwal, sehingga
	// tidak ada selisih pembulatan yang perlu direkonsiliasi.
	OutstandingProfit decimal.Decimal
	// OldestDueDate adalah jatuh tempo angsuran tertua yang belum dibayar, dipakai
	// untuk menghitung DPD dan kolektibilitas lewat aturan yang sama dengan PPAP.
	OldestDueDate *time.Time
	// FinalDueDate adalah jatuh tempo angsuran terakhir Kredit, dipakai menilai
	// dimensi "Kredit telah jatuh tempo" bersama DPD.
	FinalDueDate *time.Time
}

// LoanInterestAccrualItem adalah hasil pemrosesan satu angsuran.
type LoanInterestAccrualItem struct {
	LoanID           uuid.UUID
	LoanNumber       string
	InstallmentNo    int
	DPD              int
	Amount           decimal.Decimal
	JournalReference string
	Status           string // BatchItemAccrued / BatchItemSkipped / BatchItemFailed
	Message          string
}

// LoanInterestAccrualFailure mencatat angsuran yang gagal diproses tanpa
// menggagalkan seluruh batch.
type LoanInterestAccrualFailure struct {
	LoanID     uuid.UUID
	LoanNumber string
	Error      string
}

// LoanInterestAccrualSummary merangkum satu kali eksekusi akrual bunga kredit.
//
// Warnings berisi kondisi yang bukan kegagalan teknis tetapi membuat angsuran tidak
// diakru, mis. produk konvensional yang belum punya pemetaan INTEREST_ACCRUAL. Tanpa
// peringatan, batch tampak sukses padahal tidak mengakui pendapatan apa pun.
type LoanInterestAccrualSummary struct {
	AsOf         time.Time
	Total        int
	Processed    int
	Accrued      int
	Skipped      int
	Failed       int
	TotalAccrued decimal.Decimal
	Items        []LoanInterestAccrualItem
	Failures     []LoanInterestAccrualFailure
	Warnings     []string
}
