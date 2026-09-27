-- 000112_ojk_kelembagaan.down.sql
-- Membalik 000112_ojk_kelembagaan.up.sql: hapus tabel data kelembagaan
-- (bank_management lebih dulu karena tidak ada FK antar keduanya, urutan hanya rapi).
-- Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

DROP TABLE IF EXISTS bank_management;
DROP TABLE IF EXISTS bank_offices;
