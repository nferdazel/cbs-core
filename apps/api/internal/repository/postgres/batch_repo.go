package postgres

import (
	"context"
	"database/sql"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// BatchActivityRepository menghitung aktivitas jurnal harian untuk ringkasan EOD.
type BatchActivityRepository struct {
	db *sql.DB
}

func NewBatchActivityRepository(db *sql.DB) *BatchActivityRepository {
	return &BatchActivityRepository{db: db}
}

// DailyActivity menghitung jumlah jurnal, total setoran tunai teller, total penarikan,
// dan penempatan deposito berjangka satu tanggal. Setoran diambil dari sisi KREDIT
// dan penarikan dari sisi DEBIT rekening nasabah (transaksi DEPOSIT/WITHDRAWAL),
// sehingga sisi kas tidak menghitung ganda.
//
// Penempatan deposito (DEPOSIT_PLACEMENT) TIDAK lagi dicampur ke total setoran: ia
// dilaporkan tersendiri sebagai jumlah dan nominal. Namanya "setoran" kini hanya untuk
// DEPOSIT (tunai teller).
//
// SocialFundBalance melaporkan saldo akun 12500 "Dana Kebajikan" sampai tanggal
// tersebut (kredit dikurangi debit), sebagai subquery terpisah dari agregat harian.
// Ia baca-saja dan tidak memengaruhi angka aktivitas lain.
//
// Keterbatasan data lama: jurnal penempatan yang dibuat sebelum jenis DEPOSIT_PLACEMENT
// ada (migrasi 000057) tetap bertipe DEPOSIT dan ikut terhitung sebagai setoran tunai.
// Baris lama tidak menyimpan penanda pasti untuk membedakannya dari setoran tunai,
// sehingga reklasifikasi berbasis deskripsi/prefix sengaja tidak dilakukan.
func (r *BatchActivityRepository) DailyActivity(ctx context.Context, date time.Time) (*domain.DailyActivitySummary, error) {
	var summary domain.DailyActivitySummary
	// MAX(...) NULL berarti akun 12500 belum pernah bergerak sampai tanggal itu.
	var lastMovement sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT je.id),
		       COALESCE(SUM(CASE WHEN je.transaction_type = 'DEPOSIT' AND jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN je.transaction_type = 'WITHDRAWAL' AND jl.direction = 'DEBIT'  THEN jl.amount ELSE 0 END), 0),
		       COUNT(DISTINCT CASE WHEN je.transaction_type = 'DEPOSIT_PLACEMENT' THEN je.id END),
		       COALESCE(SUM(CASE WHEN je.transaction_type = 'DEPOSIT_PLACEMENT' AND jl.direction = 'CREDIT' THEN jl.amount ELSE 0 END), 0),
		       COALESCE((
		           SELECT SUM(CASE WHEN jl2.direction = 'CREDIT' THEN jl2.amount ELSE -jl2.amount END)
		           FROM journal_lines jl2
		           JOIN journal_entries je2 ON je2.id = jl2.journal_entry_id
		           JOIN accounts a2 ON a2.id = jl2.account_id
		           JOIN chart_of_accounts coa2 ON coa2.id = a2.coa_id
		           WHERE coa2.code = '12500' AND je2.entry_date <= $1::date
		       ), 0),
		       (SELECT MAX(je3.entry_date)
		           FROM journal_lines jl3
		           JOIN journal_entries je3 ON je3.id = jl3.journal_entry_id
		           JOIN accounts a3 ON a3.id = jl3.account_id
		           JOIN chart_of_accounts coa3 ON coa3.id = a3.coa_id
		           WHERE coa3.code = '12500' AND je3.entry_date <= $1::date)
		FROM journal_entries je
		JOIN journal_lines jl ON jl.journal_entry_id = je.id
		WHERE je.entry_date = $1::date`, date,
	).Scan(
		&summary.PostedJournals,
		&summary.TotalDepositAmount,
		&summary.TotalWithdrawalAmount,
		&summary.DepositPlacementCount,
		&summary.TotalDepositPlacementAmount,
		&summary.SocialFundBalance,
		&lastMovement,
	)
	if err != nil {
		return nil, err
	}
	if lastMovement.Valid {
		summary.SocialFundLastMovement = lastMovement.Time
	}
	return &summary, nil
}

var _ domain.BatchActivityRepository = (*BatchActivityRepository)(nil)
