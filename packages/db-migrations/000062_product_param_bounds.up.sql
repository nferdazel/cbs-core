-- CBS Migration 000062: batas atas parameter produk
-- Run after: 000061_audit_metadata.up.sql
--
-- Latar belakang:
--   Parameter produk hanya dijaga batas bawah (>= 0). Akibatnya nilai yang jelas
--   salah satuan tetap tersimpan dan langsung masuk perhitungan: mis. tax_rate
--   atau rate_annual 1e9 (maksudnya 1%) menghasilkan bunga/pajak yang tidak masuk
--   akal. Migrasi 000060 sudah menjaga rentang nisbah (0..1) dan proyeksi
--   pendapatan (0..100); migrasi ini menutup tarif lain dengan pola yang sama.
--
-- Rentang:
--   rate_annual, tax_rate, early_withdrawal_penalty_rate : 0..100 (persen,
--     per tahun untuk rate_annual/penalti). Nilai di atas 100% bukan tarif wajar.
--   admin_fee : 0..1.000.000.000.000 (rupiah). Batas atas sengaja longgar, hanya
--     jaring pengaman terhadap nilai yang jelas salah satuan; biaya admin yang sah
--     (mis. puluhan ribu rupiah) jauh di bawahnya.
--
-- Aditif dan idempotent: constraint lama >= 0 untuk kolom tarif diganti constraint
-- rentang, drop memakai IF EXISTS dan add menoleransi duplicate_object sehingga
-- aman dijalankan ulang. Produk contoh migrasi 000005 (rate maks 12, tax maks 20,
-- admin_fee 0) dan seluruh default 0 memenuhi rentang ini.

ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_rate_non_negative;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_rate_annual_range;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_tax_rate_range;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_early_withdrawal_penalty_range;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_admin_fee_max;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_rate_annual_range CHECK (rate_annual >= 0 AND rate_annual <= 100);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_tax_rate_range CHECK (tax_rate >= 0 AND tax_rate <= 100);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_early_withdrawal_penalty_range
        CHECK (early_withdrawal_penalty_rate >= 0 AND early_withdrawal_penalty_rate <= 100);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_admin_fee_max CHECK (admin_fee >= 0 AND admin_fee <= 1000000000000);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
