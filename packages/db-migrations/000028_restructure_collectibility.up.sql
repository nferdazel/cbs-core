-- CBS Migration 000028: Batas kualitas Kredit restrukturisasi (POJK 1/2024 Pasal 23)
-- Run after: 000027_collectibility_bands_pojk.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Menyimpan kualitas Kredit sesaat SEBELUM restrukturisasi terakhir. Nilai ini
-- menjadi dasar Pasal 23 POJK No. 1 Tahun 2024:
--
--   - Kredit yang sebelum direstrukturisasi Diragukan atau Macet paling tinggi
--     Kurang Lancar (ayat (1) huruf a).
--   - Kredit yang sebelum direstrukturisasi Lancar, DPK, atau Kurang Lancar tidak
--     boleh membaik (ayat (1) huruf b).
--   - Batas itu lepas setelah 3 kali periode pembayaran bersih berturut-turut
--     (ayat (2) huruf a).
--
-- Tanpa kolom ini, proses harian berikutnya menghitung ulang dari DPD yang sudah
-- nol dan Kredit bermasalah kembali menjadi Lancar dalam satu malam.
--
-- NULL berarti Kredit belum pernah direstrukturisasi.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS pre_restructure_collectibility VARCHAR(32);
