-- 000084_kpmm_config_verify.up.sql
-- Run after: 000083_eod_scheduler.up.sql
--
-- Koreksi sitasi hukum pada deskripsi kunci KPMM (di-seed migrasi 000082).
--
-- Hasil verifikasi terhadap dokumen resmi:
--   * Bobot risiko: SEOJK No. 2/SEOJK.03/2025 Lampiran II "Perhitungan ATMR".
--   * Ambang modal: POJK No. 5/POJK.03/2015 Pasal 2 (KPMM 12%), Pasal 4 (modal
--     inti 8%), Pasal 3 ayat (2) (modal pelengkap <=100% modal inti), Pasal 10
--     ayat (1) huruf c (PPAP/PPKA umum <=1,25% ATMR), dan Pasal 13 (modal inti
--     minimum Rp6.000.000.000), dilanjutkan POJK No. 7 Tahun 2026.
--
-- Koreksi yang dilakukan: kunci `kpmm.modal_inti_min_amount` pada 000082 menyebut
-- "Pasal 14" untuk angka Rp6.000.000.000. Angka itu bersumber dari Pasal 13 (BAB
-- III Modal Inti Minimum); Pasal 14 hanya mengatur cara pemenuhan kewajibannya.
-- NILAI kunci tidak berubah; hanya deskripsi/sitasi. Tidak menyentuh
-- ckpn.enabled, tabel, atau angka akuntansi apa pun.
--
-- Idempotent: UPDATE ... WHERE key = ... (aman dijalankan ulang).

UPDATE system_config
SET description = 'Modal inti minimum absolut dalam rupiah penuh. Sumber: POJK No. 5/POJK.03/2015 Pasal 13 (Rp6.000.000.000,00), dilanjutkan POJK No. 7 Tahun 2026.'
WHERE key = 'kpmm.modal_inti_min_amount';
