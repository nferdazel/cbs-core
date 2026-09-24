package service_test

import (
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// Riwayat per langkah harus benar-benar tersimpan di database nyata (bukan hanya di
// stub), dan idempotensi per tanggal bisnis harus tetap berlaku: tanggal yang sudah
// CLOSED menolak tutup hari dan TIDAK menambah baris riwayat.
func TestIntegrasiEODRiwayatPerLangkahDanIdempotensi(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanConfig(t, e, "system.business_date_status")
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")

	businessDate := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC)
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))

	beforeCount := e.eodStepRunCount(t, businessDate)
	if _, err := batchSvc.RunEOD(e.ctx, e.actor.UserID); err != nil {
		t.Fatalf("RunEOD pertama: %v", err)
	}

	rows := e.eodStepRunRows(t, businessDate)
	defs := unitEODDefinitions()
	if len(rows) != len(defs) {
		t.Fatalf("riwayat %d langkah, mau %d", len(rows), len(defs))
	}
	afterCount := e.eodStepRunCount(t, businessDate)
	if afterCount != beforeCount+len(defs) {
		t.Fatalf("riwayat bertambah %d, mau %d (satu baris per langkah run ini)",
			afterCount-beforeCount, len(defs))
	}
	seenManual := false
	for _, r := range rows {
		if r.Trigger != "MANUAL" {
			t.Fatalf("trigger riwayat %s, mau MANUAL", r.Trigger)
		}
		if r.Reason == "" && r.Status != domain.EODStepRan {
			t.Fatalf("langkah %s status %s tanpa alasan", r.StepCode, r.Status)
		}
		if r.StepCode == "loan_interest_accrual" {
			seenManual = true
			if r.Status != domain.EODStepRan {
				t.Fatalf("akrual status %s (%s), mau RAN", r.Status, r.Reason)
			}
		}
	}
	if !seenManual {
		t.Fatal("riwayat langkah akrual tidak ditemukan")
	}

	// Tanggal sudah CLOSED: tutup hari berikutnya di tanggal yang sama harus ditolak
	// dan tidak boleh menambah riwayat. Upsert agar berlaku juga bila baris status belum
	// ada pada database yang baru dimigrasi.
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('system.business_date_status', 'CLOSED', 'status tanggal bisnis untuk uji')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`); err != nil {
		t.Fatalf("menyetel status CLOSED: %v", err)
	}
	before := afterCount
	if _, err := batchSvc.RunEOD(e.ctx, e.actor.UserID); !errors.Is(err, domain.ErrEODAlreadyRunForDate) {
		t.Fatalf("tanggal CLOSED harus ditolak ErrEODAlreadyRunForDate, dapat %v", err)
	}
	if after := e.eodStepRunCount(t, businessDate); after != before {
		t.Fatalf("tutup hari yang ditolak tidak boleh menambah riwayat: sebelum %d, sesudah %d", before, after)
	}
}

func (e *moneyEnv) eodStepRunCount(t *testing.T, businessDate time.Time) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT count(*) FROM eod_step_runs WHERE business_date = $1`, businessDate).Scan(&n); err != nil {
		t.Fatalf("menghitung riwayat EOD: %v", err)
	}
	return n
}

type eodStepRunRow struct {
	StepCode string
	Status   domain.EODStepStatus
	Trigger  string
	Reason   string
}

func (e *moneyEnv) eodStepRunRows(t *testing.T, businessDate time.Time) []eodStepRunRow {
	t.Helper()
	rows, err := e.db.QueryContext(e.ctx, `
		WITH latest AS (
			SELECT run_id FROM eod_step_runs
			WHERE business_date = $1
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		)
		SELECT step_code, status, trigger, reason
		FROM eod_step_runs
		WHERE run_id = (SELECT run_id FROM latest)
		ORDER BY sequence`, businessDate)
	if err != nil {
		t.Fatalf("membaca riwayat EOD: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []eodStepRunRow
	for rows.Next() {
		var r eodStepRunRow
		var status string
		if err := rows.Scan(&r.StepCode, &status, &r.Trigger, &r.Reason); err != nil {
			t.Fatalf("scan riwayat: %v", err)
		}
		r.Status = domain.EODStepStatus(status)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterasi riwayat: %v", err)
	}
	return out
}
