-- 000088_widen_org_unit_code.up.sql
-- Run after: 000087_collateral_lampiran_ii_prep.up.sql
--
-- ALASAN (keputusan pemilik sistem): branches.code masih VARCHAR(8) dari migrasi
-- 000005. Itu cukup untuk kode cabang 3 digit, tetapi hierarki organisasi W14
-- (000081) menambah jenjang AREA dan WILAYAH yang kodenya deskriptif. Bank besar
-- memakai kode unit lebih panjang (mis. "AREA-JABAR-BARAT-01"), sehingga kode
-- wilayah/area yang wajar ditolak 422 sebelum sampai ke database. Validasi kode
-- di service dinaikkan ke lebar kolom yang sama (orgUnitCodeMaxLength) supaya
-- tidak ada dua batas berbeda antara kode dan skema.
--
-- Kolom lain yang menyimpan kode unit dengan peran sama ikut diperlebar agar kode
-- panjang tidak terpotong (silent truncation) atau gagal simpan:
--   * account_number_sequences.branch_code — FK + bagian PRIMARY KEY ke branches.code
--   * customers.cif_branch_code            — kode unit CIF nasabah
--   * staff_users.branch_code              — kode unit pengguna (dipakai resolusi cakupan)
--
-- Sifat: aditif, idempotent (ALTER TYPE ke lebar sama aman dijalankan ulang),
-- tidak mengubah data lama (hanya memperlebar), tidak mengubah angka/jurnal.
-- Tidak ada tabel/kolom/fungsi baru.

-- 1. Tabel sumber kode unit.
ALTER TABLE branches
    ALTER COLUMN code TYPE VARCHAR(32);

-- 2. Referensi kode unit pada penomoran rekening. Diperlebar bersama sumbernya
--    agar foreign key tetap dapat dibandingkan tanpa perbedaan tipe.
ALTER TABLE account_number_sequences
    ALTER COLUMN branch_code TYPE VARCHAR(32);

-- 3. Kode unit CIF nasabah (tanpa FK; nilai lama tetap sah).
ALTER TABLE customers
    ALTER COLUMN cif_branch_code TYPE VARCHAR(32);

-- 4. Kode unit pengguna staf. Sebelumnya 16; kode area/wilayah bisa lebih panjang.
ALTER TABLE staff_users
    ALTER COLUMN branch_code TYPE VARCHAR(32);
