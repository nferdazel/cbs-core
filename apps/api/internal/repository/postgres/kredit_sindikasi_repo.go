package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// KreditSindikasiRegisterRepository menyimpan register kredit sindikasi (migrasi 000124):
// rincian per rekening fasilitas kredit sindikasi untuk Form 06.02. SQL tetap di
// repository, bukan di service.
type KreditSindikasiRegisterRepository struct {
	db *sql.DB
}

func NewKreditSindikasiRegisterRepository(db *sql.DB) *KreditSindikasiRegisterRepository {
	return &KreditSindikasiRegisterRepository{db: db}
}

// kreditSindikasiRegisterColumns tidak memuat kolom III No. Identitas: kolom itu sengaja
// tidak disimpan (keputusan privasi) dan laporan menulis "-" beserta alasannya.
const kreditSindikasiRegisterColumns = `id, counterparty_id, no_rekening,
	       jumlah_pendanaan_sindikasi, bagian_pendanaan, sandi_bank_peserta,
	       plafon, baki_debet, status_kepesertaan_code, nomor_perjanjian_induk,
	       pendanaan_di_bank_pelapor_code, kualitas_code,
	       tunggakan_pokok, tunggakan_bunga, hari_tunggakan_pokok, hari_tunggakan_bunga,
	       as_of, status, note, created_at, updated_at`

const listKreditSindikasiItemsQuery = `
	SELECT ` + kreditSindikasiRegisterColumns + `
	FROM kredit_sindikasi_register
	ORDER BY as_of DESC, nomor_perjanjian_induk, no_rekening NULLS LAST, id`

// ListItems membaca seluruh register kredit sindikasi bank-wide untuk UI edit.
func (r *KreditSindikasiRegisterRepository) ListItems(ctx context.Context) ([]domain.SindikasiItem, error) {
	return r.queryKreditSindikasi(ctx, listKreditSindikasiItemsQuery)
}

const listKreditSindikasiForOJKQuery = `
	SELECT ` + kreditSindikasiRegisterColumns + `
	FROM kredit_sindikasi_register
	WHERE status = 'AKTIF'
	  AND date_trunc('month', as_of) = date_trunc('month', $1::date)
	ORDER BY status_kepesertaan_code, nomor_perjanjian_induk, no_rekening NULLS LAST, id`

// ListSindikasiForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf: sumber
// Form 06.02. Satu query, urutan deterministik.
func (r *KreditSindikasiRegisterRepository) ListSindikasiForOJK(ctx context.Context, asOf time.Time) ([]domain.SindikasiItem, error) {
	return r.queryKreditSindikasi(ctx, listKreditSindikasiForOJKQuery, asOf)
}

