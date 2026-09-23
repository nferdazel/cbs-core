package domain_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// mapConfig adalah stub SystemConfigService untuk pengujian murni tanpa database.
type mapConfig struct{ values map[string]string }

func (c *mapConfig) get(key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func (c *mapConfig) GetString(_ context.Context, key, fallback string) string {
	return c.get(key, fallback)
}

func (c *mapConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := c.values[key]; ok {
		if d, err := decimal.NewFromString(v); err == nil {
			return d
		}
	}
	return fallback
}

func (c *mapConfig) GetInt(_ context.Context, key string, fallback int) int { return fallback }

func (c *mapConfig) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := c.values[key]; ok {
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return fallback
}

func (c *mapConfig) Invalidate(string) {}

var _ domain.SystemConfigService = (*mapConfig)(nil)

// Bukti ratifikasi (keputusan panel butir 1): status FINAL hanya sah bila kelima kunci
// bukti lengkap. Fungsi murni ini diperiksa tanpa database; trigger 000095 menegakkan
// aturan yang sama pada jalur SQL.
func TestCKPNRatificationReadiness(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	buktiLengkap := func() *mapConfig {
		return &mapConfig{values: map[string]string{
			domain.ConfigKeyCKPNRatificationBANumber:      "BA/001/DIR-2026",
			domain.ConfigKeyCKPNRatificationBADate:        "2026-09-01",
			domain.ConfigKeyCKPNRatificationApprovedBy:    "Direktur A & Akuntan B",
			domain.ConfigKeyCKPNRatificationPDLGDBasis:    "12.6 net flow 3 tahun, lampiran BA/001",
			domain.ConfigKeyCKPNRatificationPDLGDFromBank: "true",
		}}
	}

	// Lengkap -> siap.
	if ready, missing := domain.CKPNRatificationReadiness(context.Background(), buktiLengkap(), now); !ready || len(missing) != 0 {
		t.Fatalf("bukti lengkap harus siap, ready=%v missing=%v", ready, missing)
	}

	// Kosong -> tidak siap, dan pesannya menyebut apa yang kurang (kelima kunci).
	if ready, missing := domain.CKPNRatificationReadiness(context.Background(), &mapConfig{values: map[string]string{}}, now); ready || len(missing) != 5 {
		t.Fatalf("bukti kosong harus menolak dan menyebut 5 kunci, ready=%v missing=%v", ready, missing)
	}

	// PD/LGD bukan dari data bank -> tidak siap.
	cfg := buktiLengkap()
	cfg.values[domain.ConfigKeyCKPNRatificationPDLGDFromBank] = "false"
	if ready, missing := domain.CKPNRatificationReadiness(context.Background(), cfg, now); ready || len(missing) != 1 {
		t.Fatalf("pd_lgd_from_bank=false harus menolak, ready=%v missing=%v", ready, missing)
	}

	// Tanggal masa depan -> tidak siap.
	cfg = buktiLengkap()
	cfg.values[domain.ConfigKeyCKPNRatificationBADate] = "2026-10-01"
	if ready, missing := domain.CKPNRatificationReadiness(context.Background(), cfg, now); ready || len(missing) != 1 {
		t.Fatalf("tanggal masa depan harus menolak, ready=%v missing=%v", ready, missing)
	}

	// Tanggal salah format -> tidak siap.
	cfg = buktiLengkap()
	cfg.values[domain.ConfigKeyCKPNRatificationBADate] = "01-09-2026"
	if ready, missing := domain.CKPNRatificationReadiness(context.Background(), cfg, now); ready || len(missing) != 1 {
		t.Fatalf("tanggal salah format harus menolak, ready=%v missing=%v", ready, missing)
	}

	// cfg nil -> gagal-aman, tidak siap.
	if ready, _ := domain.CKPNRatificationReadiness(context.Background(), nil, now); ready {
		t.Fatal("cfg nil tidak boleh dianggap siap")
	}
}

// Status parameter wajib membawa kelengkapan bukti dan daftar penahan penyalakan
// (keputusan panel butir 1 & 2), sehingga operator tahu apa yang kurang.
func TestCKPNParametersStatusMembawaBuktiDanPenahan(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)

	kosong := &mapConfig{values: map[string]string{}}
	st := domain.CKPNParametersStatusFromConfig(ctx, kosong, now)
	if !st.Sementara || st.RatificationReady {
		t.Fatalf("bawaan harus SEMENTARA dan bukti belum siap: %+v", st)
	}
	if len(st.RatificationMissing) != 5 {
		t.Fatalf("RatificationMissing = %d, mau 5: %v", len(st.RatificationMissing), st.RatificationMissing)
	}
	if len(st.EnablementGaps) == 0 {
		t.Fatal("EnablementGaps wajib menyebut penahan selama parameter belum siap")
	}

	// Lengkap + FINAL + parameter + syariah terpetakan -> tidak ada penahan.
	lengkap := &mapConfig{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus:          domain.CKPNParameterStatusFinal,
		domain.ConfigKeyCKPNRatificationBANumber:      "BA/001",
		domain.ConfigKeyCKPNRatificationBADate:        "2026-09-01",
		domain.ConfigKeyCKPNRatificationApprovedBy:    "Direksi + Akuntan",
		domain.ConfigKeyCKPNRatificationPDLGDBasis:    "12.6/12.7 data historis",
		domain.ConfigKeyCKPNRatificationPDLGDFromBank: "true",
		"ckpn.pd_frac.gol_1":                          "0.0111",
		"ckpn.pd_frac.gol_2":                          "0.0667",
		"ckpn.pd_frac.gol_3":                          "0.2222",
		"ckpn.pd_frac.gol_4":                          "1",
		"ckpn.pd_frac.gol_5":                          "1",
		"ckpn.lgd_frac":                               "0.45",
		"ckpn.coa.expense.syariah":                    "15901",
		"ckpn.coa.reserve.syariah":                    "11950",
	}}
	st = domain.CKPNParametersStatusFromConfig(ctx, lengkap, now)
	if !st.RatificationReady || len(st.RatificationMissing) != 0 {
		t.Fatalf("bukti lengkap harus siap: %+v", st)
	}
	if len(st.EnablementGaps) != 0 {
		t.Fatalf("tidak boleh ada penahan bila parameter+ratifikasi+akun syariah lengkap: %v", st.EnablementGaps)
	}

	// PD di luar 0..1 (persen keliru) -> penahan menyebut kuncinya.
	lengkap.values["ckpn.pd_frac.gol_3"] = "22.22"
	st = domain.CKPNParametersStatusFromConfig(ctx, lengkap, now)
	if len(st.EnablementGaps) == 0 {
		t.Fatal("PD di luar 0..1 harus menjadi penahan penyalakan")
	}
}
