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

// ConfigKeyExister adalah kemampuan opsional untuk membedakan kunci yang benar-benar
// ada di system_config dari kunci yang tidak ada (sehingga memakai fallback).
// GetDecimal/GetString mengembalikan fallback untuk kedua keadaan itu, jadi antarmuka
// ini dipisah agar implementasi baca-saja sederhana tidak wajib mengikutinya.
type ConfigKeyExister interface {
	Exists(ctx context.Context, key string) bool
}

// ConfigRawValueReader adalah kemampuan opsional membaca nilai mentah beserta ada
// tidaknya kunci. Dipakai pembaca yang harus membedakan kunci yang sengaja di-seed
// kosong (berarti "pakai fallback produk") dari kunci berangka, tanpa memicu peringatan
// "bukan angka" pada GetDecimal. Stub yang tidak menyediakannya tetap memakai GetDecimal.
type ConfigRawValueReader interface {
	RawValue(ctx context.Context, key string) (string, bool)
}
