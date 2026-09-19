package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Batas DPD sesuai POJK 40/2019: 0 lancar; 1-30 DPK; 31-90 kurang lancar;
// 91-180 diragukan; >180 macet.
func TestCollectibilityFromDPD_Boundaries(t *testing.T) {
	thresholds := domain.DefaultCollectibilityThresholds()

	tests := []struct {
		dpd  int
		want domain.Collectibility
	}{
		{0, domain.KolLancar},
		{1, domain.KolDPK},
		{30, domain.KolDPK},
		{31, domain.KolKurangLancar},
		{90, domain.KolKurangLancar},
		{91, domain.KolDiragukan},
		{180, domain.KolDiragukan},
		{181, domain.KolMacet},
		{999, domain.KolMacet},
	}

	for _, tt := range tests {
		got := domain.CollectibilityFromDPD(tt.dpd, thresholds)
		if got != tt.want {
			t.Errorf("DPD %d: got %s (%d), want %s (%d)",
				tt.dpd, got.Label(), got, tt.want.Label(), tt.want)
		}
	}
}

// Konfigurasi rusak (ambang tidak menaik) tidak boleh menghasilkan golongan lebih baik.
func TestCollectibilityFromDPD_InvalidThresholdsFallBack(t *testing.T) {
	got := domain.CollectibilityFromDPD(100, domain.CollectibilityThresholds{DPK: 0, KurangLancar: 5, Diragukan: 0})
	if got != domain.KolDiragukan {
		t.Fatalf("ambang rusak: got %s, want Diragukan", got.Label())
	}
}

// Tarif PPAP minimum BPR (POJK 33/POJK.03/2018 Pasal 16): PPAP umum 0,5% untuk
// Lancar, dan PPAP khusus 3% / 10% / 50% / 100% atas pokok terutang.
func TestPPAPAmount_DefaultRates(t *testing.T) {
	rates := domain.DefaultPPAPRates()
	outstanding := decimal.NewFromInt(1_000_000)

	tests := []struct {
		col  domain.Collectibility
		want decimal.Decimal
	}{
		{domain.KolLancar, decimal.NewFromInt(5_000)},         // PPAP umum 0,5%
		{domain.KolDPK, decimal.NewFromInt(30_000)},           // 3%
		{domain.KolKurangLancar, decimal.NewFromInt(100_000)}, // 10%
		{domain.KolDiragukan, decimal.NewFromInt(500_000)},    // 50%
		{domain.KolMacet, decimal.NewFromInt(1_000_000)},      // 100%
	}

	for _, tt := range tests {
		got := domain.PPAPAmount(outstanding, tt.col, rates)
		if !got.Equal(tt.want) {
			t.Errorf("kolektibilitas %s: got %s, want %s", tt.col.Label(), got, tt.want)
		}
	}
}

func TestPPAPAmount_UnknownCollectibilityIsZero(t *testing.T) {
	got := domain.PPAPAmount(decimal.NewFromInt(1_000_000), domain.Collectibility(9), domain.DefaultPPAPRates())
	if !got.IsZero() {
		t.Fatalf("kolektibilitas tak dikenal harus nol, got %s", got)
	}
}

// Target PPAP dibulatkan ke rupiah penuh, memakai Banker's Rounding yang sama
// dengan seluruh perhitungan uang sistem.
func TestPPAPAmount_RoundsToRupiah(t *testing.T) {
	rates := domain.DefaultPPAPRates()

	// 333.333 x 0,5% = 1.666,665 -> 1.667
	got := domain.PPAPAmount(decimal.NewFromInt(333_333), domain.KolLancar, rates)
	if !got.Equal(decimal.NewFromInt(1667)) {
		t.Fatalf("pembulatan 1666,665: got %s, want 1667", got)
	}

	// Banker's rounding: 1 x 50% = 0,5 -> 0 (ke genap); 3 x 50% = 1,5 -> 2.
	half := domain.PPAPRates{domain.KolLancar: domain.PPAPRate(decimal.NewFromFloat(0.5))}
	if got := domain.PPAPAmount(decimal.NewFromInt(1), domain.KolLancar, half); !got.IsZero() {
		t.Fatalf("0,5 harus dibulatkan ke 0, got %s", got)
	}
	if got := domain.PPAPAmount(decimal.NewFromInt(3), domain.KolLancar, half); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("1,5 harus dibulatkan ke 2, got %s", got)
	}
}

func TestPPAPAdjustment(t *testing.T) {
	tests := []struct {
		name     string
		target   decimal.Decimal
		existing decimal.Decimal
		want     decimal.Decimal
	}{
		{"target lebih besar (provisi)", decimal.NewFromInt(100_000), decimal.NewFromInt(40_000), decimal.NewFromInt(60_000)},
		{"target lebih kecil (reversal)", decimal.NewFromInt(40_000), decimal.NewFromInt(100_000), decimal.NewFromInt(-60_000)},
		{"sama (tidak ada jurnal)", decimal.NewFromInt(100_000), decimal.NewFromInt(100_000), decimal.Zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.PPAPAdjustment(tt.target, tt.existing)
			if !got.Equal(tt.want) {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCollectibility_LabelAndNPL(t *testing.T) {
	labels := map[domain.Collectibility]string{
		domain.KolLancar:       "Lancar",
		domain.KolDPK:          "Dalam Perhatian Khusus",
		domain.KolKurangLancar: "Kurang Lancar",
		domain.KolDiragukan:    "Diragukan",
		domain.KolMacet:        "Macet",
	}
	for col, want := range labels {
		if got := col.Label(); got != want {
			t.Errorf("label %d: got %q, want %q", col, got, want)
		}
	}

	if domain.KolDPK.IsNPL() {
		t.Error("DPK bukan NPL")
	}
	for _, col := range []domain.Collectibility{domain.KolKurangLancar, domain.KolDiragukan, domain.KolMacet} {
		if !col.IsNPL() {
			t.Errorf("%s harus dianggap NPL", col.Label())
		}
	}
}

func TestCollectibility_OJKCodeRoundTrip(t *testing.T) {
	for col := domain.KolLancar; col <= domain.KolMacet; col++ {
		if got := domain.CollectibilityFromOJK(col.OJKCode()); got != col {
			t.Errorf("round-trip %s: got %s", col.Label(), got.Label())
		}
	}
}
