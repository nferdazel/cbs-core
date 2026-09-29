package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// PropertiRegisterRepository menyimpan register properti terbengkalai (migrasi
// 000118): rincian per properti untuk Form 17.00. SQL tetap di repository, bukan di
// service.
type PropertiRegisterRepository struct {
	db *sql.DB
}

func NewPropertiRegisterRepository(db *sql.DB) *PropertiRegisterRepository {
	return &PropertiRegisterRepository{db: db}
}

// propertiRegisterColumns tidak memuat kolom IX Jumlah: kolom itu turunan yang
// dihitung laporan, bukan data yang disimpan.
const propertiRegisterColumns = `id, no_register, jenis_properti_code, alamat_properti,
	       koordinat, tanggal_penetapan, biaya_perolehan_atau_nilai_wajar,
	       akumulasi_penyusutan_atau_amortisasi, metode_pengukuran_code, as_of,
	       status, note, created_at, updated_at`

const listPropertiItemsQuery = `
	SELECT ` + propertiRegisterColumns + `
	FROM properti_terbengkalai_register
	ORDER BY as_of DESC, no_register`

// ListItems membaca seluruh register properti bank-wide untuk UI edit.
func (r *PropertiRegisterRepository) ListItems(ctx context.Context) ([]domain.PropertiItem, error) {
	return r.queryProperti(ctx, listPropertiItemsQuery)
}

const listPropertiForOJKQuery = `
	SELECT ` + propertiRegisterColumns + `
	FROM properti_terbengkalai_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY no_register`

// ListPropertiForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 17.00. Satu query, urutan deterministik.
func (r *PropertiRegisterRepository) ListPropertiForOJK(ctx context.Context, asOf time.Time) ([]domain.PropertiItem, error) {
	return r.queryProperti(ctx, listPropertiForOJKQuery, asOf)
}

// queryProperti menjalankan query pembacaan dan memetakan baris register.
func (r *PropertiRegisterRepository) queryProperti(ctx context.Context, query string, args ...any) ([]domain.PropertiItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.PropertiItem
	for rows.Next() {
		var item domain.PropertiItem
		if err := rows.Scan(
			&item.ID, &item.NoRegister, &item.JenisPropertiCode, &item.AlamatProperti,
			&item.Koordinat, &item.TanggalPenetapan, &item.BiayaPerolehanNilaiWajar,
			&item.AkumulasiPenyusutanAmortisasi, &item.MetodePengukuranCode, &item.AsOf,
			&item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt,
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

// UpsertItemTx menyimpan satu properti (idempotensi lewat PRIMARY KEY id). No. Register
// yang sudah dipakai baris lain — termasuk baris NONAKTIF karena aturan no reuse/no
// recycle Form 17.00 — ditolak basis data oleh UNIQUE penuh; pelanggaran itu dipetakan
// ke ErrPropertiNoRegisterUsed sehingga API menjelaskan sebabnya, bukan 500.
func (r *PropertiRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.PropertiItem, actorID uuid.UUID) error {
	sqlTx, err := requirePropertiTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO properti_terbengkalai_register (
			id, no_register, jenis_properti_code, alamat_properti, koordinat,
			tanggal_penetapan, biaya_perolehan_atau_nilai_wajar,
			akumulasi_penyusutan_atau_amortisasi, metode_pengukuran_code, as_of,
			status, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW(), $13, $13
		)
		ON CONFLICT (id) DO UPDATE SET
			no_register = EXCLUDED.no_register,
			jenis_properti_code = EXCLUDED.jenis_properti_code,
			alamat_properti = EXCLUDED.alamat_properti,
			koordinat = EXCLUDED.koordinat,
			tanggal_penetapan = EXCLUDED.tanggal_penetapan,
			biaya_perolehan_atau_nilai_wajar = EXCLUDED.biaya_perolehan_atau_nilai_wajar,
			akumulasi_penyusutan_atau_amortisasi = EXCLUDED.akumulasi_penyusutan_atau_amortisasi,
			metode_pengukuran_code = EXCLUDED.metode_pengukuran_code,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.NoRegister, item.JenisPropertiCode, item.AlamatProperti,
		item.Koordinat, item.TanggalPenetapan, item.BiayaPerolehanNilaiWajar,
		item.AkumulasiPenyusutanAmortisasi, item.MetodePengukuranCode, item.AsOf,
		item.Status, item.Note, nullablePropertiActor(actorID))
	if isUniqueViolation(err) {
		// Satu-satunya unique di tabel selain PK id (yang ditangani ON CONFLICT) adalah
		// properti_terbengkalai_no_register_unique.
		return fmt.Errorf("%w: %s", domain.ErrPropertiNoRegisterUsed, item.NoRegister)
	}
	return err
}

// SoftDeleteItemTx menonaktifkan satu properti tanpa menghapus barisnya, sehingga
// nomor register tetap terpakai dan tidak dapat dipakai ulang (no reuse/no recycle).
// found=false bila barisnya tidak ada.
func (r *PropertiRegisterRepository) SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error) {
	sqlTx, err := requirePropertiTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE properti_terbengkalai_register
		SET status = 'NONAKTIF', updated_at = NOW(), updated_by = $2
		WHERE id = $1`, id, nullablePropertiActor(actorID))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requirePropertiTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requirePropertiTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register properti terbengkalai: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullablePropertiActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK
// ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullablePropertiActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.PropertiRegisterRepository = (*PropertiRegisterRepository)(nil)
