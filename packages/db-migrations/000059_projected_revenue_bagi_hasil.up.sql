-- CBS Migration 000059: proyeksi pendapatan pembiayaan bagi hasil
-- Run after: 000058_maker_checker_business_date.up.sql
--
-- Latar belakang:
--   Jadwal angsuran akad bagi hasil (mudharabah/musyarakah) sebelumnya memakai
--   margin nominal yang harus diisi pemanggil. Bila tidak diisi, seluruh angsuran
--   ber-profit nol, padahal nasabah tetap menanggung pokok. Keputusan pemilik sistem:
--   jadwal memakai TARIF EKUIVALEN dari produk, yaitu nisbah x proyeksi pendapatan
--   usaha yang dibiayai. banking_products belum punya parameter proyeksi itu, sehingga
--   ditambahkan kolom projected_revenue_rate_annual.
--
-- PENTING (kebenaran angka): projected_revenue_rate_annual adalah PROYEKSI pendapatan
-- tahunan usaha yang dibiayai (persen), BUKAN pendapatan yang sudah pasti. Bagi hasil
-- sejatinya bergantung pada laba/pendapatan usaha dan baru diketahui saat realisasi,
-- jadi jadwal angsuran hanyalah proyeksi yang WAJIB disesuaikan saat bagi hasil aktual
-- dihitung. Nilai 0 (default) berarti parameter belum diisi; kode menolak membentuk
-- jadwal bagi hasil agar tidak mengarang angka. Bank harus mengisi nisbah
-- (profit_sharing_ratio) dan proyeksi ini sebelum menjual produk bagi hasil.
--
-- Aditif dan idempotent: kolom baru dengan default 0; tidak menyentuh produk
-- konvensional (bunga tetap) maupun murabahah (margin nominal).

ALTER TABLE banking_products
    ADD COLUMN IF NOT EXISTS projected_revenue_rate_annual NUMERIC(8,4) NOT NULL DEFAULT 0.0000;

COMMENT ON COLUMN banking_products.projected_revenue_rate_annual IS
    'PROYEKSI pendapatan tahunan usaha yang dibiayai (%), dasar tarif ekuivalen jadwal bagi hasil = nisbah x nilai ini. Bukan angka pasti; sesuaikan saat realisasi. 0 = belum diisi, produk bagi hasil ditolak.';

-- Batas non-negatif sejalan dengan chk_rate_non_negative. Dijaga agar idempotent.
DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_projected_revenue_non_negative
        CHECK (projected_revenue_rate_annual >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
