package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	}
}

func (s *batchProcessService) GetCurrentBusinessDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	return s.dateRepo.GetCurrentDate(ctx)
}

func (s *batchProcessService) RunEOD(ctx context.Context, executedBy uuid.UUID) (*domain.EODSummaryResult, error) {
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
	// sebelum tanggal dimajukan, agar jurnalnya masuk ke tanggal yang benar.
	summary := &domain.EODSummaryResult{ExecutedDate: curDate.CurrentDate}
	s.runDailyJobs(ctx, curDate.CurrentDate, domain.SystemActor(executedBy), summary)

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
	summary.TotalDepositPlacementsToday = activity.DepositPlacementCount
	summary.TotalDepositPlacementAmount = activity.TotalDepositPlacementAmount
	summary.TotalWithdrawalAmountToday = activity.TotalWithdrawalAmount
	summary.ExecutedBy = executedBy
	summary.CompletedAt = time.Now().UTC()
	return summary, nil
}

// runDailyJobs menjalankan pekerjaan harian yang menyertai tutup hari: perpanjangan
// otomatis deposito, akrual bunga kredit, amortisasi saldo kerugian restrukturisasi,
// perhitungan PPAP, perbandingan CKPN, akrual denda kredit, dan penandaan rekening
// dormant. Setiap pekerjaan terisolasi — kegagalannya hanya menghasilkan peringatan
// dan tidak menghentikan pekerjaan berikutnya maupun tutup hari, karena tutup hari
// yang gagal akan menghentikan seluruh operasional bank. Layanan yang belum
// dikonfigurasi (nil) dilewati tanpa peringatan.
//
// Disiplin antar-langkah: langkah yang bergantung pada hasil langkah lain menolak
// berjalan bila prasyaratnya tidak berstatus RAN pada jalur ini (jadi bisa gagal,
// tidak dikonfigurasi, atau tidak berjalan pada tanggal bisnis yang sama). Penolakan
// itu dicatat di EODSummaryResult.Steps dan dinaikkan menjadi Warnings, sehingga run
// tidak pernah tampak sah dengan angka dari run sebelumnya. Urutan mengikuti
// ketergantungan data yang sudah ada; jalur perhitungan tiap langkah tidak diubah.
//
// Ketergantungannya berantai dan berurutan: akrual bunga → amortisasi saldo kerugian
// → PPAP → perbandingan CKPN. Amortisasi berjalan sebelum PPAP agar PPKA dihitung
// atas nilai tercatat setelah amortisasi periode berjalan; bila amortisasi gagal,
// PPAP dilewati (lihat komentar pada masing-masing langkah).
func (s *batchProcessService) runDailyJobs(ctx context.Context, businessDate time.Time, actor domain.Actor, summary *domain.EODSummaryResult) {
	logger := observability.FromContext(ctx)

	if s.aroSvc == nil {
		recordEODStep(summary, eodStepARO, domain.EODStepSkipped, "layanan perpanjangan otomatis deposito tidak dikonfigurasi")
	} else {
		rolled, err := s.aroSvc.RunARO(ctx, businessDate, actor)
		summary.DepositsRolledOver = rolled
		if err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("perpanjangan otomatis deposito gagal: %v", err))
			logger.ErrorContext(ctx, "perpanjangan otomatis deposito gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepARO, domain.EODStepFailed, err.Error())
		} else {
			recordEODStep(summary, eodStepARO, domain.EODStepRan, "")
		}
	}

	// Akrual bunga kredit berjalan lebih dulu karena amortisasi saldo kerugian
	// restrukturisasi berdiri di atas pendapatan jadwal periode itu.
	if s.accrualSvc == nil {
		recordEODStep(summary, eodStepInterestAccrual, domain.EODStepSkipped, "layanan akrual bunga kredit tidak dikonfigurasi")
	} else {
		accrual, err := s.accrualSvc.AccrueInterest(ctx, businessDate, actor)
		summary.LoanInterestAccrued = accrual.Accrued
		summary.LoanInterestAccruedAmount = accrual.TotalAccrued
		if err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("akrual bunga kredit gagal: %v", err))
			logger.ErrorContext(ctx, "akrual bunga kredit gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepInterestAccrual, domain.EODStepFailed, err.Error())
		} else {
			recordEODStep(summary, eodStepInterestAccrual, domain.EODStepRan, "")
		}
		// Produk tanpa pemetaan INTEREST_ACCRUAL bukan kegagalan teknis, tetapi harus
		// terlihat: tanpa peringatan, batch tampak mengakru padahal tidak.
		summary.Warnings = append(summary.Warnings, accrual.Warnings...)
	}

	// Amortisasi saldo kerugian restrukturisasi berjalan SETELAH akrual kontraktual
	// pada tanggal bisnis yang sama, sehingga selisih bunga efektif dihitung di atas
	// pendapatan jadwal yang sudah diakui periode itu. Bila akrual gagal, amortisasi
	// MENOLAK berjalan: selisih efektifnya akan berdiri di atas pendapatan yang belum
	// diakui dan angkanya menyesatkan. Seluruh perhitungannya di balik
	// loan.restructure.loss.enabled.
	//
	// Urutan ini juga menempatkan amortisasi SEBELUM PPAP: nilai tercatat yang dipakai
	// PPKA harus sudah dikurangi amortisasi periode berjalan (Pasal 32 POJK 1/2024 jo.
	// PA BPR Bab 5.2). Bila PPAP berjalan lebih dulu, ia memakai saldo kerugian bruto
	// sehingga required_ppap terlalu besar untuk periode itu.
	if s.accrualSvc == nil {
		recordEODStep(summary, eodStepLossAmortization, domain.EODStepSkipped, "layanan akrual bunga kredit tidak dikonfigurasi", eodStepInterestAccrual)
	} else if reason := unmetPrerequisite(summary, eodStepInterestAccrual); reason != "" {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("amortisasi saldo kerugian restrukturisasi dilewati: %s", reason))
		recordEODStep(summary, eodStepLossAmortization, domain.EODStepSkipped, reason, eodStepInterestAccrual)
	} else {
		loss, err := s.accrualSvc.AmortizeRestructureLoss(ctx, businessDate, actor)
		summary.LoanLossAmortized = loss.Amortized
		summary.LoanLossAmortizedAmount = loss.TotalAmortized
		if err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("amortisasi saldo kerugian restrukturisasi gagal: %v", err))
			logger.ErrorContext(ctx, "amortisasi saldo kerugian restrukturisasi gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepLossAmortization, domain.EODStepFailed, err.Error(), eodStepInterestAccrual)
		} else {
			recordEODStep(summary, eodStepLossAmortization, domain.EODStepRan, "", eodStepInterestAccrual)
		}
		summary.Warnings = append(summary.Warnings, loss.Warnings...)
	}

	// PPAP menyimpan required_ppap dan kolektibilitas. Langkah inilah yang menghasilkan
	// angka dasar bagi perbandingan CKPN, jadi statusnya menentukan boleh tidaknya CKPN.
	//
	// Prasyaratnya adalah amortisasi saldo kerugian: PPKA dihitung atas nilai tercatat
	// SETELAH amortisasi periode berjalan. Bila amortisasi tidak berstatus RAN — gagal,
	// tidak dikonfigurasi, atau terlewat karena akrual gagal — PPAP MENOLAK berjalan
	// agar tidak menghasilkan cadangan di atas saldo kerugian yang belum dimutakhirkan.
	if s.ppapSvc == nil {
		recordEODStep(summary, eodStepPPAP, domain.EODStepSkipped, "layanan PPAP tidak dikonfigurasi")
	} else if reason := unmetPrerequisite(summary, eodStepLossAmortization); reason != "" {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("perhitungan PPAP harian dilewati: %s", reason))
		recordEODStep(summary, eodStepPPAP, domain.EODStepSkipped, reason, eodStepLossAmortization)
	} else {
		ppap, err := s.ppapSvc.RunDaily(ctx, businessDate, actor)
		summary.PPAPProcessed = ppap.Processed
		summary.PPAPAdjusted = ppap.Adjusted
		if err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("perhitungan PPAP harian gagal: %v", err))
			logger.ErrorContext(ctx, "perhitungan PPAP harian gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepPPAP, domain.EODStepFailed, err.Error(), eodStepLossAmortization)
		} else {
			recordEODStep(summary, eodStepPPAP, domain.EODStepRan, "", eodStepLossAmortization)
		}
	}

	// Perbandingan CKPN bersandar pada required_ppap yang baru saja disimpan PPAP pada
	// tanggal bisnis yang sama. Bila PPAP gagal/tidak berjalan, kredit masih menyimpan
	// required_ppap run sebelumnya; menampilkan perbandingannya berarti melaporkan
	// dasar kemarin seolah sah. Karena PPAP kini berjalan setelah amortisasi, basis
	// saldo kerugian CKPN dan PPKA sama-sama sudah dimutakhirkan periode ini.
	if s.ckpnSvc == nil {
		recordEODStep(summary, eodStepCKPN, domain.EODStepSkipped, "layanan CKPN tidak dikonfigurasi", eodStepPPAP)
	} else if reason := unmetPrerequisite(summary, eodStepPPAP); reason != "" {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("perbandingan CKPN vs PPKA dilewati: %s", reason))
		recordEODStep(summary, eodStepCKPN, domain.EODStepSkipped, reason, eodStepPPAP)
	} else {
		cmp, err := s.ckpnSvc.Compare(ctx, businessDate, actor)
		switch {
		case err != nil:
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("perbandingan CKPN vs PPKA gagal: %v", err))
			logger.ErrorContext(ctx, "perbandingan CKPN vs PPKA gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepCKPN, domain.EODStepFailed, err.Error(), eodStepPPAP)
		case cmp.Enabled:
			// Jalur resmi (perilaku lama): bidang lama diisi persis seperti sebelumnya.
			// ShadowMode dari service hanya true bila bayangan benar-benar dipakai, jadi
			// kombinasi kedua saklar menyala tidak lagi menghasilkan peringatan di sini:
			// service yang memutuskan mode mana yang berlaku.
			summary.CKPNCompared = cmp.Processed
			summary.CKPNFailed = cmp.Failed
			summary.CKPNTotalPPKA = cmp.TotalPPKA
			summary.CKPNTotalCKPN = cmp.TotalCKPN
			summary.CKPNModalIntiDeduction = cmp.ModalIntiDeduction
			if cmp.Failed > 0 {
				summary.Warnings = append(summary.Warnings, fmt.Sprintf("perbandingan CKPN vs PPKA: %d kredit gagal dihitung", cmp.Failed))
			}
			recordEODStep(summary, eodStepCKPN, domain.EODStepRan, "", eodStepPPAP)
		case cmp.ShadowMode:
			// Saklar bayangan menyala sementara ckpn.enabled masih mati: hitung dan
			// laporkan, tetapi TIDAK menulis jurnal dan TIDAK menghitung pengurangan
			// modal inti. Langkah tetap dicatat RAN karena perhitungannya dijalankan.
			reportShadowCKPN(summary, cmp)
			recordEODStep(summary, eodStepCKPN, domain.EODStepRan, "", eodStepPPAP)
		default:
			recordEODStep(summary, eodStepCKPN, domain.EODStepSkipped, "saklar ckpn.enabled mati", eodStepPPAP)
		}
	}

	if s.penaltySvc == nil {
		recordEODStep(summary, eodStepPenalty, domain.EODStepSkipped, "layanan denda kredit tidak dikonfigurasi")
	} else {
		penalty, err := s.penaltySvc.AccruePenalties(ctx, businessDate, actor)
		summary.LoanPenaltiesAccrued = penalty.Accrued
		summary.LoanPenaltyAmount = penalty.TotalPenalty
		if err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("akrual denda kredit gagal: %v", err))
			logger.ErrorContext(ctx, "akrual denda kredit gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepPenalty, domain.EODStepFailed, err.Error())
		} else {
			recordEODStep(summary, eodStepPenalty, domain.EODStepRan, "")
		}
		// Tarif yang belum diisi operator bukan kegagalan teknis, tetapi tetap harus
		// terlihat: tanpa peringatan, batch tampak sukses padahal tidak menagih apa pun.
		if penalty.Warning != "" {
			summary.Warnings = append(summary.Warnings, penalty.Warning)
		}
	}

	if s.dormantSvc == nil {
		recordEODStep(summary, eodStepDormant, domain.EODStepSkipped, "layanan dormant tidak dikonfigurasi")
	} else {
		dormant, err := s.dormantSvc.MarkDormant(ctx, businessDate, actor)
		summary.AccountsMarkedDormant = dormant.Marked
		if err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("penandaan rekening dormant gagal: %v", err))
			logger.ErrorContext(ctx, "penandaan rekening dormant gagal saat EOD", "error", err)
			recordEODStep(summary, eodStepDormant, domain.EODStepFailed, err.Error())
		} else {
			recordEODStep(summary, eodStepDormant, domain.EODStepRan, "")
		}
		// Ambang yang tidak valid bukan kegagalan teknis, tetapi tetap harus terlihat:
		// tanpa peringatan, batch tampak memakai ambang yang disetel operator.
		if dormant.Warning != "" {
			summary.Warnings = append(summary.Warnings, dormant.Warning)
		}
	}
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

	note := "MODE BAYANGAN CKPN: angka ini BUKAN kewajiban akuntansi dan belum disetujui bank; tidak ada jurnal yang ditulis, tidak ada state yang diubah, dan pengurangan modal inti BELUM dilakukan. Potensi pengurang modal inti (butir 1.1.6) adalah jumlah selisih positif PPKA-CKPN PER KREDIT (ckpn_shadow_modal_inti_deduction); kredit dengan CKPN lebih besar tidak mengurangi kelebihan kredit lain. ckpn_shadow_difference adalah selisih AGREGAT dan hanya menamai pihak yang lebih tinggi, bukan dasar pengurang modal."
	if len(cmp.ParameterGaps) > 0 {
		gaps := strings.Join(cmp.ParameterGaps, ", ")
		summary.CKPNShadowNote = fmt.Sprintf("%s CKPN belum dapat dihitung sepenuhnya; parameter berikut harus diisi/diperbaiki bank: %s.", note, gaps)
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("CKPN mode bayangan belum dapat dihitung sepenuhnya: isi/perbaiki %s", gaps))
	} else {
		summary.CKPNShadowNote = note
	}
	if cmp.Failed > 0 {
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("CKPN mode bayangan: %d kredit gagal dihitung", cmp.Failed))
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

	interest, err := s.savingsSvc.AccrueAll(ctx, period, "", createdBy)
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
func (s *batchProcessService) RunEOY(ctx context.Context, book string, executedBy uuid.UUID) (*domain.EOYSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}
	if s.yearEndRepo == nil || s.posting == nil || s.db == nil {
		return nil, errors.New("dependensi tutup buku belum dikonfigurasi")
	}

	books, err := resolveClosingBooks(book)
	if err != nil {
		return nil, err
	}

	fiscalYear := curDate.CurrentDate.Year()
	createdBy := executedBy.String()

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
