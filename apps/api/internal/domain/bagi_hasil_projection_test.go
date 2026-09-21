package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Proyeksi bagi hasil memakai tarif ekuivalen = nisbah x proyeksi pendapatan
// tahunan usaha. Angka ini proyeksi, bukan realisasi.
func TestProjectBagiHasilMargin(t *testing.T) {
	principal := decimal.NewFromInt(12_000_000)
	nisbah := decimal.NewFromFloat(0.4)
	projected := decimal.NewFromInt(12) // 12% per tahun
	term := 12

	got, err := ProjectBagiHasilMargin(principal, nisbah, projected, term)
	if err != nil {
		t.Fatalf("ProjectBagiHasilMargin: %v", err)
	}
	// 12.000.000 x (0.4 x 12%) x 12/12 = 576.000.
	want := decimal.NewFromInt(576_000)
	if !got.Equal(want) {
		t.Fatalf("proyeksi %s, ingin %s", got, want)
	}
	if !got.IsPositive() {
		t.Fatal("proyeksi bagi hasil tidak boleh nol/negatif bila parameter terisi")
	}

	// Ekuivalen dengan bunga sederhana pada tarif ekuivalen.
	equivalent := nisbah.Mul(projected)
	if !got.Equal(RoundToRupiah(interestForTerm(principal, equivalent, term))) {
		t.Fatal("proyeksi tidak sama dengan tarif ekuivalen nisbah x proyeksi pendapatan")
	}
}

// Parameter yang belum diisi harus ditolak, bukan menghasilkan bagi hasil nol yang
// menyesatkan (nasabah tetap menanggung pokok tanpa imbal hasil yang dilaporkan).
func TestProjectBagiHasilMarginMenolakParameterKosong(t *testing.T) {
	principal := decimal.NewFromInt(12_000_000)
	term := 12

	if _, err := ProjectBagiHasilMargin(principal, decimal.Zero, decimal.NewFromInt(12), term); !errors.Is(err, ErrBagiHasilNisbahMissing) {
		t.Fatalf("nisbah nol: err = %v, ingin ErrBagiHasilNisbahMissing", err)
	}
	if _, err := ProjectBagiHasilMargin(principal, decimal.NewFromFloat(0.4), decimal.Zero, term); !errors.Is(err, ErrBagiHasilProjectionMissing) {
		t.Fatalf("proyeksi nol: err = %v, ingin ErrBagiHasilProjectionMissing", err)
	}
}

// Nilai di luar rentang wajar hampir pasti salah satuan. Nisbah 40 berarti 4000%,
// dan proyeksi pendapatan 1200 berarti 1200%/tahun; keduanya harus ditolak dengan
// pesan yang menyebut satuan yang benar, bukan dipakai diam-diam.
func TestProjectBagiHasilMarginMenolakNilaiDiLuarRentang(t *testing.T) {
	principal := decimal.NewFromInt(12_000_000)
	term := 12

	if _, err := ProjectBagiHasilMargin(principal, decimal.NewFromInt(40), decimal.NewFromInt(12), term); !errors.Is(err, ErrBagiHasilNisbahOutOfRange) {
		t.Fatalf("nisbah 40: err = %v, ingin ErrBagiHasilNisbahOutOfRange", err)
	}
	if _, err := ProjectBagiHasilMargin(principal, decimal.NewFromFloat(0.4), decimal.NewFromInt(1200), term); !errors.Is(err, ErrBagiHasilProjectionOutOfRange) {
		t.Fatalf("proyeksi 1200: err = %v, ingin ErrBagiHasilProjectionOutOfRange", err)
	}

	// Nilai tepat pada batas atas masih sah.
	if _, err := ProjectBagiHasilMargin(principal, decimal.NewFromInt(1), decimal.NewFromInt(100), term); err != nil {
		t.Fatalf("nisbah 1 dan proyeksi 100 pada batas atas harus diterima: %v", err)
	}
}

// Validator rentang dipakai bersama jalur jadwal dan jalur ubah parameter produk.
// Nilai nol sah pada tingkat skema (berarti belum diisi); yang menolak nol saat
// membentuk jadwal adalah ProjectBagiHasilMargin, bukan validator ini.
func TestValidateParameterBagiHasil(t *testing.T) {
	for _, n := range []decimal.Decimal{decimal.Zero, decimal.RequireFromString("0.4"), decimal.NewFromInt(1)} {
		if err := ValidateProfitSharingRatio(n); err != nil {
			t.Fatalf("nisbah %s harus sah: %v", n, err)
		}
	}
	for _, n := range []decimal.Decimal{decimal.RequireFromString("-0.1"), decimal.NewFromInt(40)} {
		if err := ValidateProfitSharingRatio(n); !errors.Is(err, ErrBagiHasilNisbahOutOfRange) {
			t.Fatalf("nisbah %s: err = %v, ingin ErrBagiHasilNisbahOutOfRange", n, err)
		}
	}
	for _, p := range []decimal.Decimal{decimal.Zero, decimal.NewFromInt(12), decimal.NewFromInt(100)} {
		if err := ValidateProjectedRevenueRateAnnual(p); err != nil {
			t.Fatalf("proyeksi %s harus sah: %v", p, err)
		}
	}
	for _, p := range []decimal.Decimal{decimal.RequireFromString("-1"), decimal.NewFromInt(1200)} {
		if err := ValidateProjectedRevenueRateAnnual(p); !errors.Is(err, ErrBagiHasilProjectionOutOfRange) {
			t.Fatalf("proyeksi %s: err = %v, ingin ErrBagiHasilProjectionOutOfRange", p, err)
		}
	}
}

// Produk konvensional (bunga tetap) dan murabahah (margin nominal) tidak boleh
// berubah karena kolom proyeksi hanya dibaca jalur bagi hasil.
func TestBuildScheduleKonvensionalDanMurabahahTidakBerubah(t *testing.T) {
	principal := decimal.NewFromInt(10_000_000)

	conventional, conventionalTotal, _ := BuildSchedule(uuid.New(), ScheduleParams{
		Principal: principal, AnnualRate: decimal.NewFromInt(12), TermMonths: 12,
		Method: ScheduleFlat, ProfitType: ProfitTypeInterest,
	})
	if !conventionalTotal.Equal(decimal.NewFromInt(11_200_000)) {
		t.Fatalf("konvensional total %s, ingin 11200000", conventionalTotal)
	}
	if conventional[0].ProfitType != ProfitTypeInterest || !conventional[0].ProfitAmount.Equal(decimal.NewFromInt(100_000)) {
		t.Fatalf("konvensional angsuran pertama %+v", conventional[0])
	}

	margin := decimal.NewFromInt(1_500_000)
	murabahah, murabahahTotal, _ := BuildSchedule(uuid.New(), ScheduleParams{
		Principal: principal, Margin: margin, TermMonths: 12,
		Method: ScheduleFlat, ProfitType: ProfitTypeMargin,
	})
	if !murabahahTotal.Equal(principal.Add(margin)) {
		t.Fatalf("murabahah total %s, ingin principal+margin", murabahahTotal)
	}
	if !murabahah[0].ProfitAmount.Equal(decimal.NewFromInt(125_000)) {
		t.Fatalf("murabahah angsuran pertama %s, ingin 125000", murabahah[0].ProfitAmount)
	}
}
