package postgres

import (
	"context"
	"database/sql"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// SavingsCustomerRepository menyediakan agregasi internal jenis nasabah per produk
// (dulu diberi nomor Form 00.14 — nomor itu tidak ada di SEOJK 16/2024, lihat
// docs/CELAH-FORM-OJK.md §6). Satu query menggabungkan rekening tabungan/giro
// (accounts) dan kontrak deposito berjangka (time_deposits), lalu mengelompokkannya per
// produk dan golongan nasabah.
//
// Golongan nasabah dibaca dari customers.ojk_pihak_lawan_code (Lampiran 02 Daftar
// Sandi Pihak Lawan). Sandi yang belum diisi menghasilkan kelompok sendiri dengan
// kode kosong; laporan menulis "-".
//
// Posisi yang dibaca adalah AKHIR PERIODE (asOf), bukan keadaan saat ekspor dijalankan:
// saldo rekening simpanan direkonstruksi dari jurnal (entry_date <= asOf) dan kontrak
// deposito dipilih menurut tanggal mulai serta penutupannya.
type SavingsCustomerRepository struct {
	db *sql.DB
}

func NewSavingsCustomerRepository(db *sql.DB) *SavingsCustomerRepository {
	return &SavingsCustomerRepository{db: db}
}

// savingsCustomerAggregateQuery menggabungkan rekening simpanan (tabungan/giro) dan
// deposito berjangka pada posisi akhir periode $1.
//
// Seperti Form 13.00, saldo rekening diambil dari jurnal (kredit menambah, debit
// mengurangi) agar posisi periode lampau tetap dapat direkonstruksi. Rekening yang
// belum ada pada asOf dan yang saldonya nol pada asOf tidak dihitung. Kontrak deposito
// dihitung bila sudah ditempatkan (start_date <= asOf) dan belum ditutup pada asOf
// (closed_at NULL atau setelah asOf).
//
// LEFT JOIN produk dipakai agar baris lama tanpa product_id tidak hilang; kode/nama
// produk kosong akan tampil sebagai "-" pada laporan. Kolom account_type memakai
// daftar positif SAVINGS/CHECKING, konsisten dengan AccountRepository.ListAll.
const savingsCustomerAggregateQuery = `
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
	),
	simpanan AS (
		SELECT
			COALESCE(p.family::text, '') AS family,
			COALESCE(p.code, '') AS product_code,
			COALESCE(p.name, '') AS product_name,
			COALESCE(c.ojk_pihak_lawan_code, '') AS customer_type_code,
			COALESCE(sr.saldo, 0) AS amount
		FROM accounts a
		LEFT JOIN saldo_rekening sr ON sr.account_id = a.id
		LEFT JOIN customers c ON c.id = a.customer_id
		LEFT JOIN banking_products p ON p.id = a.product_id
		WHERE a.account_type IN ('SAVINGS', 'CHECKING')
			AND a.created_at::date <= $1::date
			AND COALESCE(sr.saldo, 0) <> 0
		UNION ALL
		SELECT
			COALESCE(p.family::text, '') AS family,
			COALESCE(p.code, '') AS product_code,
			COALESCE(p.name, '') AS product_name,
			COALESCE(c.ojk_pihak_lawan_code, '') AS customer_type_code,
			d.placement_amount AS amount
		FROM time_deposits d
		LEFT JOIN customers c ON c.id = d.customer_id
		LEFT JOIN banking_products p ON p.id = d.product_id
		WHERE d.start_date <= $1::date
			AND (d.closed_at IS NULL OR d.closed_at::date > $1::date)
	)
	SELECT
		family,
		product_code,
		product_name,
		customer_type_code,
		COUNT(*),
		COALESCE(SUM(amount), 0)
	FROM simpanan
	GROUP BY family, product_code, product_name, customer_type_code
	ORDER BY family, product_code, customer_type_code`

func (r *SavingsCustomerRepository) ListSavingsCustomerAggregates(ctx context.Context, asOf time.Time) ([]domain.SavingsCustomerAggregate, error) {
	result, err := r.db.QueryContext(ctx, savingsCustomerAggregateQuery, asOf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = result.Close() }()

	var out []domain.SavingsCustomerAggregate
	for result.Next() {
		var row domain.SavingsCustomerAggregate
		var family string
		if err := result.Scan(
			&family, &row.ProductCode, &row.ProductName, &row.CustomerTypeCode,
			&row.AccountCount, &row.TotalAmount,
		); err != nil {
			return nil, err
		}
		row.ProductFamily = domain.ProductFamily(family)
		out = append(out, row)
	}
	return out, result.Err()
}

var _ domain.SavingsCustomerRepository = (*SavingsCustomerRepository)(nil)
