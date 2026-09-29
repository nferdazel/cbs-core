-- CBS Migration 000122 (down): hapus register surat berharga Form 04.00.
-- Simetris dengan 000122_surat_berharga_register.up.sql. Migrasi ini hanya membatalkan
-- tabel yang dibuat up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_surat_berharga_status;
DROP INDEX IF EXISTS idx_surat_berharga_as_of;
DROP INDEX IF EXISTS idx_surat_berharga_kualitas;
DROP INDEX IF EXISTS idx_surat_berharga_jenis;
DROP INDEX IF EXISTS idx_surat_berharga_klasifikasi;
DROP TABLE IF EXISTS surat_berharga_register;
