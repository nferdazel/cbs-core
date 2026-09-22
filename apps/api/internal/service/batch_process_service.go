package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// COA laba ditahan baku BPR (migrasi 000005). Syariah memakai akun terpisah agar
// penutupan UUS tidak mencampur laba konvensional.
const (
	defaultRetainedEarningsCOAConv = "30200"
	defaultRetainedEarningsCOASyar = "13200"

	configRetainedEarningsCOAConv = "retained_earnings.coa.conventional"
	configRetainedEarningsCOASyar = "retained_earnings.coa.syariah"
)

// Catatan/label yang menemani angka ringkasan EOD agar pembaca laporan tahu batas
// maknanya tanpa harus membaca kode.
const (
	// socialFundNote menjelaskan saldo dana kebajikan (12500). Denda ta'zir bukan
	// pendapatan bank; penyalurannya menunggu keputusan Dewan Pengawas Syariah.
	socialFundNote = "Saldo akun 12500 Dana Kebajikan: akumulasi denda keterlambatan pembiayaan syariah (ta'zir), bukan pendapatan bank; menunggu keputusan penyaluran oleh Dewan Pengawas Syariah."
	// depositAmountLabel menandai keterbatasan data lama: jurnal penempatan deposito
	// yang dibuat sebelum tipe DEPOSIT_PLACEMENT ada tetap bertipe DEPOSIT sehingga
	// ikut terhitung sebagai setoran tunai.
	depositAmountLabel = "Setoran tunai teller (DEPOSIT). Keterbatasan data lama: jurnal penempatan deposito yang dibuat sebelum tipe DEPOSIT_PLACEMENT ada tetap bertipe DEPOSIT dan ikut terhitung di sini; penempatan berjenis baru dilaporkan terpisah pada total_deposit_placements_today/total_deposit_placement_amount_today."
)

// Nama langkah pekerjaan harian EOD. Dipakai sebagai kunci pada EODSummaryResult.Steps
// dan sebagai nama prasyarat antar-langkah. Sengaja berupa konstanta agar
// perbandingan prasyarat tidak bergantung pada string yang ditulis ulang.
const (
	eodStepARO              = "aro"
	eodStepPPAP             = "ppap"
	eodStepCKPN             = "ckpn_comparison"
	eodStepPenalty          = "loan_penalty_accrual"
	eodStepInterestAccrual  = "loan_interest_accrual"
	eodStepLossAmortization = "restructure_loss_amortization"
	eodStepDormant          = "dormant"
)

// Nilai kolom trigger pada riwayat eod_step_runs. Sama dengan domain.EODTrigger*.
const (
	eodTriggerManual    = "MANUAL"
	eodTriggerScheduled = "SCHEDULED"
)

type batchProcessService struct {
	dateRepo    domain.BusinessDateRepository
	batchRepo   domain.BatchActivityRepository
	savingsSvc  domain.SavingsInterestService
	yearEndRepo domain.YearEndRepository
	posting     domain.PostingService
	resolver    domain.AccountResolver
	configSvc   domain.SystemConfigService
	db          *sql.DB
	// Pekerjaan harian EOD. Boleh nil pada lingkungan yang belum menyediakannya,
	// dan kegagalannya tidak menggagalkan tutup hari (lihat runDailyJob).
	aroSvc     domain.ARORunner
	ppapSvc    domain.PPAPRunner
	penaltySvc domain.LoanPenaltyService
	dormantSvc domain.DormantRunner
	accrualSvc domain.LoanInterestAccrualRunner
	// Perbandingan CKPN vs required_ppap hasil PPAP. Baca-saja, tetapi hanya boleh
	// dijalankan bila langkah PPAP berhasil pada tanggal bisnis yang sama; jika
	// tidak, angkanya berasal dari run PPAP sebelumnya dan menyesatkan.
	ckpnSvc domain.CKPNService
	// eodRepo membaca definisi urutan langkah dari database dan menulis riwayat hasil
	// per langkah. Boleh nil hanya pada lingkungan uji tanpa database; bila nil tutup
	// hari MENOLAK berjalan (definisi tidak tersedia), bukan memakai urutan cadangan.
	eodRepo domain.EODStepRepository
}

func NewBatchProcessService(
	dateRepo domain.BusinessDateRepository,
	batchRepo domain.BatchActivityRepository,
	savingsSvc domain.SavingsInterestService,
	yearEndRepo domain.YearEndRepository,
	posting domain.PostingService,
	resolver domain.AccountResolver,
	configSvc domain.SystemConfigService,
	db *sql.DB,
	aro domain.ARORunner,
	ppap domain.PPAPRunner,
	penalty domain.LoanPenaltyService,
	dormant domain.DormantRunner,
	accrual domain.LoanInterestAccrualRunner,
	ckpn domain.CKPNService,
	eodRepo domain.EODStepRepository,
) domain.BatchProcessService {
	return &batchProcessService{
		dateRepo:    dateRepo,
		batchRepo:   batchRepo,
		savingsSvc:  savingsSvc,
		yearEndRepo: yearEndRepo,
		posting:     posting,
		resolver:    resolver,
		configSvc:   configSvc,
		db:          db,
		aroSvc:      aro,
		ppapSvc:     ppap,
		penaltySvc:  penalty,
		dormantSvc:  dormant,
		accrualSvc:  accrual,
		ckpnSvc:     ckpn,
		eodRepo:     eodRepo,
	}
}

func (s *batchProcessService) GetCurrentBusinessDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	return s.dateRepo.GetCurrentDate(ctx)
}

func (s *batchProcessService) RunEOD(ctx context.Context, executedBy uuid.UUID) (*domain.EODSummaryResult, error) {
	return s.runEOD(ctx, executedBy, eodTriggerManual)
}

