-- CBS Migration 000105: kolom penyimpanan sandi inline & field sederhana Form 06.00.
-- Run after: 000104_ojk_reference_links.up.sql (urutan; tidak bergantung isinya).
--
-- Sumber sandi: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 06.00 - 2, sebagaimana
-- diringkas di docs/LAMPIRAN-OJK.md (nomor halaman PDF, bukan hasil ekstraksi ulang):
--   * loans.ojk_jenis_penggunaan_code   -> VIII Jenis Penggunaan, inline 10/20/31/32/35/39
--   * customers.ojk_hubungan_bank_code  -> IX  Hubungan dengan Bank, inline 11/12/20
--   * loans.ojk_periode_pembayaran_code -> XI  Periode Pembayaran Pokok dan Bunga, inline 1-8
--   * loans.ojk_kabupaten_code          -> XXII Lokasi Penggunaan (Lampiran 03, ojk_kabupaten)
--
-- Kolom ini NULLABLE dan tidak diisi aplikasi mana pun. Bank mengisi nilainya lewat
-- SQL/seed sampai ada jalur API; baris yang belum diisi ditulis "-" pada laporan, bukan
-- ditebak dari loans.purpose (teks bebas) atau nama/kota.
--
-- Tipe kolom sandi disimpan TEXT agar seragam dengan tabel referensi 000102 dan kolom
-- sandi OJK lain (000104), bukan varchar berpanjang tetap.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom maupun foreign key kedua (saat kolom sudah ada, pernyataan
-- berikutnya menjadi no-op). Tidak ada data yang diubah.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS ojk_jenis_penggunaan_code TEXT,
    ADD COLUMN IF NOT EXISTS ojk_periode_pembayaran_code TEXT,
    ADD COLUMN IF NOT EXISTS ojk_kabupaten_code TEXT
        REFERENCES ojk_kabupaten (code);

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS ojk_hubungan_bank_code TEXT;

COMMENT ON COLUMN loans.ojk_jenis_penggunaan_code IS
    'Sandi inline Jenis Penggunaan (Lampiran II Form 06.00-2, PDF #page 152): 10/20/31/32/35/39. Sumber Form 06.00 kolom VIII. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN customers.ojk_hubungan_bank_code IS
    'Sandi inline Hubungan dengan Bank (Lampiran II Form 06.00-2, PDF #page 152/160): 11/12/20. Sumber Form 06.00 kolom IX. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_periode_pembayaran_code IS
    'Sandi inline Periode Pembayaran Pokok dan Bunga (Lampiran II Form 06.00-2, PDF #page 153): 1 s.d. 8 (harian ... setiap saat). Sumber Form 06.00 kolom XI. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_kabupaten_code IS
    'Sandi Kabupaten/Kota lokasi penggunaan kredit (Lampiran 03 SEOJK 16/2024, 4 digit; FK ke ojk_kabupaten). Sumber Form 06.00 kolom XXII Lokasi Penggunaan. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
