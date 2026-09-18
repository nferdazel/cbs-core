package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status hasil pemrosesan batch per rekening.
const (
	BatchItemAccrued = "ACCRUED"
	BatchItemCharged = "CHARGED"
	BatchItemPaid    = "PAID"
	BatchItemSkipped = "SKIPPED"
	BatchItemFailed  = "FAILED"
)

// SavingsAccountInfo adalah data rekening simpanan yang diperlukan untuk akrual
// bunga/bagi hasil dan pemotongan biaya administrasi bulanan.
type SavingsAccountInfo struct {
	AccountID          uuid.UUID
	AccountNumber      string
	COACode            string
	ProductID          uuid.UUID
	ProductCode        string
	Family             ProductFamily
	Book               COABook
	ProfitScheme       ProfitScheme
	RateAnnual         decimal.Decimal
	ProfitSharingRatio decimal.Decimal
	AdminFee           decimal.Decimal
	Status             AccountStatus
	Currency           string
	Balance            decimal.Decimal
	AvailableBalance   decimal.Decimal
}

// DailyNetChange adalah perubahan saldo satu rekening pada satu tanggal entri.
type DailyNetChange struct {
	AccountID uuid.UUID
	Date      time.Time
	Delta     decimal.Decimal
}

// DailyBalance adalah saldo rekening pada akhir satu hari.
type DailyBalance struct {
	Date    time.Time
	Balance decimal.Decimal
}

// InterestAccrualRecord adalah penanda idempotensi sekaligus jejak akrual satu
// rekening pada satu periode (YYYY-MM). Unique (AccountID, Period).
type InterestAccrualRecord struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	AccountNumber  string
	Period         string
	Book           COABook
	ProfitScheme   ProfitScheme
	Amount         decimal.Decimal
	AverageBalance decimal.Decimal
	ExpenseCOACode string
	PayableCOACode string
	JournalEntryID *uuid.UUID
	// PaidAt terisi setelah bunga dipindahkan ke rekening nasabah. Akrual yang
	// belum dibayar tidak boleh dibayar dua kali.
	PaidAt *time.Time
}

// AdminFeeChargeRecord adalah penanda pemotongan biaya administrasi bulanan.
type AdminFeeChargeRecord struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	AccountNumber  string
	Period         string
	Amount         decimal.Decimal
	RevenueCOACode string
	JournalEntryID *uuid.UUID
}

// SavingsInterestResult adalah hasil satu rekening pada akrual bunga/bagi hasil.
type SavingsInterestResult struct {
	AccountNumber    string
	Book             COABook
	ProfitScheme     ProfitScheme
	AverageBalance   decimal.Decimal
	Amount           decimal.Decimal
	ExpenseCOACode   string
	PayableCOACode   string
	JournalReference string
	Status           string
	Message          string
}

// SavingsInterestSummary merangkum satu kali eksekusi akrual batch.
type SavingsInterestSummary struct {
	Period            string
	Book              COABook // kosong = semua buku
	ProcessedAccounts int
	AccruedAccounts   int
	SkippedAccounts   int
	FailedAccounts    int
	TotalInterest     decimal.Decimal
	Results           []SavingsInterestResult
}

// AdminFeeResult adalah hasil pemotongan biaya administrasi satu rekening.
type AdminFeeResult struct {
	AccountNumber    string
	Amount           decimal.Decimal
	JournalReference string
	Status           string
	Message          string
}

// AdminFeeSummary merangkum satu kali eksekusi pemotongan biaya administrasi.
type AdminFeeSummary struct {
	Period            string
	ProcessedAccounts int
	ChargedAccounts   int
	SkippedAccounts   int
	FailedAccounts    int
	TotalAdminFees    decimal.Decimal
	Results           []AdminFeeResult
}

// AverageDailyBalance menghitung rata-rata saldo harian. Daftar saldo harian harus
// sudah mencakup setiap hari kalender periode; pemanggil yang mengisi hari tanpa mutasi.
func AverageDailyBalance(daily []DailyBalance) decimal.Decimal {
	if len(daily) == 0 {
		return decimal.Zero
	}
	total := decimal.Zero
	for _, d := range daily {
		total = total.Add(d.Balance)
	}
	return total.Div(decimal.NewFromInt(int64(len(daily))))
}

