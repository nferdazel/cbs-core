-- CBS Migration 000111: amortisasi provisi/biaya & komponen restrukturisasi Form 06.00.
-- Run after: 000110_ojk_bank_deposit_location.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000111 dipesan khusus pekerjaan ini.
--
-- Menutup lima kolom K2 Form 06.00 (docs/CELAH-LAPORAN-OJK.md §2c):
--   * loans.ojk_provisi_belum_diamortisasi_amount          -> XXIX  Provisi Belum Diamortisasi
--   * loans.ojk_biaya_transaksi_belum_diamortisasi_amount  -> XXX   Biaya Transaksi Belum Diamortisasi
--   * loans.ojk_pendapatan_bunga_ditangguhkan_amount       -> XXXI  Pendapatan Bunga Ditangguhkan Dalam Rangka Restrukturisasi
--   * loans.ojk_cadangan_kerugian_restrukturisasi_amount   -> XXXII Cadangan Kerugian Restrukturisasi
--   * (XXXIII Baki Debet Neto TIDAK menyimpan kolom; dihitung perakit laporan dari
--      baki debet + keempat komponen di atas sesuai definisi resmi.)
--
-- Alasan keempat kolom ini bank-fill dan bukan hasil mesin: jadwal amortisasi provisi
-- dan biaya transaksi belum dimodelkan (baru akrual/amortisasi saldo kerugian
-- restrukturisasi yang ada), dan pendapatan bunga ditangguhkan akibat kapitalisasi
-- tunggakan bunga belum dicatat per kredit. Tidak ada nilai tersimpan yang boleh
-- diturunkan menjadi angka ini; mengarang rumus amortisasi akan melaporkan angka yang
-- tidak dapat ditelusuri ke jurnal. Bank mengisi lewat SQL/seed sampai ada jalur API.
--
-- Definisi resmi (Lampiran II SEOJK 16/2024, Form 06.00 – 3 Penjelasan Daftar Kredit
-- yang Diberikan, hlm. 113 / PDF #page 165):
--   XXIX  : "bagian dari provisi yang belum menjadi pendapatan bunga periode berjalan
--           atas kredit yang diberikan."
--   XXX   : "bagian dari biaya transaksi yang belum diamortisasi dan belum menjadi
--           pengurang pendapatan bunga periode berjalan atas kredit yang diberikan."
--   XXXI  : "pendapatan bunga yang ditangguhkan dalam rangka restrukturisasi kredit
--           yang dilakukan dengan kapitalisasi tunggakan bunga ke dalam pokok kredit."
--   XXXII : "selisih antara nilai perkiraan arus kas masa depan berdasarkan perjanjian
--           restrukturisasi dengan tingkat diskonto tertentu dan baki debet kredit
--           sebelum restrukturisasi."
-- Definisi yang sama juga tertulis pada Form 01.00 – 2 Penjelasan Laporan Posisi
-- Keuangan butir 5.b–5.e (PDF #page 103, hlm. 51).
--
-- XXXIII Baki Debet Neto didefinisikan pada PDF #page 165, hlm. 113: "baki debet kredit
-- setelah dikurangi dengan provisi yang belum diamortisasi dan ditambah dengan biaya
-- transaksi yang belum diamortisasi serta dikurangi dengan pendapatan bunga yang
-- ditangguhkan dalam rangka restrukturisasi kredit dan cadangan kerugian
-- restrukturisasi." Rumusnya: Baki Debet - XXIX + XXX - XXXI - XXXII. Dihitung perakit
-- laporan (bukan disimpan) agar tidak ada salinan yang bisa menyimpang dari komponennya.
--
-- NULL dibedakan dari 0: NULL = belum diisi/belum dihitung, 0 = nilai yang benar-benar
-- nol. CHECK menjaga nilai tidak negatif bila diisi: keempatnya pos saldo (provisi/biaya/
-- pendapatan ditangguhkan/cadangan) yang bukan nilai bersaldo negatif pada kolom ini.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom/CHECK/COMMENT kedua. Tidak ada data yang diubah.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS ojk_provisi_belum_diamortisasi_amount NUMERIC(28,4)
        CHECK (ojk_provisi_belum_diamortisasi_amount IS NULL OR ojk_provisi_belum_diamortisasi_amount >= 0),
    ADD COLUMN IF NOT EXISTS ojk_biaya_transaksi_belum_diamortisasi_amount NUMERIC(28,4)
        CHECK (ojk_biaya_transaksi_belum_diamortisasi_amount IS NULL OR ojk_biaya_transaksi_belum_diamortisasi_amount >= 0),
    ADD COLUMN IF NOT EXISTS ojk_pendapatan_bunga_ditangguhkan_amount NUMERIC(28,4)
        CHECK (ojk_pendapatan_bunga_ditangguhkan_amount IS NULL OR ojk_pendapatan_bunga_ditangguhkan_amount >= 0),
    ADD COLUMN IF NOT EXISTS ojk_cadangan_kerugian_restrukturisasi_amount NUMERIC(28,4)
        CHECK (ojk_cadangan_kerugian_restrukturisasi_amount IS NULL OR ojk_cadangan_kerugian_restrukturisasi_amount >= 0);

COMMENT ON COLUMN loans.ojk_provisi_belum_diamortisasi_amount IS
    'Provisi belum diamortisasi, sumber Form 06.00 kolom XXIX. Definisi resmi Lampiran II SEOJK 16/2024 Form 06.00-3 (PDF #page 165): "bagian dari provisi yang belum menjadi pendapatan bunga periode berjalan atas kredit yang diberikan". Jadwal amortisasi provisi belum dimodelkan, jadi bank mengisinya lewat SQL/seed; NULL = belum diisi, laporan menulis "-", BUKAN nol, dan tidak diturunkan dari rumus karangan.';

COMMENT ON COLUMN loans.ojk_biaya_transaksi_belum_diamortisasi_amount IS
    'Biaya transaksi belum diamortisasi, sumber Form 06.00 kolom XXX. Definisi resmi Lampiran II SEOJK 16/2024 Form 06.00-3 (PDF #page 165): "bagian dari biaya transaksi yang belum diamortisasi dan belum menjadi pengurang pendapatan bunga periode berjalan atas kredit yang diberikan". Jadwal amortisasi biaya transaksi belum dimodelkan, jadi bank mengisinya lewat SQL/seed; NULL = belum diisi, laporan menulis "-", BUKAN nol.';

COMMENT ON COLUMN loans.ojk_pendapatan_bunga_ditangguhkan_amount IS
    'Pendapatan bunga ditangguhkan dalam rangka restrukturisasi, sumber Form 06.00 kolom XXXI. Definisi resmi Lampiran II SEOJK 16/2024 Form 06.00-3 (PDF #page 165): "pendapatan bunga yang ditangguhkan dalam rangka restrukturisasi kredit yang dilakukan dengan kapitalisasi tunggakan bunga ke dalam pokok kredit". Pencatatan per kreditnya belum dimodelkan, jadi bank mengisinya lewat SQL/seed; NULL = belum diisi, laporan menulis "-", BUKAN nol.';

COMMENT ON COLUMN loans.ojk_cadangan_kerugian_restrukturisasi_amount IS
    'Cadangan kerugian restrukturisasi, sumber Form 06.00 kolom XXXII. Definisi resmi Lampiran II SEOJK 16/2024 Form 06.00-3 (PDF #page 165): "selisih antara nilai perkiraan arus kas masa depan berdasarkan perjanjian restrukturisasi dengan tingkat diskonto tertentu dan baki debet kredit sebelum restrukturisasi". Modul kerugian restrukturisasi menyimpan saldo belum diamortisasi (loans.restructure_loss_balance), tetapi kolom laporan ini adalah saldo cadangan yang dilaporkan per kredit; pemetaannya belum diputuskan akuntan sehingga bank mengisinya lewat SQL/seed. NULL = belum diisi, laporan menulis "-", BUKAN nol.';
