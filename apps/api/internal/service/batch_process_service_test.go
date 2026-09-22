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

// eodTestConfig adalah SystemConfigService in-memory untuk uji gerbang ckpn.enabled:
// GetBool benar-benar membaca nilai sehingga saklar dapat dinyalakan tanpa database.
type eodTestConfig struct {
	values map[string]string
}

func (c *eodTestConfig) GetString(_ context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}
func (c *eodTestConfig) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (c *eodTestConfig) GetInt(context.Context, string, int) int { return 0 }
func (c *eodTestConfig) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := c.values[key]; ok {
		return v == "true"
	}
	return fallback
}
func (c *eodTestConfig) Invalidate(string) {}

var _ domain.SystemConfigService = (*eodTestConfig)(nil)

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
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &eodDefRepoStub{})

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
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &eodDefRepoStub{})

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
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &eodDefRepoStub{})

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

// stubBatchActivityRepo menggantikan pembacaan aktivitas jurnal agar pemisahan
// metrik setoran vs penempatan deposito dapat diuji tanpa database.
type stubBatchActivityRepo struct {
	summary domain.DailyActivitySummary
}

func (s stubBatchActivityRepo) DailyActivity(context.Context, time.Time) (*domain.DailyActivitySummary, error) {
	out := s.summary
	return &out, nil
}

// stubDormantRunner menggantikan penandaan dormant agar ringkasan EOD bisa diuji
// tanpa database.
type stubDormantRunner struct {
	marked  int
	warning string
	err     error
	// order mencatat urutan pemanggilan lintas langkah bila diisi uji urutan definisi.
	order *[]string
	// called menandai pemanggilan agar uji langkah nonaktif membuktikan tidak dipanggil.
	called *bool
}

func (s stubDormantRunner) MarkDormant(context.Context, time.Time, domain.Actor) (domain.DormantRunSummary, error) {
	if s.called != nil {
		*s.called = true
	}
	if s.order != nil {
		*s.order = append(*s.order, "dormant")
	}
	return domain.DormantRunSummary{Marked: s.marked, Warning: s.warning}, s.err
}

func TestRunEODReportsDormantMarking(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		stubDormantRunner{marked: 7, warning: "ambang memakai fallback"}, nil, nil, &eodDefRepoStub{})

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
	// order mencatat urutan pemanggilan lintas langkah bila diisi uji urutan.
	order *[]string
}

func (s stubInterestAccrualRunner) AccrueInterest(context.Context, time.Time, domain.Actor) (domain.LoanInterestAccrualSummary, error) {
	if s.order != nil {
		*s.order = append(*s.order, "accrual")
	}
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
	if s.order != nil {
		*s.order = append(*s.order, "amortization")
	}
	return domain.RestructureLossAmortizationSummary{
		Amortized:      s.amortized,
		TotalAmortized: decimal.NewFromInt(int64(s.amortized)),
	}, s.amortizeErr
}

// stubPPAPRunner menggantikan langkah PPAP agar gerbang CKPN dapat diuji tanpa database.
type stubPPAPRunner struct {
	processed int
	adjusted  int
	err       error
	// called dipakai uji gerbang untuk membuktikan PPAP tidak dipanggil.
	called *bool
	order  *[]string
}

func (s stubPPAPRunner) RunDaily(context.Context, time.Time, domain.Actor) (domain.PPAPRunSummary, error) {
	if s.called != nil {
		*s.called = true
	}
	if s.order != nil {
		*s.order = append(*s.order, "ppap")
	}
	return domain.PPAPRunSummary{Processed: s.processed, Adjusted: s.adjusted}, s.err
}

// stubCKPNService menggantikan perbandingan CKPN agar gerbang PPAP dapat diuji.
type stubCKPNService struct {
	summary domain.CKPNComparisonSummary
	err     error
	called  *bool
	// runCalled menandai pemanggilan Run (jalur yang memposting jurnal). EOD harus
	// memakai Compare yang baca-saja, sehingga runCalled tetap false.
	runCalled *bool
}

