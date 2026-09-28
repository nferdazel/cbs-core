package postgres

import (
	"context"
	"database/sql"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// SavingsAccountRepository menyediakan baris per rekening tabungan (produk keluarga
// SAVINGS) pada posisi akhir periode, kebutuhan Form 11.00 "Daftar Tabungan".
//
// Golongan nasabah dibaca apa adanya dari customers.ojk_pihak_lawan_code (Lampiran 02),
// hubungan dari customers.ojk_hubungan_bank_code, dan lokasi dari
// customers.ojk_kabupaten_code (Lampiran 03). Sandi yang belum diisi dibiarkan kosong
// dan form menulis "-", bukan menebak.
//
// Posisi yang dibaca adalah AKHIR PERIODE (asOf), bukan keadaan saat ekspor
// dijalankan: saldo rekening direkonstruksi dari jurnal (journal_entries.entry_date <=
// asOf), bukan dari accounts.balance yang menyimpan keadaan terakhir saja.
type SavingsAccountRepository struct {
	db *sql.DB
}

func NewSavingsAccountRepository(db *sql.DB) *SavingsAccountRepository {
	return &SavingsAccountRepository{db: db}
}

// savingsAccountReportQuery menghimpun rekening tabungan pada posisi akhir periode $1.
//
// Keluarga produk dibatasi SAVINGS agar giro (CURRENT_ACCOUNT) dan rekening deposito
// (produk TIME_DEPOSIT, yang memakai accounts row sendiri) tidak ikut sebagai tabungan.
// Rekening tanpa product_id (data lama) tidak dapat dipastikan keluarganya sehingga
// dilewati. Rekening yang belum ada pada asOf (created_at setelah asOf) dan yang
// saldonya nol pada asOf tidak dilaporkan. Filter `closed_at` pada query saat ini
// tidak menjamin apa pun: kolom `accounts.closed_at` tidak pernah ditulis aplikasi
// (penutupan hanya menyetel status dan mewajibkan saldo nol), jadi pengaman
// sebenarnya adalah syarat saldo nol di atas.
//
// Suku bunga diambil dari produk (rate_annual) dan hanya bermakna sebagai persen bila
// skema imbal hasilnya INTEREST; form yang memutuskan pemakaiannya.
const savingsAccountReportQuery = `
	WITH saldo_rekening AS (
		SELECT jl.account_id,
			COALESCE(SUM(CASE
				WHEN je.entry_date <= $1::date AND jl.direction = 'CREDIT' THEN jl.amount
				WHEN je.entry_date <= $1::date AND jl.direction = 'DEBIT' THEN -jl.amount
				ELSE 0 END), 0) AS saldo
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		JOIN accounts a ON a.id = jl.account_id
		WHERE a.account_type IN ('SAVINGS', 'CHECKING')
		GROUP BY jl.account_id
	)
	SELECT
		a.account_number,
		COALESCE(c.cif_number, '') AS counterparty_cif,
		COALESCE(c.ojk_pihak_lawan_code, '') AS customer_type_code,
		COALESCE(c.ojk_hubungan_bank_code, '') AS hubungan_bank_code,
		COALESCE(c.ojk_kabupaten_code, '') AS location_code,
		p.profit_scheme::text AS profit_scheme,
		p.rate_annual AS interest_rate_annual,
		COALESCE(sr.saldo, 0) AS saldo
	FROM accounts a
	JOIN banking_products p ON p.id = a.product_id
	JOIN customers c ON c.id = a.customer_id
	LEFT JOIN saldo_rekening sr ON sr.account_id = a.id
	WHERE p.family = 'SAVINGS'
		AND a.created_at::date <= $1::date
		AND (a.closed_at IS NULL OR a.closed_at::date > $1::date)
		AND COALESCE(sr.saldo, 0) <> 0
	ORDER BY a.account_number`

func (r *SavingsAccountRepository) ListSavingsAccountsForOJK(ctx context.Context, asOf time.Time) ([]domain.SavingsAccountAggregate, error) {
	result, err := r.db.QueryContext(ctx, savingsAccountReportQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = result.Close() }()

	var out []domain.SavingsAccountAggregate
	for result.Next() {
		var row domain.SavingsAccountAggregate
		if err := result.Scan(
			&row.AccountNumber, &row.CounterpartyCIF, &row.CustomerTypeCode,
			&row.HubunganBankCode, &row.LocationCode, &row.ProfitScheme,
			&row.InterestRateAnnual, &row.Balance,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, result.Err()
}

var _ domain.SavingsAccountReportRepository = (*SavingsAccountRepository)(nil)
