package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type stubBusinessDateRepo struct {
	currentDate time.Time
	status      domain.BusinessDateStatus
	// lockHeld meniru kunci tutup hari yang sedang dipegang proses lain.
	lockHeld bool
}

// TryEODLock meniru kunci advisory: gagal bila kunci sedang dipegang.
func (s *stubBusinessDateRepo) TryEODLock(ctx context.Context) (func() error, error) {
	if s.lockHeld {
		return nil, domain.ErrEODInProgress
	}
	s.lockHeld = true
	return func() error {
		s.lockHeld = false
		return nil
	}, nil
}

func (s *stubBusinessDateRepo) GetCurrentDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	return &domain.SystemBusinessDate{
		CurrentDate: s.currentDate,
		Status:      s.status,
		UpdatedAt:   time.Now(),
	}, nil
}

func (s *stubBusinessDateRepo) AdvanceDate(ctx context.Context, nextDate time.Time, updatedBy uuid.UUID) error {
	s.currentDate = nextDate
	s.status = domain.BusinessDateStatusOpen
	return nil
}

// ClaimEOD meniru compare-and-swap: klaim gagal bila tanggal sudah ditutup.
func (s *stubBusinessDateRepo) ClaimEOD(ctx context.Context) (bool, error) {
	if s.status == domain.BusinessDateStatusClosed {
		return false, nil
	}
	s.status = domain.BusinessDateStatusEOD
	return true, nil
}

func TestBatchProcessService_RunEOD(t *testing.T) {
	initDate := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	dateRepo := &stubBusinessDateRepo{currentDate: initDate, status: domain.BusinessDateStatusOpen}
	// Dependensi lain nil: RunEOD hanya butuh dateRepo; ringkasan tanpa batchRepo
	// dikembalikan sebagai nol (di produksi batchRepo selalu terisi). Pekerjaan
	// harian (ARO, PPAP, denda, dormant) juga nil sehingga dilewati tanpa peringatan.
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	executor := uuid.New()
	res, err := svc.RunEOD(context.Background(), executor)
	if err != nil {
		t.Fatalf("unexpected error running EOD: %v", err)
	}

	expectedNext := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	if !res.NextBusinessDate.Equal(expectedNext) {
		t.Fatalf("expected next business date 2026-09-03, got %s", res.NextBusinessDate.Format("2006-01-02"))
	}
}

// Tanggal bisnis yang sudah CLOSED tidak boleh ditutup lagi. Keputusan ini datang
// dari hasil klaim (compare-and-swap), bukan dari pembacaan status yang terpisah.
func TestRunEODRejectsClosedBusinessDate(t *testing.T) {
	initDate := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	dateRepo := &stubBusinessDateRepo{currentDate: initDate, status: domain.BusinessDateStatusClosed}
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if _, err := svc.RunEOD(context.Background(), uuid.New()); !errors.Is(err, domain.ErrEODAlreadyRunForDate) {
		t.Fatalf("tanggal tertutup harus ditolak ErrEODAlreadyRunForDate, dapat %v", err)
	}
	if dateRepo.status != domain.BusinessDateStatusClosed {
		t.Fatalf("status tidak boleh berubah, dapat %s", dateRepo.status)
	}
	if !dateRepo.currentDate.Equal(initDate) {
		t.Fatalf("tanggal bisnis tidak boleh maju, dapat %s", dateRepo.currentDate.Format("2006-01-02"))
	}
}

// Tutup hari kedua yang datang saat tutup hari lain masih berjalan harus ditolak
// sebelum pekerjaan harian apa pun dijalankan. Status saja tidak dapat membedakan
// "sedang berjalan" dari "pernah berhenti di tengah".
func TestRunEODRejectsWhenAnotherRunHoldsLock(t *testing.T) {
	initDate := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	dateRepo := &stubBusinessDateRepo{
		currentDate: initDate,
		status:      domain.BusinessDateStatusOpen,
		lockHeld:    true,
	}
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if _, err := svc.RunEOD(context.Background(), uuid.New()); !errors.Is(err, domain.ErrEODInProgress) {
		t.Fatalf("tutup hari kedua harus ditolak ErrEODInProgress, dapat %v", err)
	}
	if dateRepo.status != domain.BusinessDateStatusOpen {
		t.Fatalf("status tidak boleh berubah, dapat %s", dateRepo.status)
	}
	if !dateRepo.currentDate.Equal(initDate) {
		t.Fatalf("tanggal bisnis tidak boleh maju, dapat %s", dateRepo.currentDate.Format("2006-01-02"))
	}
}

// stubDormantRunner menggantikan penandaan dormant agar ringkasan EOD bisa diuji
// tanpa database.
type stubDormantRunner struct {
	marked  int
	warning string
	err     error
}

