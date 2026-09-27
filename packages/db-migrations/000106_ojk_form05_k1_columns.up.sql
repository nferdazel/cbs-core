-- CBS Migration 000106: kolom penyimpanan kolom K1 Form 05.00 pada lps_placements.
-- Run after: 000105_ojk_form06_sandi_inline.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000106 dipesan khusus pekerjaan ini.
--
-- Menutup kolom Form 05.00 Daftar Penempatan pada Bank Lain yang belum punya sumber
-- (docs/CELAH-LAPORAN-OJK.md §1), tanpa modul baru:
--   * lps_placements.ojk_hubungan_bank_code     -> V  Hubungan dengan Bank, sandi inline
--     12 (terkait) / 20 (tidak terkait), Lampiran II Form 05.00-2.
--   * lps_placements.blocked_amount             -> X  Nominal yang Diblokir/Dijaminkan.
--   * lps_placements.ojk_alasan_diblokir_code   -> XI Alasan Diblokir.
--   * lps_placements.accrued_interest_receivable-> XIII Pendapatan Bunga yang Akan Diterima.
--   * lps_placements.accrued_interest_pending   -> XIV Pendapatan Bunga Dalam Penyelesaian.
--   * lps_placements.counterparty_cif           -> XVI ID Pihak Lawan (nomor CIF internal
--     bank lawan, sama dengan SLIK; BUKAN sandi OJK/Lampiran 02).
--
-- Kolom ini NULLABLE dan tidak diisi aplikasi mana pun. Bank mengisi nilainya lewat
-- SQL/seed sampai ada jalur API; baris yang belum diisi ditulis "-" pada laporan, bukan
-- ditebak dari nama bank lawan atau nilai lain. Daftar sandi alasan diblokir (XI) belum
-- ada rujukannya di repo, karena itu nilainya hanya disimpan apa adanya tanpa pemetaan.
--
-- Tipe kolom sandi/teks disimpan TEXT agar seragam dengan kolom sandi OJK lain
-- (000104/000105); nominal memakai NUMERIC(28,4) seperti kolom uang lps_placements.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom, CHECK, maupun COMMENT kedua (pernyataan berikutnya no-op saat
-- kolom sudah ada). Tidak ada data yang diubah.

ALTER TABLE lps_placements
    ADD COLUMN IF NOT EXISTS ojk_hubungan_bank_code TEXT,
    ADD COLUMN IF NOT EXISTS blocked_amount NUMERIC(28,4)
        CHECK (blocked_amount IS NULL OR blocked_amount >= 0),
    ADD COLUMN IF NOT EXISTS ojk_alasan_diblokir_code TEXT,
    ADD COLUMN IF NOT EXISTS accrued_interest_receivable NUMERIC(28,4)
        CHECK (accrued_interest_receivable IS NULL OR accrued_interest_receivable >= 0),
    ADD COLUMN IF NOT EXISTS accrued_interest_pending NUMERIC(28,4)
        CHECK (accrued_interest_pending IS NULL OR accrued_interest_pending >= 0),
    ADD COLUMN IF NOT EXISTS counterparty_cif VARCHAR(32);

COMMENT ON COLUMN lps_placements.ojk_hubungan_bank_code IS
    'Sandi inline Hubungan dengan Bank (Lampiran II Form 05.00-2, PDF #page 140): 12 terkait, 20 tidak terkait. Sumber Form 05.00 kolom V. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.blocked_amount IS
    'Nominal penempatan yang diblokir/dijaminkan. Sumber Form 05.00 kolom X. NULL = belum diisi, laporan menulis "-" (bukan nol). Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.ojk_alasan_diblokir_code IS
    'Sandi alasan diblokir. Sumber Form 05.00 kolom XI. Daftar sandinya belum ada rujukannya di repo, karena itu disimpan apa adanya tanpa pemetaan. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.accrued_interest_receivable IS
    'Pendapatan bunga yang akan diterima atas penempatan (akrual/piutang bunga). Sumber Form 05.00 kolom XIII. NULL = belum diisi, laporan menulis "-" (bukan nol). Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.accrued_interest_pending IS
    'Pendapatan bunga dalam penyelesaian atas penempatan. Sumber Form 05.00 kolom XIV. NULL = belum diisi, laporan menulis "-" (bukan nol). Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.counterparty_cif IS
    'ID Pihak Lawan berupa nomor CIF internal bank lawan (harus sama dengan CIF pada SLIK; BUKAN sandi OJK). Sumber Form 05.00 kolom XVI. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