func (s stubCKPNService) Compare(context.Context, time.Time, domain.Actor) (domain.CKPNComparisonSummary, error) {
	if s.called != nil {
		*s.called = true
	}
	return s.summary, s.err
}

func (s stubCKPNService) Run(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.CKPNComparisonSummary, error) {
	if s.runCalled != nil {
		*s.runCalled = true
	}
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
		nil, &eodDefRepoStub{})

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
		nil, &eodDefRepoStub{})

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
		nil, &eodDefRepoStub{})

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
		nil, nil,
		// Amortisasi harus RAN agar PPAP benar-benar dicoba dan gagal; tanpa akrual,
		// gerbang amortisasi lebih dulu yang melewati PPAP.
		stubInterestAccrualRunner{},
		stubCKPNService{called: &ckpnCalled}, &eodDefRepoStub{})

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
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			called: &ckpnCalled,
			summary: domain.CKPNComparisonSummary{
				Enabled:            true,
				Processed:          3,
				TotalPPKA:          decimal.NewFromInt(1_000_000),
				TotalCKPN:          decimal.NewFromInt(600_000),
				ModalIntiDeduction: decimal.NewFromInt(400_000),
			},
		}, &eodDefRepoStub{})

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

// Amortisasi saldo kerugian restrukturisasi harus berjalan SEBELUM PPAP: PPKA
// dihitung atas nilai tercatat setelah amortisasi periode berjalan (Pasal 32 POJK
// 1/2024 jo. PA BPR Bab 5.2). Bila terbalik, required_ppap berdiri di atas saldo
// kerugian yang belum dikurangi amortisasi.
func TestRunEODRunsLossAmortizationBeforePPAP(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	var order []string
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1, order: &order},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1, amortized: 7, order: &order},
		nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if got := strings.Join(order, "->"); got != "accrual->amortization->ppap" {
		t.Fatalf("urutan langkah %q, mau %q", got, "accrual->amortization->ppap")
	}
	if res.LoanLossAmortized != 7 {
		t.Fatalf("loan_loss_amortized = %d, mau 7", res.LoanLossAmortized)
	}
	if step := eodStepStatus(t, res, "ppap"); step.Status != domain.EODStepRan {
		t.Fatalf("status PPAP = %s, mau RAN", step.Status)
	}
}

// Bila amortisasi gagal, PPAP tidak boleh berjalan: saldo kerugian belum dimutakhirkan
// sehingga required_ppap akan terlalu besar. Penolakan harus tercatat di ringkasan,
// bukan senyap.
func TestRunEODSkipsPPAPWhenLossAmortizationFails(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	ppapCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1, called: &ppapCalled},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1, amortizeErr: errors.New("amortisasi gagal di tengah")},
		nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if ppapCalled {
		t.Fatal("PPAP tidak boleh berjalan saat amortisasi gagal, tetapi ia dipanggil")
	}
	if res.PPAPProcessed != 0 {
		t.Fatalf("ppap_processed = %d, mau 0 karena dilewati", res.PPAPProcessed)
	}
	if step := eodStepStatus(t, res, "restructure_loss_amortization"); step.Status != domain.EODStepFailed {
		t.Fatalf("status amortisasi = %s, mau FAILED", step.Status)
	}
	ppapStep := eodStepStatus(t, res, "ppap")
	if ppapStep.Status != domain.EODStepSkipped {
		t.Fatalf("status PPAP = %s, mau SKIPPED", ppapStep.Status)
	}
	if ppapStep.Reason == "" {
		t.Fatal("PPAP yang dilewati karena prasyarat gagal harus mencantumkan alasan")
	}
	warned := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "perhitungan PPAP harian dilewati") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("penolakan PPAP harus muncul sebagai peringatan, dapat %v", res.Warnings)
	}
}

