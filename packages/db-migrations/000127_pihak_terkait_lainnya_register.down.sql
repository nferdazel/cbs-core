-- CBS Migration 000127 (down): hapus register pihak terkait lainnya Form 00.05.
-- Simetris dengan 000127_pihak_terkait_lainnya_register.up.sql. Hanya membatalkan tabel
-- yang dibuat up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_pihak_terkait_hubungan;
DROP INDEX IF EXISTS idx_pihak_terkait_jenis;
DROP TABLE IF EXISTS pihak_terkait_lainnya_register;
