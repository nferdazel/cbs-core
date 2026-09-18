package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
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

// ARORunner menjalankan perpanjangan otomatis deposito yang jatuh tempo. Dipisah
// dari DepositService agar batch tidak bergantung pada seluruh permukaan layanan
// deposito, sekaligus menghindari siklus dependensi.
type ARORunner interface {
	RunARO(ctx context.Context, asOf time.Time, actor Actor) (int, error)
}

// PPAPRunner menjalankan perhitungan kolektibilitas dan cadangan PPAP harian.
type PPAPRunner interface {
	RunDaily(ctx context.Context, asOf time.Time, actor Actor) (PPAPRunSummary, error)
}

// SystemActor membangun identitas pelaku untuk pekerjaan batch yang tidak berasal
// dari permintaan HTTP. Username diisi id pengguna yang menjalankan batch supaya
// jurnal dan audit tetap dapat ditelusuri ke orang yang memicunya.
func SystemActor(executedBy uuid.UUID) Actor {
	return Actor{UserID: executedBy, Username: executedBy.String(), Role: RoleSystem}
}
