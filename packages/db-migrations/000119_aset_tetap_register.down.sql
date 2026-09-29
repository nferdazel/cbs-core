-- 000119_aset_tetap_register.down.sql
-- Membalik 000119_aset_tetap_register.up.sql: hapus register aset tetap, inventaris,
-- dan aset tidak berwujud (Form 08.00). Idempotent (IF EXISTS) dan tidak menyentuh
-- tabel/kolom lain.

DROP TABLE IF EXISTS aset_tetap_register;
