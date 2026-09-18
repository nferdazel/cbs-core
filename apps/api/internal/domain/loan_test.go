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

// loan_type di database NOT NULL tanpa default: setiap produk kredit harus
// terpetakan ke salah satu nilai enum, kalau tidak insert kredit gagal.
func TestLoanTypeForCoversEveryProductScheme(t *testing.T) {
	cases := []struct {
		name    string
		product domain.BankingProduct
		want    domain.LoanType
	}{
		{"murabahah", domain.BankingProduct{ProfitScheme: domain.SchemeMurabahah}, domain.LoanTypeSyariahMurabahah},
		{"mudharabah", domain.BankingProduct{ProfitScheme: domain.SchemeMudharabah}, domain.LoanTypeSyariahMudharabah},
		{"musyarakah", domain.BankingProduct{ProfitScheme: domain.SchemeMusyarakah}, domain.LoanTypeSyariahMudharabah},
		{"ijarah", domain.BankingProduct{ProfitScheme: domain.SchemeIjarah}, domain.LoanTypeSyariahMudharabah},
		{"konvensional anuitas", domain.BankingProduct{ProfitScheme: domain.SchemeInterest, ScheduleMethod: domain.ScheduleAnnuity}, domain.LoanTypeConventionalAnnuity},
		{"konvensional flat", domain.BankingProduct{ProfitScheme: domain.SchemeInterest, ScheduleMethod: domain.ScheduleFlat}, domain.LoanTypeConventionalFlat},
	}
	for _, tc := range cases {
		if got := domain.LoanTypeFor(&tc.product); got != tc.want {
			t.Fatalf("%s: LoanTypeFor = %q, mau %q", tc.name, got, tc.want)
		}
	}
}
