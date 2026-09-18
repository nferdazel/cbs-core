package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Satu-satunya aturan kolektibilitas adalah CollectibilityFromDPD. Aturan lama
// (CalculateCollectibility) memakai DPK 1-90 hari dan Kurang Lancar 91-120 hari;
// aturan POJK yang berlaku: 0 lancar; 1-30 DPK; 31-90 kurang lancar;
// 91-180 diragukan; >180 macet.
func TestCollectibilityFromDPD_POJKThresholds(t *testing.T) {
	thresholds := domain.DefaultCollectibilityThresholds()

	tests := []struct {
		name string
		dpd  int
		want domain.Collectibility
	}{
		{"tepat waktu", 0, domain.KolLancar},
		{"DPK batas atas 30", 30, domain.KolDPK},
		{"kurang lancar batas bawah 31", 31, domain.KolKurangLancar},
		{"kurang lancar batas atas 90", 90, domain.KolKurangLancar},
		{"diragukan batas bawah 91", 91, domain.KolDiragukan},
		{"DPD 100 dulu Kurang Lancar, kini Diragukan", 100, domain.KolDiragukan},
		{"DPD 120 dulu Kurang Lancar, kini Diragukan", 120, domain.KolDiragukan},
		{"diragukan batas atas 180", 180, domain.KolDiragukan},
		{"macet di atas 180", 181, domain.KolMacet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.CollectibilityFromDPD(tt.dpd, thresholds); got != tt.want {
				t.Fatalf("DPD %d: got %s, want %s", tt.dpd, got.Label(), tt.want.Label())
			}
		})
	}
}
