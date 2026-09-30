package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// PihakLawanRepository menyimpan register pihak lawan Form 00.16 (migrasi 000130). SQL
// tetap di repository, bukan di service.
type PihakLawanRepository struct {
	db *sql.DB
}

func NewPihakLawanRepository(db *sql.DB) *PihakLawanRepository {
	return &PihakLawanRepository{db: db}
}

// pihakLawanColumns tidak memuat kolom III Nomor Identitas dan VI NPWP: keduanya sengaja
// tidak disimpan (keputusan privasi) dan laporan menulis "-" beserta alasannya.
const pihakLawanColumns = `id, pihak_lawan_id, jenis_identitas_code, jenis_kelamin_code, nama,
	       kewarganegaraan_code, negara_code, jenis_usaha_code, hubungan_bank_code,
	       golongan_code, lembaga_pemeringkat_code, peringkat_code,
	       tanggal_pemeringkatan, tanggal_lahir, lokasi_code, grup_id, grup_nama,
	       telepon, alamat, note, created_at, updated_at`

const listPihakLawanItemsQuery = `
	SELECT ` + pihakLawanColumns + `
	FROM pihak_lawan_register
	ORDER BY golongan_code, nama, id`

// ListItems membaca seluruh register pihak lawan bank-wide untuk UI edit.
func (r *PihakLawanRepository) ListItems(ctx context.Context) ([]domain.PihakLawanItem, error) {
	return r.queryPihakLawan(ctx, listPihakLawanItemsQuery)
}

// ListPihakLawanForOJK membaca baris register untuk Form 00.16, urutan deterministik.
func (r *PihakLawanRepository) ListPihakLawanForOJK(ctx context.Context) ([]domain.PihakLawanItem, error) {
	return r.queryPihakLawan(ctx, listPihakLawanItemsQuery)
}

// queryPihakLawan menjalankan query pembacaan dan memetakan baris register.
func (r *PihakLawanRepository) queryPihakLawan(ctx context.Context, query string, args ...any) ([]domain.PihakLawanItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.PihakLawanItem
	for rows.Next() {
		var item domain.PihakLawanItem
		var tanggalPemeringkatan, tanggalLahir sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.PihakLawanID, &item.JenisIdentitasCode, &item.JenisKelaminCode,
			&item.Nama, &item.KewarganegaraanCode, &item.NegaraCode, &item.JenisUsahaCode,
			&item.HubunganBankCode, &item.GolonganCode, &item.LembagaPemeringkatCode,
			&item.PeringkatCode, &tanggalPemeringkatan, &tanggalLahir, &item.LokasiCode,
			&item.GrupID, &item.GrupNama, &item.Telepon, &item.Alamat, &item.Note,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if tanggalPemeringkatan.Valid {
			item.TanggalPemeringkatan = &tanggalPemeringkatan.Time
		}
		if tanggalLahir.Valid {
			item.TanggalLahir = &tanggalLahir.Time
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id).
func (r *PihakLawanRepository) UpsertItemTx(ctx context.Context, tx any, item domain.PihakLawanItem, actorID uuid.UUID) error {
	sqlTx, err := requirePihakLawanTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO pihak_lawan_register (
			id, pihak_lawan_id, jenis_identitas_code, jenis_kelamin_code, nama,
			kewarganegaraan_code, negara_code, jenis_usaha_code, hubungan_bank_code,
			golongan_code, lembaga_pemeringkat_code, peringkat_code,
			tanggal_pemeringkatan, tanggal_lahir, lokasi_code, grup_id, grup_nama,
			telepon, alamat, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			$18, $19, $20, NOW(), NOW(), $21, $21
		)
		ON CONFLICT (id) DO UPDATE SET
			pihak_lawan_id = EXCLUDED.pihak_lawan_id,
			jenis_identitas_code = EXCLUDED.jenis_identitas_code,
			jenis_kelamin_code = EXCLUDED.jenis_kelamin_code,
			nama = EXCLUDED.nama,
			kewarganegaraan_code = EXCLUDED.kewarganegaraan_code,
			negara_code = EXCLUDED.negara_code,
			jenis_usaha_code = EXCLUDED.jenis_usaha_code,
			hubungan_bank_code = EXCLUDED.hubungan_bank_code,
			golongan_code = EXCLUDED.golongan_code,
			lembaga_pemeringkat_code = EXCLUDED.lembaga_pemeringkat_code,
			peringkat_code = EXCLUDED.peringkat_code,
			tanggal_pemeringkatan = EXCLUDED.tanggal_pemeringkatan,
			tanggal_lahir = EXCLUDED.tanggal_lahir,
			lokasi_code = EXCLUDED.lokasi_code,
			grup_id = EXCLUDED.grup_id,
			grup_nama = EXCLUDED.grup_nama,
			telepon = EXCLUDED.telepon,
			alamat = EXCLUDED.alamat,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.PihakLawanID, item.JenisIdentitasCode, item.JenisKelaminCode, item.Nama,
		item.KewarganegaraanCode, item.NegaraCode, item.JenisUsahaCode, item.HubunganBankCode,
		item.GolonganCode, item.LembagaPemeringkatCode, item.PeringkatCode,
		nullablePihakLawanDate(item.TanggalPemeringkatan), nullablePihakLawanDate(item.TanggalLahir),
		item.LokasiCode, item.GrupID, item.GrupNama, item.Telepon, item.Alamat, item.Note,
		nullablePihakLawanActor(actorID))
	return err
}

// DeleteItemTx menghapus satu baris secara fisik. Form 00.16 tidak menetapkan nomor
// register unik, sehingga DELETE fisik dibenarkan. found=false bila barisnya tidak ada.
func (r *PihakLawanRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requirePihakLawanTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM pihak_lawan_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requirePihakLawanTx memastikan transaksi tulis sah, konsisten dengan pola repository lain.
func requirePihakLawanTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register pihak lawan: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullablePihakLawanDate menulis NULL untuk tanggal yang tidak dicatat.
func nullablePihakLawanDate(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// nullablePihakLawanActor menulis NULL untuk aktor tanpa UUID (mis. sistem).
func nullablePihakLawanActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.PihakLawanRepository = (*PihakLawanRepository)(nil)
