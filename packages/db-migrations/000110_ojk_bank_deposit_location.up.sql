-- CBS Migration 000110: sandi lokasi bank lawan untuk Form 13.00 kolom VI.
-- Run after: 000109_ojk_form06_agunan_ppka_kelonggaran.up.sql (urutan; tidak
-- bergantung isinya). Nomor 000110 dipesan khusus pekerjaan ini.
--
-- Menutup Form 13.00 "Daftar Simpanan dari Bank Lain" (docs/CELAH-LAPORAN-OJK.md §3)
-- pada kolom VI Lokasi Bank. Kolom itu mengacu pada Lampiran 03 - Daftar Sandi
-- Kabupaten/Kota (Form 13.00 – 2 butir VI, PDF #page 209; penjelasan PDF #page 211).
-- Alamat/cabang bank lawan tidak punya kolom tersendiri; sandi disimpan per nasabah
-- bank lawan, sama seperti pola 000104 (customers.ojk_pihak_lawan_code) dan 000105.
--
-- NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed sampai ada jalur
-- API. Foreign key ke ojk_kabupaten (000102) agar sandi yang tidak ada di Lampiran 03
-- tertolak, bukan tersimpan sebagai teks bebas.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom/foreign key kedua. Tidak ada data yang diubah.

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS ojk_kabupaten_code TEXT
        REFERENCES ojk_kabupaten (code);

COMMENT ON COLUMN customers.ojk_kabupaten_code IS
    'Sandi Kabupaten/Kota bank lawan (Lampiran 03 SEOJK 16/2024, 4 digit; FK ke ojk_kabupaten). Sumber Form 13.00 kolom VI Lokasi Bank. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
