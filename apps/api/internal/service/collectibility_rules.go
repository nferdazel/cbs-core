package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi aturan kolektibilitas. Semua tarif dan ambang DPD dibaca dari
// system_config lewat helper di file ini, sehingga jalur PPAP harian dan jalur
// restrukturisasi kredit selalu memakai aturan yang sama (satu sumber kebenaran).
// Nilai fallback diambil dari domain.DefaultCollectibilityThresholds/DefaultPPAPRates
// agar proses tetap berjalan di lingkungan yang belum di-provision.
const (
	cfgCollectLancarDays       = "ppap.dpd.lancar"
	cfgCollectDPKDays          = "ppap.dpd.dpk"
	cfgCollectKurangLancarDays = "ppap.dpd.kurang_lancar"
	cfgCollectDiragukanDays    = "ppap.dpd.diragukan"
	cfgCollectRatePrefix       = "ppap.rate." // ppap.rate.1 .. ppap.rate.5
)

// collectibilityThresholds membaca ambang DPD dari konfigurasi. Fallback POJK
// 1/2024 Lampiran II (angsuran bulanan): Lancar 30, DPK 90, Kurang Lancar 180,
// Diragukan 360.
func collectibilityThresholds(ctx context.Context, config domain.SystemConfigService) domain.CollectibilityThresholds {
	def := domain.DefaultCollectibilityThresholds()
	return domain.CollectibilityThresholds{
		Lancar:       configIntOr(ctx, config, cfgCollectLancarDays, def.Lancar),
		DPK:          configIntOr(ctx, config, cfgCollectDPKDays, def.DPK),
		KurangLancar: configIntOr(ctx, config, cfgCollectKurangLancarDays, def.KurangLancar),
		Diragukan:    configIntOr(ctx, config, cfgCollectDiragukanDays, def.Diragukan),
	}
}

// collectibilityRates membaca tarif PPAP per golongan dari konfigurasi. Fallback
// adalah tarif minimum BPR (POJK 1/2024 Pasal 19): 0,5% / 3% / 10% / 50% / 100%.
func collectibilityRates(ctx context.Context, config domain.SystemConfigService) domain.PPAPRates {
	def := domain.DefaultPPAPRates()
	out := make(domain.PPAPRates, len(def))
	for c, r := range def {
		key := fmt.Sprintf("%s%d", cfgCollectRatePrefix, int(c))
		out[c] = domain.PPAPRate(configDecimalOr(ctx, config, key, decimal.Decimal(r)))
	}
	return out
}

// CollectibilityForPosition adalah satu-satunya jalur penentuan golongan kredit di
// service: ia memakai dimensi tunggakan angsuran (dpd) dan dimensi jatuh tempo Kredit
// dengan konfigurasi yang sama, sehingga PPAP harian, restrukturisasi, dan akrual
// bunga tidak pernah memakai aturan yang berbeda.
func CollectibilityForPosition(ctx context.Context, config domain.SystemConfigService, dpd, daysPastMaturity int) domain.Collectibility {
	return domain.CollectibilityFromPosition(dpd, daysPastMaturity, collectibilityThresholds(ctx, config))
}

// DaysPastMaturity menghitung umur Kredit sejak jatuh tempo terakhirnya dalam hari.
// Nilai 0 berarti Kredit belum jatuh tempo, atau jadwalnya belum diketahui.
func DaysPastMaturity(asOf time.Time, finalDueDate *time.Time) int {
	if finalDueDate == nil {
		return 0
	}
	return daysPastDue(asOf, *finalDueDate)
}

// AccrualForCollectibility memetakan golongan akhir ke status akrualnya. Wajib
// dipakai setelah golongan diubah (mis. dibatasi Pasal 31), agar penghentian akrual
// selalu mengikuti golongan yang benar-benar disimpan.
func AccrualForCollectibility(c domain.Collectibility) domain.AccrualStatus {
	if c.IsNPL() {
		return domain.AccrualStatusCash
	}
	return domain.AccrualStatusAccrual
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