// RunScheduledEOD menjalankan tutup hari dari pemicu terjadwal. Penjadwal (daemon/cron)
// belum ada di aplikasi; metode ini menyediakan jalur yang jelas: penjadwal yang
// dipasang kelak memanggilnya, dan ia hanya berjalan bila ada pemicu SCHEDULED aktif
// yang jatuh tempo di eod_triggers. Tanpa pemicu jatuh tempo ia menolak dengan
// ErrEODNoDueTrigger, bukan menutup hari atas inisiatif sendiri.
func (s *batchProcessService) RunScheduledEOD(ctx context.Context, executedBy uuid.UUID) (*domain.EODSummaryResult, error) {
	if s.eodRepo == nil {
		return nil, fmt.Errorf("%w: repositori definisi langkah tidak dikonfigurasi", domain.ErrEODDefinitionsUnavailable)
	}
	due, err := s.eodRepo.ListDueScheduledTriggers(ctx, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("membaca pemicu EOD terjadwal: %w", err)
	}
	if len(due) == 0 {
		return nil, domain.ErrEODNoDueTrigger
	}

	result, err := s.runEOD(ctx, executedBy, eodTriggerScheduled)
	if err != nil {
		return nil, err
	}
	for _, trg := range due {
		// next_run_at TIDAK dihitung di sini: tidak ada evaluator cron di aplikasi.
		// Mengosongkannya mencegah pemicu yang sama langsung jatuh tempo lagi.
		if err := s.eodRepo.MarkTriggerRun(ctx, trg.Name, time.Now().UTC(), nil); err != nil {
			observability.FromContext(ctx).ErrorContext(ctx, "gagal menandai pemicu EOD terjadwal", "pemicu", trg.Name, "error", err)
		}
	}
	return result, nil
}

// loadEODDefinitions membaca definisi langkah dari database, memvalidasinya dengan
// pengaman penuh, lalu mengurutkannya. Kegagalan apa pun menjadi error sehingga tutup
// hari MENOLAK berjalan — salah konfigurasi urutan bisa merusak akuntansi, jadi tidak
// boleh diam-diam dilewati.
func (s *batchProcessService) loadEODDefinitions(ctx context.Context) ([]domain.EODStepDefinition, error) {
	if s.eodRepo == nil {
		return nil, fmt.Errorf("%w: repositori definisi langkah tidak dikonfigurasi", domain.ErrEODDefinitionsUnavailable)
	}
	defs, err := s.eodRepo.ListDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrEODDefinitionsUnavailable, err)
	}
	if err := domain.ValidateEODStepDefinitions(defs); err != nil {
		return nil, fmt.Errorf("definisi langkah EOD tidak sah: %w", err)
	}
	return domain.SortEODStepDefinitions(defs), nil
}

// runEOD menjalankan satu siklus tutup hari dengan pemicu tertentu. Dipisah dari RunEOD
// agar jalur manual dan terjadwal memakai mesin yang sama persis.
func (s *batchProcessService) runEOD(ctx context.Context, executedBy uuid.UUID, trigger string) (*domain.EODSummaryResult, error) {
	// Kunci eksklusif lebih dulu. Klaim status saja tidak memadai: status EOD berarti
	// "sedang berjalan" sekaligus "pernah berhenti di tengah", sedangkan kunci ini
	// hanya membedakan yang pertama dan dilepas otomatis bila proses mati.
	release, err := s.dateRepo.TryEODLock(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := release(); err != nil {
			observability.FromContext(ctx).ErrorContext(ctx, "gagal melepas kunci tutup hari", "error", err)
		}
	}()

	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}

	// Definisi dibaca & divalidasi SEBELUM mengklaim tanggal: konfigurasi rusak harus
	// gagal tanpa mengubah status tutup hari dan tidak boleh diam-diam melewati langkah.
	defs, err := s.loadEODDefinitions(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Klaim tanggal bisnis untuk tutup hari dalam satu statement. Pola
	// baca-lalu-tulis sebelumnya membuat dua permintaan tutup hari yang datang
	// bersamaan dapat sama-sama lolos dan menjalankan pekerjaan harian dua kali.
	claimed, err := s.dateRepo.ClaimEOD(ctx)
	if err != nil {
		return nil, fmt.Errorf("mengunci sistem untuk tutup hari: %w", err)
	}
	if !claimed {
		return nil, domain.ErrEODAlreadyRunForDate
	}

	// 2. Pekerjaan harian dijalankan untuk tanggal bisnis yang sedang ditutup,
	// sebelum tanggal dimajukan, agar jurnalnya masuk ke tanggal yang benar. Urutannya
	// datang dari definisi database, bukan dari kode.
	summary := &domain.EODSummaryResult{ExecutedDate: curDate.CurrentDate}
	s.runDailyJobs(ctx, curDate.CurrentDate, s.systemBatchActor(ctx, executedBy), summary, defs, trigger)

	// 3. Calculate next business date (+1 day)
	nextDate := curDate.CurrentDate.AddDate(0, 0, 1)

	// 4. Advance system business date and reopen system
	if err := s.dateRepo.AdvanceDate(ctx, nextDate, executedBy); err != nil {
		return nil, fmt.Errorf("failed to advance business date: %w", err)
	}

	// Ringkasan dihitung dari jurnal tanggal bisnis yang ditutup, bukan konstanta.
	activity := &domain.DailyActivitySummary{}
	if s.batchRepo != nil {
		activity, err = s.batchRepo.DailyActivity(ctx, curDate.CurrentDate)
		if err != nil {
			return nil, fmt.Errorf("menghitung aktivitas harian: %w", err)
		}
	}

	summary.NextBusinessDate = nextDate
	summary.TotalPostedJournalsToday = activity.PostedJournals
	summary.TotalDepositAmountToday = activity.TotalDepositAmount
	summary.TotalDepositAmountTodayLabel = depositAmountLabel
	summary.TotalDepositPlacementsToday = activity.DepositPlacementCount
	summary.TotalDepositPlacementAmount = activity.TotalDepositPlacementAmount
	summary.TotalWithdrawalAmountToday = activity.TotalWithdrawalAmount
	// Saldo dana kebajikan dibaca dari akun 12500 (bukan konstanta, bukan angka
	// produksi): EOD hanya melaporkan, tidak menulis jurnal dan tidak mengubah akrual.
	summary.SocialFundBalance = activity.SocialFundBalance
	summary.SocialFundNote = socialFundNote
	summary.ExecutedBy = executedBy
	summary.CompletedAt = time.Now().UTC()
	return summary, nil
}

