package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// EODStepRepository menyimpan definisi langkah tutup hari, riwayat hasil per langkah,
// dan definisi pemicu. Urutan EOD adalah konfigurasi bisnis yang dapat disesuaikan bank
// (keputusan pemilik sistem), karena itu ia hidup di database, bukan di kode.
type EODStepRepository struct {
	db *sql.DB
}

func NewEODStepRepository(db *sql.DB) *EODStepRepository {
	return &EODStepRepository{db: db}
}

var _ domain.EODStepRepository = (*EODStepRepository)(nil)

// ListDefinitions membaca seluruh definisi langkah terurut menaik. Kegagalan membaca
// dikembalikan apa adanya; pemanggil (tutup hari) MENOLAK berjalan, bukan melewati
// langkah.
func (r *EODStepRepository) ListDefinitions(ctx context.Context) ([]domain.EODStepDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT code, sequence, enabled, core, category, prerequisites, description
		FROM eod_step_definitions
		ORDER BY sequence`)
	if err != nil {
		return nil, fmt.Errorf("membaca definisi langkah EOD: %w", err)
	}
	defer rows.Close()

	var defs []domain.EODStepDefinition
	for rows.Next() {
		var def domain.EODStepDefinition
		var prereqRaw []byte
		if err := rows.Scan(&def.Code, &def.Sequence, &def.Enabled, &def.Core, &def.Category, &prereqRaw, &def.Description); err != nil {
			return nil, err
		}
		if len(prereqRaw) > 0 {
			if err := json.Unmarshal(prereqRaw, &def.Prerequisites); err != nil {
				return nil, fmt.Errorf("prasyarat langkah EOD %s tidak valid: %w", def.Code, err)
			}
		}
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

// SaveDefinitions memperbarui urutan/saklar/prasyarat untuk kode yang SUDAH ada.
// Kolom core sengaja tidak ikut ditulis: operator tidak boleh melucuti penanda inti.
// Kode yang tidak dikenal ditolak; tidak ada penambahan langkah baru lewat jalur ini
// karena hanya mesin EOD yang tahu cara menjalankan sebuah langkah.
//
// Urutan baru bisa berupa penukaran posisi. Constraint unik sequence dibuat DEFERRABLE
// dan ditunda di sini supaya pemeriksaannya terjadi saat COMMIT, bukan tiap UPDATE.
func (r *EODStepRepository) SaveDefinitions(ctx context.Context, tx any, defs []domain.EODStepDefinition, updatedBy uuid.UUID) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("eod: konteks transaksi tidak valid")
	}
	if _, err := sqlTx.ExecContext(ctx, "SET CONSTRAINTS uq_eod_step_definitions_sequence DEFERRED"); err != nil {
		return fmt.Errorf("menunda pemeriksaan urutan: %w", err)
	}

	for _, def := range defs {
		prereq, err := json.Marshal(def.Prerequisites)
		if err != nil {
			return fmt.Errorf("menyusun prasyarat %s: %w", def.Code, err)
		}
		res, err := sqlTx.ExecContext(ctx, `
			UPDATE eod_step_definitions
			   SET sequence = $2, enabled = $3, category = $4, prerequisites = $5::jsonb,
			       description = $6, updated_at = NOW(), updated_by = $7
			 WHERE code = $1`,
			def.Code, def.Sequence, def.Enabled, def.Category, prereq, def.Description, updatedBy)
		if err != nil {
			return fmt.Errorf("menyimpan langkah EOD %s: %w", def.Code, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("langkah EOD %q tidak dikenal; langkah baru tidak dapat ditambah lewat API", def.Code)
		}
	}
	return nil
}

// RecordStepResult menyimpan hasil satu langkah. Kegagalan menulis riwayat TIDAK boleh
// menggagalkan tutup hari yang sudah berjalan; pemanggil mencatatnya sebagai peringatan.
func (r *EODStepRepository) RecordStepResult(ctx context.Context, rec domain.EODStepRunRecord) error {
	summary, err := json.Marshal(rec.Summary)
	if err != nil || len(rec.Summary) == 0 {
		// Ringkasan kosong disimpan sebagai objek JSON kosong, bukan JSON null, agar
		// kolom JSONB selalu dapat dibaca sebagai objek.
		summary = []byte("{}")
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO eod_step_runs (
			run_id, business_date, step_code, sequence, status, trigger,
			reason, summary, executed_by, started_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11)`,
		rec.RunID, rec.BusinessDate, rec.StepCode, rec.Sequence, string(rec.Status), rec.Trigger,
		rec.Reason, summary, nullableUUID(rec.ExecutedBy), rec.StartedAt, rec.CompletedAt)
	if err != nil {
		return fmt.Errorf("menulis riwayat langkah EOD %s: %w", rec.StepCode, err)
	}
	return nil
}

