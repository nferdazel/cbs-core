package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// DailyActivitySummary adalah aktivitas jurnal satu tanggal bisnis, dihitung dari
// journal_entries/journal_lines agar EOD melaporkan angka nyata, bukan konstanta.
type DailyActivitySummary struct {
	PostedJournals        int
	TotalDepositAmount    decimal.Decimal
	TotalWithdrawalAmount decimal.Decimal
}

// BatchActivityRepository membaca aktivitas harian untuk ringkasan EOD.
type BatchActivityRepository interface {
	DailyActivity(ctx context.Context, date time.Time) (*DailyActivitySummary, error)
}
