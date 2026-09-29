-- 000118_properti_terbengkalai_register.down.sql
-- Membalik 000118_properti_terbengkalai_register.up.sql: hapus register properti
-- terbengkalai (Form 17.00). Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom
-- lain.

DROP TABLE IF EXISTS properti_terbengkalai_register;
