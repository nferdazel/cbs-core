package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// keyCapturingConfig mencatat setiap kunci yang diminta pembaca konfigurasi, supaya
// kunci yang dibangun saat runtime (bukan literali utuh) dapat diperiksa namanya.
type keyCapturingConfig struct{ keys []string }

func (c *keyCapturingConfig) capture(key string) { c.keys = append(c.keys, key) }

func (c *keyCapturingConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	c.capture(key)
	return fallback
}

func (c *keyCapturingConfig) GetInt(_ context.Context, key string, fallback int) int {
	c.capture(key)
	return fallback
}

func (c *keyCapturingConfig) GetString(_ context.Context, key, fallback string) string {
	c.capture(key)
	return fallback
}

func (c *keyCapturingConfig) GetBool(_ context.Context, key string, fallback bool) bool {
	c.capture(key)
	return fallback
}

func (c *keyCapturingConfig) Invalidate(string) {}

var _ domain.SystemConfigService = (*keyCapturingConfig)(nil)

// TestConfigKeyConventionSatuanDanIndeks menjaga konvensi W3 pada kunci yang dibangun
// saat runtime sehingga tidak terlihat pemindai literali statis: tarif PPAP dan PD
// CKPN memakai satuan _frac dengan indeks nama gol_<n>, dan haircut agunan memakai
// satuan _pct. Tanpa uji ini, perubahan kunci runtime bisa lolos dari invarian seed
// (enumerasi di runtimeGeneratedConfigKeys ditulis terpisah dari kode produksi).
func TestConfigKeyConventionSatuanDanIndeks(t *testing.T) {
	ctx := context.Background()
	cfg := &keyCapturingConfig{}
	_ = collectibilityRates(ctx, cfg)

	wantPPAP := map[string]bool{}
	for i := 1; i <= 5; i++ {
		wantPPAP[fmt.Sprintf("ppap.rate_frac.gol_%d", i)] = true
	}
	seen := map[string]bool{}
	for _, key := range cfg.keys {
		if !wantPPAP[key] {
			t.Fatalf("collectibilityRates meminta kunci %q, di luar pola ppap.rate_frac.gol_<n>", key)
		}
		seen[key] = true
	}
	for key := range wantPPAP {
		if !seen[key] {
			t.Fatalf("collectibilityRates tidak meminta %q", key)
		}
	}

	for i, c := range ckpnCollectibilityOrder {
		want := fmt.Sprintf("ckpn.pd_frac.gol_%d", i+1)
		if got := ckpnPDKey(c); got != want {
			t.Fatalf("ckpnPDKey(%s) = %q, mau %q", c.Label(), got, want)
		}
	}

	for _, jenis := range []domain.CollateralType{
		domain.CollateralTanahBangunan,
		domain.CollateralKendaraan,
		domain.CollateralDeposit,
		domain.CollateralMesinPeralatan,
		domain.CollateralLainnya,
	} {
		key := domain.HaircutConfigKey(jenis)
		if !strings.HasSuffix(key, "_pct") {
			t.Fatalf("HaircutConfigKey(%s) = %q, wajib bersufiks _pct", jenis, key)
		}
	}
}
