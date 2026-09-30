package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// HapusBukuRepository menyimpan register aset produktif yang dihapus buku Form 15.00
// (migrasi 000129). SQL tetap di repository, bukan di service.
type HapusBukuRepository struct {
	db *sql.DB
}

func NewHapusBukuRepository(db *sql.DB) *HapusBukuRepository {
	return &HapusBukuRepository{db: db}
}

const hapusBukuColumns = `id, jenis_aset_code, pihak_lawan_id, nomor_rekening, jenis_debitur_code,
	       hubungan_bank_code, tanggal_hapus_buku,
	       saldo_pokok_saat_hapus, saldo_pokok_akum_tertagih, saldo_pokok_per_posisi,
	       bunga_saat_hapus, bunga_akum_tertagih, bunga_akum_tambahan, bunga_per_posisi,
	       agunan_jenis_code, agunan_alamat, agunan_nilai, note,
	       created_at, updated_at`

const listHapusBukuItemsQuery = `
	SELECT ` + hapusBukuColumns + `
	FROM hapus_buku_register
	ORDER BY tanggal_hapus_buku, jenis_aset_code, id`

// ListItems membaca seluruh register hapus buku bank-wide untuk UI edit.
func (r *HapusBukuRepository) ListItems(ctx context.Context) ([]domain.HapusBukuItem, error) {
	return r.queryHapusBuku(ctx, listHapusBukuItemsQuery)
}

// ListHapusBukuForOJK membaca baris register untuk Form 15.00, urutan deterministik.
func (r *HapusBukuRepository) ListHapusBukuForOJK(ctx context.Context) ([]domain.HapusBukuItem, error) {
	return r.queryHapusBuku(ctx, listHapusBukuItemsQuery)
}

// queryHapusBuku menjalankan query pembacaan dan memetakan baris register.
func (r *HapusBukuRepository) queryHapusBuku(ctx context.Context, query string, args ...any) ([]domain.HapusBukuItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.HapusBukuItem
	for rows.Next() {
		var item domain.HapusBukuItem
		if err := rows.Scan(
			&item.ID, &item.JenisAsetCode, &item.PihakLawanID, &item.NomorRekening,
			&item.JenisDebiturCode, &item.HubunganBankCode, &item.TanggalHapusBuku,
			&item.SaldoPokokSaatHapus, &item.SaldoPokokAkumTertagih, &item.SaldoPokokPerPosisi,
			&item.BungaSaatHapus, &item.BungaAkumTertagih, &item.BungaAkumTambahan, &item.BungaPerPosisi,
			&item.AgunanJenisCode, &item.AgunanAlamat, &item.AgunanNilai, &item.Note,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id).
func (r *HapusBukuRepository) UpsertItemTx(ctx context.Context, tx any, item domain.HapusBukuItem, actorID uuid.UUID) error {
	sqlTx, err := requireHapusBukuTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO hapus_buku_register (
			id, jenis_aset_code, pihak_lawan_id, nomor_rekening, jenis_debitur_code,
			hubungan_bank_code, tanggal_hapus_buku,
			saldo_pokok_saat_hapus, saldo_pokok_akum_tertagih, saldo_pokok_per_posisi,
			bunga_saat_hapus, bunga_akum_tertagih, bunga_akum_tambahan, bunga_per_posisi,
			agunan_jenis_code, agunan_alamat, agunan_nilai, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, NOW(), NOW(), $19, $19
		)
		ON CONFLICT (id) DO UPDATE SET
			jenis_aset_code = EXCLUDED.jenis_aset_code,
			pihak_lawan_id = EXCLUDED.pihak_lawan_id,
			nomor_rekening = EXCLUDED.nomor_rekening,
			jenis_debitur_code = EXCLUDED.jenis_debitur_code,
			hubungan_bank_code = EXCLUDED.hubungan_bank_code,
			tanggal_hapus_buku = EXCLUDED.tanggal_hapus_buku,
			saldo_pokok_saat_hapus = EXCLUDED.saldo_pokok_saat_hapus,
			saldo_pokok_akum_tertagih = EXCLUDED.saldo_pokok_akum_tertagih,
			saldo_pokok_per_posisi = EXCLUDED.saldo_pokok_per_posisi,
			bunga_saat_hapus = EXCLUDED.bunga_saat_hapus,
			bunga_akum_tertagih = EXCLUDED.bunga_akum_tertagih,
			bunga_akum_tambahan = EXCLUDED.bunga_akum_tambahan,
			bunga_per_posisi = EXCLUDED.bunga_per_posisi,
			agunan_jenis_code = EXCLUDED.agunan_jenis_code,
			agunan_alamat = EXCLUDED.agunan_alamat,
			agunan_nilai = EXCLUDED.agunan_nilai,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.JenisAsetCode, item.PihakLawanID, item.NomorRekening, item.JenisDebiturCode,
		item.HubunganBankCode, item.TanggalHapusBuku,
		item.SaldoPokokSaatHapus, item.SaldoPokokAkumTertagih, item.SaldoPokokPerPosisi,
		item.BungaSaatHapus, item.BungaAkumTertagih, item.BungaAkumTambahan, item.BungaPerPosisi,
		item.AgunanJenisCode, item.AgunanAlamat, item.AgunanNilai, item.Note,
		nullableHapusBukuActor(actorID))
	return err
}

// DeleteItemTx menghapus satu baris secara fisik. Form 15.00 tidak menetapkan nomor
// register unik, sehingga DELETE fisik dibenarkan. found=false bila barisnya tidak ada.
func (r *HapusBukuRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireHapusBukuTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM hapus_buku_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireHapusBukuTx memastikan transaksi tulis sah, konsisten dengan pola repository OJK lain.
func requireHapusBukuTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register hapus buku: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableHapusBukuActor menulis NULL untuk aktor tanpa UUID (mis. sistem).
func nullableHapusBukuActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.HapusBukuRepository = (*HapusBukuRepository)(nil)
