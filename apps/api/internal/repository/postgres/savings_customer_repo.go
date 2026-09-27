package postgres

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
)

// SavingsCustomerRepository menyediakan agregasi Form 00.14 "Jenis Nasabah dan Produk
// Simpanan". Satu query menggabungkan rekening tabungan/giro (accounts) dan kontrak
// deposito berjangka (time_deposits), lalu mengelompokkannya per produk dan golongan
// nasabah.
//
// Golongan nasabah dibaca dari customers.ojk_pihak_lawan_code (Lampiran 02 Daftar
// Sandi Pihak Lawan). Sandi yang belum diisi menghasilkan kelompok sendiri dengan
// kode kosong; laporan menulis "-".
//
// Sama seperti Form 06.00, yang dilaporkan adalah keadaan saat query dijalankan:
// sistem belum menyimpan riwayat saldo per akhir bulan.
type SavingsCustomerRepository struct {
	db *sql.DB
}

func NewSavingsCustomerRepository(db *sql.DB) *SavingsCustomerRepository {
	return &SavingsCustomerRepository{db: db}
}

// savingsCustomerAggregateQuery menggabungkan rekening simpanan (tabungan/giro) dan
// deposito berjangka. Rekening CLOSED dan deposito CLOSED/BROKEN tidak dihitung karena
// tidak lagi menjadi kewajiban pada posisi ini. Deposito MATURED tetap dihitung:
// kontrak yang sudah jatuh tempo belum dicairkan sampai statusnya CLOSED, sehingga
// masih menjadi kewajiban bank.
//
// LEFT JOIN produk dipakai agar baris lama tanpa product_id tidak hilang; kode/nama
// produk kosong akan tampil sebagai "-" pada laporan. Kolom account_type memakai
// daftar positif SAVINGS/CHECKING, konsisten dengan AccountRepository.ListAll.
const savingsCustomerAggregateQuery = `
	WITH simpanan AS (
		SELECT
			COALESCE(p.family::text, '') AS family,
			COALESCE(p.code, '') AS product_code,
			COALESCE(p.name, '') AS product_name,
			COALESCE(c.ojk_pihak_lawan_code, '') AS customer_type_code,
			a.balance AS amount
		FROM accounts a
		LEFT JOIN customers c ON c.id = a.customer_id
		LEFT JOIN banking_products p ON p.id = a.product_id
		WHERE a.account_type IN ('SAVINGS', 'CHECKING')
			AND a.status <> 'CLOSED'
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
		WHERE d.status IN ('PLACED', 'MATURED')
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

func (r *SavingsCustomerRepository) ListSavingsCustomerAggregates(ctx context.Context) ([]domain.SavingsCustomerAggregate, error) {
	result, err := r.db.QueryContext(ctx, savingsCustomerAggregateQuery)
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
