package postgres

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
)

// BankDepositRepository menyediakan agregasi Form 13.00 "Daftar Simpanan dari Bank
// Lain". Satu query menggabungkan rekening tabungan/giro (accounts) dan kontrak
// deposito berjangka (time_deposits) milik nasabah yang bergolongan bank, lalu
// mengelompokkannya per bank lawan, kantor, dan jenis simpanan.
//
// Golongan bank dibaca dari customers.ojk_pihak_lawan_code (Lampiran 02 Daftar Sandi
// Pihak Lawan, migrasi 000102/000104). Hanya sandi kategori bank yang diikutkan:
// 001 Bank Indonesia, 600 BPR, 601 BPRS, 700 Bank Umum, 701 Bank Umum Syariah, dan
// 901 Unit Usaha Syariah. Nasabah tanpa sandi bank tidak dapat dikenali.
//
// Sama seperti agregasi internal jenis nasabah per produk (dulu Form 00.14), yang
// dilaporkan adalah keadaan saat query dijalankan: sistem belum menyimpan riwayat
// saldo per akhir bulan.
type BankDepositRepository struct {
	db *sql.DB
}

func NewBankDepositRepository(db *sql.DB) *BankDepositRepository {
	return &BankDepositRepository{db: db}
}

// bankDepositAggregateQuery menghimpun simpanan bank lawan dari dua sumber.
// Rekening CLOSED dan deposito CLOSED/BROKEN tidak dihitung karena tidak lagi menjadi
// kewajiban; deposito MATURED tetap dihitung sampai dicairkan (status CLOSED).
//
// accounts.hold_balance adalah bagian yang diblokir pada rekening simpanan; kontrak
// deposito belum punya kolom padanannya sehingga menyumbang 0.
//
// Sandi kategori bank pada IN bersumber dari Lampiran 02 (tabel ojk_pihak_lawan,
// migrasi 000102); daftarnya tetap agar tidak berubah tanpa pembacaan ulang lampiran.
const bankDepositAggregateQuery = `
	WITH simpanan AS (
		SELECT
			COALESCE(br.code, '') AS branch_code,
			COALESCE(c.cif_number, '') AS counterparty_cif,
			COALESCE(c.ojk_pihak_lawan_code, '') AS jenis_bank_code,
			COALESCE(c.ojk_hubungan_bank_code, '') AS hubungan_bank_code,
			COALESCE(c.ojk_kabupaten_code, '') AS location_code,
			'01' AS jenis,
			a.balance AS nominal,
			a.hold_balance AS blocked
		FROM accounts a
		JOIN customers c ON c.id = a.customer_id
		LEFT JOIN branches br ON br.id = a.branch_id
		WHERE a.account_type IN ('SAVINGS', 'CHECKING')
			AND a.status <> 'CLOSED'
			AND c.ojk_pihak_lawan_code IN ('001', '600', '601', '700', '701', '901')
		UNION ALL
		SELECT
			COALESCE(br.code, '') AS branch_code,
			COALESCE(c.cif_number, '') AS counterparty_cif,
			COALESCE(c.ojk_pihak_lawan_code, '') AS jenis_bank_code,
			COALESCE(c.ojk_hubungan_bank_code, '') AS hubungan_bank_code,
			COALESCE(c.ojk_kabupaten_code, '') AS location_code,
			'02' AS jenis,
			d.placement_amount AS nominal,
			0::numeric AS blocked
		FROM time_deposits d
		JOIN customers c ON c.id = d.customer_id
		LEFT JOIN branches br ON br.id = d.branch_id
		WHERE d.status IN ('PLACED', 'MATURED')
			AND c.ojk_pihak_lawan_code IN ('001', '600', '601', '700', '701', '901')
	)
	SELECT
		branch_code,
		counterparty_cif,
		jenis_bank_code,
		hubungan_bank_code,
		location_code,
		jenis,
		COUNT(*),
		COALESCE(SUM(nominal), 0),
		COALESCE(SUM(blocked), 0)
	FROM simpanan
	GROUP BY branch_code, counterparty_cif, jenis_bank_code, hubungan_bank_code, location_code, jenis
	ORDER BY jenis_bank_code, counterparty_cif, branch_code, jenis`

func (r *BankDepositRepository) ListBankDeposits(ctx context.Context) ([]domain.BankDepositAggregate, error) {
	result, err := r.db.QueryContext(ctx, bankDepositAggregateQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = result.Close() }()

	var out []domain.BankDepositAggregate
	for result.Next() {
		var row domain.BankDepositAggregate
		if err := result.Scan(
			&row.BranchCode, &row.CounterpartyCIF, &row.JenisBankCode,
			&row.HubunganBankCode, &row.LocationCode, &row.Jenis,
			&row.AccountCount, &row.TotalNominal, &row.TotalBlocked,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, result.Err()
}

var _ domain.BankDepositRepository = (*BankDepositRepository)(nil)