// runDailyJobs menjalankan pekerjaan harian yang menyertai tutup hari MENURUT DEFINISI
// YANG DIMUAT DARI DATABASE (tabel eod_step_definitions), bukan urutan yang ditulis di
// kode. Setiap pekerjaan terisolasi — kegagalannya hanya menghasilkan peringatan dan
// tidak menghentikan pekerjaan berikutnya maupun tutup hari, karena tutup hari yang
// gagal akan menghentikan seluruh operasional bank. Layanan yang belum dikonfigurasi
// (nil) dilewati dengan alasan, sama seperti perilaku lama. Langkah yang dinonaktifkan
// di definisi dilewati dengan peringatan.
// eodStepOutcome adalah hasil satu langkah EOD: status, alasan, dan angka penting yang
// disimpan ke riwayat per langkah (eod_step_runs).
type eodStepOutcome struct {
	Status  domain.EODStepStatus
	Reason  string
	Metrics map[string]any
}

func eodStepRan(metrics map[string]any) eodStepOutcome {
	return eodStepOutcome{Status: domain.EODStepRan, Metrics: metrics}
}

func eodStepSkipped(reason string, metrics ...map[string]any) eodStepOutcome {
	out := eodStepOutcome{Status: domain.EODStepSkipped, Reason: reason}
	if len(metrics) > 0 {
		out.Metrics = metrics[0]
	}
	return out
}

func eodStepFailed(reason string) eodStepOutcome {
	return eodStepOutcome{Status: domain.EODStepFailed, Reason: reason}
}

// eodPrerequisiteSkip memeriksa prasyarat sebuah langkah. Bila tidak terpenuhi, ia
// mengembalikan outcome SKIPPED beralasan dan mencatat peringatan dengan awalan yang
// khas langkah itu (agar pesan lama tetap terbaca).
func eodPrerequisiteSkip(summary *domain.EODSummaryResult, warningPrefix string, prereqs ...string) (eodStepOutcome, bool) {
	reason := unmetPrerequisite(summary, prereqs...)
	if reason == "" {
		return eodStepOutcome{}, false
	}
	summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s dilewati: %s", warningPrefix, reason))
	return eodStepSkipped(reason), true
}

// runDailyJobs menjalankan pekerjaan harian yang menyertai tutup hari MENURUT DEFINISI
// YANG DIMUAT DARI DATABASE (tabel eod_step_definitions), bukan urutan yang ditulis di
// kode. Setiap pekerjaan terisolasi — kegagalannya hanya menghasilkan peringatan dan
// tidak menghentikan pekerjaan berikutnya maupun tutup hari, karena tutup hari yang
// gagal akan menghentikan seluruh operasional bank. Layanan yang belum dikonfigurasi
// (nil) dilewati dengan alasan, sama seperti perilaku lama. Langkah yang dinonaktifkan
// di definisi dilewati dengan peringatan.
//
// Definisi sudah divalidasi sebelum fungsi ini dipanggil (kode dikenal, urutan unik,
// prasyarat ada dan tidak bersiklus, langkah inti ada & aktif). Karena itu kode yang
// tiba di sini selalu punya pelaksana.
//
// Hasil tiap langkah dicatat ke eod_step_runs agar dapat ditelusuri per tanggal bisnis;
// kegagalan menulis riwayat hanya menjadi peringatan, tidak membatalkan tutup hari.
func (s *batchProcessService) runDailyJobs(ctx context.Context, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, defs []domain.EODStepDefinition, trigger string) {
	logger := observability.FromContext(ctx)

	// Peringatan konfigurasi mengikuti cakupan buku instalasi dan dicatat pada run
	// yang sama agar tidak diam. Tidak mengubah langkah atau angka apa pun.
	summary.Warnings = append(summary.Warnings, InstallationValidationWarnings(ctx, s.configSvc)...)

	runID := uuid.New()
	for _, def := range defs {
		startedAt := time.Now().UTC()
		outcome := s.executeEODStep(ctx, def, businessDate, actor, summary, logger)
		recordEODStep(summary, def.Code, outcome.Status, outcome.Reason, def.Prerequisites...)

		if s.eodRepo == nil {
			continue
		}
		rec := domain.EODStepRunRecord{
			RunID:        runID,
			BusinessDate: businessDate,
			StepCode:     def.Code,
			Sequence:     def.Sequence,
			Status:       outcome.Status,
			Trigger:      trigger,
			Reason:       outcome.Reason,
			Summary:      outcome.Metrics,
			ExecutedBy:   actor.UserID,
			StartedAt:    startedAt,
			CompletedAt:  time.Now().UTC(),
		}
		if err := s.eodRepo.RecordStepResult(ctx, rec); err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("riwayat langkah EOD %s tidak tersimpan: %v", def.Code, err))
			logger.ErrorContext(ctx, "gagal menulis riwayat langkah EOD", "langkah", def.Code, "error", err)
		}
	}
}

// executeEODStep menjalankan satu langkah berdasarkan kodenya. Langkah nonaktif
// dilewati; kode tak dikenal mesin dicatat FAILED — gagal berisik, bukan diam-diam
// dilewati (validasi seharusnya sudah menolaknya lebih dulu).
func (s *batchProcessService) executeEODStep(ctx context.Context, def domain.EODStepDefinition, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if !def.Enabled {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("langkah EOD %s dinonaktifkan di definisi; langkah ini tidak dijalankan", def.Code))
		return eodStepSkipped("langkah dinonaktifkan di definisi EOD")
	}

	switch def.Code {
	case eodStepARO:
		return s.runAROStep(ctx, businessDate, actor, summary, logger)
	case eodStepInterestAccrual:
		return s.runInterestAccrualStep(ctx, businessDate, actor, summary, logger)
	case eodStepLossAmortization:
		return s.runLossAmortizationStep(ctx, def, businessDate, actor, summary, logger)
	case eodStepPPAP:
		return s.runPPAPStep(ctx, def, businessDate, actor, summary, logger)
	case eodStepCKPN:
		return s.runCKPNStep(ctx, def, businessDate, actor, summary, logger)
	case eodStepPenalty:
		return s.runPenaltyStep(ctx, businessDate, actor, summary, logger)
	case eodStepDormant:
		return s.runDormantStep(ctx, businessDate, actor, summary, logger)
	default:
		reason := fmt.Sprintf("kode langkah EOD %q tidak dikenal mesin", def.Code)
		summary.Warnings = append(summary.Warnings, reason)
		logger.ErrorContext(ctx, "kode langkah EOD tidak dikenal", "langkah", def.Code)
		return eodStepFailed(reason)
	}
}

