-- 000115_ayda_register.down.sql
-- Membalik 000115_ayda_register.up.sql: hapus register AYDA (Form 07.00).
-- Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain; akun COA 10500 yang
-- sudah ada sebelum migrasi ini tidak ikut dihapus.

DROP TABLE IF EXISTS ayda_register;