// Bila akrual gagal, amortisasi dilewati; karena amortisasi tidak RAN, PPAP juga harus
// dilewati. Keduanya tercatat sebagai langkah SKIPPED beserta alasannya.
func TestRunEODSkipsAmortizationAndPPAPWhenAccrualFails(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	amortizeCalled := false
	ppapCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{called: &ppapCalled},
		nil, nil,
		stubInterestAccrualRunner{
			err:            errors.New("akrual gagal di tengah"),
			amortizeCalled: &amortizeCalled,
		},
		nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if amortizeCalled {
		t.Fatal("amortisasi tidak boleh berjalan saat akrual gagal")
	}
	if ppapCalled {
		t.Fatal("PPAP tidak boleh berjalan saat amortisasi dilewati")
	}
	amortStep := eodStepStatus(t, res, "restructure_loss_amortization")
	if amortStep.Status != domain.EODStepSkipped || amortStep.Reason == "" {
		t.Fatalf("amortisasi harus SKIPPED beralasan, dapat %+v", amortStep)
	}
	ppapStep := eodStepStatus(t, res, "ppap")
	if ppapStep.Status != domain.EODStepSkipped || ppapStep.Reason == "" {
		t.Fatalf("PPAP harus SKIPPED beralasan, dapat %+v", ppapStep)
	}
}

// Setoran tunai teller dan penempatan deposito berjangka adalah dua peristiwa berbeda:
// ringkasan EOD harus melaporkannya terpisah, bukan menjumlahkan keduanya sebagai
// "setoran". Bidang total setoran lama tidak berubah maknanya bagi klien lama, tetapi
// kini hanya memuat DEPOSIT.
func TestRunEODReportsDepositAndPlacementSeparately(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	batchRepo := stubBatchActivityRepo{summary: domain.DailyActivitySummary{
		PostedJournals:              4,
		TotalDepositAmount:          decimal.NewFromInt(500_000),
		TotalWithdrawalAmount:       decimal.NewFromInt(100_000),
		DepositPlacementCount:       2,
		TotalDepositPlacementAmount: decimal.NewFromInt(50_000_000),
	}}
	svc := service.NewBatchProcessService(
		dateRepo, batchRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if !res.TotalDepositAmountToday.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("total_deposit_amount_today = %s, mau 500.000 (DEPOSIT saja)", res.TotalDepositAmountToday)
	}
	if res.TotalDepositPlacementsToday != 2 {
		t.Fatalf("total_deposit_placements_today = %d, mau 2", res.TotalDepositPlacementsToday)
	}
	if !res.TotalDepositPlacementAmount.Equal(decimal.NewFromInt(50_000_000)) {
		t.Fatalf("total_deposit_placement_amount_today = %s, mau 50.000.000", res.TotalDepositPlacementAmount)
	}
	if !res.TotalWithdrawalAmountToday.Equal(decimal.NewFromInt(100_000)) {
		t.Fatalf("total_withdrawal_amount_today = %s, mau 100.000", res.TotalWithdrawalAmountToday)
	}
}

// ppap_processed adalah jumlah kredit yang dievaluasi, sedangkan ppap_adjusted adalah
// jumlah kredit yang PPAP-nya berubah sehingga menulis jurnal. Keduanya harus berbeda
// supaya "5 diproses, 1 jurnal" tidak menyesatkan.
func TestRunEODReportsPPAPProcessedAndAdjusted(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 5, adjusted: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.PPAPProcessed != 5 {
		t.Fatalf("ppap_processed = %d, mau 5", res.PPAPProcessed)
	}
	if res.PPAPAdjusted != 1 {
		t.Fatalf("ppap_adjusted = %d, mau 1", res.PPAPAdjusted)
	}
}

