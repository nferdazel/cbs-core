-- 000120_penyertaan_modal_register.down.sql
-- Membalik 000120_penyertaan_modal_register.up.sql: hapus register penyertaan modal
-- (Form 16.00). Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

DROP TABLE IF EXISTS penyertaan_modal_register;