// runAROStep menjalankan perpanjangan otomatis deposito.
func (s *batchProcessService) runAROStep(ctx context.Context, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.aroSvc == nil {
		return eodStepSkipped("layanan perpanjangan otomatis deposito tidak dikonfigurasi")
	}
	rolled, err := s.aroSvc.RunARO(ctx, businessDate, actor)
	summary.DepositsRolledOver = rolled
	if err != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("perpanjangan otomatis deposito gagal: %v", err))
		logger.ErrorContext(ctx, "perpanjangan otomatis deposito gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	}
	return eodStepRan(map[string]any{"deposits_rolled_over": rolled})
}

// runInterestAccrualStep mengakru pendapatan bunga kredit. Berjalan lebih dulu karena
// amortisasi saldo kerugian restrukturisasi berdiri di atas pendapatan jadwal periode
// itu.
func (s *batchProcessService) runInterestAccrualStep(ctx context.Context, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.accrualSvc == nil {
		return eodStepSkipped("layanan akrual bunga kredit tidak dikonfigurasi")
	}
	accrual, err := s.accrualSvc.AccrueInterest(ctx, businessDate, actor)
	summary.LoanInterestAccrued = accrual.Accrued
	summary.LoanInterestAccruedAmount = accrual.TotalAccrued
	if err != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("akrual bunga kredit gagal: %v", err))
		logger.ErrorContext(ctx, "akrual bunga kredit gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	}
	// Produk tanpa pemetaan INTEREST_ACCRUAL bukan kegagalan teknis, tetapi harus
	// terlihat: tanpa peringatan, batch tampak mengakru padahal tidak.
	summary.Warnings = append(summary.Warnings, accrual.Warnings...)
	return eodStepRan(map[string]any{
		"accrued":       accrual.Accrued,
		"total_accrued": accrual.TotalAccrued.String(),
	})
}

// runLossAmortizationStep memulihkan saldo kerugian restrukturisasi SETELAH akrual
// kontraktual pada tanggal bisnis yang sama. Bila akrual gagal, amortisasi MENOLAK
// berjalan: selisih efektifnya akan berdiri di atas pendapatan yang belum diakui.
func (s *batchProcessService) runLossAmortizationStep(ctx context.Context, def domain.EODStepDefinition, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.accrualSvc == nil {
		return eodStepSkipped("layanan akrual bunga kredit tidak dikonfigurasi")
	}
	if out, skip := eodPrerequisiteSkip(summary, "amortisasi saldo kerugian restrukturisasi", def.Prerequisites...); skip {
		return out
	}
	loss, err := s.accrualSvc.AmortizeRestructureLoss(ctx, businessDate, actor)
	summary.LoanLossAmortized = loss.Amortized
	summary.LoanLossAmortizedAmount = loss.TotalAmortized
	summary.Warnings = append(summary.Warnings, loss.Warnings...)
	if err != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("amortisasi saldo kerugian restrukturisasi gagal: %v", err))
		logger.ErrorContext(ctx, "amortisasi saldo kerugian restrukturisasi gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	}
	return eodStepRan(map[string]any{
		"amortized":       loss.Amortized,
		"total_amortized": loss.TotalAmortized.String(),
	})
}

// runPPAPStep menghitung kolektibilitas dan cadangan PPAP. PPKA dihitung atas nilai
// tercatat SETELAH amortisasi periode berjalan, karena itu prasyaratnya adalah
// amortisasi.
func (s *batchProcessService) runPPAPStep(ctx context.Context, def domain.EODStepDefinition, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.ppapSvc == nil {
		return eodStepSkipped("layanan PPAP tidak dikonfigurasi")
	}
	if out, skip := eodPrerequisiteSkip(summary, "perhitungan PPAP harian", def.Prerequisites...); skip {
		return out
	}
	ppap, err := s.ppapSvc.RunDaily(ctx, businessDate, actor)
	summary.PPAPProcessed = ppap.Processed
	summary.PPAPAdjusted = ppap.Adjusted
	if err != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("perhitungan PPAP harian gagal: %v", err))
		logger.ErrorContext(ctx, "perhitungan PPAP harian gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	}
	return eodStepRan(map[string]any{"processed": ppap.Processed, "adjusted": ppap.Adjusted})
}

// runCKPNStep membandingkan CKPN dengan required_ppap hasil PPAP pada tanggal bisnis
// yang sama. Bila PPAP tidak RAN, perbandingan menolak berjalan agar tidak melaporkan
// dasar run sebelumnya seolah sah.
func (s *batchProcessService) runCKPNStep(ctx context.Context, def domain.EODStepDefinition, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.ckpnSvc == nil {
		return eodStepSkipped("layanan CKPN tidak dikonfigurasi")
	}
	if out, skip := eodPrerequisiteSkip(summary, "perbandingan CKPN vs PPKA", def.Prerequisites...); skip {
		return out
	}
	cmp, err := s.ckpnSvc.Compare(ctx, businessDate, actor)
	switch {
	case err != nil:
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("perbandingan CKPN vs PPKA gagal: %v", err))
		logger.ErrorContext(ctx, "perbandingan CKPN vs PPKA gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	case cmp.Enabled:
		// Jalur resmi (perilaku lama): bidang lama diisi persis seperti sebelumnya.
		summary.CKPNCompared = cmp.Processed
		summary.CKPNFailed = cmp.Failed
		summary.CKPNTotalPPKA = cmp.TotalPPKA
		summary.CKPNTotalCKPN = cmp.TotalCKPN
		summary.CKPNModalIntiDeduction = cmp.ModalIntiDeduction
		if cmp.Failed > 0 {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("perbandingan CKPN vs PPKA: %d kredit gagal dihitung", cmp.Failed))
		}
		return eodStepRan(map[string]any{
			"mode":                 "resmi",
			"compared":             cmp.Processed,
			"failed":               cmp.Failed,
			"total_ppka":           cmp.TotalPPKA.String(),
			"total_ckpn":           cmp.TotalCKPN.String(),
			"modal_inti_deduction": cmp.ModalIntiDeduction.String(),
		})
	case cmp.ShadowMode:
		// Saklar bayangan menyala sementara ckpn.enabled masih mati: hitung dan
		// laporkan, tetapi TIDAK menulis jurnal dan TIDAK menghitung pengurangan modal
		// inti. Langkah tetap dicatat RAN karena perhitungannya dijalankan.
		reportShadowCKPN(summary, cmp)
		return eodStepRan(map[string]any{
			"mode":        "bayangan",
			"processed":   cmp.Processed,
			"failed":      cmp.Failed,
			"total_ppka":  cmp.TotalPPKA.String(),
			"total_ckpn":  cmp.TotalCKPN.String(),
			"aset_baik":   cmp.AsetBaikCount,
			"difference":  cmp.Difference.String(),
			"higher":      string(cmp.Higher),
			"basis_note":  cmp.BasisNote,
			"shadow_note": cmp.BasisNote,
		})
	default:
		return eodStepSkipped("saklar ckpn.enabled mati")
	}
}

