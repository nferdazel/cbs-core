-- CBS Migration 000123 (down): hapus register kas valuta asing Form 03.00.
-- Simetris dengan 000123_kas_valas_register.up.sql. Migrasi ini hanya membatalkan tabel
-- yang dibuat up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_kas_valas_status;
DROP INDEX IF EXISTS idx_kas_valas_as_of;
DROP INDEX IF EXISTS idx_kas_valas_jenis;
DROP TABLE IF EXISTS kas_valas_register;