func (s stubDormantRunner) MarkDormant(context.Context, time.Time, domain.Actor) (domain.DormantRunSummary, error) {
	return domain.DormantRunSummary{Marked: s.marked, Warning: s.warning}, s.err
}

func TestRunEODReportsDormantMarking(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		stubDormantRunner{marked: 7, warning: "ambang memakai fallback"}, nil, nil,
	)

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.AccountsMarkedDormant != 7 {
		t.Fatalf("accounts_marked_dormant = %d, mau 7", res.AccountsMarkedDormant)
	}
	// Peringatan konfigurasi harus terlihat, bukan disamarkan sebagai sukses penuh.
	found := false
	for _, w := range res.Warnings {
		if w == "ambang memakai fallback" {
			found = true
		}
	}
	if !found {
		t.Fatalf("peringatan dormant harus masuk Warnings, dapat %v", res.Warnings)
	}
}

// stubInterestAccrualRunner menggantikan akrual bunga kredit agar ringkasan EOD bisa
// diuji tanpa database.
type stubInterestAccrualRunner struct {
	accrued  int
	amount   decimal.Decimal
	warnings []string
	err      error
	// amortized/amortizeErr mengisi hasil amortisasi; amortizeCalled dipakai uji
	// gerbang untuk membuktikan amortisasi benar-benar tidak dipanggil saat akrual gagal.
	amortized      int
	amortizeErr    error
	amortizeCalled *bool
}

func (s stubInterestAccrualRunner) AccrueInterest(context.Context, time.Time, domain.Actor) (domain.LoanInterestAccrualSummary, error) {
	return domain.LoanInterestAccrualSummary{
		Accrued:      s.accrued,
		TotalAccrued: s.amount,
		Warnings:     s.warnings,
	}, s.err
}

func (s stubInterestAccrualRunner) AmortizeRestructureLoss(context.Context, time.Time, domain.Actor) (domain.RestructureLossAmortizationSummary, error) {
	if s.amortizeCalled != nil {
		*s.amortizeCalled = true
	}
	return domain.RestructureLossAmortizationSummary{
		Amortized:      s.amortized,
		TotalAmortized: decimal.NewFromInt(int64(s.amortized)),
	}, s.amortizeErr
}

// stubPPAPRunner menggantikan langkah PPAP agar gerbang CKPN dapat diuji tanpa database.
type stubPPAPRunner struct {
	processed int
	err       error
}

func (s stubPPAPRunner) RunDaily(context.Context, time.Time, domain.Actor) (domain.PPAPRunSummary, error) {
	return domain.PPAPRunSummary{Processed: s.processed}, s.err
}

// stubCKPNService menggantikan perbandingan CKPN agar gerbang PPAP dapat diuji.
type stubCKPNService struct {
	summary domain.CKPNComparisonSummary
	err     error
	called  *bool
}

func (s stubCKPNService) Compare(context.Context, time.Time, domain.Actor) (domain.CKPNComparisonSummary, error) {
	if s.called != nil {
		*s.called = true
	}
	return s.summary, s.err
}

func (s stubCKPNService) Run(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.CKPNComparisonSummary, error) {
	return s.Compare(ctx, asOf, actor)
}

// eodStepStatus mencari status satu langkah pada ringkasan EOD.
func eodStepStatus(t *testing.T, summary *domain.EODSummaryResult, name string) domain.EODStepResult {
	t.Helper()
	for _, step := range summary.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("langkah %q tidak ada pada ringkasan EOD: %+v", name, summary.Steps)
	return domain.EODStepResult{}
}

func TestRunEODReportsLoanInterestAccrual(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		stubInterestAccrualRunner{
			accrued:  3,
			amount:   decimal.NewFromInt(450_000),
			warnings: []string{"produk X tanpa pemetaan"},
		},
		nil,
	)

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.LoanInterestAccrued != 3 {
		t.Fatalf("loan_interest_accrued = %d, mau 3", res.LoanInterestAccrued)
	}
	if !res.LoanInterestAccruedAmount.Equal(decimal.NewFromInt(450_000)) {
		t.Fatalf("loan_interest_accrued_amount = %s, mau 450.000", res.LoanInterestAccruedAmount)
	}
	found := false
	for _, w := range res.Warnings {
		if w == "produk X tanpa pemetaan" {
			found = true
		}
	}
	if !found {
		t.Fatalf("peringatan akrual harus masuk Warnings, dapat %v", res.Warnings)
	}
}

