package postgres

import (
	"context"
	"database/sql"
	"time"

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
// Posisi yang dibaca adalah AKHIR PERIODE (asOf), bukan keadaan saat ekspor dijalankan:
// saldo rekening simpanan direkonstruksi dari jurnal (entry_date <= asOf), dan kontrak
// deposito dipilih menurut tanggal mulai serta penutupannya. Dengan begitu ekspor yang
// ditunda tetap melaporkan posisi bulan yang diminta.
type BankDepositRepository struct {
	db *sql.DB
}

func NewBankDepositRepository(db *sql.DB) *BankDepositRepository {
	return &BankDepositRepository{db: db}
}

// bankDepositAggregateQuery menghimpun simpanan bank lawan dari dua sumber pada posisi
// akhir periode $1.
//
// Rekening simpanan: saldo diambil dari jurnal sebagai kewajiban (kredit menambah,
// debit mengurangi), bukan dari accounts.balance yang hanya menyimpan keadaan terakhir
// (pola sama dengan SavingsInterestRepository.OpeningBalances). Rekening yang belum ada
// pada asOf (created_at setelah asOf) dan yang saldonya nol pada asOf tidak dihitung.
//
// Deposito berjangka: kontrak dihitung bila sudah ditempatkan pada asOf (start_date <=
// asOf) dan belum ditutup/dicairkan pada asOf (closed_at NULL atau setelah asOf).
// Rollover menulis ulang start_date/placement_amount sehingga riwayat kontrak sebelum
// rollover tidak dapat direkonstruksi; yang dilaporkan adalah kontrak sebagaimana
// tercatat pada asOf.
//
// Kolom "diblokir" tidak punya sumber historis: accounts.hold_balance hanya menyimpan
// keadaan saat ini dan kontrak deposito belum punya kolom padanannya. Karena posisi
// yang diblokir tidak dapat direkonstruksi per akhir periode, kolom ini ditulis "-"
// beserta alasan di form (bukan nol) — lihat form13.go.
//
// Sandi kategori bank pada IN bersumber dari Lampiran 02 (tabel ojk_pihak_lawan,
// migrasi 000102); daftarnya tetap agar tidak berubah tanpa pembacaan ulang lampiran.
const bankDepositAggregateQuery = `
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
			COALESCE(br.code, '') AS branch_code,
			COALESCE(c.cif_number, '') AS counterparty_cif,
			COALESCE(c.ojk_pihak_lawan_code, '') AS jenis_bank_code,
			COALESCE(c.ojk_hubungan_bank_code, '') AS hubungan_bank_code,
			COALESCE(c.ojk_kabupaten_code, '') AS location_code,
			'01' AS jenis,
			COALESCE(sr.saldo, 0) AS nominal
		FROM accounts a
		LEFT JOIN saldo_rekening sr ON sr.account_id = a.id
		JOIN customers c ON c.id = a.customer_id
		LEFT JOIN branches br ON br.id = a.branch_id
		WHERE a.account_type IN ('SAVINGS', 'CHECKING')
			AND a.created_at::date <= $1::date
			AND COALESCE(sr.saldo, 0) <> 0
			AND c.ojk_pihak_lawan_code IN ('001', '600', '601', '700', '701', '901')
		UNION ALL
		SELECT
			COALESCE(br.code, '') AS branch_code,
			COALESCE(c.cif_number, '') AS counterparty_cif,
			COALESCE(c.ojk_pihak_lawan_code, '') AS jenis_bank_code,
			COALESCE(c.ojk_hubungan_bank_code, '') AS hubungan_bank_code,
			COALESCE(c.ojk_kabupaten_code, '') AS location_code,
			'02' AS jenis,
			d.placement_amount AS nominal
		FROM time_deposits d
		JOIN customers c ON c.id = d.customer_id
		LEFT JOIN branches br ON br.id = d.branch_id
		WHERE d.start_date <= $1::date
			AND (d.closed_at IS NULL OR d.closed_at::date > $1::date)
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
		COALESCE(SUM(nominal), 0)
	FROM simpanan
	GROUP BY branch_code, counterparty_cif, jenis_bank_code, hubungan_bank_code, location_code, jenis
	ORDER BY jenis_bank_code, counterparty_cif, branch_code, jenis`

func (r *BankDepositRepository) ListBankDeposits(ctx context.Context, asOf time.Time) ([]domain.BankDepositAggregate, error) {
	result, err := r.db.QueryContext(ctx, bankDepositAggregateQuery, asOf)
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
			&row.AccountCount, &row.TotalNominal,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, result.Err()
}

var _ domain.BankDepositRepository = (*BankDepositRepository)(nil)
