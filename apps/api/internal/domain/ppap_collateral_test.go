package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Agunan mengurangi eksposur yang dikenai tarif PPAP, dengan lantai nol: penyisihan
// negatif tidak punya arti, dan agunan yang lebih besar dari baki debet tidak boleh
// membuat bank mencadangkan lebih sedikit dari nol.
func TestPPAPExposure_MengurangiDenganLantaiNol(t *testing.T) {
	cases := []struct {
		name        string
		outstanding string
		collateral  string
		want        string
	}{
		{"tanpa agunan", "1000000", "0", "1000000"},
		{"agunan sebagian", "1000000", "400000", "600000"},
		{"agunan melebihi baki debet", "1000000", "1500000", "0"},
		{"agunan sama dengan baki debet", "1000000", "1000000", "0"},
		{"nilai agunan negatif diabaikan", "1000000", "-500", "1000000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.PPAPExposure(
				decimal.RequireFromString(tc.outstanding),
				decimal.RequireFromString(tc.collateral),
			)
			if !got.Equal(decimal.RequireFromString(tc.want)) {
				t.Fatalf("eksposur %s, mau %s", got, tc.want)
			}
		})
	}
}
