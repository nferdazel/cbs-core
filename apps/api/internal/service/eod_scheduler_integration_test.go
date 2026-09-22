package service_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// Uji integrasi penjadwal EOD (W17) terhadap PostgreSQL sungguhan: pemicu jatuh tempo
// dijalankan tepat sekali, jadwal berikutnya maju, pemicu yang belum jatuh tempo tidak
// dijalankan, ekspresi cron tidak sah ditolak, dan jalur terjadwal tidak menggandakan
// EOD saat tanggal sudah ditutup atau saat kunci tutup hari dipegang proses lain.

// eodAdvisoryLockKey adalah kunci advisory tutup hari di
// repository/postgres/system_date_repo.go; disalin ke uji agar dapat meniru proses
// lain yang sedang menutup hari.
const eodAdvisoryLockKey = 4217001

func setEODSchedulerConfig(t *testing.T, e *moneyEnv, key, value string) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi penjadwal EOD')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

// insertScheduledTrigger menyisipkan pemicu terjadwal unik dan membersihkannya di akhir.
func (e *moneyEnv) insertScheduledTrigger(t *testing.T, name, cron string, next time.Time) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO eod_triggers (name, trigger_type, schedule_cron, enabled, description, next_run_at)
		VALUES ($1, 'SCHEDULED', $2, TRUE, 'uji integrasi penjadwal EOD', $3)`,
		name, cron, next); err != nil {
		t.Fatalf("menyiapkan pemicu terjadwal: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(context.Background(), `DELETE FROM eod_triggers WHERE name = $1`, name)
	})
}

type scheduledTriggerState struct {
	next    *time.Time
	last    *time.Time
	enabled bool
}

func (e *moneyEnv) scheduledTrigger(t *testing.T, name string) scheduledTriggerState {
	t.Helper()
	var st scheduledTriggerState
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT next_run_at, last_run_at, enabled FROM eod_triggers WHERE name = $1`, name).
		Scan(&st.next, &st.last, &st.enabled); err != nil {
		t.Fatalf("membaca pemicu terjadwal %s: %v", name, err)
	}
	return st
}

// newSchedulerForTest merangkai penjadwal produksi di atas database uji.
func (e *moneyEnv) newSchedulerForTest(t *testing.T, batch domain.BatchProcessService) *service.EODScheduler {
	t.Helper()
	return service.NewEODScheduler(batch, postgres.NewEODStepRepository(e.db), e.configSvc, slog.Default())
}

func eodSchedulerTestBusinessDate() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// Pemicu jatuh tempo dijalankan TEPAT SEKALI, riwayatnya bertanda SCHEDULED, dan
// next_run_at maju ke kemunculan cron berikutnya; tick kedua tidak mengulang.
func TestIntegrasiEODPenjadwalMenjalankanSekaliDanMemajukanJadwal(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `UPDATE system_config SET value='OPEN' WHERE key='system.business_date_status'`)
		setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "false")
	})
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "false")
	setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "true")
	setEODSchedulerConfig(t, e, "eod.scheduler.executed_by", e.actor.UserID.String())

	name := fmt.Sprintf("scheduled-uji-%d", time.Now().UnixNano())
	// Cron 1 Januari: setelah dijalankan, kemunculan berikutnya jauh di masa depan
	// sehingga tick kedua pasti tidak jatuh tempo.
	e.insertScheduledTrigger(t, name, "0 0 1 1 *", time.Now().Add(-time.Minute))

	businessDate := eodSchedulerTestBusinessDate()
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))
	sched := e.newSchedulerForTest(t, batchSvc)

	before := e.eodStepRunCount(t, businessDate)
	if err := sched.Tick(e.ctx); err != nil {
		t.Fatalf("Tick pertama: %v", err)
	}
	after := e.eodStepRunCount(t, businessDate)
	if after != before+len(unitEODDefinitions()) {
		t.Fatalf("riwayat bertambah %d, mau %d (EOD terjadwal harus dijalankan sekali)",
			after-before, len(unitEODDefinitions()))
	}
	for _, row := range e.eodStepRunRows(t, businessDate) {
		if row.Trigger != "SCHEDULED" {
			t.Fatalf("trigger riwayat %s, mau SCHEDULED", row.Trigger)
		}
	}

	st := e.scheduledTrigger(t, name)
	if st.last == nil {
		t.Fatal("pemicu harus tercatat last_run_at")
	}
	if st.next == nil || !st.next.After(time.Now()) {
		t.Fatalf("next_run_at harus maju ke masa depan, dapat %v", st.next)
	}
	nextWIB := st.next.In(domain.BankZone)
	if nextWIB.Month() != time.January || nextWIB.Day() != 1 || nextWIB.Hour() != 0 {
		t.Fatalf("next_run_at = %s, mau 1 Januari 00:00 WIB", nextWIB)
	}

	// Tick kedua dalam rentang singkat: pemicu sudah tidak jatuh tempo.
	countAfterFirst := after
	if err := sched.Tick(e.ctx); err != nil {
		t.Fatalf("Tick kedua: %v", err)
	}
	if got := e.eodStepRunCount(t, businessDate); got != countAfterFirst {
		t.Fatalf("tick kedua tidak boleh mengulang EOD: %d -> %d", countAfterFirst, got)
	}
}