// runPenaltyStep mengakru denda kredit.
func (s *batchProcessService) runPenaltyStep(ctx context.Context, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.penaltySvc == nil {
		return eodStepSkipped("layanan denda kredit tidak dikonfigurasi")
	}
	penalty, err := s.penaltySvc.AccruePenalties(ctx, businessDate, actor)
	summary.LoanPenaltiesAccrued = penalty.Accrued
	summary.LoanPenaltyAmount = penalty.TotalPenalty
	summary.LoanPenaltiesCapped = penalty.Capped
	summary.LoanPenaltyCapPercent = penalty.CapPercent
	summary.LoanPenaltiesSyariahSocialFund = penalty.SyariahSocialFund
	if err != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("akrual denda kredit gagal: %v", err))
		logger.ErrorContext(ctx, "akrual denda kredit gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	}
	// Tarif yang belum diisi operator bukan kegagalan teknis, tetapi tetap harus
	// terlihat: tanpa peringatan, batch tampak sukses padahal tidak menagih apa pun.
	if penalty.Warning != "" {
		summary.Warnings = append(summary.Warnings, penalty.Warning)
	}
	return eodStepRan(map[string]any{
		"accrued":             penalty.Accrued,
		"total_penalty":       penalty.TotalPenalty.String(),
		"capped":              penalty.Capped,
		"cap_percent":         penalty.CapPercent.String(),
		"syariah_social_fund": penalty.SyariahSocialFund,
	})
}

// runDormantStep menandai rekening tidak aktif sebagai DORMANT.
func (s *batchProcessService) runDormantStep(ctx context.Context, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult, logger *slog.Logger) eodStepOutcome {
	if s.dormantSvc == nil {
		return eodStepSkipped("layanan dormant tidak dikonfigurasi")
	}
	dormant, err := s.dormantSvc.MarkDormant(ctx, businessDate, actor)
	summary.AccountsMarkedDormant = dormant.Marked
	if err != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("penandaan rekening dormant gagal: %v", err))
		logger.ErrorContext(ctx, "penandaan rekening dormant gagal saat EOD", "error", err)
		return eodStepFailed(err.Error())
	}
	// Ambang yang tidak valid bukan kegagalan teknis, tetapi tetap harus terlihat:
	// tanpa peringatan, batch tampak memakai ambang yang disetel operator.
	if dormant.Warning != "" {
		summary.Warnings = append(summary.Warnings, dormant.Warning)
	}
	return eodStepRan(map[string]any{"marked": dormant.Marked})
}

