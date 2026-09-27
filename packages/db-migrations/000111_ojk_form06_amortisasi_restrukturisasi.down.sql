-- 000111_ojk_form06_amortisasi_restrukturisasi.down.sql
-- Membalik 000111_ojk_form06_amortisasi_restrukturisasi.up.sql: hapus empat kolom
-- penyimpanan komponen Form 06.00 (XXIX provisi belum diamortisasi, XXX biaya transaksi
-- belum diamortisasi, XXXI pendapatan bunga ditangguhkan restrukturisasi, XXXII cadangan
-- kerugian restrukturisasi). Idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

ALTER TABLE loans
    DROP COLUMN IF EXISTS ojk_provisi_belum_diamortisasi_amount,
    DROP COLUMN IF EXISTS ojk_biaya_transaksi_belum_diamortisasi_amount,
    DROP COLUMN IF EXISTS ojk_pendapatan_bunga_ditangguhkan_amount,
    DROP COLUMN IF EXISTS ojk_cadangan_kerugian_restrukturisasi_amount;
