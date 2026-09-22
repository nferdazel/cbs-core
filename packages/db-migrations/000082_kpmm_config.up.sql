-- 000082_kpmm_config.up.sql
-- Run after: 000081_branch_hierarchy.up.sql
--
-- Parameter perhitungan KPMM/ATMR BPR. KPMM = modal / ATMR (aset neraca berbobot
-- risiko). Modul ini BARU; sebelumnya rasio KPMM ditandai tidak tersedia
-- (apps/api/internal/ojkreport/ratios.go:74-79) karena komponen modal dan ATMR
-- belum dihitung.
--
-- Dasar hukum angka-angka:
--   * POJK No. 5/POJK.03/2015 tentang Kewajiban Penyediaan Modal Minimum dan
--     Pemenuhan Modal Inti Minimum BPR (masih berlaku; digantikan POJK No. 7
--     Tahun 2026). Rasio KPMM minimum 12% (Pasal 2), modal inti minimum 8%
--     (Pasal 4) dan Rp6.000.000.000 (Pasal 14), modal pelengkap paling tinggi
--     100% modal inti (Pasal 3 ayat (2)), PPKA umum paling tinggi 1,25% ATMR
--     masuk modal pelengkap (Pasal 10 ayat (1) huruf c).
--   * SEOJK No. 2/SEOJK.03/2025 (KPMM BPR, berlaku posisi Maret 2025): rincian
--     bobot risiko ATMR pada Lampiran II.
--   * SEOJK No. 21/SEOJK.03/2024 butir 1.1.6 (PA BPR): selisih PPKA lebih besar
--     dari CKPN menjadi faktor pengurang modal inti. Tafsir per-kredit vs
--     agregat belum final; dijadikan setelan.
--
-- SATUAN (konvensi W3): sufiks _frac = fraksi 0..1 (0,12 = 12%), _amount =
-- rupiah penuh. Sisanya kode/teks. Deskripsi menyebut satuan secara eksplisit
-- supaya bank tidak mengisi persen pada kunci fraksi.
--
-- Idempotent: INSERT ... ON CONFLICT (key) DO NOTHING. Tidak mengubah tabel,
-- kolom, akun jurnal, maupun angka akuntansi apa pun. ckpn.enabled TIDAK disentuh.

INSERT INTO system_config (key, value, description) VALUES
    ('kpmm.min_frac', '0.12',
     'Rasio KPMM minimum (fraksi 0..1; 0.12 = 12%). Sumber: POJK No. 5/POJK.03/2015 Pasal 2, dilanjutkan POJK No. 7 Tahun 2026.'),
    ('kpmm.modal_inti_min_frac', '0.08',
     'Rasio modal inti minimum terhadap ATMR (fraksi 0..1; 0.08 = 8%). Sumber: POJK No. 5/POJK.03/2015 Pasal 4.'),
    ('kpmm.modal_inti_min_amount', '6000000000',
     'Modal inti minimum absolut dalam rupiah penuh. Sumber: POJK No. 5/POJK.03/2015 Pasal 14 (Rp6.000.000.000).'),
    ('kpmm.modal_pelengkap_max_frac', '1.00',
     'Batas atas modal pelengkap terhadap modal inti (fraksi 0..1; 1.00 = 100%). Sumber: POJK No. 5/POJK.03/2015 Pasal 3 ayat (2).'),
    ('kpmm.ppka_umum_rwa_max_frac', '0.0125',
     'Batas PPKA umum yang dapat diperhitungkan sebagai modal pelengkap terhadap ATMR (fraksi 0..1; 0.0125 = 1,25%). Sumber: POJK No. 5/POJK.03/2015 Pasal 10 ayat (1) huruf c.'),
    ('kpmm.deduction_basis', 'per_kredit',
     'Dasar pengurang modal inti dari selisih PPKA-CKPN: per_kredit = jumlah max(PPKA_i - CKPN_i, 0) tiap kredit (konservatif, nilai awal); agregat = max(total PPKA - total CKPN, 0) pada tingkat portofolio. Sumber: SEOJK No. 21/SEOJK.03/2024 butir 1.1.6; tafsirnya belum final sehingga dapat disetel bank/OJK.'),
    ('kpmm.rwa_frac.kas', '0.00',
     'Bobot risiko ATMR Kas (fraksi 0..1; 0.00 = 0%). Sumber: SEOJK No. 2/SEOJK.03/2025 Lampiran II (Kas 0%).'),
    ('kpmm.rwa_frac.antar_bank', '0.20',
     'Bobot risiko ATMR penempatan pada bank lain (fraksi 0..1; 0.20 = 20%). Sesuaikan dengan Lampiran II SEOJK No. 2/SEOJK.03/2025 bila bank memakai bobot berbeda.'),
    ('kpmm.rwa_frac.kredit', '1.00',
     'Bobot risiko ATMR kredit/pembiayaan agregat (fraksi 0..1). Nilai awal 1.00 = 100% (konservatif) karena jenis agunan per kredit belum tersedia pada perhitungan ATMR; turunkan hanya bila bank memetakan agunan sesuai SEOJK No. 2/SEOJK.03/2025 (mis. agunan tanah 30%, kendaraan 70%).'),
    ('kpmm.rwa_frac.ayda', '1.00',
     'Bobot risiko ATMR AYDA yang belum melampaui 1 tahun (fraksi 0..1). Sumber: SEOJK No. 2/SEOJK.03/2025 Lampiran II; AYDA >1 tahun berbobot 0% dan menjadi pengurang modal inti.'),
    ('kpmm.rwa_frac.aset_tetap', '1.00',
     'Bobot risiko ATMR aset tetap, inventaris, dan aset tidak berwujud (fraksi 0..1; 1.00 = 100%). Sumber: SEOJK No. 2/SEOJK.03/2025 Lampiran II.'),
    ('kpmm.rwa_frac.antar_kantor', '1.00',
     'Bobot risiko ATMR aset antarkantor (fraksi 0..1). Nilai awal 1.00 = 100% (konservatif); sesuaikan bila bank memakai bobot lain.'),
    ('kpmm.rwa_frac.lainnya', '1.00',
     'Bobot risiko ATMR pos aset lain yang tidak punya kategori khusus (fraksi 0..1; 1.00 = 100%, konservatif). Sumber: SEOJK No. 2/SEOJK.03/2025 Lampiran II (kredit/tagihan lain 100%).')
ON CONFLICT (key) DO NOTHING;