// Mode bayangan CKPN pada tutup hari: hasil dilaporkan di bidang ADITIF, tetapi jalur
// resmi tidak diisi, tidak ada pengurangan modal inti, dan EOD tidak pernah memanggil
// Run (jalur yang memposting jurnal) — hanya Compare yang baca-saja.
func TestRunEODReportsShadowCKPNWithoutJournaling(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	runCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			runCalled: &runCalled,
			summary: domain.CKPNComparisonSummary{
				Enabled:     false,
				ShadowMode:  true,
				Processed:   1,
				TotalPPKA:   decimal.NewFromInt(1_000_000),
				TotalCKPN:   decimal.NewFromInt(500_000),
				Difference:  decimal.NewFromInt(500_000),
				Higher:      domain.CKPNLargerPPKA,
				Assumptions: []string{"PD golongan 3 = 0.1"},
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if runCalled {
		t.Fatal("EOD harus memakai Compare yang baca-saja, bukan Run yang memposting")
	}
	if !res.CKPNShadowMode {
		t.Fatal("bidang mode bayangan harus terisi")
	}
	if res.CKPNShadowProcessed != 1 {
		t.Fatalf("ckpn_shadow_processed = %d, mau 1", res.CKPNShadowProcessed)
	}
	if !res.CKPNShadowTotalPPKA.Equal(decimal.NewFromInt(1_000_000)) || !res.CKPNShadowTotalCKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("total bayangan PPKA/CKPN = %s/%s, mau 1000000/500000", res.CKPNShadowTotalPPKA, res.CKPNShadowTotalCKPN)
	}
	if res.CKPNShadowHigher != string(domain.CKPNLargerPPKA) {
		t.Fatalf("ckpn_shadow_higher = %q, mau PPKA", res.CKPNShadowHigher)
	}
	if !strings.Contains(res.CKPNShadowNote, "MODE BAYANGAN") || !strings.Contains(res.CKPNShadowNote, "pengurangan modal inti") {
		t.Fatalf("catatan bayangan harus melabeli angka ini, dapat %q", res.CKPNShadowNote)
	}
	if len(res.CKPNShadowAssumptions) != 1 {
		t.Fatalf("asumsi bayangan %v, mau diteruskan dari perhitungan", res.CKPNShadowAssumptions)
	}
	// Bidang jalur resmi TIDAK boleh terisi oleh mode bayangan.
	if res.CKPNCompared != 0 || !res.CKPNTotalCKPN.IsZero() || !res.CKPNModalIntiDeduction.IsZero() {
		t.Fatalf("mode bayangan tidak boleh mengisi bidang resmi: %+v", res)
	}
	if step := eodStepStatus(t, res, "ckpn_comparison"); step.Status != domain.EODStepRan {
		t.Fatalf("status CKPN bayangan = %s, mau RAN", step.Status)
	}
}

// P1: bidang bayangan potensi pengurang modal inti harus meneruskan Σ max per kredit,
// BUKAN selisih agregat. Ringkasan sengaja dibuat dengan agregat 0 (SAMA) tetapi
// potensi pengurang 500.000 agar kesalahan pemetaan bidang langsung terlihat.
func TestRunEODLaporkanPengurangModalIntiBayanganPerKredit(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			summary: domain.CKPNComparisonSummary{
				ShadowMode:         true,
				Processed:          2,
				TotalPPKA:          decimal.NewFromInt(1_000_000),
				TotalCKPN:          decimal.NewFromInt(1_000_000),
				Difference:         decimal.Zero,
				Higher:             domain.CKPNLargerSame,
				ModalIntiDeduction: decimal.NewFromInt(500_000),
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if !res.CKPNShadowModalIntiDeduction.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("ckpn_shadow_modal_inti_deduction = %s, mau 500000 (Σ per kredit)", res.CKPNShadowModalIntiDeduction)
	}
	if !res.CKPNShadowDifference.IsZero() {
		t.Fatalf("ckpn_shadow_difference = %s, mau 0 (agregat, berbeda dari potensi pengurang)", res.CKPNShadowDifference)
	}
	if !strings.Contains(res.CKPNShadowNote, "PER KREDIT") {
		t.Fatalf("catatan harus menjelaskan dasar per kredit, dapat %q", res.CKPNShadowNote)
	}
}

