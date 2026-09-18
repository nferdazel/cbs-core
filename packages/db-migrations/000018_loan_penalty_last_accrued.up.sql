-- CBS Migration 000018: Tanggal akrual denda kredit terakhir
-- Run after: 000017_interest_accrual_payment.up.sql
-- Idempotent: aman dijalankan ulang.

-- Akrual denda bersifat harian dan diskret per tanggal. Tanpa penanda tanggal
-- terakhir, setiap eksekusi EOD menambah pokok_tunggakan x tarif x DPD, sehingga
-- total setelah n hari menjadi kuadratik (1+2+...+n), bukan linier. Kolom ini
-- menyimpan tanggal akrual terakhir per kredit agar eksekusi berikutnya hanya
-- menagih selisih hari yang belum diakru dan bisa menyembuhkan diri bila ada
-- tanggal yang terlewat. Nilainya hanya dimajukan (GREATEST), tidak pernah mundur.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS penalty_last_accrued_on DATE;
