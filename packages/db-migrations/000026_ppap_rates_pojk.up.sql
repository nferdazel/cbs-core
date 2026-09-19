-- CBS Migration 000026: Tarif PPAP sesuai POJK 33/POJK.03/2018
-- Run after: 000025_bank_profile.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Nilai yang ter-seed sebelumnya salah menurut POJK 33/POJK.03/2018 Pasal 16
-- ayat (3): Dalam Perhatian Khusus 10% (seharusnya 3%) dan Kurang Lancar 15%
-- (seharusnya 10%). Nilai Lancar 0,5% (PPAP umum, ayat 2), Diragukan 50%, dan
-- Macet 100% sudah benar.
--
-- Perubahan hanya dilakukan bila nilainya MASIH nilai salah hasil seed. Bila
-- operator sudah sengaja menyetel nilai lain (mis. kebijakan konservatif internal),
-- nilai itu tidak ditimpa oleh migrasi ini.
--
-- Catatan: POJK menghitung PPAP khusus setelah dikurangi nilai agunan pengurang
-- (Pasal 17). Sistem belum punya modul agunan, sehingga tarif ini dikenakan atas
-- pokok penuh dan hasilnya lebih besar dari kewajiban untuk kredit beragunan.

UPDATE system_config
SET value = '0.03',
    description = 'Tarif PPAP khusus kolektibilitas 2 - Dalam Perhatian Khusus (POJK 33/2018 Pasal 16)',
    updated_at = NOW()
WHERE key = 'ppap.rate.2' AND value = '0.1';

UPDATE system_config
SET value = '0.10',
    description = 'Tarif PPAP khusus kolektibilitas 3 - Kurang Lancar (POJK 33/2018 Pasal 16)',
    updated_at = NOW()
WHERE key = 'ppap.rate.3' AND value = '0.15';

-- Deskripsi tarif lain diberi rujukan pasalnya agar tidak lagi ambigu.
UPDATE system_config
SET description = 'Tarif PPAP umum kolektibilitas 1 - Lancar (POJK 33/2018 Pasal 16 ayat 2)',
    updated_at = NOW()
WHERE key = 'ppap.rate.1';

UPDATE system_config
SET description = 'Tarif PPAP khusus kolektibilitas 4 - Diragukan (POJK 33/2018 Pasal 16)',
    updated_at = NOW()
WHERE key = 'ppap.rate.4';

UPDATE system_config
SET description = 'Tarif PPAP khusus kolektibilitas 5 - Macet (POJK 33/2018 Pasal 16)',
    updated_at = NOW()
WHERE key = 'ppap.rate.5';
