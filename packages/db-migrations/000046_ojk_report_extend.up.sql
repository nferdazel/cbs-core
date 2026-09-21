-- CBS Migration 000046: kunci konfigurasi perluasan ekspor laporan OJK.
-- Run after: 000045 (nomor dipesan untuk gelombang ini).
-- Idempotent: INSERT ... ON CONFLICT DO NOTHING; tidak mengubah tabel/kolom mana pun.
--
-- Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 00.00 "Informasi Pokok BPR".
-- Tabel bank_profile (migrasi 000025) hanya memuat nama, alamat, kota, telepon, dan
-- NPWP. Field identitas lain yang diminta Form 00.00 tidak punya tempat penyimpanan,
-- jadi disimpan sebagai system_config agar bank dapat mengisinya tanpa mengubah kode.
-- Nilai kosong berarti field itu BELUM TERSEDIA; ekspor menyebutkan kuncinya, bukan
-- mengisi angka/nilai karangan.
--
-- Kunci ini sengaja tidak diisi nilai (''), sehingga perilaku ekspor sebelum bank
-- mengisi identitas tetap "belum tersedia".

INSERT INTO system_config (key, value, description) VALUES
    ('ojk.bank.email', '',
     'Alamat surel kantor pusat BPR untuk Form 00.00 butir 6 (Lampiran II SEOJK 16/2024).'),
    ('ojk.bank.website', '',
     'Alamat situs web kantor pusat BPR untuk Form 00.00 butir 7; kosongkan bila belum ada.'),
    ('ojk.bank.city_code', '',
     'Sandi Kabupaten/Kota kedudukan kantor pusat (Lampiran 03 SEOJK 16/2024) untuk Form 00.00 butir 3.'),
    ('ojk.bank.ojk_region_code', '',
     'Sandi wilayah kerja OJK kantor pusat (Lampiran 06 SEOJK 16/2024) untuk Form 00.00 butir 4.'),
    ('ojk.report.pic_name', '',
     'Nama penanggung jawab laporan OJK untuk Form 00.00 butir 9.a.'),
    ('ojk.report.pic_division', '',
     'Bagian/divisi penanggung jawab laporan OJK untuk Form 00.00 butir 9.b.'),
    ('ojk.report.pic_phone', '',
     'Nomor telepon penanggung jawab laporan OJK untuk Form 00.00 butir 9.c.'),
    ('ojk.report.pic_email', '',
     'Alamat surel penanggung jawab laporan OJK untuk Form 00.00 butir 9.d.')
ON CONFLICT (key) DO NOTHING;