// reportShadowCKPN mengisi bidang-bidang ADITIF mode bayangan CKPN pada ringkasan
// tutup hari. Ia TIDAK menyentuh bidang resmi (CKPNCompared/CKPNTotal*/
// CKPNModalIntiDeduction), TIDAK menulis jurnal, dan TIDAK mengubah state apa pun:
// hanya melaporkan hasil perhitungan baca-saja dari ckpnService.Compare.
//
// Pelabelan tegas: angka ini BAYANGAN — belum menjadi kewajiban akuntansi, perlu
// persetujuan bank, dan pengurangan modal inti belum dilakukan. Pengurang modal inti
// dihitung PER KREDIT, jadi potensinya adalah Σ max(PPKA_i - CKPN_i, 0) yang diteruskan
// dari cmp.ModalIntiDeduction (CKPNShadowModalIntiDeduction). CKPNShadowDifference
// adalah selisih AGREGAT total PPKA - total CKPN yang pada portofolio campuran bisa
// 0/negatif walau ada kelebihan per kredit; ia hanya menamai pihak yang lebih tinggi,
// BUKAN dasar pengurang modal. Bila parameter PD/LGD belum lengkap, CKPN tidak dapat
// dihitung: yang dilaporkan adalah daftar kunci yang harus diisi, bukan angka karangan.
func reportShadowCKPN(summary *domain.EODSummaryResult, cmp domain.CKPNComparisonSummary) {
	summary.CKPNShadowMode = true
	summary.CKPNShadowProcessed = cmp.Processed
	summary.CKPNShadowFailed = cmp.Failed
	summary.CKPNShadowTotalPPKA = cmp.TotalPPKA
	summary.CKPNShadowTotalCKPN = cmp.TotalCKPN
	summary.CKPNShadowDifference = cmp.Difference
	summary.CKPNShadowModalIntiDeduction = cmp.ModalIntiDeduction
	summary.CKPNShadowHigher = string(cmp.Higher)
	summary.CKPNShadowAssumptions = cmp.Assumptions
	summary.CKPNShadowAsetBaik = cmp.AsetBaikCount
	summary.CKPNShadowAsetBaikOutstanding = cmp.AsetBaikOutstanding
	summary.CKPNShadowParameterGaps = cmp.ParameterGaps

	// Basis KEDUA "setara PPKA": aset baik tetap dinilai model yang sama agar
	// sebanding dengan PPKA atas seluruh kredit. Bidang CKPNShadow* di atas tetap
	// basis pertama "sesuai kebijakan" (aset baik dikecualikan); tidak ada yang diubah.
	summary.CKPNShadowSetaraPPKAProcessed = cmp.SetaraPPKAProcessed
	summary.CKPNShadowSetaraPPKAFailed = cmp.SetaraPPKAFailed
	summary.CKPNShadowSetaraPPKATotalPPKA = cmp.SetaraPPKATotalPPKA
	summary.CKPNShadowSetaraPPKATotalCKPN = cmp.SetaraPPKATotalCKPN
	summary.CKPNShadowSetaraPPKADifference = cmp.SetaraPPKADifference
	summary.CKPNShadowSetaraPPKAModalIntiDeduction = cmp.SetaraPPKAModalIntiDeduction
	summary.CKPNShadowSetaraPPKAHigher = string(cmp.SetaraPPKAHigher)
	summary.CKPNShadowBasisNote = cmp.BasisNote

	note := "MODE BAYANGAN CKPN: angka ini BUKAN kewajiban akuntansi dan belum disetujui bank; tidak ada jurnal yang ditulis, tidak ada state yang diubah, dan pengurangan modal inti BELUM dilakukan. Potensi pengurang modal inti (butir 1.1.6) adalah jumlah selisih positif PPKA-CKPN PER KREDIT (ckpn_shadow_modal_inti_deduction); kredit dengan CKPN lebih besar tidak mengurangi kelebihan kredit lain. ckpn_shadow_difference adalah selisih AGREGAT dan hanya menamai pihak yang lebih tinggi, bukan dasar pengurang modal."
	parts := []string{note}
	if cmp.BasisNote != "" {
		parts = append(parts, cmp.BasisNote)
	}
	if len(cmp.ParameterGaps) > 0 {
		gaps := strings.Join(cmp.ParameterGaps, ", ")
		parts = append(parts, fmt.Sprintf("CKPN belum dapat dihitung sepenuhnya; parameter berikut harus diisi/diperbaiki bank (satuan FRAKSI 0..1, bukan persen): %s.", gaps))
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("CKPN mode bayangan belum dapat dihitung sepenuhnya: isi/perbaiki %s", gaps))
	}
	// Penjelasan TotalCKPN nol: bila kredit yang diproses seluruhnya dikecualikan
	// sebagai aset baik (butir 12.3.a.2.a), nol adalah HASIL perhitungan, bukan tanda
	// model belum dijalankan. Tanpa penjelasan ini laporan menyesatkan pembaca.
	if cmp.AsetBaikCount > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d dari %d kredit (sisa pokok %s) dikecualikan sebagai aset baik: PD/LGD tidak dipakai dan CKPN-nya nol, jadi nol itu hasil perhitungan, bukan tanda model belum dijalankan.",
			cmp.AsetBaikCount, cmp.Processed, cmp.AsetBaikOutstanding))
		if cmp.TotalCKPN.IsZero() && cmp.Processed > 0 && cmp.AsetBaikCount == cmp.Processed {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf(
				"CKPN mode bayangan: seluruh %d kredit dikecualikan sebagai aset baik sehingga total CKPN nol; bukan karena model belum dijalankan", cmp.AsetBaikCount))
		}
	}
	summary.CKPNShadowNote = strings.Join(parts, " ")
	if cmp.Failed > 0 {
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("CKPN mode bayangan: %d kredit gagal dihitung", cmp.Failed))
	}
	// Basis setara PPKA dapat gagal pada kredit yang justru berhasil di basis pertama
	// (aset baik tanpa PD/LGD). Tanpa peringatan, total basis kedua yang lebih kecil
	// terbaca sebagai perbandingan utuh padahal cakupannya kurang.
	if cmp.SetaraPPKAFailed > 0 {
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("CKPN mode bayangan basis setara PPKA: %d kredit belum dapat dihitung (PD/LGD belum lengkap)", cmp.SetaraPPKAFailed))
	}
}

// recordEODStep menambahkan status satu langkah ke ringkasan tutup hari.
func recordEODStep(summary *domain.EODSummaryResult, name string, status domain.EODStepStatus, reason string, prerequisites ...string) {
	summary.Steps = append(summary.Steps, domain.EODStepResult{
		Name:          name,
		Status:        status,
		Prerequisites: prerequisites,
		Reason:        reason,
	})
}

// unmetPrerequisite mengembalikan alasan bila salah satu prasyarat tidak berstatus
// RAN pada jalur EOD ini; kosong berarti seluruh prasyarat terpenuhi. Status prasyarat
// disertakan agar operator dapat membedakan "gagal" dari "tidak dijalankan", bukan
// menerima pesan umum yang menyembunyikan sebabnya.
func unmetPrerequisite(summary *domain.EODSummaryResult, names ...string) string {
	for _, name := range names {
		status := domain.EODStepSkipped
		found := false
		for _, step := range summary.Steps {
			if step.Name == name {
				status = step.Status
				found = true
				break
			}
		}
		if found && status == domain.EODStepRan {
			continue
		}
		if !found {
			status = "TIDAK_JALAN"
		}
		return fmt.Sprintf("prasyarat %s berstatus %s, bukan %s; angka akan berasal dari run sebelumnya",
			name, status, domain.EODStepRan)
	}
	return ""
}

// eomPeriod menentukan periode bulan yang ditutup oleh EOM, sekaligus menolak
// menjalankannya di luar dua keadaan yang sah.
//
// Tutup buku bulanan menutup satu bulan penuh dan membayarkan bunga/bagi hasil bulan
// itu ke rekening nasabah. Dijalankan pada hari terakhir bulan tersebut, atau pada hari
// pertama bulan berikutnya bila tutup hari terakhir sudah dijalankan lebih dulu (tanggal
// bisnis sudah maju satu hari). Di luar keduanya, akrual memakai bulan yang belum
// selesai: bank membayarkan bunga untuk periode yang belum dijalani nasabah.
func eomPeriod(businessDate time.Time) (time.Time, error) {
	first := time.Date(businessDate.Year(), businessDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)

	switch {
	case sameBusinessDay(businessDate, last):
		return first, nil
	case sameBusinessDay(businessDate, first):
		return first.AddDate(0, -1, 0), nil
	default:
		return time.Time{}, fmt.Errorf(
			"EOM hanya dapat dijalankan pada hari terakhir bulan yang ditutup atau pada hari pertama bulan berikutnya; tanggal bisnis sekarang %s",
			businessDate.Format("2006-01-02"))
	}
}

