package service_test

import (
	"context"
	"errors"
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
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
		stubDormantRunner{marked: 7, warning: "ambang memakai fallback"}, nil,
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
}

func (s stubInterestAccrualRunner) AccrueInterest(context.Context, time.Time, domain.Actor) (domain.LoanInterestAccrualSummary, error) {
	return domain.LoanInterestAccrualSummary{
		Accrued:      s.accrued,
		TotalAccrued: s.amount,
		Warnings:     s.warnings,
	}, s.err
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
