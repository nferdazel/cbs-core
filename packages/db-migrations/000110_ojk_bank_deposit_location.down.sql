-- 000110_ojk_bank_deposit_location.down.sql
-- Membalik 000110_ojk_bank_deposit_location.up.sql: hapus kolom sandi lokasi bank
-- lawan Form 13.00. Foreign key ikut terhapus bersama kolomnya; tabel referensi
-- ojk_kabupaten TIDAK dihapus di sini (itu urusan 000102.down.sql). Idempotent
-- (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

ALTER TABLE customers
    DROP COLUMN IF EXISTS ojk_kabupaten_code;