// Amortisasi saldo kerugian restrukturisasi berdiri di atas pendapatan jadwal yang
// sudah diakui akrual kontraktual. Bila akrual gagal, amortisasi harus MENOLAK
// berjalan — bukan memposting selisih efektif di atas pendapatan yang belum diakui —
// dan penolakan itu harus terlihat di ringkasan, bukan senyap.
func TestRunEODSkipsLossAmortizationWhenAccrualFails(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	amortizeCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		stubInterestAccrualRunner{
			err:            errors.New("akrual gagal di tengah"),
			amortized:      9,
			amortizeCalled: &amortizeCalled,
		},
		nil,
	)

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if amortizeCalled {
		t.Fatal("amortisasi harus menolak berjalan saat akrual gagal, tetapi ia dipanggil")
	}
	if res.LoanLossAmortized != 0 {
		t.Fatalf("loan_loss_amortized = %d, mau 0 karena dilewati", res.LoanLossAmortized)
	}
	if step := eodStepStatus(t, res, "loan_interest_accrual"); step.Status != domain.EODStepFailed {
		t.Fatalf("status akrual = %s, mau FAILED", step.Status)
	}
	step := eodStepStatus(t, res, "restructure_loss_amortization")
	if step.Status != domain.EODStepSkipped {
		t.Fatalf("status amortisasi = %s, mau SKIPPED", step.Status)
	}
	if step.Reason == "" {
		t.Fatal("langkah yang dilewati karena prasyarat gagal harus mencantumkan alasan")
	}
	warned := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "amortisasi saldo kerugian restrukturisasi dilewati") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("penolakan amortisasi harus muncul sebagai peringatan, dapat %v", res.Warnings)
	}
}

// Bila akrual berhasil, amortisasi berjalan seperti biasa.
func TestRunEODRunsLossAmortizationWhenAccrualSucceeds(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	amortizeCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		stubInterestAccrualRunner{
			accrued:        2,
			amortized:      7,
			amortizeCalled: &amortizeCalled,
		},
		nil,
	)

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if !amortizeCalled {
		t.Fatal("amortisasi harus berjalan saat prasyaratnya berhasil")
	}
	if res.LoanLossAmortized != 7 {
		t.Fatalf("loan_loss_amortized = %d, mau 7", res.LoanLossAmortized)
	}
	if step := eodStepStatus(t, res, "restructure_loss_amortization"); step.Status != domain.EODStepRan {
		t.Fatalf("status amortisasi = %s, mau RAN", step.Status)
	}
}

// Perbandingan CKPN memakai required_ppap hasil PPAP pada tanggal bisnis yang sama.
// Bila PPAP gagal, kredit masih menyimpan required_ppap run sebelumnya: perbandingan
// harus menolak berjalan agar laporan tidak tampak sah dengan dasar kemarin.
func TestRunEODSkipsCKPNComparisonWhenPPAPFails(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	ckpnCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{err: errors.New("PPAP gagal")},
		nil, nil, nil,
		stubCKPNService{called: &ckpnCalled},
	)

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if ckpnCalled {
		t.Fatal("perbandingan CKPN tidak boleh berjalan saat PPAP gagal")
	}
	if step := eodStepStatus(t, res, "ppap"); step.Status != domain.EODStepFailed {
		t.Fatalf("status PPAP = %s, mau FAILED", step.Status)
	}
	step := eodStepStatus(t, res, "ckpn_comparison")
	if step.Status != domain.EODStepSkipped {
		t.Fatalf("status CKPN = %s, mau SKIPPED", step.Status)
	}
	if step.Reason == "" {
		t.Fatal("CKPN yang dilewati harus mencantumkan alasan prasyarat")
	}
}

// Bila PPAP berhasil, perbandingan CKPN berjalan dan angkanya masuk ringkasan EOD.
func TestRunEODComparesCKPNWhenPPAPRuns(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	ckpnCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 3},
		nil, nil, nil,
		stubCKPNService{
			called: &ckpnCalled,
			summary: domain.CKPNComparisonSummary{
				Enabled:            true,
				Processed:          3,
				TotalPPKA:          decimal.NewFromInt(1_000_000),
				TotalCKPN:          decimal.NewFromInt(600_000),
				ModalIntiDeduction: decimal.NewFromInt(400_000),
			},
		},
	)

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if !ckpnCalled {
		t.Fatal("perbandingan CKPN harus berjalan saat PPAP berhasil")
	}
	if res.CKPNCompared != 3 {
		t.Fatalf("ckpn_compared = %d, mau 3", res.CKPNCompared)
	}
	if !res.CKPNModalIntiDeduction.Equal(decimal.NewFromInt(400_000)) {
		t.Fatalf("ckpn_modal_inti_deduction = %s, mau 400.000", res.CKPNModalIntiDeduction)
	}
	if step := eodStepStatus(t, res, "ckpn_comparison"); step.Status != domain.EODStepRan {
		t.Fatalf("status CKPN = %s, mau RAN", step.Status)
	}
}
