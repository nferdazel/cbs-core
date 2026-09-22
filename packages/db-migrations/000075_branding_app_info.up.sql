-- 000075_branding_app_info.up.sql
-- Run after: 000074_institution_book_scope.up.sql
--
-- Menambahkan kunci konfigurasi BRANDING tingkat INSTALASI:
--   branding.display_name, branding.short_name, branding.description, branding.logo_url
--
-- Latar: nama aplikasi dan branding sebelumnya ditulis sebagai literal di kode web
-- (metadata Next, kamus i18n, header, halaman login), sehingga berganti nama produk
-- menuntut build ulang. Padahal CBS Core dijual per bank (satu instalasi per badan
-- hukum) dan setiap pelanggan dapat memakai nama tampilan sendiri. Endpoint publik
-- GET /api/v1/app-info merakit identitas ini untuk halaman login dan judul tab web.
--
-- Nama PT TIDAK menjadi kunci di sini: sumbernya tabel bank_profile.bank_name
-- (migrasi 000025) supaya hanya ada satu sumber kebenaran, sama seperti dokumen
-- cetak. Kunci di sini hanya branding yang khas aplikasi.
--
-- Nilai awal sengaja mereproduksi tampilan yang berlaku supaya tidak ada perubahan
-- visual tak sengaja pada instalasi yang sudah berjalan. logo_url awal kosong:
-- web memakai ikon bawaannya sampai bank mengisi logo.
--
-- Idempotent: aman dijalankan ulang. Nilai yang sudah disesuaikan operator TIDAK
-- ditimpa; kunci yang sudah ada hanya deskripsinya yang dirapikan.

INSERT INTO system_config (key, value, description) VALUES
    ('branding.display_name', 'CBS Core Backoffice',
     'Nama tampilan aplikasi pada header dan judul tab browser. Dipakai endpoint publik /api/v1/app-info. Kosong berarti memakai bawaan aplikasi.'),
    ('branding.short_name', 'CBS Core',
     'Nama pendek aplikasi untuk ruang sempit. Dipakai endpoint publik /api/v1/app-info. Kosong berarti memakai bawaan aplikasi.'),
    ('branding.description', 'Core Banking & Akuntansi Double-Entry',
     'Keterangan singkat aplikasi di bawah nama tampilan pada header dan halaman login. Dipakai endpoint publik /api/v1/app-info. Kosong berarti memakai bawaan aplikasi.'),
    ('branding.logo_url', '',
     'URL logo aplikasi. Kosong berarti web memakai ikon bawaannya. Dipakai endpoint publik /api/v1/app-info.')
ON CONFLICT (key) DO UPDATE
    SET description = EXCLUDED.description;
