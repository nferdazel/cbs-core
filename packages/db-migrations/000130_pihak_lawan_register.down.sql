-- CBS Migration 000130 (down): hapus register pihak lawan Form 00.16.
-- Simetris dengan 000130_pihak_lawan_register.up.sql. Hanya membatalkan tabel yang dibuat
-- up; tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_pihak_lawan_grup;
DROP INDEX IF EXISTS idx_pihak_lawan_golongan;
DROP TABLE IF EXISTS pihak_lawan_register;
