package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Urutan alokasi pembayaran kredit: denda lebih dulu, lalu bunga/margin, terakhir
// pokok. Kelebihan setelah seluruh kewajiban ditutup tidak boleh ditelan.
func TestAllocateLoanPayment_UrutanDendaBungaLaluPokok(t *testing.T) {
	tests := []struct {
		name                                                  string
		amount, penalty, profit, principal                    int64
		wantPenalty, wantProfit, wantPrincipal, wantUnapplied int64
	}{
		{"hanya menutup sebagian denda", 10, 25, 100, 1000, 10, 0, 0, 0},
		{"menutup denda lalu sebagian bunga", 100, 25, 100, 1000, 25, 75, 0, 0},
		{"menutup denda, bunga, lalu sebagian pokok", 200, 25, 100, 1000, 25, 100, 75, 0},
		{"menutup seluruh kewajiban", 1125, 25, 100, 1000, 25, 100, 1000, 0},
		{"kelebihan dikembalikan sebagai sisa", 1200, 25, 100, 1000, 25, 100, 1000, 75},
		{"tanpa kewajiban seluruhnya menjadi sisa", 50, 0, 0, 0, 0, 0, 0, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.AllocateLoanPayment(
				decimal.NewFromInt(tt.amount),
				decimal.NewFromInt(tt.penalty),
				decimal.NewFromInt(tt.profit),
				decimal.NewFromInt(tt.principal),
			)
			if !got.Penalty.Equal(decimal.NewFromInt(tt.wantPenalty)) ||
				!got.Profit.Equal(decimal.NewFromInt(tt.wantProfit)) ||
				!got.Principal.Equal(decimal.NewFromInt(tt.wantPrincipal)) ||
				!got.Unapplied.Equal(decimal.NewFromInt(tt.wantUnapplied)) {
				t.Fatalf("denda=%s bunga=%s pokok=%s sisa=%s, ingin %d/%d/%d/%d",
					got.Penalty, got.Profit, got.Principal, got.Unapplied,
					tt.wantPenalty, tt.wantProfit, tt.wantPrincipal, tt.wantUnapplied)
			}
			if !got.Applied().Equal(decimal.NewFromInt(tt.amount - tt.wantUnapplied)) {
				t.Fatalf("applied %s tidak sesuai", got.Applied())
			}
		})
	}
}

// Nominal negatif tidak boleh menghasilkan alokasi negatif.
func TestAllocateLoanPayment_NominalNegatifTidakMengalokasi(t *testing.T) {
	got := domain.AllocateLoanPayment(decimal.NewFromInt(-50), decimal.NewFromInt(25), decimal.NewFromInt(100), decimal.NewFromInt(1000))
	if got.Applied().IsPositive() {
		t.Fatalf("nominal negatif tidak boleh membayar kewajiban: %+v", got)
	}
}