// Pemicu yang belum jatuh tempo tidak dijalankan dan jadwalnya tidak disentuh.
func TestIntegrasiEODPenjadwalTidakMenjalankanPemicuBelumJatuhTempo(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() {
		setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "false")
	})
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "true")
	setEODSchedulerConfig(t, e, "eod.scheduler.executed_by", e.actor.UserID.String())

	future := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	name := fmt.Sprintf("scheduled-belum-%d", time.Now().UnixNano())
	e.insertScheduledTrigger(t, name, "0 0 1 1 *", future)

	businessDate := eodSchedulerTestBusinessDate()
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))
	sched := e.newSchedulerForTest(t, batchSvc)

	before := e.eodStepRunCount(t, businessDate)
	if err := sched.Tick(e.ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if got := e.eodStepRunCount(t, businessDate); got != before {
		t.Fatalf("pemicu belum jatuh tempo tidak boleh menjalankan EOD: %d -> %d", before, got)
	}
	st := e.scheduledTrigger(t, name)
	if st.next == nil || !st.next.Equal(future) {
		t.Fatalf("jadwal pemicu belum jatuh tempo tidak boleh diubah, dapat %v", st.next)
	}
	if st.last != nil {
		t.Fatal("pemicu belum jatuh tempo tidak boleh tercatat sudah jalan")
	}
}

// Ekspresi cron tidak sah ditolak dengan galat jelas; EOD tidak berjalan dan pemicu
// tidak ditandai.
func TestIntegrasiEODPenjadwalMenolakCronTidakValid(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() {
		setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "false")
	})
	setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "true")
	setEODSchedulerConfig(t, e, "eod.scheduler.executed_by", e.actor.UserID.String())

	name := fmt.Sprintf("scheduled-rusak-%d", time.Now().UnixNano())
	e.insertScheduledTrigger(t, name, "bukan cron", time.Now().Add(-time.Minute))

	businessDate := eodSchedulerTestBusinessDate()
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))
	sched := e.newSchedulerForTest(t, batchSvc)

	before := e.eodStepRunCount(t, businessDate)
	err := sched.Tick(e.ctx)
	if !errors.Is(err, domain.ErrEODInvalidTriggerSchedule) {
		t.Fatalf("mau ErrEODInvalidTriggerSchedule, dapat %v", err)
	}
	if got := e.eodStepRunCount(t, businessDate); got != before {
		t.Fatalf("EOD tidak boleh berjalan saat cron tidak sah: %d -> %d", before, got)
	}
	if st := e.scheduledTrigger(t, name); st.last != nil {
		t.Fatal("pemicu dengan cron tidak sah tidak boleh ditandai sudah jalan")
	}
}

