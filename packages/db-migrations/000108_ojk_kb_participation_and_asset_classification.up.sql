-- CBS Migration 000108: kolom sandi klasifikasi aset (DK).
-- Run after: 000107_ojk_form06_k1_columns.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000108 dipesan khusus pekerjaan ini.
--
-- Menutup butir DK docs/CELAH-LAPORAN-OJK.md: kolom klasifikasi aset disediakan sebagai
-- kolom sandi, BUKAN ditebak aplikasi.
--
-- Kolom sandi opsional klasifikasi aset SAK EP per baris. Keputusan pemilik: SEDIAKAN
-- kolom, TANPA auto-klasifikasi. Nilainya diisi bank lewat SQL/seed sampai ada jalur API;
-- baris kosong ditulis "-", bukan ditebak dari kolektibilitas/kualitas.
--   * loans.ojk_klasifikasi_aset_code          -> Form 06.00 kolom XLVII.
--   * lps_placements.ojk_klasifikasi_aset_code -> Form 05.00 kolom XX.
--
-- Butir KB (Form 06.00 VI/XXXIX/XL/XLIII dan LAPORAN_LAKU_PANDAI) TIDAK memakai saklar
-- system_config: keputusannya adalah KEBIJAKAN (bank bukan peserta secara bawaan,
-- partisipasi dikonfirmasi saat onboarding, kolom ditulis "-" selama bukan peserta).
-- Saklar sengaja TIDAK di-seed sampai modul KUR/LPBBTI/Laku Pandai benar-benar dibangun
-- dan ada kode yang membacanya: kunci config tanpa pembaca hanya menjadi sampah, dan
-- menaruh namanya di string dokumentasi akan meloloskan invarian config secara semu.
--
-- Kolom XLIV-XLVI (CKPN per golongan) dan XXXVI (bunga dalam penyelesaian) Form 06.00
-- sengaja TIDAK diubah: keputusannya tetap belum tersedia (stage CKPN tidak berlaku
-- untuk SAK EP; bunga dalam penyelesaian tidak diturunkan dari kolektibilitas).
-- NIK/NPWP (kolom III) tetap tidak dibuka.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom/COMMENT kedua. Tidak ada data yang diubah.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS ojk_klasifikasi_aset_code TEXT;

ALTER TABLE lps_placements
    ADD COLUMN IF NOT EXISTS ojk_klasifikasi_aset_code TEXT;

COMMENT ON COLUMN loans.ojk_klasifikasi_aset_code IS
    'Sandi klasifikasi aset keuangan SAK EP per kredit. Sumber Form 06.00 kolom XLVII. Daftar sandinya belum ada rujukannya di repo, karena itu disimpan apa adanya TANPA auto-klasifikasi (keputusan pemilik: sediakan kolom, jangan tebak dari kolektibilitas). NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN lps_placements.ojk_klasifikasi_aset_code IS
    'Sandi klasifikasi aset keuangan SAK EP per penempatan pada bank lain. Sumber Form 05.00 kolom XX. Daftar sandinya belum ada rujukannya di repo, karena itu disimpan apa adanya TANPA auto-klasifikasi. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