// Parameter PD/LGD kosong pada mode bayangan: EOD tidak boleh gagal, langkah tetap
// dilaporkan, dan peringatan menyebut kunci yang harus diisi — bukan angka karangan.
func TestRunEODShadowCKPNParameterKosongTidakMenggagalkanEOD(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			summary: domain.CKPNComparisonSummary{
				ShadowMode:    true,
				Failed:        1,
				ParameterGaps: []string{"ckpn.pd_frac.gol_3 (PD golongan Kurang Lancar belum diisi)", "ckpn.lgd_frac (LGD belum diisi)"},
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("parameter kosong tidak boleh menggagalkan EOD: %v", err)
	}
	if !strings.Contains(res.CKPNShadowNote, "belum dapat dihitung") {
		t.Fatalf("catatan harus menyatakan CKPN belum dapat dihitung, dapat %q", res.CKPNShadowNote)
	}
	warned := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "ckpn.pd_frac.gol_3") && strings.Contains(w, "ckpn.lgd_frac") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("peringatan harus menyebut kunci yang harus diisi, dapat %v", res.Warnings)
	}
	if step := eodStepStatus(t, res, "ckpn_comparison"); step.Status != domain.EODStepSkipped && step.Status != domain.EODStepRan {
		t.Fatalf("status CKPN bayangan tak terduga: %s", step.Status)
	}
}

