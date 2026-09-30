-- CBS Migration 000129 (down): hapus register aset produktif yang dihapus buku Form 15.00.
-- Simetris dengan 000129_hapus_buku_register.up.sql. Hanya membatalkan tabel yang dibuat
-- up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_hapus_buku_tanggal;
DROP INDEX IF EXISTS idx_hapus_buku_jenis_aset;
DROP TABLE IF EXISTS hapus_buku_register;
