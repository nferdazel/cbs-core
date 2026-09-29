package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// AsetTetapRegisterRepository menyimpan register aset tetap, inventaris, dan aset
// tidak berwujud (migrasi 000119): rincian per aset untuk Form 08.00. SQL tetap di
// repository, bukan di service.
type AsetTetapRegisterRepository struct {
	db *sql.DB
}

func NewAsetTetapRegisterRepository(db *sql.DB) *AsetTetapRegisterRepository {
	return &AsetTetapRegisterRepository{db: db}
}

// asetTetapRegisterColumns tidak memuat kolom VIII Nilai Tercatat: kolom itu turunan
// yang dihitung laporan, bukan data yang disimpan.
const asetTetapRegisterColumns = `id, jenis_aset_code, sumber_perolehan_code,
	       status_aset_code, biaya_perolehan, akumulasi_penyusutan_amortisasi,
	       akumulasi_kerugian_penurunan_nilai, metode_pengukuran_code, as_of,
	       status, note, created_at, updated_at`

const listAsetTetapItemsQuery = `
	SELECT ` + asetTetapRegisterColumns + `
	FROM aset_tetap_register
	ORDER BY as_of DESC, jenis_aset_code, sumber_perolehan_code, metode_pengukuran_code`

// ListItems membaca seluruh register aset bank-wide untuk UI edit.
func (r *AsetTetapRegisterRepository) ListItems(ctx context.Context) ([]domain.AsetTetapItem, error) {
	return r.queryAsetTetap(ctx, listAsetTetapItemsQuery)
}

const listAsetTetapForOJKQuery = `
	SELECT ` + asetTetapRegisterColumns + `
	FROM aset_tetap_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY jenis_aset_code, sumber_perolehan_code, metode_pengukuran_code, id`

// ListAsetTetapForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 08.00. Satu query, urutan deterministik agar pengelompokan stabil.
func (r *AsetTetapRegisterRepository) ListAsetTetapForOJK(ctx context.Context, asOf time.Time) ([]domain.AsetTetapItem, error) {
	return r.queryAsetTetap(ctx, listAsetTetapForOJKQuery, asOf)
}

// queryAsetTetap menjalankan query pembacaan dan memetakan baris register. status_aset_code
// NULL dipetakan ke string kosong.
func (r *AsetTetapRegisterRepository) queryAsetTetap(ctx context.Context, query string, args ...any) ([]domain.AsetTetapItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.AsetTetapItem
	for rows.Next() {
		var item domain.AsetTetapItem
		var statusAset sql.NullString
		if err := rows.Scan(
			&item.ID, &item.JenisAsetCode, &item.SumberPerolehanCode,
			&statusAset, &item.BiayaPerolehan, &item.AkumulasiPenyusutanAmortisasi,
			&item.AkumulasiKerugianPenurunanNilai, &item.MetodePengukuranCode, &item.AsOf,
			&item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.StatusAsetCode = statusAset.String
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertItemTx menyimpan satu aset (idempotensi lewat PRIMARY KEY id).
func (r *AsetTetapRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.AsetTetapItem, actorID uuid.UUID) error {
	sqlTx, err := requireAsetTetapTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO aset_tetap_register (
			id, jenis_aset_code, sumber_perolehan_code, status_aset_code,
			biaya_perolehan, akumulasi_penyusutan_amortisasi,
			akumulasi_kerugian_penurunan_nilai, metode_pengukuran_code, as_of,
			status, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW(), $12, $12
		)
		ON CONFLICT (id) DO UPDATE SET
			jenis_aset_code = EXCLUDED.jenis_aset_code,
			sumber_perolehan_code = EXCLUDED.sumber_perolehan_code,
			status_aset_code = EXCLUDED.status_aset_code,
			biaya_perolehan = EXCLUDED.biaya_perolehan,
			akumulasi_penyusutan_amortisasi = EXCLUDED.akumulasi_penyusutan_amortisasi,
			akumulasi_kerugian_penurunan_nilai = EXCLUDED.akumulasi_kerugian_penurunan_nilai,
			metode_pengukuran_code = EXCLUDED.metode_pengukuran_code,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.JenisAsetCode, item.SumberPerolehanCode,
		nullableAsetTetapStatusAset(item.StatusAsetCode),
		item.BiayaPerolehan, item.AkumulasiPenyusutanAmortisasi,
		item.AkumulasiKerugianPenurunanNilai, item.MetodePengukuranCode, item.AsOf,
		item.Status, item.Note, nullableAsetTetapActor(actorID))
	return err
}

// DeleteItemTx menghapus satu aset; found=false bila barisnya tidak ada.
func (r *AsetTetapRegisterRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireAsetTetapTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `
		DELETE FROM aset_tetap_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireAsetTetapTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requireAsetTetapTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register aset tetap: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableAsetTetapStatusAset menulis NULL untuk status aset kosong (aset tidak
// berwujud atau belum diisi), bukan string kosong, agar aturan nullable kolom terjaga.
func nullableAsetTetapStatusAset(code string) any {
	if code == "" {
		return nil
	}
	return code
}

// nullableAsetTetapActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK
// ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableAsetTetapActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.AsetTetapRegisterRepository = (*AsetTetapRegisterRepository)(nil)
