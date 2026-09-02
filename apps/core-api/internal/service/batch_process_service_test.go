package service_test

import (
	"context"
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

func (s *stubBusinessDateRepo) SetStatus(ctx context.Context, status domain.BusinessDateStatus) error {
	s.status = status
	return nil
}

func TestBatchProcessService_RunEOD(t *testing.T) {
	initDate := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	dateRepo := &stubBusinessDateRepo{currentDate: initDate, status: domain.BusinessDateStatusOpen}
	svc := service.NewBatchProcessService(dateRepo, nil, nil, nil)

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

func TestBatchProcessService_RunEOY(t *testing.T) {
	initDate := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	dateRepo := &stubBusinessDateRepo{currentDate: initDate, status: domain.BusinessDateStatusOpen}
	reportSvc := service.NewReportService(&stubReportRepo{})

	svc := service.NewBatchProcessService(dateRepo, nil, nil, reportSvc)
	executor := uuid.New()

	res, err := svc.RunEOY(context.Background(), "30201", executor)
	if err != nil {
		t.Fatalf("unexpected error running EOY: %v", err)
	}

	if res.FiscalYear != 2026 {
		t.Fatalf("expected fiscal year 2026, got %d", res.FiscalYear)
	}
	if !res.NetRetainedEarnings.Equal(decimal.NewFromInt(15000000)) {
		t.Fatalf("expected Net Income 15M to transfer to Retained Earnings, got %s", res.NetRetainedEarnings.String())
	}
}