// Kedua saklar menyala: service melaporkan ShadowMode=false karena bayangan diabaikan,
// sehingga EOD memakai jalur resmi dan TIDAK mengisi bidang bayangan. Kontrak
// "ShadowMode hanya true bila bayangan benar-benar dipakai" diuji di
// ckpn_service_test.go (TestCKPN_KeduaSaklarMenyalaModeResmiBerlaku).
func TestRunEODKeduaSaklarCKPNMenyalaMemakaiJalurResmi(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			summary: domain.CKPNComparisonSummary{
				Enabled:   true,
				Processed: 2,
				TotalPPKA: decimal.NewFromInt(2_000_000),
				TotalCKPN: decimal.NewFromInt(1_000_000),
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.CKPNCompared != 2 {
		t.Fatalf("ckpn_compared = %d, mau 2 (jalur resmi)", res.CKPNCompared)
	}
	if res.CKPNShadowMode {
		t.Fatal("mode bayangan diabaikan saat ckpn.enabled menyala, bidang bayangan tidak boleh terisi")
	}
	if !res.CKPNShadowTotalCKPN.IsZero() || !res.CKPNShadowModalIntiDeduction.IsZero() {
		t.Fatalf("bidang bayangan tidak boleh terisi pada jalur resmi: %+v", res)
	}
}

// Saat ckpn.enabled menyala, langkah EOD CKPN harus MEMBENTUK dan MENJURNAL lewat Run,
// bukan sekadar Compare baca-saja. Memakai Compare saat saklar menyala membuat bank
// mengira patuh padahal pembukuan tidak memuat CKPN. Test ini gagal bila langkah kembali
// selalu memanggil Compare (runCalled tetap false).
func TestRunEODCKPNMenyalaMemakaiRunMenjurnal(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	runCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil,
		&eodTestConfig{values: map[string]string{"ckpn.enabled": "true"}}, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			runCalled: &runCalled,
			summary: domain.CKPNComparisonSummary{
				Enabled:            true,
				Processed:          2,
				TotalPPKA:          decimal.NewFromInt(1_000_000),
				TotalCKPN:          decimal.NewFromInt(600_000),
				ModalIntiDeduction: decimal.NewFromInt(400_000),
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if !runCalled {
		t.Fatal("ckpn.enabled menyala harus memakai Run yang menjurnal, bukan Compare baca-saja")
	}
	if res.CKPNCompared != 2 {
		t.Fatalf("ckpn_compared = %d, mau 2 (jalur resmi)", res.CKPNCompared)
	}
	if !res.CKPNModalIntiDeduction.Equal(decimal.NewFromInt(400_000)) {
		t.Fatalf("ckpn_modal_inti_deduction = %s, mau 400000", res.CKPNModalIntiDeduction)
	}
	if step := eodStepStatus(t, res, "ckpn_comparison"); step.Status != domain.EODStepRan {
		t.Fatalf("status CKPN = %s, mau RAN", step.Status)
	}
}

// Saat ckpn.enabled mati, EOD tetap memakai Compare baca-saja (perilaku lama): mode
// bayangan dihitung dan dilaporkan tanpa jurnal. Test ini mengunci gerbang saklar.
func TestRunEODCKPNMatiTetapCompareBacaSaja(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	runCalled := false
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil,
		&eodTestConfig{values: map[string]string{"ckpn.enabled": "false"}}, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			runCalled: &runCalled,
			summary: domain.CKPNComparisonSummary{
				ShadowMode: true,
				Processed:  1,
				TotalPPKA:  decimal.NewFromInt(1_000_000),
				TotalCKPN:  decimal.NewFromInt(500_000),
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if runCalled {
		t.Fatal("ckpn.enabled mati tidak boleh memakai Run yang menjurnal")
	}
	if !res.CKPNShadowMode {
		t.Fatal("mode bayangan harus tetap dilaporkan saat ckpn.enabled mati")
	}
	if res.CKPNCompared != 0 {
		t.Fatalf("bidang resmi tidak boleh terisi saat saklar mati: compared=%d", res.CKPNCompared)
	}
}

// stubPenaltyRunner menggantikan akrual denda agar ringkasan EOD bisa diuji tanpa
// database.
type stubPenaltyRunner struct {
	summary domain.LoanPenaltySummary
	err     error
}

func (s stubPenaltyRunner) AccruePenalties(context.Context, time.Time, domain.Actor) (domain.LoanPenaltySummary, error) {
	return s.summary, s.err
}

// Jumlah kredit yang dendanya dihentikan plafon dan denda syariah yang masuk Dana
// Kebajikan harus terlihat di ringkasan EOD, beserta peringatan plafon. Tanpa ini,
// akrual denda berhenti bertambah tanpa penjelasan.
func TestRunEODReportsPenaltyCapAndSyariahSocialFund(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPenaltyRunner{summary: domain.LoanPenaltySummary{
			Accrued:           4,
			TotalPenalty:      decimal.NewFromInt(250_000),
			CapPercent:        decimal.NewFromInt(10),
			Capped:            2,
			SyariahSocialFund: 3,
			Warning:           "plafon denda 10% dari pokok tunggakan tercapai pada 2 kredit",
		}},
		nil, nil, nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.LoanPenaltiesCapped != 2 {
		t.Fatalf("loan_penalties_capped = %d, mau 2", res.LoanPenaltiesCapped)
	}
	if !res.LoanPenaltyCapPercent.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("loan_penalty_cap_percent = %s, mau 10", res.LoanPenaltyCapPercent)
	}
	if res.LoanPenaltiesSyariahSocialFund != 3 {
		t.Fatalf("loan_penalties_syariah_social_fund = %d, mau 3", res.LoanPenaltiesSyariahSocialFund)
	}
	warned := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "plafon denda") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("peringatan plafon harus masuk Warnings, dapat %v", res.Warnings)
	}
}

// Kedua saklar mati: langkah ckpn_comparison tetap dilewati seperti sebelum mode
// bayangan ada (perilaku sekarang).
func TestRunEODKeduaSaklarCKPNMatiTetapDilewati(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{summary: domain.CKPNComparisonSummary{}}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	step := eodStepStatus(t, res, "ckpn_comparison")
	if step.Status != domain.EODStepSkipped {
		t.Fatalf("status CKPN = %s, mau SKIPPED", step.Status)
	}
	if res.CKPNShadowMode {
		t.Fatal("saklar mati tidak boleh mengisi bidang bayangan")
	}
}