// SavingsInterest menghitung bunga dari saldo harian dengan tarif tahunan (persen).
// Nilai dibulatkan sekali di akhir agar total penjumlahan sama dengan jumlah rekening.
func SavingsInterest(daily []DailyBalance, rateAnnual decimal.Decimal, daysInYear int) decimal.Decimal {
	if daysInYear <= 0 {
		daysInYear = 365
	}
	if rateAnnual.IsZero() {
		return decimal.Zero
	}
	dailyRate := rateAnnual.Div(decimal.NewFromInt(100)).Div(decimal.NewFromInt(int64(daysInYear)))
	total := decimal.Zero
	for _, d := range daily {
		total = total.Add(d.Balance.Mul(dailyRate))
	}
	return RoundToRupiah(total)
}

// MudharabahShare menghitung bagi hasil satu rekening: nisbah dikali laba yang
// didistribusikan, dialokasikan proporsional terhadap rata-rata saldo harian rekening
// dibanding total saldo seluruh rekening mudharabah. Dibulatkan per rekening.
func MudharabahShare(distributableProfit, nisbah, averageBalance, totalAverageBalance decimal.Decimal) decimal.Decimal {
	if totalAverageBalance.IsZero() || distributableProfit.IsZero() || nisbah.IsZero() {
		return decimal.Zero
	}
	share := distributableProfit.Mul(nisbah).Mul(averageBalance).Div(totalAverageBalance)
	return RoundToRupiah(share)
}

// SavingsInterestRepository menyediakan data rekening simpanan, saldo harian, dan
// penanda idempotensi. Semua SQL berada di lapisan repository.
type SavingsInterestRepository interface {
	// ListSavingsAccounts mengembalikan rekening simpanan/giro aktif beserta produknya.
	ListSavingsAccounts(ctx context.Context) ([]SavingsAccountInfo, error)
	GetSavingsAccount(ctx context.Context, accountNumber string) (*SavingsAccountInfo, error)

	// OpeningBalances mengembalikan saldo awal sebelum satu tanggal untuk rekening
	// yang punya jejak jurnal. Rekening tanpa entri apa pun tidak muncul di peta.
	OpeningBalances(ctx context.Context, before time.Time) (map[uuid.UUID]decimal.Decimal, error)
	// DailyNetChanges mengembalikan perubahan saldo per rekening per tanggal entri.
	DailyNetChanges(ctx context.Context, from, to time.Time) ([]DailyNetChange, error)

	InsertInterestAccrual(ctx context.Context, tx any, rec *InterestAccrualRecord) (bool, error)
	UpdateInterestAccrualJournal(ctx context.Context, tx any, accountID uuid.UUID, period string, journalID uuid.UUID) error
	// ListUnpaidAccruals mengembalikan akrual satu periode yang belum dipindahkan
	// ke rekening nasabah.
	ListUnpaidAccruals(ctx context.Context, period string) ([]InterestAccrualRecord, error)
	// MarkAccrualPaid menandai akrual sudah dibayar. Hanya berlaku bila belum
	// pernah ditandai, sehingga pembayaran ganda tidak terjadi.
	MarkAccrualPaid(ctx context.Context, tx any, id uuid.UUID, journalID uuid.UUID) error
	InsertAdminFeeCharge(ctx context.Context, tx any, rec *AdminFeeChargeRecord) (bool, error)
	UpdateAdminFeeChargeJournal(ctx context.Context, tx any, accountID uuid.UUID, period string, journalID uuid.UUID) error
}

// SavingsInterestService menjalankan operasi batch tingkat rekening bulanan. Akrual
// tersedia untuk batch (AccrueAll) maupun satu rekening (AccrueAccount).
type SavingsInterestService interface {
	AccrueAll(ctx context.Context, period time.Time, book COABook, createdBy string) (*SavingsInterestSummary, error)
	AccrueAccount(ctx context.Context, accountNumber string, period time.Time, createdBy string) (*SavingsInterestResult, error)
	// PayInterestToAccounts memindahkan akrual yang belum dibayar ke rekening
	// nasabah: debit utang bunga, kredit rekening nasabah.
	PayInterestToAccounts(ctx context.Context, period time.Time, createdBy string) (*SavingsInterestSummary, error)
	ChargeAdminFees(ctx context.Context, period time.Time, createdBy string) (*AdminFeeSummary, error)
}
