package service_test

import (
	"context"
	"strings"
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

// Penanda saldo mengendap dana kebajikan (akun 12500): saldo > 0 yang tidak bergerak
// (tidak ada penyaluran) melewati satu kuartal harus memunculkan peringatan JELAS di
// ringkasan tutup hari, dan peringatan itu hilang begitu ada penyaluran/pergerakan
// atau saldonya nol. Ini peringatan saja: tidak memblokir tutup hari.
func TestRunEODWarnsIdleSocialFund(t *testing.T) {
	businessDate := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		lastMove    time.Time
		balance     int64
		wantWarning bool
	}{
		{"tidak bergerak lebih dari satu kuartal", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 60_000, true},
		{"baru ada penyaluran/pergerakan", time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), 10_000, false},
		{"saldo nol, tidak ada dana menunggu", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dateRepo := &stubBusinessDateRepo{currentDate: businessDate, status: domain.BusinessDateStatusOpen}
			batchRepo := stubBatchActivityRepo{summary: domain.DailyActivitySummary{
				SocialFundBalance:      decimal.NewFromInt(tc.balance),
				SocialFundLastMovement: tc.lastMove,
			}}
			svc := service.NewBatchProcessService(
				dateRepo, batchRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, &eodDefRepoStub{})

			res, err := svc.RunEOD(context.Background(), uuid.New())
			if err != nil {
				t.Fatalf("EOD gagal: %v", err)
			}
			found := false
			for _, w := range res.Warnings {
				if strings.Contains(w, "Dana kebajikan (akun 12500)") {
					found = true
				}
			}
			if found != tc.wantWarning {
				t.Fatalf("peringatan saldo mengendap = %v, mau %v (warnings=%v)", found, tc.wantWarning, res.Warnings)
			}
		})
	}
}
