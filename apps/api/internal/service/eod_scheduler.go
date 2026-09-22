package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Kunci system_config penjadwal EOD (migrasi 000083). Konvensi penamaan mengikuti
// kunci saklar lain (mis. ckpn.enabled, institution.book_scope): huruf kecil, bertitik.
const (
	// configKeyEODSchedulerEnabled adalah SAKLAR utama penjadwal. Bawaannya false
	// (di-seed migrasi) sehingga perilaku produksi tidak berubah sampai bank
	// menyalakannya; setelah dinyalakan, ia berlaku dalam satu siklus tick tanpa
	// rilis ulang.
	configKeyEODSchedulerEnabled = "eod.scheduler.enabled"
	// configKeyEODSchedulerExecutedBy adalah ID staf atas nama siapa EOD terjadwal
	// dijalankan. Wajib staf nyata karena kolom updated_by system_config dan
	// eod_step_runs.executed_by ber-FK ke staff_users; jejak audit juga harus
	// menunjuk orang yang dapat dimintai pertanggungjawaban.
	configKeyEODSchedulerExecutedBy = "eod.scheduler.executed_by"
)

// eodSchedulerTickInterval adalah jarak pemeriksaan pemicu jatuh tempo. Satu menit
// cukup halus untuk jadwal per menit dan cukup hemat untuk query ringan.
const eodSchedulerTickInterval = time.Minute

// EODScheduler memeriksa pemicu EOD terjadwal yang jatuh tempo lalu menjalankannya
// lewat jalur yang sama dengan pemicu manual (BatchProcessService.RunScheduledEOD).
//
// Ia MATI secara default: saklar eod.scheduler.enabled di-seed false, dan setiap tick
// membaca ulang saklar itu. Karena itu goroutine ini boleh berjalan tanpa mengubah
// perilaku produksi; bank menyalakannya kapan saja tanpa restart. Pemicu terjadwal
// bawaan (scheduled-daily) juga masih nonaktif, jadi ada dua lapis: saklar penjadwal
// dan enabled per pemicu.
type EODScheduler struct {
	batch  domain.BatchProcessService
	repo   domain.EODStepRepository
	config domain.SystemConfigService
	logger *slog.Logger
	// now dapat diganti pada uji; produksi memakai waktu nyata.
	now func() time.Time
}

// NewEODScheduler merangkai penjadwal. logger nil memakai logger default agar
// pemanggil sederhana (mis. uji) tidak wajib menyediakan satu.
func NewEODScheduler(batch domain.BatchProcessService, repo domain.EODStepRepository,
	config domain.SystemConfigService, logger *slog.Logger) *EODScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EODScheduler{
		batch:  batch,
		repo:   repo,
		config: config,
		logger: logger,
		now:    time.Now,
	}
}

// Start menjalankan loop tick sampai konteks dibatalkan. Dipanggil sebagai goroutine
// dari main; tidak ada jalan keluar lain selain pembatalan konteks atau akhir proses.
func (s *EODScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(eodSchedulerTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Tick(ctx); err != nil {
				s.logger.ErrorContext(ctx, "penjadwal EOD gagal pada satu siklus", "error", err)
			}
		}
	}
}

// Tick menjalankan satu siklus: membaca saklar, memvalidasi/mengisi jadwal pemicu,
// lalu menjalankan EOD bila ada pemicu yang jatuh tempo. Saklar mati berarti siklus
// tidak menyentuh apa pun. Galat dikembalikan (bukan hanya dicatat) agar uji dan
// pemanggil dapat memastikannya; loop Start mencatatnya.
func (s *EODScheduler) Tick(ctx context.Context) error {
	if s.config == nil || s.repo == nil || s.batch == nil {
		return nil
	}
	// Saklar dibaca tiap siklus: bank dapat menyalakan/mematikan tanpa restart.
	if !s.config.GetBool(ctx, configKeyEODSchedulerEnabled, false) {
		return nil
	}

	executedBy, err := s.executor(ctx)
	if err != nil {
		return err
	}

	now := s.now().In(domain.BankZone)
	triggers, err := s.repo.ListScheduledTriggers(ctx)
	if err != nil {
		return fmt.Errorf("membaca pemicu EOD terjadwal: %w", err)
	}

	var errs []error
	for _, trg := range triggers {
		sched, err := domain.ParseCron(trg.ScheduleCron)
		if err != nil {
			// Berisik: ekspresi rusak tidak boleh dianggap "tidak pernah jatuh tempo".
			errs = append(errs, fmt.Errorf("%w: pemicu %q: %v", domain.ErrEODInvalidTriggerSchedule, trg.Name, err))
			continue
		}
		// Pemicu aktif yang belum punya jadwal (mis. baru diaktifkan bank) diberi
		// jadwal pertama dari sekarang supaya tidak diam tanpa batas.
		if trg.NextRunAt == nil {
			next, err := sched.Next(now)
			if err != nil {
				errs = append(errs, fmt.Errorf("%w: pemicu %q: %v", domain.ErrEODInvalidTriggerSchedule, trg.Name, err))
				continue
			}
			if err := s.repo.SetTriggerNextRun(ctx, trg.Name, next); err != nil {
				errs = append(errs, fmt.Errorf("menyimpan jadwal awal pemicu %q: %w", trg.Name, err))
			}
		}
	}

	if _, err := s.batch.RunScheduledEOD(ctx, executedBy); err != nil {
		// Tidak ada pemicu jatuh tempo adalah keadaan normal, bukan kegagalan.
		if !errors.Is(err, domain.ErrEODNoDueTrigger) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// executor membaca staf pelaksana EOD terjadwal. Nilai kosong/rusak DITOLAK dengan
// galat jelas, bukan diganti identitas kosong: tanpa staf nyata, FK updated_by gagal
// dan jejak audit tidak dapat dipertanggungjawabkan.
func (s *EODScheduler) executor(ctx context.Context) (uuid.UUID, error) {
	raw := strings.TrimSpace(s.config.GetString(ctx, configKeyEODSchedulerExecutedBy, ""))
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: kunci %s harus berisi UUID staf yang sah, nilainya %q",
			domain.ErrEODSchedulerExecutorInvalid, configKeyEODSchedulerExecutedBy, raw)
	}
	return id, nil
}
