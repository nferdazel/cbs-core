package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// ReportingRepository menghitung laporan keuangan langsung dari journal_lines
// (sumber kebenaran akuntansi), bukan dari accounts.balance yang bisa tidak sinkron.
type ReportingRepository struct {
	db *sql.DB
}

func NewReportingRepository(db *sql.DB) *ReportingRepository {
	return &ReportingRepository{db: db}
}

// cashAndBankCOACodes adalah kode COA kelompok kas dan bank pada bagan baku
// (seed 000001 dan 000005). Konvensional: 10100 Kas, 10101 Kas Teller, 10200
// Penempatan pada Bank Lain. Syariah: 11100 Kas Syariah, 11200 Penempatan pada
// Bank Syariah. Semuanya bertipe ASSET; hanya kode ini yang dihitung sebagai kas.
var cashAndBankCOACodes = []string{"10100", "10101", "10200", "11100", "11200"}

// appendPlaceholders menulis "$n, $n+1, ..." untuk values lalu menambahkannya ke args.
// Dipakai hanya untuk daftar kode COA konstanta, bukan input pengguna.
func appendPlaceholders(args *[]any, values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("$%d", len(*args)+1)
		*args = append(*args, v)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// TrialBalance mengagregasi per akun COA daun: mutasi debit/kredit periode,
// saldo awal (jurnal sebelum from), dan saldo akhir menurut sifat saldo normal.
func (r *ReportingRepository) TrialBalance(ctx context.Context, from, to time.Time, book string) ([]domain.TrialBalanceRow, error) {
	// from dan to adalah tanggal inklusif; periode memakai entry_date, bukan created_at.
	args := []any{from, to}
	bookFilter := ""
	if book != "" {
		args = append(args, book)
		bookFilter = fmt.Sprintf(" AND c.book = $%d", len(args))
	}

	q := fmt.Sprintf(`
		SELECT c.code, c.name, c.book, c.normal_balance,
			COALESCE(SUM(CASE WHEN je.entry_date >= $1::date AND je.entry_date <= $2::date AND jl.direction = 'DEBIT'  THEN jl.amount ELSE 0 END), 0) AS period_debit,
			COALESCE(SUM(CASE WHEN je.entry_date >= $1::date AND je.entry_date <= $2::date AND jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0) AS period_credit,
			COALESCE(SUM(CASE WHEN je.entry_date < $1::date AND jl.direction = 'DEBIT'  THEN jl.amount ELSE 0 END), 0) AS opening_debit,
			COALESCE(SUM(CASE WHEN je.entry_date < $1::date AND jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0) AS opening_credit
		FROM chart_of_accounts c
		LEFT JOIN accounts a ON a.coa_id = c.id
		LEFT JOIN journal_lines jl ON jl.account_id = a.id
		LEFT JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE c.is_header = FALSE%s
		GROUP BY c.code, c.name, c.book, c.normal_balance
		ORDER BY c.code ASC`, bookFilter)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.TrialBalanceRow, 0)
	for rows.Next() {
		var row domain.TrialBalanceRow
		var periodDebit, periodCredit, openingDebit, openingCredit decimal.Decimal
		if err := rows.Scan(&row.AccountCode, &row.AccountName, &row.Book, &row.NormalBalance,
			&periodDebit, &periodCredit, &openingDebit, &openingCredit); err != nil {
			return nil, err
		}
		row.TotalDebit = periodDebit
		row.TotalCredit = periodCredit
		row.OpeningBalance = domain.ClosingBalance(row.NormalBalance, decimal.Zero, openingDebit, openingCredit)
		row.ClosingBalance = domain.ClosingBalance(row.NormalBalance, row.OpeningBalance, periodDebit, periodCredit)
		list = append(list, row)
	}
	return list, rows.Err()
}

