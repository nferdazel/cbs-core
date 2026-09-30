-- CBS Migration 000128 (down): hapus register modal Form 00.06.
-- Simetris dengan 000128_modal_register.up.sql. Hanya membatalkan tabel yang dibuat up;
-- tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_modal_jenis;
DROP INDEX IF EXISTS idx_modal_jenis_modal;
DROP TABLE IF EXISTS modal_register;