// sameBusinessDay membandingkan tanggal saja, tanpa jam dan zona.
func sameBusinessDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// RunEOM menjalankan akrual bunga/bagi hasil tabungan periode berjalan dan memotong
// biaya administrasi bulanan. Tidak ada tarif yang diterima dari pemanggil: tarif
// dibaca dari produk dan system_config. Ringkasan diisi dari hasil nyata tiap rekening.
func (s *batchProcessService) RunEOM(ctx context.Context, executedBy uuid.UUID) (*domain.EOMSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}
	if s.savingsSvc == nil {
		return nil, errors.New("layanan akrual tabungan belum dikonfigurasi")
	}

	period, err := eomPeriod(curDate.CurrentDate)
	if err != nil {
		return nil, err
	}
	createdBy := executedBy.String()

	// Akrual tabungan dibatasi ke buku aktif instalasi; DUAL (Book kosong) tetap
	// memproses kedua buku persis seperti sebelumnya.
	eomActor := s.systemBatchActor(ctx, executedBy)
	interest, err := s.savingsSvc.AccrueAll(ctx, period, eomActor.Book, createdBy)
	if err != nil {
		return nil, fmt.Errorf("akrual bunga tabungan: %w", err)
	}

	// Akrual disimpan sebagai utang; pembayaran memindahkannya ke rekening nasabah.
	payment, err := s.savingsSvc.PayInterestToAccounts(ctx, period, createdBy)
	if err != nil {
		return nil, fmt.Errorf("pembayaran bunga tabungan: %w", err)
	}

	fees, err := s.savingsSvc.ChargeAdminFees(ctx, period, createdBy)
	if err != nil {
		return nil, fmt.Errorf("pemotongan biaya administrasi: %w", err)
	}

	processed := interest.ProcessedAccounts
	if fees.ProcessedAccounts > processed {
		processed = fees.ProcessedAccounts
	}

	return &domain.EOMSummaryResult{
		ExecutedMonth:          period.Format("2006-01"),
		TotalAdminFeesDeducted: fees.TotalAdminFees,
		TotalInterestPaid:      payment.TotalInterest,
		ProcessedAccounts:      processed,
		FailedAccounts:         interest.FailedAccounts + payment.FailedAccounts + fees.FailedAccounts,
		CompletedAt:            time.Now().UTC(),
	}, nil
}

// RunEOY memposting jurnal penutup tahun buku: saldo akun pendapatan dan beban
// dipindahkan ke laba ditahan. book kosong berarti seluruh buku diproses, tetapi
// setiap buku ditutup terpisah agar konvensional dan syariah tidak tercampur.
//
// Parameter book dari body DIBATASI pada buku aktor: permintaan buku lain ditolak
// tegas, dan aktor satu buku yang mengirim book kosong dipaksa ke bukunya sendiri
// (ConstrainedBook) sehingga tidak dapat menutup kedua buku. Aktor lintas buku
// (SYSTEM/SUPERADMIN/AUDITOR) tetap dapat menutup buku mana pun atau keduanya.
func (s *batchProcessService) RunEOY(ctx context.Context, book string, actor domain.Actor) (*domain.EOYSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}
	if s.yearEndRepo == nil || s.posting == nil || s.db == nil {
		return nil, errors.New("dependensi tutup buku belum dikonfigurasi")
	}

	requested := strings.ToUpper(strings.TrimSpace(book))
	if requested != "" && !actor.CanAccessBook(domain.COABook(requested)) {
		return nil, domain.ErrCrossBookAccess
	}
	books, err := resolveClosingBooks(actor.ConstrainedBook(requested))
	if err != nil {
		return nil, err
	}

	fiscalYear := curDate.CurrentDate.Year()
	createdBy := actor.UserID.String()

	results := make([]domain.EOYBookResult, 0, len(books))
	var refs []string
	totalRevenue := decimal.Zero
	totalExpense := decimal.Zero
	netIncome := decimal.Zero

	for _, b := range books {
		result, err := s.closeBook(ctx, fiscalYear, b, createdBy)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
		totalRevenue = totalRevenue.Add(result.TotalRevenueClosed)
		totalExpense = totalExpense.Add(result.TotalExpenseClosed)
		netIncome = netIncome.Add(result.NetRetainedEarnings)
		if result.ClosingJournalRef != "" {
			refs = append(refs, result.ClosingJournalRef)
		}
	}

	return &domain.EOYSummaryResult{
		FiscalYear:          fiscalYear,
		TotalRevenueClosed:  totalRevenue,
		TotalExpenseClosed:  totalExpense,
		NetRetainedEarnings: netIncome,
		ClosingJournalRef:   strings.Join(refs, ", "),
		Books:               results,
		CompletedAt:         time.Now().UTC(),
	}, nil
}

