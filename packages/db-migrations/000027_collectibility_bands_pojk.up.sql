-- CBS Migration 000027: Pita DPD kolektibilitas sesuai POJK No. 1 Tahun 2024
-- Run after: 000026_ppap_rates_pojk.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- POJK 33/POJK.03/2018 dicabut dan dinyatakan tidak berlaku oleh POJK No. 1 Tahun
-- 2024 tentang Kualitas Aset Bank Perekonomian Rakyat (berlaku 11 Januari 2024).
-- Pita kualitas Kredit menurut Lampiran II untuk Kredit dengan angsuran 1 bulan
-- atau lebih:
--
--   Lancar        : tanpa tunggakan, atau tunggakan <= 30 hari dan belum jatuh tempo
--   DPK           : tunggakan > 30 s/d 90 hari, dan/atau jatuh tempo <= 15 hari
--   Kurang Lancar : tunggakan > 90 s/d 180 hari, dan/atau jatuh tempo > 15 s/d 30 hari
--   Diragukan     : tunggakan > 180 s/d 360 hari, dan/atau jatuh tempo > 30 s/d 60 hari
--   Macet         : tunggakan > 360 hari, dan/atau jatuh tempo > 60 hari
--
-- Nilai seed lama (30/90/180) berasal dari kerangka Bank Umum, bukan BPR, dan
-- membuat setiap Kredit satu golongan LEBIH BERAT dari seharusnya: kredit telat
-- 40 hari dihitung Kurang Lancar (PPAP 10%) padahal seharusnya DPK (3%), dan
-- kredit telat 200 hari dihitung Macet (100%) padahal seharusnya Diragukan (50%).
-- Akibat lanjutannya, akrual bunga dihentikan terlalu cepat.
--
-- Perubahan hanya dilakukan bila nilainya MASIH nilai salah hasil seed. Bila
-- operator sudah sengaja menyetel nilai lain, nilai itu tidak ditimpa.

UPDATE system_config
SET value = '90',
    description = 'Batas atas DPD (hari) kolektibilitas 2 - Dalam Perhatian Khusus (POJK 1/2024 Lampiran II)',
    updated_at = NOW()
WHERE key = 'ppap.dpd.dpk' AND value = '30';

UPDATE system_config
SET value = '180',
    description = 'Batas atas DPD (hari) kolektibilitas 3 - Kurang Lancar (POJK 1/2024 Lampiran II)',
    updated_at = NOW()
WHERE key = 'ppap.dpd.kurang_lancar' AND value = '90';

UPDATE system_config
SET value = '360',
    description = 'Batas atas DPD (hari) kolektibilitas 4 - Diragukan (POJK 1/2024 Lampiran II)',
    updated_at = NOW()
WHERE key = 'ppap.dpd.diragukan' AND value = '180';

-- Kunci baru: batas atas tunggakan yang masih Lancar selama Kredit belum jatuh
-- tempo. Tanpa kunci ini, Kredit telat beberapa hari langsung turun ke DPK.
INSERT INTO system_config (key, value, description)
VALUES (
    'ppap.dpd.lancar',
    '30',
    'Batas atas DPD (hari) yang masih Lancar selama Kredit belum jatuh tempo (POJK 1/2024 Lampiran II)'
)
ON CONFLICT (key) DO NOTHING;
