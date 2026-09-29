package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// SuratBerhargaRegisterRepository menyimpan register surat berharga (migrasi 000122):
// rincian per surat berharga untuk Form 04.00. SQL tetap di repository, bukan di service.
type SuratBerhargaRegisterRepository struct {
	db *sql.DB
}

func NewSuratBerhargaRegisterRepository(db *sql.DB) *SuratBerhargaRegisterRepository {
	return &SuratBerhargaRegisterRepository{db: db}
}

// suratBerhargaRegisterColumns tidak memuat kolom turunan: Form 04.00 tidak punya kolom
// yang dihitung laporan, seluruh nilai adalah isian bank.
const suratBerhargaRegisterColumns = `id, klasifikasi_code, suku_bunga, tanggal_mulai,
	       tanggal_jatuh_tempo, nominal, nominal_dijaminkan, biaya_perolehan,
	       diskonto_premium_belum_diamortisasi, biaya_transaksi_belum_diamortisasi,
	       laba_rugi_belum_direalisasi, biaya_perolehan_diamortisasi, nomor_surat_berharga,
	       counterparty_id, jenis_code, kualitas_code, ckpn, lembaga_pemeringkat_code,
	       peringkat_surat_berharga_code, tanggal_pemeringkatan, tanggal_penerbitan,
	       ckpn_aset_baik, ckpn_aset_kurang_baik, ckpn_aset_tidak_baik,
	       klasifikasi_aset_keuangan_code, jenis_ckpn_code, as_of, status, note,
	       created_at, updated_at`

const listSuratBerhargaItemsQuery = `
	SELECT ` + suratBerhargaRegisterColumns + `
	FROM surat_berharga_register
	ORDER BY as_of DESC, tanggal_penerbitan DESC, id`

// ListItems membaca seluruh register surat berharga bank-wide untuk UI edit.
func (r *SuratBerhargaRegisterRepository) ListItems(ctx context.Context) ([]domain.SuratBerhargaItem, error) {
	return r.querySuratBerharga(ctx, listSuratBerhargaItemsQuery)
}

const listSuratBerhargaForOJKQuery = `
	SELECT ` + suratBerhargaRegisterColumns + `
	FROM surat_berharga_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY tanggal_penerbitan, id`

// ListSuratBerhargaForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf:
// sumber Form 04.00. Satu query, urutan deterministik.
func (r *SuratBerhargaRegisterRepository) ListSuratBerhargaForOJK(ctx context.Context, asOf time.Time) ([]domain.SuratBerhargaItem, error) {
	return r.querySuratBerharga(ctx, listSuratBerhargaForOJKQuery, asOf)
}

