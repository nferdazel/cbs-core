-- 000114_form09_aset_lainnya_coa.down.sql
-- Membalik 000114_form09_aset_lainnya_coa.up.sql: hapus enam akun COA Aset Lainnya
-- (Form 09.00) beserta akun GL internalnya. Idempotent (IF EXISTS/IN) dan tidak
-- menyentuh tabel/kolom lain.
--
-- Akun GL dihapus lebih dulu karena accounts.coa_id mereferensikan chart_of_accounts.
-- Bila salah satu COA sudah terpakai jurnal, DELETE COA akan gagal (ON DELETE RESTRICT);
-- itu disengaja: migrasi balik tidak boleh menghapus jejak akuntansi.

DELETE FROM accounts
WHERE coa_id IN (
    SELECT id FROM chart_of_accounts
    WHERE code IN ('10310', '10320', '10330', '10340', '10350', '10360')
);

DELETE FROM chart_of_accounts
WHERE code IN ('10310', '10320', '10330', '10340', '10350', '10360');
