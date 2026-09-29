package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// AsetKeuanganRegisterRepository menyimpan register aset keuangan lainnya (migrasi
// 000121): rincian per rekening untuk Form 18.00. SQL tetap di repository, bukan di
// service.
type AsetKeuanganRegisterRepository struct {
	db *sql.DB
}

func NewAsetKeuanganRegisterRepository(db *sql.DB) *AsetKeuanganRegisterRepository {
	return &AsetKeuanganRegisterRepository{db: db}
}

// asetKeuanganRegisterColumns tidak memuat kolom turunan: Form 18.00 tidak punya kolom
// yang dihitung laporan, seluruh nilai adalah isian bank.
const asetKeuanganRegisterColumns = `id, no_rekening, counterparty_id, jenis_code,
	       tanggal_mulai, tanggal_jatuh_tempo, suku_bunga, nominal,
	       nilai_agunan_diperhitungkan, ckpn, ckpn_aset_baik, ckpn_aset_kurang_baik,
	       ckpn_aset_tidak_baik, klasifikasi_aset_keuangan_code, jenis_ckpn_code,
	       as_of, status, note, created_at, updated_at`

const listAsetKeuanganItemsQuery = `
	SELECT ` + asetKeuanganRegisterColumns + `
	FROM aset_keuangan_lainnya_register
	ORDER BY as_of DESC, no_rekening`

// ListItems membaca seluruh register aset keuangan lainnya bank-wide untuk UI edit.
func (r *AsetKeuanganRegisterRepository) ListItems(ctx context.Context) ([]domain.AsetKeuanganItem, error) {
	return r.queryAsetKeuangan(ctx, listAsetKeuanganItemsQuery)
}

const listAsetKeuanganForOJKQuery = `
	SELECT ` + asetKeuanganRegisterColumns + `
	FROM aset_keuangan_lainnya_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY no_rekening`

// ListAsetKeuanganForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf:
// sumber Form 18.00. Satu query, urutan deterministik.
func (r *AsetKeuanganRegisterRepository) ListAsetKeuanganForOJK(ctx context.Context, asOf time.Time) ([]domain.AsetKeuanganItem, error) {
	return r.queryAsetKeuangan(ctx, listAsetKeuanganForOJKQuery, asOf)
}

// queryAsetKeuangan menjalankan query pembacaan dan memetakan baris register.
func (r *AsetKeuanganRegisterRepository) queryAsetKeuangan(ctx context.Context, query string, args ...any) ([]domain.AsetKeuanganItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.AsetKeuanganItem
	for rows.Next() {
		var item domain.AsetKeuanganItem
		if err := rows.Scan(
			&item.ID, &item.NoRekening, &item.CounterpartyID, &item.JenisCode,
			&item.TanggalMulai, &item.TanggalJatuhTempo, &item.SukuBunga, &item.Nominal,
			&item.NilaiAgunanDiperhitungkan, &item.CKPN, &item.CKPNAsetBaik,
			&item.CKPNAsetKurangBaik, &item.CKPNAsetTidakBaik,
			&item.KlasifikasiAsetKeuanganCode, &item.JenisCKPNCode,
			&item.AsOf, &item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt,
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

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id). No. Rekening yang
// sudah dipakai baris lain — termasuk baris NONAKTIF karena aturan "nomor rekening tidak
// boleh sama" Form 18.00 — ditolak basis data oleh UNIQUE penuh; pelanggaran itu dipetakan
// ke ErrAsetKeuanganNoRekeningUsed sehingga API menjelaskan sebabnya, bukan 500.
func (r *AsetKeuanganRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.AsetKeuanganItem, actorID uuid.UUID) error {
	sqlTx, err := requireAsetKeuanganTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO aset_keuangan_lainnya_register (
			id, no_rekening, counterparty_id, jenis_code, tanggal_mulai,
			tanggal_jatuh_tempo, suku_bunga, nominal, nilai_agunan_diperhitungkan,
			ckpn, ckpn_aset_baik, ckpn_aset_kurang_baik, ckpn_aset_tidak_baik,
			klasifikasi_aset_keuangan_code, jenis_ckpn_code, as_of, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
			NOW(), NOW(), $19, $19
		)
		ON CONFLICT (id) DO UPDATE SET
			no_rekening = EXCLUDED.no_rekening,
			counterparty_id = EXCLUDED.counterparty_id,
			jenis_code = EXCLUDED.jenis_code,
			tanggal_mulai = EXCLUDED.tanggal_mulai,
			tanggal_jatuh_tempo = EXCLUDED.tanggal_jatuh_tempo,
			suku_bunga = EXCLUDED.suku_bunga,
			nominal = EXCLUDED.nominal,
			nilai_agunan_diperhitungkan = EXCLUDED.nilai_agunan_diperhitungkan,
			ckpn = EXCLUDED.ckpn,
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
		item.ID, item.NoRekening, item.CounterpartyID, item.JenisCode, item.TanggalMulai,
		item.TanggalJatuhTempo, item.SukuBunga, item.Nominal, item.NilaiAgunanDiperhitungkan,
		item.CKPN, item.CKPNAsetBaik, item.CKPNAsetKurangBaik, item.CKPNAsetTidakBaik,
		item.KlasifikasiAsetKeuanganCode, item.JenisCKPNCode, item.AsOf, item.Status,
		item.Note, nullableAsetKeuanganActor(actorID))
	if isUniqueViolation(err) {
		// Satu-satunya unique di tabel selain PK id (yang ditangani ON CONFLICT) adalah
		// aset_keuangan_lainnya_no_rekening_unique.
		return fmt.Errorf("%w: %s", domain.ErrAsetKeuanganNoRekeningUsed, item.NoRekening)
	}
	return err
}

// SoftDeleteItemTx menonaktifkan satu baris tanpa menghapusnya, sehingga nomor rekening
// tetap terpakai dan tidak dapat dipakai ulang. found=false bila barisnya tidak ada.
func (r *AsetKeuanganRegisterRepository) SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error) {
	sqlTx, err := requireAsetKeuanganTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE aset_keuangan_lainnya_register
		SET status = 'NONAKTIF', updated_at = NOW(), updated_by = $2
		WHERE id = $1`, id, nullableAsetKeuanganActor(actorID))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireAsetKeuanganTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requireAsetKeuanganTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register aset keuangan lainnya: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableAsetKeuanganActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya FK
// ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableAsetKeuanganActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.AsetKeuanganRegisterRepository = (*AsetKeuanganRegisterRepository)(nil)
