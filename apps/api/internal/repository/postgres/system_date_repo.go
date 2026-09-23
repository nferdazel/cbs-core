package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type BusinessDateRepository struct {
	db *sql.DB
}

func NewBusinessDateRepository(db *sql.DB) *BusinessDateRepository {
	return &BusinessDateRepository{db: db}
}

// wibZone adalah zona waktu bank (Asia/Jakarta, UTC+7) tanpa ketergantungan tzdata
// sistem. Satu sumber kebenaran ada di domain.BankZone (dipakai juga oleh evaluator
// cron EOD) supaya tanggal bisnis dan jadwal terjadwal tidak dapat berbeda zona.
var wibZone = domain.BankZone

func (r *BusinessDateRepository) GetCurrentDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	var dateStr, statusStr string
	var updatedByStr sql.NullString

	err := r.db.QueryRowContext(ctx,
		"SELECT value, description FROM system_config WHERE key = 'system.business_date'").Scan(&dateStr, &updatedByStr)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Tanggal bisnis belum pernah disetel (pemasangan baru): hari ini menurut zona
		// waktu bank (WIB) dipakai sebagai titik awal, bukan tanggal kalender UTC yang
		// bisa berbeda sehari di pagi/malam WIB.
		dateStr = time.Now().In(wibZone).Format("2006-01-02")
	case err != nil:
		// Kegagalan membaca tidak boleh berbuntut tanggal karangan: seluruh proses
		// tutup buku memakai tanggal ini sebagai periode posting.
		return nil, fmt.Errorf("membaca tanggal bisnis: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		"SELECT value FROM system_config WHERE key = 'system.business_date_status'").Scan(&statusStr)
	switch {
	case errors.Is(err, sql.ErrNoRows), statusStr == "":
		statusStr = string(domain.BusinessDateStatusOpen)
	case err != nil:
		return nil, fmt.Errorf("membaca status tanggal bisnis: %w", err)
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		// Tanggal tersimpan yang rusak harus terlihat, bukan diganti tanggal hari ini:
		// posting ke periode yang salah lebih sulit diperbaiki daripada gagal jelas.
		return nil, fmt.Errorf("tanggal bisnis tersimpan tidak valid (%q): %w", dateStr, err)
	}

	var updatedBy *uuid.UUID
	if updatedByStr.Valid {
		id, err := uuid.Parse(updatedByStr.String)
		if err == nil {
			updatedBy = &id
		}
	}

	return &domain.SystemBusinessDate{
		CurrentDate: parsedDate,
		Status:      domain.BusinessDateStatus(statusStr),
		UpdatedBy:   updatedBy,
		UpdatedAt:   time.Now().UTC(),
	}, nil
}

func (r *BusinessDateRepository) AdvanceDate(ctx context.Context, nextDate time.Time, updatedBy uuid.UUID) error {
	dateStr := nextDate.Format("2006-01-02")

	// Kedua penulisan wajib atomik. Bila hanya salah satu yang tersimpan (mis. proses
	// berhenti di antaranya), bank terjebak: tanggal tidak maju sementara statusnya
	// menandakan tutup hari, dan operasional berhenti sampai diperbaiki manual.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	q1 := `INSERT INTO system_config (key, value, description, updated_by, updated_at)
		VALUES ('system.business_date', $1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, description = $2, updated_by = $3, updated_at = NOW()`
	if _, err := tx.ExecContext(ctx, q1, dateStr, updatedBy.String(), updatedBy); err != nil {
		return err
	}

	q2 := `INSERT INTO system_config (key, value, description, updated_by, updated_at)
		VALUES ('system.business_date_status', 'OPEN', 'Operational status', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = 'OPEN', updated_by = $1, updated_at = NOW()`
	if _, err := tx.ExecContext(ctx, q2, updatedBy); err != nil {
		return err
	}

	return tx.Commit()
}

// ClaimEOD mengklaim tanggal bisnis untuk tutup hari secara atomik: status hanya
// berpindah ke EOD bila status saat ini bukan CLOSED, dan hasilnya dilaporkan lewat
// jumlah baris yang berubah. Dengan begitu keputusan "boleh lanjut" dan perubahan
// statusnya tidak dapat disela proses lain.
func (r *BusinessDateRepository) ClaimEOD(ctx context.Context) (bool, error) {
	q := `INSERT INTO system_config (key, value, description, updated_at)
		VALUES ('system.business_date_status', $1, 'Operational status', NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
		WHERE system_config.value <> $2`
	res, err := r.db.ExecContext(ctx, q, string(domain.BusinessDateStatusEOD), string(domain.BusinessDateStatusClosed))
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// eodAdvisoryLockKey adalah kunci advisory PostgreSQL yang dipakai khusus tutup
// hari. Nilainya cukup unik di dalam satu database.
const eodAdvisoryLockKey int64 = 4217001

// TryEODLock mengambil kunci advisory pada satu sesi database yang dipegang selama
// tutup hari berlangsung. Kunci advisory bersifat per sesi, jadi koneksinya harus
// dipegang utuh — bukan dipinjam-lepas dari pool setiap query.
func (r *BusinessDateRepository) TryEODLock(ctx context.Context) (func() error, error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, err
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", eodAdvisoryLockKey).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !acquired {
		_ = conn.Close()
		return nil, domain.ErrEODInProgress
	}

	return func() error {
		// Pelepasan memakai konteks baru: konteks permintaan bisa sudah dibatalkan saat
		// tutup hari selesai, dan kunci tidak boleh tertinggal sampai koneksi ditutup.
		defer func() { _ = conn.Close() }()
		_, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", eodAdvisoryLockKey)
		return err
	}, nil
}

// HasOperationalActivity melaporkan apakah instalasi sudah pernah beroperasi: ada
// jurnal tersimpan, atau tanggal bisnis sudah pernah disetel/dimajukan. Pemasangan
// baru belum punya keduanya, sehingga peringatan kesiapan CKPN tidak menyala sebelum
// bank benar-benar beroperasi.
func (r *BusinessDateRepository) HasOperationalActivity(ctx context.Context) (bool, error) {
	var hasJournal bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM journal_entries)`).Scan(&hasJournal); err != nil {
		return false, fmt.Errorf("memeriksa jurnal tersimpan: %w", err)
	}
	if hasJournal {
		return true, nil
	}
	var hasBusinessDate bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM system_config WHERE key = 'system.business_date')`).Scan(&hasBusinessDate); err != nil {
		return false, fmt.Errorf("memeriksa tanggal bisnis tersimpan: %w", err)
	}
	return hasBusinessDate, nil
}

var _ domain.BusinessDateRepository = (*BusinessDateRepository)(nil)
var _ domain.OperationalActivityReader = (*BusinessDateRepository)(nil)

// CurrentBusinessDate mengembalikan tanggal bisnis berjalan sebagai time.Time untuk
// pemakai yang tidak perlu status tanggal (mis. penjaga batas harian).
func (r *BusinessDateRepository) CurrentBusinessDate(ctx context.Context) (time.Time, error) {
	current, err := r.GetCurrentDate(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return current.CurrentDate, nil
}

var _ domain.BusinessDateProvider = (*BusinessDateRepository)(nil)
