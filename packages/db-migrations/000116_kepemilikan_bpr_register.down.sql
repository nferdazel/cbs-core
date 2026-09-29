-- 000116_kepemilikan_bpr_register.down.sql
-- Membalik 000116_kepemilikan_bpr_register.up.sql: hapus register pemegang saham
-- BPR (Form 00.01). Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

DROP TABLE IF EXISTS kepemilikan_bpr_register;
