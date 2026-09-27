-- 000113_ojk_off_balance.down.sql
-- Membalik 000113_ojk_off_balance.up.sql: hapus register rekening administratif.
-- Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

DROP TABLE IF EXISTS off_balance_items;
