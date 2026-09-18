package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	// ErrNoNominalAccounts menandakan tidak ada saldo pendapatan/beban untuk ditutup.
	ErrNoNominalAccounts = errors.New("tidak ada akun nominal yang perlu ditutup")
	// ErrClosingUnbalanced menandakan jurnal penutup tidak seimbang. Jurnal rusak
	// tidak boleh diposting.
	ErrClosingUnbalanced = errors.New("jurnal penutup tidak seimbang")
)

// NominalAccountBalance adalah saldo satu akun pendapatan/beban pada satu periode,
// dinyatakan menurut sifat alaminya (pendapatan positif saat bersaldo kredit, beban
// positif saat bersaldo debit), diambil dari jurnal bukan dari accounts.balance.
type NominalAccountBalance struct {
	COACode     string
	COAName     string
	AccountType COAType
	Book        COABook
	Amount      decimal.Decimal
}

// ClosingEntry adalah satu sisi jurnal penutup yang belum diterjemahkan ke nomor akun GL.
type ClosingEntry struct {
	COACode     string
	AccountType COAType
	Direction   EntryDirection
	Amount      decimal.Decimal
}

// YearEndClosingRecord adalah penanda idempotensi tutup buku per tahun per buku.
type YearEndClosingRecord struct {
	ID                  uuid.UUID
	FiscalYear          int
	Book                COABook
	TotalRevenue        decimal.Decimal
	TotalExpense        decimal.Decimal
	NetIncome           decimal.Decimal
	RetainedEarningsCOA string
	JournalEntryID      *uuid.UUID
	JournalReference    string
}

// EOYBookResult adalah ringkasan penutupan satu buku. Buku konvensional dan syariah
// dipisah agar laba ditahan keduanya tidak tercampur.
type EOYBookResult struct {
	Book                    COABook         `json:"book"`
	TotalRevenueClosed      decimal.Decimal `json:"total_revenue_closed"`
	TotalExpenseClosed      decimal.Decimal `json:"total_expense_closed"`
	NetRetainedEarnings     decimal.Decimal `json:"net_retained_earnings"`
	RetainedEarningsCOACode string          `json:"retained_earnings_coa_code"`
	ClosingJournalRef       string          `json:"closing_journal_ref"`
	AlreadyClosed           bool            `json:"already_closed"`
}

// YearEndRepository membaca saldo akun nominal dari jurnal dan menyimpan penanda
// tutup buku. SQL agregasi jurnal berada di lapisan repository.
type YearEndRepository interface {
	// NominalBalances mengembalikan saldo akun REVENUE/EXPENSE periode fiskal untuk
	// satu buku. book wajib agar buku syariah tidak tercampur konvensional.
	NominalBalances(ctx context.Context, from, to time.Time, book COABook) ([]NominalAccountBalance, error)
	GetYearEndClosing(ctx context.Context, fiscalYear int, book COABook) (*YearEndClosingRecord, error)
	InsertYearEndClosing(ctx context.Context, tx any, rec *YearEndClosingRecord) (bool, error)
	UpdateYearEndClosingJournal(ctx context.Context, tx any, fiscalYear int, book COABook, journalID uuid.UUID) error
}

// ComputeClosingEntries membentuk sisi jurnal penutup dari saldo akun nominal.
// Untuk akun pendapatan, saldo alami kredit ditutup dengan DEBIT; untuk akun beban,
// saldo alami debit ditutup dengan KREDIT. Selisihnya (laba/rugi) masuk ke akun laba
// ditahan: laba dikredit, rugi didebit. Fungsi murni, tanpa DB, agar dapat diuji.
//
// Saldo alami negatif (mis. beban yang pembatalannya melebihi beban) ditutup dengan
// arah berlawanan agar jurnal tetap seimbang.
func ComputeClosingEntries(
	balances []NominalAccountBalance,
	retainedEarningsCOA string,
) (entries []ClosingEntry, totalRevenue, totalExpense, netIncome decimal.Decimal, err error) {
	for _, b := range balances {
		if b.Amount.IsZero() {
			continue
		}
		switch b.AccountType {
		case COATypeRevenue:
			totalRevenue = totalRevenue.Add(b.Amount)
			entries = append(entries, ClosingEntry{
				COACode:     b.COACode,
				AccountType: b.AccountType,
				Direction:   closingDirection(COATypeRevenue, b.Amount),
				Amount:      b.Amount.Abs(),
			})
		case COATypeExpense:
			totalExpense = totalExpense.Add(b.Amount)
			entries = append(entries, ClosingEntry{
				COACode:     b.COACode,
				AccountType: b.AccountType,
				Direction:   closingDirection(COATypeExpense, b.Amount),
				Amount:      b.Amount.Abs(),
			})
		default:
			// Akun non-nominal tidak boleh masuk jalur penutupan.
			continue
		}
	}

	netIncome = NetIncome(totalRevenue, totalExpense)
	if !netIncome.IsZero() {
		if retainedEarningsCOA == "" {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero,
				errors.New("kode COA laba ditahan wajib diisi untuk menutup selisih")
		}
		direction := DirectionCredit
		if netIncome.IsNegative() {
			direction = DirectionDebit
		}
		entries = append(entries, ClosingEntry{
			COACode:     retainedEarningsCOA,
			AccountType: COATypeEquity,
			Direction:   direction,
			Amount:      netIncome.Abs(),
		})
	}

	if len(entries) == 0 {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, ErrNoNominalAccounts
	}

	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, e := range entries {
		if e.Direction == DirectionDebit {
			totalDebit = totalDebit.Add(e.Amount)
		} else {
			totalCredit = totalCredit.Add(e.Amount)
		}
	}
	if !totalDebit.Equal(totalCredit) {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, ErrClosingUnbalanced
	}

	return entries, totalRevenue, totalExpense, netIncome, nil
}

// closingDirection menentukan arah jurnal penutup: kebalikan sifat saldo alami akun.
func closingDirection(t COAType, amount decimal.Decimal) EntryDirection {
	natural := NatureOf(t)
	if amount.IsNegative() {
		if natural == BalanceTypeDebit {
			return DirectionDebit
		}
		return DirectionCredit
	}
	if natural == BalanceTypeDebit {
		return DirectionCredit
	}
	return DirectionDebit
}
