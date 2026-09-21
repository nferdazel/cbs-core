-- CBS Migration 000063: parameter produk yang boleh diubah lewat API
-- Run after: 000060_bagi_hasil_range.up.sql
--
-- Latar belakang:
--   Sebelum ini repo produk hanya bisa dibaca, sehingga bank tidak punya jalur resmi
--   untuk mengubah parameter produk (suku bunga/margin, biaya admin, batas plafon,
--   tenor, pajak, penalti). Padahal parameter seperti projected_revenue_rate_annual
--   wajib diisi operator agar pembiayaan bagi hasil dapat dibentuk. Migrasi 000060
--   sudah menjaga rentang nisbah (0..1) dan proyeksi pendapatan (0..100); migrasi ini
--   menjadi jaring pengaman database untuk parameter lain yang kini dapat ditulis
--   lewat API, sejalan dengan validasi service.
--
-- Letak batas:
--   Service menolak lebih dulu dengan pesan bersatuan, lalu constraint di sini
--   menjaga agar jalur tulis lain tidak menembusnya. Keduanya harus sepakat:
--     rate_annual, tax_rate, early_withdrawal_penalty_rate : >= 0 (persen)
--     admin_fee, min_amount, max_amount                     : >= 0 (rupiah)
--     min_term_months, max_term_months                      : >= 0 (bulan)
--   Hubungan min/max plafon tetap dijaga chk_min_max_amount dari migrasi 000005.
--
-- Aditif dan idempotent: hanya menambah constraint; aman dijalankan ulang. Tidak
-- menyentuh data transaksi lama maupun mengubah arti produk.

ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_admin_fee_non_negative;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_tax_rate_non_negative;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_early_withdrawal_penalty_non_negative;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_min_amount_non_negative;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_max_amount_non_negative;
ALTER TABLE banking_products DROP CONSTRAINT IF EXISTS chk_term_months_non_negative;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_admin_fee_non_negative CHECK (admin_fee >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_tax_rate_non_negative CHECK (tax_rate >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_early_withdrawal_penalty_non_negative CHECK (early_withdrawal_penalty_rate >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_min_amount_non_negative CHECK (min_amount >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_max_amount_non_negative CHECK (max_amount >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE banking_products
        ADD CONSTRAINT chk_term_months_non_negative CHECK (min_term_months >= 0 AND max_term_months >= 0);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