// IncomeStatement mengagregasi pendapatan dan beban periode dari kolom type COA.
// Nominal akun disajikan menurut sifat alaminya agar pendapatan positif.
func (r *ReportingRepository) IncomeStatement(ctx context.Context, from, to time.Time, book string) (domain.IncomeStatement, error) {
	args := []any{from, to}
	bookFilter := ""
	if book != "" {
		args = append(args, book)
		bookFilter = fmt.Sprintf(" AND c.book = $%d", len(args))
	}

	q := fmt.Sprintf(`
		SELECT c.code, c.name, c.book, c.type,
			COALESCE(SUM(CASE WHEN jl.direction = 'DEBIT'  THEN jl.amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0) AS total_credit
		FROM chart_of_accounts c
		JOIN accounts a ON a.coa_id = c.id
		JOIN journal_lines jl ON jl.account_id = a.id
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE c.is_header = FALSE
			AND c.type IN ('REVENUE', 'EXPENSE')
			AND je.entry_date >= $1::date AND je.entry_date <= $2::date%s
		GROUP BY c.code, c.name, c.book, c.type
		ORDER BY c.code ASC`, bookFilter)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return domain.IncomeStatement{}, err
	}
	defer rows.Close()

	result := domain.IncomeStatement{Rows: make([]domain.ReportRow, 0)}
	for rows.Next() {
		var code, name, bookCode string
		var acctype domain.COAType
		var debit, credit decimal.Decimal
		if err := rows.Scan(&code, &name, &bookCode, &acctype, &debit, &credit); err != nil {
			return domain.IncomeStatement{}, err
		}

		// Sifat alami: pendapatan (CREDIT) = kredit - debit; beban (DEBIT) = debit - kredit.
		amount := domain.ClosingBalance(domain.NatureOf(acctype), decimal.Zero, debit, credit)
		result.Rows = append(result.Rows, domain.ReportRow{
			AccountCode: code,
			AccountName: name,
			Book:        bookCode,
			Amount:      amount,
		})

		if acctype == domain.COATypeRevenue {
			result.TotalRevenue = result.TotalRevenue.Add(amount)
		} else {
			result.TotalExpense = result.TotalExpense.Add(amount)
		}
	}
	if err := rows.Err(); err != nil {
		return domain.IncomeStatement{}, err
	}

	result.NetIncome = domain.NetIncome(result.TotalRevenue, result.TotalExpense)
	return result, nil
}

// BalanceSheet menyusun aset, kewajiban, dan ekuitas per tanggal dari jurnal.
//
// Catatan penyeimbang: akun pendapatan/beban belum ditutup ke ekuitas selama periode
// berjalan, sehingga Aset = Kewajiban + Ekuitas baru terpenuhi bila laba/rugi berjalan
// disertakan. Karena itu laba/rugi berjalan ditambahkan sebagai baris penyeimbang.
// Bila setelah itu masih ada selisih (jurnal tidak seimbang atau akun tanpa COA),
// selisih ditambahkan sebagai baris eksplisit agar laporan tetap tie-out.
func (r *ReportingRepository) BalanceSheet(ctx context.Context, asOf time.Time, book string) (domain.BalanceSheet, error) {
	args := []any{asOf}
	bookFilter := ""
	if book != "" {
		args = append(args, book)
		bookFilter = fmt.Sprintf(" AND c.book = $%d", len(args))
	}

	// LEFT JOIN: semua akun daun tetap muncul; mutasi dibatasi entry_date <= asOf
	// lewat CASE agar akun tanpa mutasi tidak menghilangkan akun lain.
	q := fmt.Sprintf(`
		SELECT c.code, c.name, c.book, c.type,
			COALESCE(SUM(CASE WHEN je.entry_date <= $1::date AND jl.direction = 'DEBIT'  THEN jl.amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN je.entry_date <= $1::date AND jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0) AS total_credit
		FROM chart_of_accounts c
		LEFT JOIN accounts a ON a.coa_id = c.id
		LEFT JOIN journal_lines jl ON jl.account_id = a.id
		LEFT JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE c.is_header = FALSE%s
		GROUP BY c.code, c.name, c.book, c.type
		ORDER BY c.code ASC`, bookFilter)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return domain.BalanceSheet{}, err
	}
	defer rows.Close()

	result := domain.BalanceSheet{Rows: make([]domain.ReportRow, 0)}
	totalRevenue := decimal.Zero
	totalExpense := decimal.Zero

	for rows.Next() {
		var code, name, bookCode string
		var acctype domain.COAType
		var debit, credit decimal.Decimal
		if err := rows.Scan(&code, &name, &bookCode, &acctype, &debit, &credit); err != nil {
			return domain.BalanceSheet{}, err
		}

		amount := domain.ClosingBalance(domain.NatureOf(acctype), decimal.Zero, debit, credit)
		if amount.IsZero() {
			continue
		}

		switch acctype {
		case domain.COATypeAsset:
			result.TotalAssets = result.TotalAssets.Add(amount)
		case domain.COATypeLiability:
			result.TotalLiabilities = result.TotalLiabilities.Add(amount)
		case domain.COATypeEquity:
			result.TotalEquity = result.TotalEquity.Add(amount)
		case domain.COATypeRevenue:
			totalRevenue = totalRevenue.Add(amount)
		case domain.COATypeExpense:
			totalExpense = totalExpense.Add(amount)
		}

		result.Rows = append(result.Rows, domain.ReportRow{
			AccountCode: code,
			AccountName: name,
			Book:        bookCode,
			Amount:      amount,
		})
	}
	if err := rows.Err(); err != nil {
		return domain.BalanceSheet{}, err
	}

	result.NetIncome = domain.NetIncome(totalRevenue, totalExpense)

	// Laba/rugi berjalan adalah baris penyeimbang antara aset dan kewajiban+ekuitas.
	if !result.TotalAssets.Equal(result.TotalLiabilities.Add(result.TotalEquity)) {
		result.Rows = append(result.Rows, domain.ReportRow{
			AccountCode: "-",
			AccountName: "Laba/Rugi Berjalan",
			Book:        book,
			Amount:      result.NetIncome,
		})
	}

	if residual := result.TotalAssets.Sub(result.TotalLiabilities.Add(result.TotalEquity).Add(result.NetIncome)); !residual.IsZero() {
		result.Rows = append(result.Rows, domain.ReportRow{
			AccountCode: "00000",
			AccountName: "Penyeimbang (selisih jurnal)",
			Book:        book,
			Amount:      residual,
		})
		result.NetIncome = result.NetIncome.Add(residual)
	}

	return result, nil
}