// Idempotensi dengan pemicu manual: bila tanggal bisnis sudah ditutup, jalur terjadwal
// TIDAK menggandakan EOD. Pemicunya tetap ditandai dan jadwalnya maju agar tidak
// dicoba ulang setiap tick.
func TestIntegrasiEODPenjadwalTidakMenggandakanSaatTanggalSudahDitutup(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `UPDATE system_config SET value='OPEN' WHERE key='system.business_date_status'`)
		setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "false")
	})
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "true")
	setEODSchedulerConfig(t, e, "eod.scheduler.executed_by", e.actor.UserID.String())

	// Tanggal bisnis ditandai CLOSED: meniru pemicu manual yang sudah menjalankan EOD
	// untuk tanggal ini.
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE system_config SET value='CLOSED' WHERE key='system.business_date_status'`); err != nil {
		t.Fatalf("menyetel status CLOSED: %v", err)
	}

	name := fmt.Sprintf("scheduled-tutup-%d", time.Now().UnixNano())
	e.insertScheduledTrigger(t, name, "0 0 1 1 *", time.Now().Add(-time.Minute))

	businessDate := eodSchedulerTestBusinessDate()
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))
	sched := e.newSchedulerForTest(t, batchSvc)

	before := e.eodStepRunCount(t, businessDate)
	err := sched.Tick(e.ctx)
	if !errors.Is(err, domain.ErrEODAlreadyRunForDate) {
		t.Fatalf("mau ErrEODAlreadyRunForDate, dapat %v", err)
	}
	if got := e.eodStepRunCount(t, businessDate); got != before {
		t.Fatalf("tanggal CLOSED tidak boleh menambah riwayat: %d -> %d", before, got)
	}
	st := e.scheduledTrigger(t, name)
	if st.last == nil {
		t.Fatal("pemicu harus ditandai terlayani walau EOD tanggal ini sudah berjalan")
	}
	if st.next == nil || !st.next.After(time.Now()) {
		t.Fatalf("jadwal pemicu harus maju, dapat %v", st.next)
	}
}

// Kunci advisory tutup hari yang dipegang proses lain (mis. pemicu manual yang sedang
// berjalan) membuat jalur terjadwal MENOLAK, bukan menjalankan EOD kedua.
func TestIntegrasiEODPenjadwalMenghormatiKunciTutupHari(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() {
		setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "false")
	})
	setRestructureLossConfig(t, e, "loan.restructure.loss.enabled", "false")
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setEODSchedulerConfig(t, e, "eod.scheduler.enabled", "true")
	setEODSchedulerConfig(t, e, "eod.scheduler.executed_by", e.actor.UserID.String())

	// Pegang kunci pada koneksi khusus supaya koneksi lain di pool tidak bisa
	// mengambilnya, meniru proses tutup hari yang sedang berjalan.
	lockConn, err := e.db.Conn(e.ctx)
	if err != nil {
		t.Fatalf("mengambil koneksi kunci: %v", err)
	}
	defer lockConn.Close()
	var acquired bool
	if err := lockConn.QueryRowContext(e.ctx, "SELECT pg_try_advisory_lock($1)", eodAdvisoryLockKey).Scan(&acquired); err != nil {
		t.Fatalf("mengambil kunci advisory: %v", err)
	}
	if !acquired {
		t.Fatal("kunci advisory uji tidak dapat diambil")
	}
	defer func() {
		_, _ = lockConn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", eodAdvisoryLockKey)
	}()

	name := fmt.Sprintf("scheduled-kunci-%d", time.Now().UnixNano())
	e.insertScheduledTrigger(t, name, "0 0 1 1 *", time.Now().Add(-time.Minute))

	businessDate := eodSchedulerTestBusinessDate()
	batchSvc := e.newBatchSvcForTest(t, newCKPNSvcForTest(e))
	sched := e.newSchedulerForTest(t, batchSvc)

	before := e.eodStepRunCount(t, businessDate)
	err = sched.Tick(e.ctx)
	if !errors.Is(err, domain.ErrEODInProgress) {
		t.Fatalf("mau ErrEODInProgress, dapat %v", err)
	}
	if got := e.eodStepRunCount(t, businessDate); got != before {
		t.Fatalf("kunci dipegang: EOD tidak boleh berjalan, %d -> %d", before, got)
	}
	if st := e.scheduledTrigger(t, name); st.last != nil {
		t.Fatal("pemicu tidak boleh ditandai jalan saat kunci dipegang proses lain")
	}
}
