-- CBS Migration 000131 (down): hapus register divisi/satuan kerja Form 00.19.
-- Simetris dengan 000131_bank_work_units.up.sql. Hanya membatalkan tabel yang dibuat up;
-- tidak menyentuh objek lain.

DROP INDEX IF EXISTS idx_bank_work_units_parent;
DROP TABLE IF EXISTS bank_work_units;
