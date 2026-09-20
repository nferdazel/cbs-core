package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Pita kolektibilitas mengikuti POJK No. 1 Tahun 2024 (Kualitas Aset Bank
// Perekonomian Rakyat) Lampiran II, baris "Kredit dengan angsuran 1 bulan atau
// lebih". Tunggakan 1-30 hari yang belum jatuh tempo MASIH Lancar; itu bagian dari
// definisi Lancar, bukan kelonggaran.
func TestCollectibilityFromDPD_MonthlyBands(t *testing.T) {
	thresholds := domain.DefaultCollectibilityThresholds()

	tests := []struct {
		name string
		dpd  int
		want domain.Collectibility
	}{
		{"tepat waktu", 0, domain.KolLancar},
		{"tunggakan 1 hari masih Lancar", 1, domain.KolLancar},
		{"tunggakan 30 hari batas atas Lancar", 30, domain.KolLancar},
		{"tunggakan 31 hari masuk DPK", 31, domain.KolDPK},
		{"tunggakan 90 hari batas atas DPK", 90, domain.KolDPK},
		{"tunggakan 91 hari masuk Kurang Lancar", 91, domain.KolKurangLancar},
		{"tunggakan 180 hari batas atas Kurang Lancar", 180, domain.KolKurangLancar},
		{"tunggakan 181 hari masuk Diragukan", 181, domain.KolDiragukan},
		{"tunggakan 360 hari batas atas Diragukan", 360, domain.KolDiragukan},
		{"tunggakan 361 hari masuk Macet", 361, domain.KolMacet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.CollectibilityFromDPD(tt.dpd, thresholds); got != tt.want {
				t.Fatalf("DPD %d: got %s, want %s", tt.dpd, got.Label(), tt.want.Label())
			}
		})
	}
}

// Pita angsuran kurang dari 1 bulan (Lancar 15 / DPK 30 / Kurang Lancar 90 /
// Diragukan 180) disediakan lebih dulu meski jadwal produksi masih bulanan.
func TestCollectibilityFromDPD_SubMonthlyBands(t *testing.T) {
	thresholds := domain.SubMonthlyCollectibilityThresholds()

	tests := []struct {
		dpd  int
		want domain.Collectibility
	}{
		{0, domain.KolLancar},
		{15, domain.KolLancar},
		{16, domain.KolDPK},
		{30, domain.KolDPK},
		{31, domain.KolKurangLancar},
		{90, domain.KolKurangLancar},
		{91, domain.KolDiragukan},
		{180, domain.KolDiragukan},
		{181, domain.KolMacet},
	}

	for _, tt := range tests {
		if got := domain.CollectibilityFromDPD(tt.dpd, thresholds); got != tt.want {
			t.Errorf("angsuran <1 bulan DPD %d: got %s, want %s", tt.dpd, got.Label(), tt.want.Label())
		}
	}
}

// Dimensi "Kredit telah jatuh tempo" (15/30/60 hari) dinilai terpisah dari
// tunggakan angsuran; yang berlaku adalah golongan terburuk di antara keduanya.
func TestCollectibilityFromPosition_TakesWorseOfArrearsAndMaturity(t *testing.T) {
	thresholds := domain.DefaultCollectibilityThresholds()

	tests := []struct {
		name             string
		dpd              int
		daysPastMaturity int
		want             domain.Collectibility
	}{
		{"lancar dan belum jatuh tempo", 0, 0, domain.KolLancar},
		{"tunggakan 20 hari, belum jatuh tempo", 20, 0, domain.KolLancar},
		{"tunggakan 0 hari tetapi jatuh tempo 10 hari", 0, 10, domain.KolDPK},
		{"tunggakan 200 hari tetapi belum jatuh tempo", 200, 0, domain.KolDiragukan},
		{"tunggakan 0 hari tetapi jatuh tempo 45 hari", 0, 45, domain.KolDiragukan},
		{"tunggakan 0 hari tetapi jatuh tempo 90 hari", 0, 90, domain.KolMacet},
		{"jatuh tempo 20 hari menang atas tunggakan 0", 0, 20, domain.KolKurangLancar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.CollectibilityFromPosition(tt.dpd, tt.daysPastMaturity, thresholds)
			if got != tt.want {
				t.Fatalf("dpd=%d jatuh tempo=%d: got %s, want %s",
					tt.dpd, tt.daysPastMaturity, got.Label(), tt.want.Label())
			}
		})
	}
}

