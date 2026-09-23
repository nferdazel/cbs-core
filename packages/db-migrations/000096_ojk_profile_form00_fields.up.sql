-- CBS Migration 000096: penyimpanan 12 field Form 00.00 yang belum punya tempat.
-- Run after: 000094 (000095 dan 000097 dipakai gelombang paralel).
-- Idempotent: INSERT ... ON CONFLICT DO NOTHING; tidak mengubah tabel/kolom mana pun.
--
-- Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 00.00 "Informasi Pokok BPR".
-- Migrasi 000046 sudah menyediakan 8 kunci ojk.* untuk butir 3, 4, 6, 7, dan 9.a-9.d.
-- Dua belas butir berikutnya (10 s.d. 21) sebelumnya tidak punya penyimpanan sama
-- sekali, sehingga ekspor menyatakannya "belum tersedia" tanpa cara mengisinya.
--
-- Nilai sengaja dikosongkan (''). Kosong berarti field BELUM TERSEDIA; ekspor
-- menyebutkan kunci yang harus diisi, bukan mengisi angka/nilai karangan. Bank
-- mengisi kunci ini lewat API (rute /api/v1/system/ojk-profile), bukan SQL.
--
-- Butir yang bentuknya kuantitatif (mis. nominal saham, jumlah agen) tetap disimpan
-- sebagai teks agar bank dapat menuliskan apa adanya; validasi bentuk dilakukan API.

INSERT INTO system_config (key, value, description) VALUES
    ('ojk.report.dividends_paid', '',
     'Dividen yang dibayar pada tahun berjalan untuk Form 00.00 butir 10; kosongkan bila tidak ada.'),
    ('ojk.report.annual_bonus_tantiem', '',
     'Bonus tahunan dan tantiem yang dibayar untuk Form 00.00 butir 11; kosongkan bila tidak ada.'),
    ('ojk.report.audit_info', '',
     'Informasi audit laporan keuangan tahunan (nama KAP/AP dan opini) untuk Form 00.00 butir 12.'),
    ('ojk.report.share_nominal_value', '',
     'Nilai nominal per lembar saham BPR untuk Form 00.00 butir 13.'),
    ('ojk.report.public_offering_status', '',
     'Status penawaran umum efek BPR (mis. tidak melakukan penawaran umum) untuk Form 00.00 butir 14.'),
    ('ojk.report.pva_status', '',
     'Status dan jumlah Pedagang Valuta Asing (PVA) untuk Form 00.00 butir 15; kosongkan bila tidak ada.'),
    ('ojk.report.ebanking_status', '',
     'Jenis layanan perbankan elektronik (e-banking) yang diselenggarakan untuk Form 00.00 butir 16.'),
    ('ojk.report.it_provider', '',
     'Penyelenggara teknologi informasi (mandiri atau pihak ketiga) untuk Form 00.00 butir 17.'),
    ('ojk.report.laku_pandai_provider', '',
     'Penyelenggara Laku Pandai (bank sendiri atau kerja sama) untuk Form 00.00 butir 18.'),
    ('ojk.report.laku_pandai_agent_count', '',
     'Jumlah agen Laku Pandai (angka) untuk Form 00.00 butir 19; kosongkan bila tidak menyelenggarakan.'),
    ('ojk.report.rups_ownership_change', '',
     'Informasi RUPS perubahan kepemilikan (tanggal dan akta) untuk Form 00.00 butir 20.'),
    ('ojk.report.ultimate_shareholders', '',
     'Nama ultimate shareholders BPR untuk Form 00.00 butir 21.')
ON CONFLICT (key) DO NOTHING;
