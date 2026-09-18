package domain

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrLimitPerTransaction = errors.New("nominal melebihi batas per transaksi")
	ErrLimitDaily          = errors.New("akumulasi transaksi harian melebihi batas")
	ErrRequiresApproval    = errors.New("nominal melebihi ambang yang memerlukan persetujuan")
)

// TransactionLimit adalah batas transaksi seorang pelaku untuk satu jenis transaksi.
type TransactionLimit struct {
	PerTransaction        decimal.Decimal
	DailyAmount           decimal.Decimal
	RequiresApprovalAbove decimal.Decimal
}

// DailyDebitSumReader menjumlahkan sisi debit jurnal milik satu pelaku pada satu
// tanggal. Dipakai menghitung akumulasi harian tanpa memuat seluruh repository jurnal.
type DailyDebitSumReader interface {
	SumDebitByCreatedByAndDate(ctx context.Context, createdBy string, date time.Time) (decimal.Decimal, error)
}

type TransactionLimitService interface {
	// ForActor mengembalikan batas yang berlaku bagi role pelaku pada jenis transaksi.
	ForActor(ctx context.Context, actor Actor, txType string) (TransactionLimit, error)
	// Check memvalidasi nominal terhadap batas per transaksi, akumulasi harian, dan
	// ambang persetujuan. Pelanggaran dikembalikan sebagai sentinel error di domain.
	Check(ctx context.Context, actor Actor, txType string, amount decimal.Decimal) error
}
