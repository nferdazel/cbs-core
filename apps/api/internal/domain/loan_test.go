package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Aturan kolektibilitas diuji lengkap di ppap_test.go: satu tempat untuk satu
// sumber kebenaran (domain.CollectibilityFromPosition).

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
