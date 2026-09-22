package service_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// recordingBatchSvc menggantikan BatchProcessService pada uji penjadwal: yang diuji
// adalah apakah jalur terjadwal dipanggil dan dengan staf siapa, bukan isi EOD-nya.
type recordingBatchSvc struct {
	calls          int
	lastExecutedBy uuid.UUID
	err            error
}

func (r *recordingBatchSvc) GetCurrentBusinessDate(context.Context) (*domain.SystemBusinessDate, error) {
	return nil, nil
}
func (r *recordingBatchSvc) RunEOD(context.Context, uuid.UUID) (*domain.EODSummaryResult, error) {
	return nil, nil
}
func (r *recordingBatchSvc) RunScheduledEOD(_ context.Context, executedBy uuid.UUID) (*domain.EODSummaryResult, error) {
	r.calls++
	r.lastExecutedBy = executedBy
	if r.err != nil {
		return nil, r.err
	}
	return &domain.EODSummaryResult{}, nil
}
func (r *recordingBatchSvc) RunEOM(context.Context, uuid.UUID) (*domain.EOMSummaryResult, error) {
	return nil, nil
}
func (r *recordingBatchSvc) RunEOY(context.Context, string, domain.Actor) (*domain.EOYSummaryResult, error) {
	return nil, nil
}

var _ domain.BatchProcessService = (*recordingBatchSvc)(nil)

const ujiExecutorUUID = "c0000000-0000-0000-0000-000000000001"

func schedulerTestConfig(enabled, executor string) *eodTestConfig {
	return &eodTestConfig{values: map[string]string{
		"eod.scheduler.enabled":     enabled,
		"eod.scheduler.executed_by": executor,
	}}
}

// Penjadwal WAJIB mati secara default: tanpa kunci saklar (bawaan migrasi false), satu
// siklus tidak menyentuh pemicu maupun menjalankan EOD.
func TestEODSchedulerOffByDefault(t *testing.T) {
	batch := &recordingBatchSvc{}
	repo := &eodDefRepoStub{scheduled: []domain.EODTrigger{{
		Name: "scheduled-daily", TriggerType: "SCHEDULED", ScheduleCron: "0 23 * * *", Enabled: true,
	}}}
	sched := service.NewEODScheduler(batch, repo, &eodTestConfig{}, slog.Default())

	if err := sched.Tick(context.Background()); err != nil {
		t.Fatalf("Tick saat saklar mati: %v", err)
	}
	if batch.calls != 0 {
		t.Fatalf("saklar mati tidak boleh menjalankan EOD, dapat %d panggilan", batch.calls)
	}
	if len(repo.nextSet) != 0 {
		t.Fatalf("saklar mati tidak boleh menyentuh jadwal pemicu, dapat %v", repo.nextSet)
	}
}

// Saklar menyala: jadwal awal pemicu diisi dari cron, lalu jalur terjadwal dijalankan
// dengan staf pelaksana dari konfigurasi.
func TestEODSchedulerRunsWithConfiguredExecutor(t *testing.T) {
	batch := &recordingBatchSvc{}
	repo := &eodDefRepoStub{scheduled: []domain.EODTrigger{{
		Name: "scheduled-daily", TriggerType: "SCHEDULED", ScheduleCron: "0 23 * * *", Enabled: true,
	}}}
	sched := service.NewEODScheduler(batch, repo, schedulerTestConfig("true", ujiExecutorUUID), slog.Default())

	if err := sched.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(repo.nextSet) != 1 || repo.nextSet[0] != "scheduled-daily" {
		t.Fatalf("jadwal awal pemicu harus diisi, dapat %v", repo.nextSet)
	}
	if batch.calls != 1 {
		t.Fatalf("jalur terjadwal harus dipanggil sekali, dapat %d", batch.calls)
	}
	if batch.lastExecutedBy.String() != ujiExecutorUUID {
		t.Fatalf("staf pelaksana %s, mau %s", batch.lastExecutedBy, ujiExecutorUUID)
	}
}

// Ekspresi cron rusak harus berisik, bukan diam-diam dilewati seolah pemicu tidak ada.
func TestEODSchedulerReportsInvalidCron(t *testing.T) {
	batch := &recordingBatchSvc{}
	repo := &eodDefRepoStub{scheduled: []domain.EODTrigger{{
		Name: "scheduled-rusak", TriggerType: "SCHEDULED", ScheduleCron: "bukan cron", Enabled: true,
	}}}
	sched := service.NewEODScheduler(batch, repo, schedulerTestConfig("true", ujiExecutorUUID), slog.Default())

	err := sched.Tick(context.Background())
	if !errors.Is(err, domain.ErrEODInvalidTriggerSchedule) {
		t.Fatalf("mau ErrEODInvalidTriggerSchedule, dapat %v", err)
	}
}

// Staf pelaksana tidak sah harus MENGHENTIKAN siklus: tanpa staf nyata, jejak audit
// dan kolom updated_by tidak dapat diisi.
func TestEODSchedulerRejectsInvalidExecutor(t *testing.T) {
	batch := &recordingBatchSvc{}
	repo := &eodDefRepoStub{scheduled: []domain.EODTrigger{{
		Name: "scheduled-daily", TriggerType: "SCHEDULED", ScheduleCron: "0 23 * * *", Enabled: true,
	}}}
	sched := service.NewEODScheduler(batch, repo, schedulerTestConfig("true", ""), slog.Default())

	err := sched.Tick(context.Background())
	if !errors.Is(err, domain.ErrEODSchedulerExecutorInvalid) {
		t.Fatalf("mau ErrEODSchedulerExecutorInvalid, dapat %v", err)
	}
	if batch.calls != 0 {
		t.Fatalf("EOD tidak boleh berjalan tanpa staf pelaksana sah, dapat %d panggilan", batch.calls)
	}
}
