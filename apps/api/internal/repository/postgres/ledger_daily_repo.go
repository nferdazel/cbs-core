package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// SumDebitByCreatedByAndDate menjumlahkan sisi DEBIT jurnal milik satu pelaku pada
// satu TANGGAL BISNIS (kolom entry_date), bukan tanggal kalender created_at. Hanya
// sisi debit yang dihitung: transfer internal menulis dua baris (debit dan kredit)
// dengan nominal sama, sehingga menjumlahkan keduanya akan menghitung satu transaksi
// dua kali. entry_date diisi posting engine dari tanggal bisnis bank (WIB), jadi
// parameter businessDate harus berasal dari BusinessDateRepository yang sama.
func (r *LedgerRepository) SumDebitByCreatedByAndDate(ctx context.Context, createdBy string, businessDate time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(jl.amount), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE je.created_by = $1
		  AND je.entry_date = $2::date
		  AND jl.direction = 'DEBIT'`,
		createdBy, businessDate,
	).Scan(&total)
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

var _ domain.DailyDebitSumReader = (*LedgerRepository)(nil)

// SumPendingDebitByMakerAndAction menjumlahkan nominal pengajuan maker-checker yang
// masih PENDING milik satu pembuat untuk satu jenis aksi pada satu TANGGAL BISNIS.
// Penjaga batas harian ikut menghitungnya agar N pengajuan yang masing-masing di bawah
// batas harian tidak dapat disetujui semua dan melampaui batas setelah terposting.
//
// Tanggal bisnis diambil dari kolom business_date (migrasi 000058), ditulis CreateRequest
// dari BusinessDateRepository saat pengajuan dibuat. Tidak memakai created_at::date:
// tanggal kalender UTC membuat pengajuan yang dibuat setelah tutup hari belum berjalan
// tidak terhitung pada hari bisnis yang sedang berjalan. Baris lama yang business_date-nya
// NULL (pra-migrasi) jatuh ke tanggal kalender WIB created_at, bukan UTC.
//
// Nominal dibaca dari payload->>'amount'. Kunci itu SELALU diisi CreateRequest dari
// nominal input, terlepas dari bentuk payload pemanggil, sehingga aman dipakai untuk
// semua jenis aksi (mis. PLACE_DEPOSIT yang payload aslinya memakai placement_amount).
// Pencocokan pembuat memakai maker_username atau maker_id agar tetap benar baik Username
// terisi maupun hanya UUID. Ekspresi regex menjaga satu baris payload rusak tidak
// menggagalkan seluruh kueri (cast numeric yang gagal akan menolak transaksi).
func (r *LedgerRepository) SumPendingDebitByMakerAndAction(ctx context.Context, maker, actionType string, businessDate time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			CASE WHEN payload->>'amount' ~ '^-?[0-9]+(\.[0-9]+)?$'
			     THEN (payload->>'amount')::numeric
			     ELSE 0 END), 0)
		FROM maker_checker_requests
		WHERE status = 'PENDING'
		  AND action_type = $1
		  AND (payload->>'maker_username' = $2 OR payload->>'maker_id' = $2)
		  AND COALESCE(business_date, (created_at AT TIME ZONE 'Asia/Jakarta')::date) = $3::date`,
		actionType, maker, businessDate,
	).Scan(&total)
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

// LockDailyEvaluation mengambil kunci advisory transaksi untuk bucket akumulasi harian
// (pembuat, jenis transaksi, tanggal bisnis). Kunci bersifat transaction-scoped:
// dilepas otomatis saat transaksi eksekusi commit/rollback. Urutan penguncian di jalur
// eksekusi adalah: kunci baris pengajuan (UPDATE status), lalu kunci advisory ini,
// lalu kunci rekening saat jurnal ditulis. Karena kunci advisory selalu diambil SEBELUM
// rekening dikunci dan tidak ada alur lain yang memegangnya, tidak ada siklus tunggu.
//
// Kunci diambil lewat transaksi pemanggil (tx), bukan pool, agar masa pegangnya persis
// selama satu transaksi eksekusi. Bila pemanggil tidak punya transaksi konkret, evaluasi
// ditolak alih-alih berjalan tanpa serialisasi.
func (r *LedgerRepository) LockDailyEvaluation(ctx context.Context, tx any, maker, txType string, businessDate time.Time) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("batas harian: evaluasi serial memerlukan transaksi database")
	}
	key := maker + "|" + strings.ToLower(strings.TrimSpace(txType)) + "|" + businessDate.Format("2006-01-02")
	if _, err := sqlTx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return fmt.Errorf("batas harian: mengambil kunci evaluasi: %w", err)
	}
	return nil
}