// Total CKPN nol pada mode bayangan karena SELURUH kredit dikecualikan sebagai aset
// baik (butir 12.3.a.2.a) harus DIJELASKAN, bukan dibiarkan terbaca seolah model belum
// dijalankan. Reproduksi keadaan produksi: 6 kredit, total PPKA 929.407, CKPN nol.
func TestRunEODShadowCKPNJelaskanTotalNolKarenaAsetBaik(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			summary: domain.CKPNComparisonSummary{
				ShadowMode:          true,
				Processed:           6,
				TotalPPKA:           decimal.NewFromInt(929_407),
				TotalCKPN:           decimal.Zero,
				Difference:          decimal.NewFromInt(929_407),
				Higher:              domain.CKPNLargerPPKA,
				ModalIntiDeduction:  decimal.NewFromInt(929_407),
				AsetBaikCount:       6,
				AsetBaikOutstanding: decimal.NewFromInt(185_000_000),
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.CKPNShadowAsetBaik != 6 {
		t.Fatalf("ckpn_shadow_aset_baik = %d, mau 6", res.CKPNShadowAsetBaik)
	}
	if !res.CKPNShadowAsetBaikOutstanding.Equal(decimal.NewFromInt(185_000_000)) {
		t.Fatalf("ckpn_shadow_aset_baik_outstanding = %s, mau 185000000", res.CKPNShadowAsetBaikOutstanding)
	}
	if !strings.Contains(res.CKPNShadowNote, "aset baik") || !strings.Contains(res.CKPNShadowNote, "bukan tanda model belum dijalankan") {
		t.Fatalf("catatan harus menjelaskan nol karena aset baik, dapat %q", res.CKPNShadowNote)
	}
	warned := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "aset baik") && strings.Contains(w, "total CKPN nol") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("harus ada peringatan bahwa total CKPN nol karena aset baik, dapat %v", res.Warnings)
	}
	// Bidang resmi tetap tidak terisi: nol karena aset baik bukan pengurangan modal inti.
	if res.CKPNCompared != 0 || !res.CKPNModalIntiDeduction.IsZero() {
		t.Fatalf("mode bayangan tidak boleh mengisi bidang resmi: %+v", res)
	}
}