// closeBook menutup satu buku: hitung saldo nominal dari jurnal, bentuk jurnal
// penutup, lalu posting sekaligus dengan penanda idempotensi. Jurnal yang tidak
// seimbang ditolak sebelum menyentuh database.
func (s *batchProcessService) closeBook(ctx context.Context, fiscalYear int, book domain.COABook, createdBy string) (*domain.EOYBookResult, error) {
	existing, err := s.yearEndRepo.GetYearEndClosing(ctx, fiscalYear, book)
	if err != nil {
		return nil, fmt.Errorf("membaca penanda tutup buku: %w", err)
	}
	if existing != nil {
		return existingClosingResult(existing), nil
	}

	start := time.Date(fiscalYear, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(fiscalYear, 12, 31, 0, 0, 0, 0, time.UTC)

	balances, err := s.yearEndRepo.NominalBalances(ctx, start, end, book)
	if err != nil {
		return nil, fmt.Errorf("membaca saldo akun nominal: %w", err)
	}

	retainedCOA := s.retainedEarningsCOA(ctx, book)
	entries, totalRevenue, totalExpense, netIncome, err := domain.ComputeClosingEntries(balances, retainedCOA)
	if err != nil {
		if errors.Is(err, domain.ErrNoNominalAccounts) {
			// Tidak ada yang perlu ditutup; tidak ada jurnal dan tidak ada penanda.
			return &domain.EOYBookResult{Book: book, RetainedEarningsCOACode: retainedCOA}, nil
		}
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	inserted, err := s.yearEndRepo.InsertYearEndClosing(ctx, tx, &domain.YearEndClosingRecord{
		FiscalYear:          fiscalYear,
		Book:                book,
		TotalRevenue:        totalRevenue,
		TotalExpense:        totalExpense,
		NetIncome:           netIncome,
		RetainedEarningsCOA: retainedCOA,
	})
	if err != nil {
		return nil, fmt.Errorf("menyimpan penanda tutup buku: %w", err)
	}
	if !inserted {
		// Balapan dengan eksekusi lain: pakai penanda yang sudah ada.
		tx.Rollback()
		existing, err := s.yearEndRepo.GetYearEndClosing(ctx, fiscalYear, book)
		if err != nil || existing == nil {
			return nil, fmt.Errorf("tutup buku %d/%s sudah berjalan tetapi penanda tidak terbaca", fiscalYear, book)
		}
		return existingClosingResult(existing), nil
	}

	lines := make([]domain.PostingLine, 0, len(entries))
	for _, e := range entries {
		accountNumber, err := s.resolver.ResolveGLAccount(ctx, tx, e.COACode)
		if err != nil {
			return nil, fmt.Errorf("akun GL %s: %w", e.COACode, err)
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: accountNumber,
			Direction:     e.Direction,
			Amount:        e.Amount,
			Description:   fmt.Sprintf("Penutupan tahun buku %d", fiscalYear),
		})
	}

	entry, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("Jurnal penutup tahun buku %d (%s)", fiscalYear, book),
		IdempotencyKey:  fmt.Sprintf("EOY-CLOSE-%d-%s", fiscalYear, book),
		CreatedBy:       createdBy,
		// Jurnal penutup harus masuk ke tahun fiskal yang ditutup, bukan tahun
		// saat batch dijalankan (bisa sudah lewat 31 Desember).
		EntryDate: time.Date(fiscalYear, 12, 31, 0, 0, 0, 0, time.UTC),
		Lines:     lines,
	})
	if err != nil {
		return nil, fmt.Errorf("posting jurnal penutup: %w", err)
	}

	if err := s.yearEndRepo.UpdateYearEndClosingJournal(ctx, tx, fiscalYear, book, entry.ID); err != nil {
		return nil, fmt.Errorf("menautkan jurnal penutup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &domain.EOYBookResult{
		Book:                    book,
		TotalRevenueClosed:      totalRevenue,
		TotalExpenseClosed:      totalExpense,
		NetRetainedEarnings:     netIncome,
		RetainedEarningsCOACode: retainedCOA,
		ClosingJournalRef:       entry.ReferenceNumber,
	}, nil
}

// retainedEarningsCOA menentukan akun laba ditahan per buku, dengan override konfigurasi.
func (s *batchProcessService) retainedEarningsCOA(ctx context.Context, book domain.COABook) string {
	fallback := defaultRetainedEarningsCOAConv
	key := configRetainedEarningsCOAConv
	if book == domain.BookSyariah {
		fallback = defaultRetainedEarningsCOASyar
		key = configRetainedEarningsCOASyar
	}
	if s.configSvc == nil {
		return fallback
	}
	return s.configSvc.GetString(ctx, key, fallback)
}

// systemBatchActor membangun aktor SYSTEM untuk pekerjaan EOD dengan cakupan buku
// instalasi terisi. Tanpa ini aktor batch selalu lintas buku, sehingga instalasi
// satu buku tetap memproses lini usaha yang tidak dilayaninya lewat filter repository.
func (s *batchProcessService) systemBatchActor(ctx context.Context, executedBy uuid.UUID) domain.Actor {
	actor := domain.SystemActor(executedBy)
	if s.configSvc == nil {
		return actor
	}
	actor.BookScope = domain.ParseInstitutionBookScope(
		s.configSvc.GetString(ctx, domain.ConfigKeyInstitutionBookScope, string(domain.ScopeDual)))
	if book := actor.BookScope.SingleBook(); book != "" {
		actor.Book = book
	}
	return actor
}

// resolveClosingBooks menerjemahkan parameter buku menjadi daftar buku yang ditutup.
// Kosong berarti kedua buku; nilai lain harus CONVENTIONAL atau SYARIAH.
func resolveClosingBooks(book string) ([]domain.COABook, error) {
	switch strings.ToUpper(strings.TrimSpace(book)) {
	case "":
		return []domain.COABook{domain.BookConventional, domain.BookSyariah}, nil
	case string(domain.BookConventional):
		return []domain.COABook{domain.BookConventional}, nil
	case string(domain.BookSyariah):
		return []domain.COABook{domain.BookSyariah}, nil
	default:
		return nil, fmt.Errorf("buku %q tidak dikenal; gunakan CONVENTIONAL atau SYARIAH", book)
	}
}

// existingClosingResult mengubah penanda tutup buku yang sudah ada menjadi ringkasan.
func existingClosingResult(rec *domain.YearEndClosingRecord) *domain.EOYBookResult {
	result := &domain.EOYBookResult{
		Book:                    rec.Book,
		TotalRevenueClosed:      rec.TotalRevenue,
		TotalExpenseClosed:      rec.TotalExpense,
		NetRetainedEarnings:     rec.NetIncome,
		RetainedEarningsCOACode: rec.RetainedEarningsCOA,
		AlreadyClosed:           true,
		ClosingJournalRef:       rec.JournalReference,
	}
	return result
}

var _ domain.BatchProcessService = (*batchProcessService)(nil)
