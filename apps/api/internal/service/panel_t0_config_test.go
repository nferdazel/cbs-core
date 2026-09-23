package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kerangka T0 (keputusan panel butir 3 & 4): pembaca konfigurasi harus benar-benar
// meminta kunci yang di-seed migrasi 000094, dengan bawaan aman. Uji ini menjaga agar
// kunci yang di-seed tidak menjadi basi (invarian seed memeriksa pembacanya statis;
// uji ini membuktikan nama kunci yang diminta).

func TestPanelT0_CKPNIndividualPolicyMemintaSeluruhKunci(t *testing.T) {
	ctx := context.Background()
	cfg := &keyCapturingConfig{}

	p := ckpnIndividualPolicy(ctx, cfg)

	diminta := map[string]bool{}
	for _, k := range cfg.keys {
		diminta[k] = true
	}
	for _, key := range []string{
		cfgCKPNIndividualEnabled,
		cfgCKPNIndividualSignificanceAmount,
		cfgCKPNIndividualSignificanceTopN,
		cfgCKPNIndividualMethod,
		cfgCKPNIndividualDiscountRateAnnualPct,
		cfgCKPNIndividualMandatoryOnMacet,
		cfgCKPNIndividualMandatoryOnRestructured,
		cfgCKPNIndividualMandatoryDPDDays,
		cfgCKPNIndividualMandatoryOnCollateralDrop,
		cfgCKPNIndividualMandatoryOnObjectiveEvi,
	} {
		if !diminta[key] {
			t.Errorf("ckpnIndividualPolicy tidak meminta kunci %s", key)
		}
	}

	if p.Enabled {
		t.Fatal("bawaan CKPN individual harus MATI (T0: nol perubahan perilaku)")
	}
	if p.Method != domain.CKPNIndividualMethodMax {
		t.Fatalf("metode bawaan %s, mau MAX", p.Method)
	}
	if p.SignificanceTopN != 20 {
		t.Fatalf("significance_top_n bawaan %d, mau 20", p.SignificanceTopN)
	}
	if !p.SignificanceAmount.Equal(decimal.NewFromInt(1000000000)) {
		t.Fatalf("significance_amount bawaan %s, mau 1.000.000.000", p.SignificanceAmount)
	}
	if !p.MandatoryOnMacet || !p.MandatoryOnRestructured || !p.MandatoryOnCollateralDrop || !p.MandatoryOnObjectiveEvidence {
		t.Fatal("pemicu non-nominal wajib harus menyala pada bawaan")
	}
	if p.MandatoryDPDDays != 90 {
		t.Fatalf("mandatory_dpd_days bawaan %d, mau 90", p.MandatoryDPDDays)
	}
}

func TestPanelT0_CollateralWeightPolicyMemintaSeluruhKunci(t *testing.T) {
	ctx := context.Background()
	cfg := &keyCapturingConfig{}

	p := collateralWeightActivationPolicy(ctx, cfg)

	diminta := map[string]bool{}
	for _, k := range cfg.keys {
		diminta[k] = true
	}
	for _, key := range []string{cfgCollateralWeightShadowMonths, cfgCollateralWeightCoverageMinFrac} {
		if !diminta[key] {
			t.Errorf("collateralWeightActivationPolicy tidak meminta kunci %s", key)
		}
	}
	if p.ShadowMonths != 2 {
		t.Fatalf("shadow_months bawaan %d, mau 2", p.ShadowMonths)
	}
	if !p.CoverageMinFrac.Equal(decimal.NewFromFloat(0.90)) {
		t.Fatalf("coverage_min_frac bawaan %s, mau 0.90", p.CoverageMinFrac)
	}
}

// Panel butir 2.1(6): saldo dana kebajikan yang tidak bergerak > 2 kuartal (6 bulan)
// memicu peringatan TINGKAT TINGGI. Satu kuartal tetap peringatan biasa, bukan tinggi.
func TestPanelT0_PeringatanTinggiSaldoMengendapEnamBulan(t *testing.T) {
	business := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	saldo := decimal.NewFromInt(60_000)

	baru := socialFundIdleWarning(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), saldo, business)
	if baru != "" {
		t.Fatalf("pergerakan dalam satu kuartal tidak boleh diperingatkan: %q", baru)
	}

	satuKuartal := socialFundIdleWarning(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), saldo, business)
	if !strings.Contains(satuKuartal, "Dana kebajikan (akun 12500)") {
		t.Fatalf("lewat satu kuartal harus ada peringatan: %q", satuKuartal)
	}
	if strings.Contains(satuKuartal, "PERINGATAN TINGGI") {
		t.Fatalf("lewat satu kuartal BUKAN tingkat tinggi: %q", satuKuartal)
	}

	duaKuartal := socialFundIdleWarning(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), saldo, business)
	if !strings.Contains(duaKuartal, "PERINGATAN TINGGI") {
		t.Fatalf("lewat dua kuartal (6 bulan) harus peringatan tingkat tinggi: %q", duaKuartal)
	}
}
