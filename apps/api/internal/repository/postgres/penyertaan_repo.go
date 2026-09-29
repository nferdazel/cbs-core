package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// PenyertaanRegisterRepository menyimpan register penyertaan modal (migrasi 000120):
// rincian per penyertaan untuk Form 16.00. SQL tetap di repository, bukan di service.
type PenyertaanRegisterRepository struct {
	db *sql.DB
}

func NewPenyertaanRegisterRepository(db *sql.DB) *PenyertaanRegisterRepository {
	return &PenyertaanRegisterRepository{db: db}
}

// penyertaanRegisterColumns tidak memuat kolom turunan: Form 16.00 tidak punya kolom
// yang dihitung laporan, seluruh nilai adalah isian bank.
const penyertaanRegisterColumns = `id, no_register, counterparty_id, metode_penyertaan_code,
	       kualitas_code, tujuan_penyertaan_code, tanggal_mulai, persentase_penyertaan,
	       nominal, jumlah_bulan_laporan, ckpn, ckpn_aset_baik, ckpn_aset_kurang_baik,
	       ckpn_aset_tidak_baik, jenis_ckpn_code, as_of, status, note, created_at, updated_at`

const listPenyertaanItemsQuery = `
	SELECT ` + penyertaanRegisterColumns + `
	FROM penyertaan_modal_register
	ORDER BY as_of DESC, no_register`

// ListItems membaca seluruh register penyertaan modal bank-wide untuk UI edit.
func (r *PenyertaanRegisterRepository) ListItems(ctx context.Context) ([]domain.PenyertaanItem, error) {
	return r.queryPenyertaan(ctx, listPenyertaanItemsQuery)
}

const listPenyertaanForOJKQuery = `
	SELECT ` + penyertaanRegisterColumns + `
	FROM penyertaan_modal_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY no_register`

// ListPenyertaanForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 16.00. Satu query, urutan deterministik.
func (r *PenyertaanRegisterRepository) ListPenyertaanForOJK(ctx context.Context, asOf time.Time) ([]domain.PenyertaanItem, error) {
	return r.queryPenyertaan(ctx, listPenyertaanForOJKQuery, asOf)
}

// queryPenyertaan menjalankan query pembacaan dan memetakan baris register.
func (r *PenyertaanRegisterRepository) queryPenyertaan(ctx context.Context, query string, args ...any) ([]domain.PenyertaanItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.PenyertaanItem
	for rows.Next() {
		var item domain.PenyertaanItem
		if err := rows.Scan(
			&item.ID, &item.NoRegister, &item.CounterpartyID, &item.MetodePenyertaanCode,
			&item.KualitasCode, &item.TujuanPenyertaanCode, &item.TanggalMulai,
			&item.PersentasePenyertaan, &item.Nominal, &item.JumlahBulanLaporan, &item.CKPN,
			&item.CKPNAsetBaik, &item.CKPNAsetKurangBaik, &item.CKPNAsetTidakBaik,
			&item.JenisCKPNCode, &item.AsOf, &item.Status, &item.Note,
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

// UpsertItemTx menyimpan satu penyertaan (idempotensi lewat PRIMARY KEY id). No. Register
// yang sudah dipakai baris lain — termasuk baris NONAKTIF karena aturan no reuse/no
// recycle Form 16.00 — ditolak basis data oleh UNIQUE penuh; pelanggaran itu dipetakan
// ke ErrPenyertaanNoRegisterUsed sehingga API menjelaskan sebabnya, bukan 500.
func (r *PenyertaanRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.PenyertaanItem, actorID uuid.UUID) error {
	sqlTx, err := requirePenyertaanTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO penyertaan_modal_register (
			id, no_register, counterparty_id, metode_penyertaan_code, kualitas_code,
			tujuan_penyertaan_code, tanggal_mulai, persentase_penyertaan, nominal,
			jumlah_bulan_laporan, ckpn, ckpn_aset_baik, ckpn_aset_kurang_baik,
			ckpn_aset_tidak_baik, jenis_ckpn_code, as_of, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			NOW(), NOW(), $19, $19
		)
		ON CONFLICT (id) DO UPDATE SET
			no_register = EXCLUDED.no_register,
			counterparty_id = EXCLUDED.counterparty_id,
			metode_penyertaan_code = EXCLUDED.metode_penyertaan_code,
			kualitas_code = EXCLUDED.kualitas_code,
			tujuan_penyertaan_code = EXCLUDED.tujuan_penyertaan_code,
			tanggal_mulai = EXCLUDED.tanggal_mulai,
			persentase_penyertaan = EXCLUDED.persentase_penyertaan,
			nominal = EXCLUDED.nominal,
			jumlah_bulan_laporan = EXCLUDED.jumlah_bulan_laporan,
			ckpn = EXCLUDED.ckpn,
			ckpn_aset_baik = EXCLUDED.ckpn_aset_baik,
			ckpn_aset_kurang_baik = EXCLUDED.ckpn_aset_kurang_baik,
			ckpn_aset_tidak_baik = EXCLUDED.ckpn_aset_tidak_baik,
			jenis_ckpn_code = EXCLUDED.jenis_ckpn_code,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.NoRegister, item.CounterpartyID, item.MetodePenyertaanCode,
		item.KualitasCode, item.TujuanPenyertaanCode, item.TanggalMulai,
		item.PersentasePenyertaan, item.Nominal, item.JumlahBulanLaporan, item.CKPN,
		item.CKPNAsetBaik, item.CKPNAsetKurangBaik, item.CKPNAsetTidakBaik,
		item.JenisCKPNCode, item.AsOf, item.Status, item.Note, nullablePenyertaanActor(actorID))
	if isUniqueViolation(err) {
		// Satu-satunya unique di tabel selain PK id (yang ditangani ON CONFLICT) adalah
		// penyertaan_modal_no_register_unique.
		return fmt.Errorf("%w: %s", domain.ErrPenyertaanNoRegisterUsed, item.NoRegister)
	}
	return err
}

// SoftDeleteItemTx menonaktifkan satu penyertaan tanpa menghapus barisnya, sehingga
// nomor register tetap terpakai dan tidak dapat dipakai ulang (no reuse/no recycle).
// found=false bila barisnya tidak ada.
func (r *PenyertaanRegisterRepository) SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error) {
	sqlTx, err := requirePenyertaanTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE penyertaan_modal_register
		SET status = 'NONAKTIF', updated_at = NOW(), updated_by = $2
		WHERE id = $1`, id, nullablePenyertaanActor(actorID))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requirePenyertaanTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requirePenyertaanTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register penyertaan modal: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullablePenyertaanActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK
// ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullablePenyertaanActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.PenyertaanRegisterRepository = (*PenyertaanRegisterRepository)(nil)
