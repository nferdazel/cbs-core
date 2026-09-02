package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func TestCalculateCollectibility_OJKRules(t *testing.T) {
	tests := []struct {
		name                 string
		dpd                  int
		expectedCol          domain.OJKCollectibility
		expectedPPAPPercent  decimal.Decimal
		expectedAccrualState domain.AccrualStatus
	}{
		{
			name:                 "Kol 1 Lancar (DPD 0)",
			dpd:                  0,
			expectedCol:          domain.CollectibilityKol1,
			expectedPPAPPercent:  decimal.NewFromFloat(0.005),
			expectedAccrualState: domain.AccrualStatusAccrual,
		},
		{
			name:                 "Kol 1 Lancar (DPD 45)",
			dpd:                  45,
			expectedCol:          domain.CollectibilityKol1,
			expectedPPAPPercent:  decimal.NewFromFloat(0.005),
			expectedAccrualState: domain.AccrualStatusAccrual,
		},
		{
			name:                 "Kol 2 Dalam Perhatian Khusus DPK (DPD 100)",
			dpd:                  100,
			expectedCol:          domain.CollectibilityKol2,
			expectedPPAPPercent:  decimal.NewFromFloat(0.010),
			expectedAccrualState: domain.AccrualStatusAccrual,
		},
		{
			name:                 "Kol 3 Kurang Lancar NPL (DPD 150)",
			dpd:                  150,
			expectedCol:          domain.CollectibilityKol3,
			expectedPPAPPercent:  decimal.NewFromFloat(0.150),
			expectedAccrualState: domain.AccrualStatusCash, // Stop Accrual -> Cash Basis
		},
		{
			name:                 "Kol 4 Diragukan NPL (DPD 200)",
			dpd:                  200,
			expectedCol:          domain.CollectibilityKol4,
			expectedPPAPPercent:  decimal.NewFromFloat(0.500),
			expectedAccrualState: domain.AccrualStatusCash,
		},
		{
			name:                 "Kol 5 Macet NPL (DPD 300)",
			dpd:                  300,
			expectedCol:          domain.CollectibilityKol5,
			expectedPPAPPercent:  decimal.NewFromFloat(1.000),
			expectedAccrualState: domain.AccrualStatusCash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, ppapRate, accrualSt := domain.CalculateCollectibility(tt.dpd)
			if col != tt.expectedCol {
				t.Errorf("expected collectibility %s, got %s", tt.expectedCol, col)
			}
			if !ppapRate.Equal(tt.expectedPPAPPercent) {
				t.Errorf("expected PPAP percent %s, got %s", tt.expectedPPAPPercent.String(), ppapRate.String())
			}
			if accrualSt != tt.expectedAccrualState {
				t.Errorf("expected accrual status %s, got %s", tt.expectedAccrualState, accrualSt)
			}
		})
	}
}
