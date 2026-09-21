package service

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// scheduleTermsFor adalah tempat metode jadwal dan jenis imbal hasil diputuskan.
// Untuk akad bagi hasil, margin nominal yang dikirim pemanggil tidak lagi menjadi
// satu-satunya dasar: bila kosong, proyeksi dihitung dari nisbah x proyeksi
// pendapatan usaha produk. Angka itu PROYEKSI, bukan realisasi.
func TestScheduleTermsForBagiHasilDariNisbah(t *testing.T) {
	product := &domain.BankingProduct{
		ProfitScheme:               domain.SchemeMudharabah,
		ProfitSharingRatio:         decimal.NewFromFloat(0.4),
		ProjectedRevenueRateAnnual: decimal.NewFromInt(12),
	}

	method, profitType, margin, err := scheduleTermsFor(product, decimal.Zero, decimal.NewFromInt(12_000_000), 12)
	if err != nil {
		t.Fatalf("scheduleTermsFor: %v", err)
	}
	if method != domain.ScheduleBagiHasil || profitType != domain.ProfitTypeBagiHasil {
		t.Fatalf("method/type = %s/%s, ingin BAGI_HASIL/BAGI_HASIL", method, profitType)
	}
	if !margin.Equal(decimal.NewFromInt(576_000)) {
		t.Fatalf("proyeksi margin %s, ingin 576000 (0.4 x 12%% x 12.000.000)", margin)
	}
	if !margin.IsPositive() {
		t.Fatal("proyeksi bagi hasil tidak boleh nol")
	}
}

// Bila produk belum mengisi nisbah atau proyeksi pendapatan, jadwal bagi hasil
// ditolak dengan pesan yang menyebut apa yang harus diisi bank — bukan diterbitkan
// ber-profit nol.
func TestScheduleTermsForBagiHasilMenolakParameterKosong(t *testing.T) {
	base := &domain.BankingProduct{
		ProfitScheme:               domain.SchemeMudharabah,
		ProfitSharingRatio:         decimal.NewFromFloat(0.4),
		ProjectedRevenueRateAnnual: decimal.NewFromInt(12),
	}

	noNisbah := *base
	noNisbah.ProfitSharingRatio = decimal.Zero
	if _, _, _, err := scheduleTermsFor(&noNisbah, decimal.Zero, decimal.NewFromInt(1_000_000), 12); !errors.Is(err, domain.ErrBagiHasilNisbahMissing) {
		t.Fatalf("nisbah kosong: err = %v, ingin ErrBagiHasilNisbahMissing", err)
	}

	noProjection := *base
	noProjection.ProjectedRevenueRateAnnual = decimal.Zero
	if _, _, _, err := scheduleTermsFor(&noProjection, decimal.Zero, decimal.NewFromInt(1_000_000), 12); !errors.Is(err, domain.ErrBagiHasilProjectionMissing) {
		t.Fatalf("proyeksi kosong: err = %v, ingin ErrBagiHasilProjectionMissing", err)
	}

	// Parameter yang gagal harus menyebut kolom yang perlu diisi bank.
	err := domain.ErrBagiHasilProjectionMissing.Error()
	if err == "" {
		t.Fatal("pesan error proyeksi tidak boleh kosong")
	}
}

// Margin nominal eksplisit tetap dihormati bila pemanggil mengisinya.
func TestScheduleTermsForBagiHasilMarginEksplisitMenang(t *testing.T) {
	product := &domain.BankingProduct{
		ProfitScheme:               domain.SchemeMudharabah,
		ProfitSharingRatio:         decimal.NewFromFloat(0.4),
		ProjectedRevenueRateAnnual: decimal.NewFromInt(12),
	}
	explicit := decimal.NewFromInt(999_000)

	if _, _, margin, err := scheduleTermsFor(product, explicit, decimal.NewFromInt(12_000_000), 12); err != nil || !margin.Equal(explicit) {
		t.Fatalf("margin eksplisit = %s (err=%v), ingin %s", margin, err, explicit)
	}
}

// Produk konvensional (bunga tetap) dan murabahah (margin nominal) tidak boleh
// berubah: margin nominal dipakai apa adanya, bukan diproyeksikan dari nisbah.
func TestScheduleTermsForKonvensionalDanMurabahahTidakBerubah(t *testing.T) {
	margin := decimal.NewFromInt(1_500_000)

	murabahah := &domain.BankingProduct{ProfitScheme: domain.SchemeMurabahah}
	method, profitType, got, err := scheduleTermsFor(murabahah, margin, decimal.NewFromInt(10_000_000), 12)
	if err != nil || method != domain.ScheduleFlat || profitType != domain.ProfitTypeMargin || !got.Equal(margin) {
		t.Fatalf("murabahah = %s/%s/%s (err=%v), ingin FLAT/MARGIN/%s", method, profitType, got, err, margin)
	}

	conv := &domain.BankingProduct{ProfitScheme: domain.SchemeInterest, ScheduleMethod: domain.ScheduleAnnuity}
	method, profitType, got, err = scheduleTermsFor(conv, margin, decimal.NewFromInt(10_000_000), 12)
	if err != nil || method != domain.ScheduleAnnuity || profitType != domain.ProfitTypeInterest || !got.Equal(margin) {
		t.Fatalf("konvensional = %s/%s/%s (err=%v), ingin ANNUITY/INTEREST/%s", method, profitType, got, err, margin)
	}
}