// Konfigurasi rusak tidak boleh menghasilkan golongan lebih baik daripada yang
// dimaksud konfigurasi.
func TestCollectibilityFromDPD_InvalidThresholdsFallBack(t *testing.T) {
	// Batas Kurang Lancar (120) tidak menaik di atas DPK (200): dikembalikan ke
	// default 180, sehingga DPD 100 tetap Kurang Lancar.
	got := domain.CollectibilityFromDPD(100, domain.CollectibilityThresholds{Lancar: 30, DPK: 200, KurangLancar: 120, Diragukan: 360})
	if got != domain.KolKurangLancar {
		t.Fatalf("ambang tidak menaik: got %s, want Kurang Lancar", got.Label())
	}

	// Ambang kosong memakai default POJK.
	got = domain.CollectibilityFromDPD(0, domain.CollectibilityThresholds{})
	if got != domain.KolLancar {
		t.Fatalf("ambang kosong: got %s, want Lancar", got.Label())
	}

	// Toleransi Lancar bawaan (30) lebih longgar daripada batas DPK yang ketat (10):
	// toleransi dipersempit, ambang ketat operator tetap berlaku.
	got = domain.CollectibilityFromDPD(25, domain.CollectibilityThresholds{DPK: 10, KurangLancar: 20, Diragukan: 30})
	if got != domain.KolDiragukan {
		t.Fatalf("ambang ketat: got %s, want Diragukan", got.Label())
	}
}

// Invarian inti: konfigurasi apa pun tidak boleh menggolongkan Kredit lebih ringan
// daripada default POJK. Bank boleh lebih konservatif, tidak boleh lebih longgar.
func TestCollectibilityFromDPD_NeverLighterThanDefault(t *testing.T) {
	configs := []domain.CollectibilityThresholds{
		{},                                      // tidak dikonfigurasi
		{DPK: 0, KurangLancar: 5, Diragukan: 0}, // ambang tidak menaik
		{Lancar: 30, DPK: 30, KurangLancar: 90, Diragukan: 180},   // nilai seed lama
		{Lancar: 30, DPK: 200, KurangLancar: 120, Diragukan: 360}, // lebih longgar dari POJK
		{DPK: 10, KurangLancar: 20, Diragukan: 30},                // sengaja lebih ketat
	}
	def := domain.DefaultCollectibilityThresholds()

	for _, cfg := range configs {
		for dpd := 0; dpd <= 400; dpd += 7 {
			got := domain.CollectibilityFromDPD(dpd, cfg)
			want := domain.CollectibilityFromDPD(dpd, def)
			if got < want {
				t.Fatalf("konfigurasi %+v pada DPD %d memberi %s, lebih ringan dari default %s",
					cfg, dpd, got.Label(), want.Label())
			}
		}
	}
}

// Pasal 23 POJK 1/2024: restrukturisasi tidak boleh menaikkan kualitas Kredit.
func TestRestructureCollectibility_Pasal23(t *testing.T) {
	tests := []struct {
		name         string
		before       domain.Collectibility
		computed     domain.Collectibility
		cleanPeriods int
		want         domain.Collectibility
	}{
		{"sebelum Macet, DPD nol, belum 3 periode bersih", domain.KolMacet, domain.KolLancar, 0, domain.KolKurangLancar},
		{"sebelum Diragukan, 2 periode bersih belum cukup", domain.KolDiragukan, domain.KolLancar, 2, domain.KolKurangLancar},
		{"sebelum Macet dan makin buruk tetap Macet", domain.KolMacet, domain.KolMacet, 0, domain.KolMacet},
		{"sebelum Lancar tetap Lancar", domain.KolLancar, domain.KolLancar, 0, domain.KolLancar},
		{"sebelum DPK tidak boleh membaik", domain.KolDPK, domain.KolLancar, 0, domain.KolDPK},
		{"sebelum Kurang Lancar tidak berubah", domain.KolKurangLancar, domain.KolLancar, 0, domain.KolKurangLancar},
		{"3 periode bersih membebaskan batas", domain.KolMacet, domain.KolLancar, 3, domain.KolLancar},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.RestructureCollectibility(tt.before, tt.computed, tt.cleanPeriods)
			if got != tt.want {
				t.Fatalf("sebelum=%s dihitung=%s bersih=%d: got %s, want %s",
					tt.before.Label(), tt.computed.Label(), tt.cleanPeriods, got.Label(), tt.want.Label())
			}
		})
	}
}

// Tarif PPAP minimum BPR (POJK 1/2024 Pasal 19): PPAP umum 0,5% untuk Lancar, dan
// PPAP khusus 3% / 10% / 50% / 100% atas pokok terutang.
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
