-- 000117_pinjaman_diterima_register.down.sql
-- Membalik 000117_pinjaman_diterima_register.up.sql: hapus register pinjaman yang
-- diterima (Form 00.07). Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

DROP TABLE IF EXISTS pinjaman_diterima_register;
