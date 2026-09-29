-- CBS Migration 000124 (down): hapus register kredit sindikasi Form 06.02.
-- Simetris dengan 000124_kredit_sindikasi_register.up.sql. Migrasi ini hanya membatalkan
-- tabel yang dibuat up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_kredit_sindikasi_status;
DROP INDEX IF EXISTS idx_kredit_sindikasi_as_of;
DROP INDEX IF EXISTS idx_kredit_sindikasi_kepesertaan;
DROP INDEX IF EXISTS idx_kredit_sindikasi_kualitas;
DROP TABLE IF EXISTS kredit_sindikasi_register;
