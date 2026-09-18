package service

import (
	"context"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi aturan kolektibilitas. Semua tarif dan ambang DPD dibaca dari
// system_config lewat helper di file ini, sehingga jalur PPAP harian dan jalur
// restrukturisasi kredit selalu memakai aturan yang sama (satu sumber kebenaran).
// Nilai fallback diambil dari domain.DefaultCollectibilityThresholds/DefaultPPAPRates
// agar proses tetap berjalan di lingkungan yang belum di-provision.
const (
	cfgCollectDPKDays          = "ppap.dpd.dpk"
	cfgCollectKurangLancarDays = "ppap.dpd.kurang_lancar"
	cfgCollectDiragukanDays    = "ppap.dpd.diragukan"
	cfgCollectRatePrefix       = "ppap.rate." // ppap.rate.1 .. ppap.rate.5
)

// collectibilityThresholds membaca ambang DPD dari konfigurasi. Fallback POJK:
// DPK 30, Kurang Lancar 90, Diragukan 180.
func collectibilityThresholds(ctx context.Context, config domain.SystemConfigService) domain.CollectibilityThresholds {
	def := domain.DefaultCollectibilityThresholds()
	return domain.CollectibilityThresholds{
		DPK:          configIntOr(ctx, config, cfgCollectDPKDays, def.DPK),
		KurangLancar: configIntOr(ctx, config, cfgCollectKurangLancarDays, def.KurangLancar),
		Diragukan:    configIntOr(ctx, config, cfgCollectDiragukanDays, def.Diragukan),
	}
}

// collectibilityRates membaca tarif PPAP per golongan dari konfigurasi. Fallback
// adalah tarif minimum BPR: 0,5% / 10% / 15% / 50% / 100%.
func collectibilityRates(ctx context.Context, config domain.SystemConfigService) domain.PPAPRates {
	def := domain.DefaultPPAPRates()
	out := make(domain.PPAPRates, len(def))
	for c, r := range def {
		key := fmt.Sprintf("%s%d", cfgCollectRatePrefix, int(c))
		out[c] = domain.PPAPRate(configDecimalOr(ctx, config, key, decimal.Decimal(r)))
	}
	return out
}

// CollectibilityForDPD adalah satu-satunya jalur penentuan golongan kredit beserta
// status akrualnya. Golongan 3-5 (NPL) memakai cash basis sesuai POJK.
func CollectibilityForDPD(ctx context.Context, config domain.SystemConfigService, dpd int) (domain.Collectibility, domain.AccrualStatus) {
	col := domain.CollectibilityFromDPD(dpd, collectibilityThresholds(ctx, config))
	accrual := domain.AccrualStatusAccrual
	if col.IsNPL() {
		accrual = domain.AccrualStatusCash
	}
	return col, accrual
}

// Pembaca konfigurasi dengan toleransi service nil (mis. test) agar fallback tetap dipakai.
func configIntOr(ctx context.Context, config domain.SystemConfigService, key string, fallback int) int {
	if config == nil {
		return fallback
	}
	return config.GetInt(ctx, key, fallback)
}

func configDecimalOr(ctx context.Context, config domain.SystemConfigService, key string, fallback decimal.Decimal) decimal.Decimal {
	if config == nil {
		return fallback
	}
	return config.GetDecimal(ctx, key, fallback)
}

func configStringOr(ctx context.Context, config domain.SystemConfigService, key, fallback string) string {
	if config == nil {
		return fallback
	}
	if v := config.GetString(ctx, key, fallback); v != "" {
		return v
	}
	return fallback
}
