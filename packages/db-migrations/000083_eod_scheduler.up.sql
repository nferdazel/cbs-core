-- 000083_eod_scheduler.up.sql
-- Run after: 000081_branch_hierarchy.up.sql (nomor 000082 dipakai gelombang lain;
-- scripts/migrate.sh memproses urutan glob sehingga gap tidak masalah).
--
-- ALASAN (W17, lanjutan W6): tabel eod_triggers sudah ada sejak 000077 tetapi
-- penjadwal dan evaluator cron-nya belum. Perilaku produksi TIDAK BOLEH berubah
-- sampai bank menyalakannya, jadi penjadwal dikendalikan konfigurasi, bukan kode.
--
-- Dua kunci system_config:
--   eod.scheduler.enabled     saklar utama. false (bawaan) = tidak ada tutup hari
--                             otomatis. Penjadwal membaca ulang kunci ini setiap
--                             siklus tick, sehingga bank dapat menyalakannya tanpa
--                             rilis/restart.
--   eod.scheduler.executed_by ID staf (UUID) atas nama siapa EOD terjadwal berjalan.
--                             Wajib staf nyata: kolom system_config.updated_by dan
--                             eod_step_runs.executed_by ber-FK ke staff_users, dan
--                             jejak audit harus menunjuk orang yang dapat dimintai
--                             pertanggungjawaban. Nilai awal = akun SUPERADMIN
--                             bootstrap (000002); bank menggantinya bila perlu.
--
-- Selain saklar ini, tiap pemicu di eod_triggers masih punya kolom enabled sendiri
-- (pemicu bawaan scheduled-daily NONAKTIF), sehingga ada dua lapis pengaman.
--
-- Sifat: aditif & idempotent (ON CONFLICT DO NOTHING). Nilai yang sudah disesuaikan
-- operator TIDAK ditimpa. Tidak ada tabel/kolom baru dan tidak ada DML ke tabel lain.

INSERT INTO system_config (key, value, description) VALUES
('eod.scheduler.enabled', 'false',
 'Saklar penjadwal tutup hari otomatis (EOD terjadwal). false = tidak ada tutup hari otomatis; bank menyalakannya setelah jam, pemicu, dan staf pelaksana disetujui.'),
('eod.scheduler.executed_by', 'c0000000-0000-0000-0000-000000000001',
 'ID staf (UUID) atas nama siapa EOD terjadwal dijalankan. Dipakai untuk kolom updated_by dan executed_by serta jejak audit; ganti bila akun bawaan tidak dipakai.')
ON CONFLICT (key) DO NOTHING;
