-- 000106_ojk_form05_k1_columns.down.sql
-- Membalik 000106_ojk_form05_k1_columns.up.sql: hapus enam kolom penyimpanan kolom K1
-- Form 05.00. CHECK yang menyertai kolom ikut terhapus bersama kolomnya. Tidak ada
-- tabel referensi yang disentuh.

ALTER TABLE lps_placements
    DROP COLUMN IF EXISTS counterparty_cif,
    DROP COLUMN IF EXISTS accrued_interest_pending,
    DROP COLUMN IF EXISTS accrued_interest_receivable,
    DROP COLUMN IF EXISTS ojk_alasan_diblokir_code,
    DROP COLUMN IF EXISTS blocked_amount,
    DROP COLUMN IF EXISTS ojk_hubungan_bank_code;