// queryKreditSindikasi menjalankan query pembacaan dan memetakan baris register.
func (r *KreditSindikasiRegisterRepository) queryKreditSindikasi(ctx context.Context, query string, args ...any) ([]domain.SindikasiItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.SindikasiItem
	for rows.Next() {
		item, err := scanKreditSindikasiRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scanKreditSindikasiRow memetakan satu baris. no_rekening NULL dipetakan ke string kosong
// karena domain memakai string kosong untuk "tidak ada rekening".
func scanKreditSindikasiRow(rows *sql.Rows) (domain.SindikasiItem, error) {
	var (
		item       domain.SindikasiItem
		noRekening sql.NullString
	)
	if err := rows.Scan(
		&item.ID, &item.CounterpartyID, &noRekening,
		&item.JumlahPendanaanSindikasi, &item.BagianPendanaan, &item.SandiBankPeserta,
		&item.Plafon, &item.BakiDebet, &item.StatusKepesertaanCode, &item.NomorPerjanjianInduk,
		&item.PendanaanDiBankPelaporCode, &item.KualitasCode,
		&item.TunggakanPokok, &item.TunggakanBunga, &item.HariTunggakanPokok, &item.HariTunggakanBunga,
		&item.AsOf, &item.Status, &item.Note, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return domain.SindikasiItem{}, err
	}
	if noRekening.Valid {
		item.NoRekening = noRekening.String
	}
	return item, nil
}

// UpsertItemTx menyimpan satu baris (idempotensi lewat PRIMARY KEY id). Pelanggaran UNIQUE
// no_rekening dipetakan ke ErrSindikasiNoRekeningUsed supaya pemanggil dapat membedakannya
// dari galat basis data lain. Nomor rekening unik "tidak boleh sama" (Form 06.02,
// PDF #page 177), termasuk terhadap baris NONAKTIF.
func (r *KreditSindikasiRegisterRepository) UpsertItemTx(ctx context.Context, tx any, item domain.SindikasiItem, actorID uuid.UUID) error {
	sqlTx, err := requireKreditSindikasiTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO kredit_sindikasi_register (
			id, counterparty_id, no_rekening,
			jumlah_pendanaan_sindikasi, bagian_pendanaan, sandi_bank_peserta,
			plafon, baki_debet, status_kepesertaan_code, nomor_perjanjian_induk,
			pendanaan_di_bank_pelapor_code, kualitas_code,
			tunggakan_pokok, tunggakan_bunga, hari_tunggakan_pokok, hari_tunggakan_bunga,
			as_of, status, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, NULLIF($3, ''),
			$4, $5, $6,
			$7, $8, $9, $10,
			$11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, NOW(), NOW(), $20, $20
		)
		ON CONFLICT (id) DO UPDATE SET
			counterparty_id = EXCLUDED.counterparty_id,
			no_rekening = EXCLUDED.no_rekening,
			jumlah_pendanaan_sindikasi = EXCLUDED.jumlah_pendanaan_sindikasi,
			bagian_pendanaan = EXCLUDED.bagian_pendanaan,
			sandi_bank_peserta = EXCLUDED.sandi_bank_peserta,
			plafon = EXCLUDED.plafon,
			baki_debet = EXCLUDED.baki_debet,
			status_kepesertaan_code = EXCLUDED.status_kepesertaan_code,
			nomor_perjanjian_induk = EXCLUDED.nomor_perjanjian_induk,
			pendanaan_di_bank_pelapor_code = EXCLUDED.pendanaan_di_bank_pelapor_code,
			kualitas_code = EXCLUDED.kualitas_code,
			tunggakan_pokok = EXCLUDED.tunggakan_pokok,
			tunggakan_bunga = EXCLUDED.tunggakan_bunga,
			hari_tunggakan_pokok = EXCLUDED.hari_tunggakan_pokok,
			hari_tunggakan_bunga = EXCLUDED.hari_tunggakan_bunga,
			as_of = EXCLUDED.as_of,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		item.ID, item.CounterpartyID, item.NoRekening,
		item.JumlahPendanaanSindikasi, item.BagianPendanaan, item.SandiBankPeserta,
		item.Plafon, item.BakiDebet, item.StatusKepesertaanCode, item.NomorPerjanjianInduk,
		item.PendanaanDiBankPelaporCode, item.KualitasCode,
		item.TunggakanPokok, item.TunggakanBunga, item.HariTunggakanPokok, item.HariTunggakanBunga,
		item.AsOf, item.Status, item.Note, nullableKreditSindikasiActor(actorID))
	if err != nil {
		if isKreditSindikasiNoRekeningViolation(err) {
			return domain.ErrSindikasiNoRekeningUsed
		}
		return err
	}
	return nil
}

// SoftDeleteItemTx menonaktifkan satu baris (status -> NONAKTIF). Form 06.02 menetapkan
// nomor rekening unik dan tidak boleh sama, sehingga baris yang tidak lagi dilaporkan tetap
// memegang nomornya; DELETE fisik akan membuka peluang pemakaian ulang. found=false bila
// barisnya tidak ada.
func (r *KreditSindikasiRegisterRepository) SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error) {
	sqlTx, err := requireKreditSindikasiTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE kredit_sindikasi_register
		SET status = 'NONAKTIF', updated_at = NOW(), updated_by = $2
		WHERE id = $1`, id, nullableKreditSindikasiActor(actorID))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireKreditSindikasiTx memastikan transaksi tulis sah, konsisten dengan pola repository
// OJK lain.
func requireKreditSindikasiTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("register kredit sindikasi: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// isKreditSindikasiNoRekeningViolation mengenali pelanggaran UNIQUE no_rekening
// (SQLSTATE 23505) dengan menyaring nama constraint agar galat unique lain tidak salah
// dipetakan.
func isKreditSindikasiNoRekeningViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "kredit_sindikasi_no_rekening_unique"
}

// nullableKreditSindikasiActor menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya
// FK ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableKreditSindikasiActor(actorID uuid.UUID) any {
	if actorID == uuid.Nil {
		return nil
	}
	return actorID
}

var _ domain.SindikasiRegisterRepository = (*KreditSindikasiRegisterRepository)(nil)
