package postgres

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// SumDebitByCreatedByAndDate menjumlahkan sisi DEBIT jurnal milik satu pelaku pada
// satu tanggal entri. Hanya sisi debit yang dihitung: transfer internal menulis dua
// baris (debit dan kredit) dengan nominal sama, sehingga menjumlahkan keduanya akan
// menghitung satu transaksi dua kali.
func (r *LedgerRepository) SumDebitByCreatedByAndDate(ctx context.Context, createdBy string, date time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(jl.amount), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE je.created_by = $1
		  AND je.entry_date = $2::date
		  AND jl.direction = 'DEBIT'`,
		createdBy, date,
	).Scan(&total)
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

var _ domain.DailyDebitSumReader = (*LedgerRepository)(nil)