// ListStepRuns membaca riwayat seluruh langkah pada satu tanggal bisnis, urut sesuai
// posisi langkah. Tanggal bisnis kosong berarti tanpa filter (dipakai operator untuk
// melihat run terakhir).
func (r *EODStepRepository) ListStepRuns(ctx context.Context, businessDate time.Time) ([]domain.EODStepRunRecord, error) {
	query := `
		SELECT run_id, business_date, step_code, sequence, status, trigger,
		       COALESCE(reason, ''), summary, COALESCE(executed_by, '00000000-0000-0000-0000-000000000000'),
		       started_at, completed_at
		FROM eod_step_runs`
	var args []any
	if !businessDate.IsZero() {
		query += " WHERE business_date = $1"
		args = append(args, businessDate)
	}
	query += " ORDER BY business_date DESC, sequence, id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("membaca riwayat langkah EOD: %w", err)
	}
	defer rows.Close()

	var out []domain.EODStepRunRecord
	for rows.Next() {
		var rec domain.EODStepRunRecord
		var status string
		var summaryRaw []byte
		if err := rows.Scan(&rec.RunID, &rec.BusinessDate, &rec.StepCode, &rec.Sequence, &status, &rec.Trigger,
			&rec.Reason, &summaryRaw, &rec.ExecutedBy, &rec.StartedAt, &rec.CompletedAt); err != nil {
			return nil, err
		}
		rec.Status = domain.EODStepStatus(status)
		if len(summaryRaw) > 0 {
			_ = json.Unmarshal(summaryRaw, &rec.Summary)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ListDueScheduledTriggers mengembalikan pemicu terjadwal yang aktif dan sudah jatuh
// tempo. Penjadwal (daemon) belum ada; metode ini adalah jalur sambung yang jelas
// begitu penjadwal dipasang.
func (r *EODStepRepository) ListDueScheduledTriggers(ctx context.Context, now time.Time) ([]domain.EODTrigger, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, trigger_type, schedule_cron, enabled, description, last_run_at, next_run_at
		FROM eod_triggers
		WHERE trigger_type = 'SCHEDULED' AND enabled = TRUE
		  AND next_run_at IS NOT NULL AND next_run_at <= $1
		ORDER BY next_run_at`, now)
	if err != nil {
		return nil, fmt.Errorf("membaca pemicu EOD terjadwal: %w", err)
	}
	defer rows.Close()

	var out []domain.EODTrigger
	for rows.Next() {
		var t domain.EODTrigger
		if err := rows.Scan(&t.ID, &t.Name, &t.TriggerType, &t.ScheduleCron, &t.Enabled, &t.Description, &t.LastRunAt, &t.NextRunAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MarkTriggerRun mencatat waktu jalan terakhir dan jadwal berikutnya untuk satu pemicu.
// nextRunAt boleh nil: penjadwal belum menghitung jadwal, operator/penjadwal mengisinya.
func (r *EODStepRepository) MarkTriggerRun(ctx context.Context, name string, ranAt time.Time, nextRunAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE eod_triggers
		   SET last_run_at = $2, next_run_at = $3, updated_at = NOW()
		 WHERE name = $1`, name, ranAt, nextRunAt)
	return err
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