// querySuratBerharga menjalankan query pembacaan dan memetakan baris register.
func (r *SuratBerhargaRegisterRepository) querySuratBerharga(ctx context.Context, query string, args ...any) ([]domain.SuratBerhargaItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.SuratBerhargaItem
	for rows.Next() {
		var item domain.SuratBerhargaItem
		if err := rows.Scan(
			&item.ID, &item.KlasifikasiCode, &item.SukuBunga, &item.TanggalMulai,
			&item.TanggalJatuhTempo, &item.Nominal, &item.NominalDijaminkan,
			&item.BiayaPerolehan, &item.DiskontoPremiumBelumDiamortisasi,
			&item.BiayaTransaksiBelumDiamortisasi, &item.LabaRugiBelumDirealisasi,
			&item.BiayaPerolehanDiamortisasi, &item.NomorSuratBerharga,
			&item.CounterpartyID, &item.JenisCode, &item.KualitasCode, &item.CKPN,
			&item.LembagaPemeringkatCode, &item.PeringkatSuratBerhargaCode,
			&item.TanggalPemeringkatan, &item.TanggalPenerbitan,
			&item.CKPNAsetBaik, &item.CKPNAsetKurangBaik, &item.CKPNAsetTidakBaik,
			&item.KlasifikasiAsetKeuanganCode, &item.JenisCKPNCode, &item.AsOf,
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

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id). Form 04.00 tidak
// menetapkan nomor register unik, sehingga tidak ada pemetaan pelanggaran keunikan.
func (r *SuratBerhargaRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.SuratBerhargaItem, actorID uuid.UUID) error {
	sqlTx, err := requireSuratBerhargaTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO surat_berharga_register (
			id, klasifikasi_code, suku_bunga, tanggal_mulai, tanggal_jatuh_tempo,
			nominal, nominal_dijaminkan, biaya_perolehan,
			diskonto_premium_belum_diamortisasi, biaya_transaksi_belum_diamortisasi,
			laba_rugi_belum_direalisasi, biaya_perolehan_diamortisasi,
			nomor_surat_berharga, counterparty_id, jenis_code, kualitas_code, ckpn,
			lembaga_pemeringkat_code, peringkat_surat_berharga_code,
			tanggal_pemeringkatan, tanggal_penerbitan, ckpn_aset_baik,
			ckpn_aset_kurang_baik, ckpn_aset_tidak_baik,
			klasifikasi_aset_keuangan_code, jenis_ckpn_code, as_of, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29,
			NOW(), NOW(), $30, $30
		)
		ON CONFLICT (id) DO UPDATE SET
			klasifikasi_code = EXCLUDED.klasifikasi_code,
			suku_bunga = EXCLUDED.suku_bunga,
			tanggal_mulai = EXCLUDED.tanggal_mulai,
			tanggal_jatuh_tempo = EXCLUDED.tanggal_jatuh_tempo,
			nominal = EXCLUDED.nominal,
			nominal_dijaminkan = EXCLUDED.nominal_dijaminkan,
			biaya_perolehan = EXCLUDED.biaya_perolehan,
			diskonto_premium_belum_diamortisasi = EXCLUDED.diskonto_premium_belum_diamortisasi,
			biaya_transaksi_belum_diamortisasi = EXCLUDED.biaya_transaksi_belum_diamortisasi,
			laba_rugi_belum_direalisasi = EXCLUDED.laba_rugi_belum_direalisasi,
			biaya_perolehan_diamortisasi = EXCLUDED.biaya_perolehan_diamortisasi,
			nomor_surat_berharga = EXCLUDED.nomor_surat_berharga,
			counterparty_id = EXCLUDED.counterparty_id,
			jenis_code = EXCLUDED.jenis_code,
			kualitas_code = EXCLUDED.kualitas_code,
			ckpn = EXCLUDED.ckpn,
			lembaga_pemeringkat_code = EXCLUDED.lembaga_pemeringkat_code,
			peringkat_surat_berharga_code = EXCLUDED.peringkat_surat_berharga_code,
			tanggal_pemeringkatan = EXCLUDED.tanggal_pemeringkatan,
			tanggal_penerbitan = EXCLUDED.tanggal_penerbitan,
			ckpn_aset_baik = EXCLUDED.ckpn_aset_baik,
			ckpn_aset_kurang_baik = EXCLUDED.ckpn_aset_kurang_baik,
			ckpn_aset_tidak_baik = EXCLUDED.ckpn_aset_tidak_baik,
			klasifikasi_aset_keuangan_code = EXCLUDED.klasifikasi_aset_keuangan_code,
			jenis_ckpn_code = EXCLUDED.jenis_ckpn_code,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.KlasifikasiCode, item.SukuBunga, item.TanggalMulai,
		item.TanggalJatuhTempo, item.Nominal, item.NominalDijaminkan,
		item.BiayaPerolehan, item.DiskontoPremiumBelumDiamortisasi,
		item.BiayaTransaksiBelumDiamortisasi, item.LabaRugiBelumDirealisasi,
		item.BiayaPerolehanDiamortisasi, item.NomorSuratBerharga,
		item.CounterpartyID, item.JenisCode, item.KualitasCode, item.CKPN,
		item.LembagaPemeringkatCode, item.PeringkatSuratBerhargaCode,
		item.TanggalPemeringkatan, item.TanggalPenerbitan, item.CKPNAsetBaik,
		item.CKPNAsetKurangBaik, item.CKPNAsetTidakBaik,
		item.KlasifikasiAsetKeuanganCode, item.JenisCKPNCode, item.AsOf,
		item.Status, item.Note, nullableSuratBerhargaActor(actorID))
	return err
}

// DeleteItemTx menghapus satu baris surat berharga secara fisik. Form 04.00 tidak
// menetapkan nomor register unik (tidak ada aturan no reuse/no recycle), sehingga DELETE
// fisik dibenarkan. found=false bila barisnya tidak ada.
func (r *SuratBerhargaRegisterRepository) DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireSuratBerhargaTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM surat_berharga_register WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireSuratBerhargaTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requireSuratBerhargaTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register surat berharga: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableSuratBerhargaActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK
// ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableSuratBerhargaActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.SuratBerhargaRegisterRepository = (*SuratBerhargaRegisterRepository)(nil)