// Dua basis CKPN harus berdampingan pada hasil EOD: bidang lama (basis "sesuai
// kebijakan") tetap, ditambah bidang "setara PPKA" (aset baik tetap dinilai) dan
// catatan yang menegaskan keduanya belum menjadi kebijakan bank.
func TestRunEODLaporkanDuaBasisCKPN(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			summary: domain.CKPNComparisonSummary{
				ShadowMode: true,
				Processed:  6,
				// Basis 1: aset baik dikecualikan -> total CKPN nol, pengurang = seluruh PPKA.
				TotalPPKA:          decimal.NewFromInt(929_407),
				TotalCKPN:          decimal.Zero,
				Difference:         decimal.NewFromInt(929_407),
				Higher:             domain.CKPNLargerPPKA,
				ModalIntiDeduction: decimal.NewFromInt(929_407),
				AsetBaikCount:      6,
				// Basis 2: aset baik tetap dinilai -> angka lebih kecil.
				SetaraPPKAProcessed:          6,
				SetaraPPKATotalPPKA:          decimal.NewFromInt(929_407),
				SetaraPPKATotalCKPN:          decimal.NewFromInt(120_000),
				SetaraPPKADifference:         decimal.NewFromInt(809_407),
				SetaraPPKAModalIntiDeduction: decimal.NewFromInt(809_407),
				SetaraPPKAHigher:             domain.CKPNLargerPPKA,
				BasisNote:                    "DUA BASIS CKPN: ... Keduanya BUKAN kebijakan bank yang berlaku.",
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	// Basis 1 (bidang lama) tidak berubah.
	if !res.CKPNShadowTotalCKPN.IsZero() || !res.CKPNShadowModalIntiDeduction.Equal(decimal.NewFromInt(929_407)) {
		t.Fatalf("basis kebijakan berubah: ckpn=%s ded=%s", res.CKPNShadowTotalCKPN, res.CKPNShadowModalIntiDeduction)
	}
	// Basis 2 terpetakan lengkap.
	if res.CKPNShadowSetaraPPKAProcessed != 6 {
		t.Fatalf("ckpn_shadow_setara_ppka_processed = %d, mau 6", res.CKPNShadowSetaraPPKAProcessed)
	}
	if !res.CKPNShadowSetaraPPKATotalPPKA.Equal(decimal.NewFromInt(929_407)) {
		t.Fatalf("total PPKA setara %s, mau 929407 (sama dengan basis kebijakan)", res.CKPNShadowSetaraPPKATotalPPKA)
	}
	if !res.CKPNShadowSetaraPPKATotalCKPN.Equal(decimal.NewFromInt(120_000)) {
		t.Fatalf("total CKPN setara %s, mau 120000", res.CKPNShadowSetaraPPKATotalCKPN)
	}
	if !res.CKPNShadowSetaraPPKADifference.Equal(decimal.NewFromInt(809_407)) {
		t.Fatalf("difference setara %s, mau 809407", res.CKPNShadowSetaraPPKADifference)
	}
	if !res.CKPNShadowSetaraPPKAModalIntiDeduction.Equal(decimal.NewFromInt(809_407)) {
		t.Fatalf("pengurang modal setara %s, mau 809407", res.CKPNShadowSetaraPPKAModalIntiDeduction)
	}
	if res.CKPNShadowSetaraPPKAHigher != string(domain.CKPNLargerPPKA) {
		t.Fatalf("higher setara %q, mau PPKA", res.CKPNShadowSetaraPPKAHigher)
	}
	if !strings.Contains(res.CKPNShadowBasisNote, "DUA BASIS") {
		t.Fatalf("basis note harus diteruskan, dapat %q", res.CKPNShadowBasisNote)
	}
	if !strings.Contains(res.CKPNShadowNote, "DUA BASIS") {
		t.Fatalf("basis note harus ikut pada catatan bayangan, dapat %q", res.CKPNShadowNote)
	}
	// Bidang resmi tetap tidak terisi.
	if res.CKPNCompared != 0 || !res.CKPNModalIntiDeduction.IsZero() {
		t.Fatalf("bidang resmi tidak boleh terisi: %+v", res)
	}
}

// Bila basis setara PPKA tidak dapat menghitung sebagian kredit (PD/LGD belum
// lengkap), EOD harus memberi peringatan agar total yang lebih kecil tidak terbaca
// sebagai perbandingan utuh.
func TestRunEODPeringatkanBasisSetaraPPKAGagal(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	svc := service.NewBatchProcessService(
		dateRepo, nil, nil, nil, nil, nil, nil, nil, nil,
		stubPPAPRunner{processed: 1},
		nil, nil,
		stubInterestAccrualRunner{accrued: 1},
		stubCKPNService{
			summary: domain.CKPNComparisonSummary{
				ShadowMode:          true,
				Processed:           3,
				SetaraPPKAProcessed: 2,
				SetaraPPKAFailed:    1,
			},
		}, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if res.CKPNShadowSetaraPPKAFailed != 1 {
		t.Fatalf("ckpn_shadow_setara_ppka_failed = %d, mau 1", res.CKPNShadowSetaraPPKAFailed)
	}
	warned := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "setara PPKA") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("harus ada peringatan basis setara PPKA gagal, dapat %v", res.Warnings)
	}
}