// CashFlow menyusun arus kas dari jurnal pada akun kas/bank. Setiap baris lawan
// (counterpart) dari jurnal yang menyentuh kas diklasifikasikan menurut tipe akunnya:
//   - REVENUE/EXPENSE          -> operasi
//   - ASSET non-kas            -> investasi
//   - LIABILITY/EQUITY         -> pendanaan
//
// Kontribusi = amount untuk sisi CREDIT, -amount untuk sisi DEBIT. Jurnal yang hanya
// memindahkan antar akun kas tidak punya counterpart sehingga netto nol.
func (r *ReportingRepository) CashFlow(ctx context.Context, from, to time.Time, book string) (domain.CashFlow, error) {
	args := []any{from, to}
	bookFilter := ""
	if book != "" {
		args = append(args, book)
		bookFilter = fmt.Sprintf(" AND cc.book = $%d", len(args))
	}
	cashIn := appendPlaceholders(&args, cashAndBankCOACodes)
	cashNotIn := appendPlaceholders(&args, cashAndBankCOACodes)

	q := fmt.Sprintf(`
		WITH cash_entries AS (
			SELECT DISTINCT je.id AS entry_id
			FROM journal_entries je
			JOIN journal_lines cl ON cl.journal_entry_id = je.id
			JOIN accounts ca ON ca.id = cl.account_id
			JOIN chart_of_accounts cc ON cc.id = ca.coa_id
			WHERE cc.is_header = FALSE
				AND cc.type = 'ASSET'
				AND cc.code IN %s
				AND je.entry_date >= $1::date AND je.entry_date <= $2::date%s
		)
		SELECT cp_coa.code, cp_coa.name, cp_coa.book, cp_coa.type, cp.direction,
			COALESCE(SUM(cp.amount), 0) AS amount
		FROM journal_lines cp
		JOIN accounts cpa ON cpa.id = cp.account_id
		JOIN chart_of_accounts cp_coa ON cp_coa.id = cpa.coa_id
		WHERE cp_coa.is_header = FALSE
			AND cp_coa.code NOT IN %s
			AND cp.journal_entry_id IN (SELECT entry_id FROM cash_entries)
		GROUP BY cp_coa.code, cp_coa.name, cp_coa.book, cp_coa.type, cp.direction
		ORDER BY cp_coa.code ASC`, cashIn, bookFilter, cashNotIn)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return domain.CashFlow{}, err
	}
	defer rows.Close()

	result := domain.CashFlow{Rows: make([]domain.ReportRow, 0)}
	for rows.Next() {
		var code, name, bookCode string
		var acctype domain.COAType
		var direction domain.EntryDirection
		var amount decimal.Decimal
		if err := rows.Scan(&code, &name, &bookCode, &acctype, &direction, &amount); err != nil {
			return domain.CashFlow{}, err
		}

		contribution := amount
		if direction == domain.DirectionDebit {
			contribution = amount.Neg()
		}

		switch acctype {
		case domain.COATypeRevenue, domain.COATypeExpense:
			result.Operating = result.Operating.Add(contribution)
		case domain.COATypeAsset:
			result.Investing = result.Investing.Add(contribution)
		case domain.COATypeLiability, domain.COATypeEquity:
			result.Financing = result.Financing.Add(contribution)
		}

		result.Rows = append(result.Rows, domain.ReportRow{
			AccountCode: code,
			AccountName: name,
			Book:        bookCode,
			Amount:      contribution,
		})
	}
	if err := rows.Err(); err != nil {
		return domain.CashFlow{}, err
	}

	result.NetChange = result.Operating.Add(result.Investing).Add(result.Financing)
	return result, nil
}
