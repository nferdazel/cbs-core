-- CBS Migration 000121 (down): hapus register aset keuangan lainnya Form 18.00.
-- Simetris dengan 000121_aset_keuangan_lainnya_register.up.sql. Migrasi ini hanya
-- membatalkan tabel yang dibuat up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_aset_keuangan_lainnya_status;
DROP INDEX IF EXISTS idx_aset_keuangan_lainnya_as_of;
DROP INDEX IF EXISTS idx_aset_keuangan_lainnya_jenis_ckpn;
DROP INDEX IF EXISTS idx_aset_keuangan_lainnya_klasifikasi;
DROP INDEX IF EXISTS idx_aset_keuangan_lainnya_jenis;
DROP TABLE IF EXISTS aset_keuangan_lainnya_register;
