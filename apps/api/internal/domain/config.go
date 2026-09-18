package domain

import (
	"context"

	"github.com/shopspring/decimal"
)

// SystemConfigService membaca konfigurasi bisnis dari tabel system_config. Nilai
// yang hilang atau rusak tidak membuat pemanggil gagal: fallback yang diberikan
// dipakai, sehingga transaksi tetap berjalan di lingkungan yang belum di-provision.
type SystemConfigService interface {
	GetDecimal(ctx context.Context, key string, fallback decimal.Decimal) decimal.Decimal
	GetInt(ctx context.Context, key string, fallback int) int
	GetString(ctx context.Context, key string, fallback string) string
	GetBool(ctx context.Context, key string, fallback bool) bool
	// Invalidate membuang cache satu key agar perubahan konfigurasi langsung berlaku.
	Invalidate(key string)
}
