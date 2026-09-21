-- CBS Migration 000060: batas rentang parameter bagi hasil
-- Run after: 000059_projected_revenue_bagi_hasil.up.sql
--
-- Latar belakang:
--   000059 menambahkan projected_revenue_rate_annual dengan batas hanya >= 0.
--   Celah itu meloloskan nilai yang salah satuan: nisbah 40 (maksudnya 0,4) atau
--   proyeksi pendapatan 1200 (maksudnya 12%) tetap tersimpan dan menghasilkan
--   proyeksi imbal hasil seratus kali lipat. Kode kini menolaknya (lihat
--   domain.ProjectBagiHasilMargin), dan constraint di sini menjaga agar nilai yang
--   sudah tersimpan tidak lolos hanya karena jalur tulis lain menembus service.
--
-- Rentang:
--   profit_sharing_ratio: 0 <= n <= 1. Nilai 0 berarti parameter belum diisi
--     (produk konvensional); kode menolak membentuk jadwal bagi hasil bila 0.
--   projected_revenue_rate_annual: 0 <= p <= 100 (persen per tahun). Nilai 0
--     berarti parameter belum diisi; kode menolak bila 0. Di atas 100% bukan
--     proyeksi pendapatan usaha yang wajar.
--
-- Aditif dan idempotent: constraint lama >= 0 diganti constraint rentang; aman
-- dijalankan ulang. Produk contoh pada migrasi 000005 (nisbah 0-0,6) dan seluruh
-- default 0 memenuhi rentang ini.

ALTER TABLE banking_products
    DROP CONSTRAINT IF EXISTS chk_projected_revenue_non_negative;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_projected_revenue_range
        CHECK (projected_revenue_rate_annual >= 0 AND projected_revenue_rate_annual <= 100);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_profit_sharing_ratio_range
        CHECK (profit_sharing_ratio >= 0 AND profit_sharing_ratio <= 1);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
