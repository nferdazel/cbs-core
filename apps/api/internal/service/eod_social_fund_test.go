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

// Saldo dana kebajikan (akun 12500) dan label keterbatasan setoran lama harus ikut
// pada ringkasan EOD. Nilai saldo berasal dari pembacaan aktivitas jurnal, bukan
// konstanta, dan EOD hanya melaporkan tanpa menulis jurnal.
func TestRunEODReportsSocialFundBalanceAndDepositLabel(t *testing.T) {
	dateRepo := &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
	batchRepo := stubBatchActivityRepo{summary: domain.DailyActivitySummary{
		TotalDepositAmount: decimal.NewFromInt(500_000),
		SocialFundBalance:  decimal.NewFromInt(60_000),
	}}
	svc := service.NewBatchProcessService(
		dateRepo, batchRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &eodDefRepoStub{})

	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("EOD gagal: %v", err)
	}
	if want := decimal.NewFromInt(60_000); !res.SocialFundBalance.Equal(want) {
		t.Fatalf("social_fund_balance = %s, mau %s", res.SocialFundBalance, want)
	}
	if res.SocialFundNote == "" {
		t.Fatal("social_fund_note harus terisi agar pembaca tahu dana menunggu keputusan DPS")
	}
	if res.TotalDepositAmountTodayLabel == "" {
		t.Fatal("total_deposit_amount_today_label harus terisi agar keterbatasan data lama terlihat")
	}
}
