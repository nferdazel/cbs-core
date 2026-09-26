-- CBS Migration 000104: sambungkan tabel referensi sandi OJK (000102) ke kolom
-- laporan yang masih kosong.
-- Run after: 000102_ojk_reference_codes.up.sql (tabel referensi) — migrasi ini
-- TIDAK bergantung isinya selain urutan; nomor 000103 dipesan pekerjaan lain.
--
-- Tiga kolom yang dibuka (docs/CELAH-LAPORAN-OJK.md):
--   * customers.ojk_pihak_lawan_code    -> Form 06.00 XVIII Jenis Debitur/Pihak Lawan
--     (Daftar Sandi Pihak Lawan, Lampiran 02, sandi 3 digit)
--   * customers.ojk_sektor_ekonomi_code -> Form 06.00 XX Sektor Ekonomi
--     (Daftar Sandi Sektor Ekonomi, Lampiran 05, sandi 6 digit)
--   * lps_placements.ojk_kabupaten_code -> Form 05.00 III Lokasi Bank
--     (Daftar Sandi Kabupaten/Kota, Lampiran 03, sandi 4 digit)
--
-- Tipe kolom mengikuti kunci tabel referensi (TEXT, bukan varchar berpanjang tetap)
-- agar foreign key membandingkan tipe yang sama persis.
--
-- Yang diisi bank: kolom ini NULLABLE dan tidak diisi aplikasi mana pun. Bank mengisi
-- nilainya lewat SQL/seed sampai ada jalur API; baris yang belum diisi ditulis "-"
-- pada laporan, bukan ditebak dari nama/kota teks bebas.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom maupun foreign key kedua (saat kolom sudah ada, pernyataan
-- berikutnya menjadi no-op). Tidak ada data yang diubah.

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS ojk_pihak_lawan_code TEXT
        REFERENCES ojk_pihak_lawan (code),
    ADD COLUMN IF NOT EXISTS ojk_sektor_ekonomi_code TEXT
        REFERENCES ojk_sektor_ekonomi (code);

ALTER TABLE lps_placements
    ADD COLUMN IF NOT EXISTS ojk_kabupaten_code TEXT
        REFERENCES ojk_kabupaten (code);

COMMENT ON COLUMN customers.ojk_pihak_lawan_code IS
    'Sandi pihak lawan (Lampiran 02 SEOJK 16/2024, 3 digit); sumber Form 06.00 kolom XVIII Jenis Debitur/Pihak Lawan. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN customers.ojk_sektor_ekonomi_code IS
    'Sandi sektor ekonomi (Lampiran 05 SEOJK 16/2024, 6 digit); sumber Form 06.00 kolom XX Sektor Ekonomi. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.ojk_kabupaten_code IS
    'Sandi Kabupaten/Kota bank lawan (Lampiran 03 SEOJK 16/2024, 4 digit); sumber Form 05.00 kolom III Lokasi Bank. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
