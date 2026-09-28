-- CBS Migration 000115: register AYDA (Agunan yang Diambil Alih) untuk Form 07.00
-- "Daftar Agunan yang Diambil Alih".
-- Run after: 000114_form09_aset_lainnya_coa.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000115 dipesan khusus pekerjaan ini.
--
-- Menutup satu laporan triase (docs/CELAH-FORM-OJK.md:62): Form 07.00 sebelumnya
-- "BELUM DIMODELKAN" karena baru ada saldo agregat COA 10500 (coa_mapping.go:110).
-- Tabel ADITIF di bawah menjadi register per kasus AYDA; tidak ada tabel/kolom yang
-- diubah. Akun 10500 TIDAK disentuh: register = rincian per kasus yang bank catat,
-- sedangkan 10500 = posisi buku besar. Keduanya dapat berbeda bila bank belum mengisi
-- semua kasus, sehingga laporan TIDAK memaksa total register sama dengan saldo COA.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 07.00 - 1 "DAFTAR AGUNAN YANG DIAMBIL ALIH": PDF #page 179 (hlm. tercetak
--     127). Delapan kolom: I Sandi Kantor, II Jenis Agunan, III Alamat Agunan,
--     IV Tanggal Pengambilalihan (TT-BB-TTTT), V Nilai Pengakuan Awal,
--     VI Akumulasi Kerugian Penurunan Nilai, VII Jumlah,
--     VIII Nilai Bersih Yang Dapat Direalisasikan. Ada baris JUMLAH di kaki tabel.
--   * Form 07.00 - 2 "SANDI DAFTAR AGUNAN YANG DIAMBIL ALIH": PDF #page 180 (hlm.
--     128). Sandi II Jenis Agunan: 01 Emas perhiasan; 02 Tanah dan/atau bangunan;
--     03 Resi Gudang; 04 Tempat usaha (los/kios/lapak); 05 Kendaraan bermotor, kapal,
--     perahu bermotor, alat berat, mesin yang menjadi kesatuan dengan tanah; 99 Lainnya.
--   * Form 07.00 - 3 "PENJELASAN": PDF #page 181 (hlm. 129). V = nilai wajar AYDA
--     setelah dikurangi estimasi biaya penjualan (net realizable value) saat diambil
--     alih, paling tinggi sebesar baki debet kredit debitur; VI = NRV posisi laporan
--     dikurangi nilai pengakuan awal; VII = nilai yang lebih rendah dari NRV posisi
--     laporan atau nilai tercatat; VIII = nilai wajar AYDA setelah dikurangi estimasi
--     biaya pelepasan.
--
-- Kolom VII Jumlah SENGAJA TIDAK disimpan: ia turunan yang dihitung laporan dari
-- nilai pengakuan awal (V) dan akumulasi kerugian penurunan nilai (VI). Menyimpannya
-- akan membuat dua sumber kebenaran yang bisa berbeda.
--
-- Kolom V/VI/VIII adalah DATA BANK. Sistem tidak menurunkan V dari nilai wajar,
-- estimasi biaya penjualan, dan baki debet karena ketiganya bukan kolom register;
-- laporan memakai angka bank apa adanya dan mencatat batas itu pada Notes.
--
-- Semua NILAI (sandi jenis agunan, alamat, tanggal, nominal) adalah isian bank. Tidak
-- ada seed angka. Sandi Kantor (kolom I) juga tidak disimpan di sini: ia diambil dari
-- kantor pelapor tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS ayda_register (
    id                        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Sandi jenis agunan baku Form 07.00 - 2 (01/02/03/04/05/99). Baku karena
    -- maknanya baku di form; bukan sandi pos regulasi yang berubah.
    collateral_type_code      VARCHAR(2) NOT NULL
        CHECK (collateral_type_code IN ('01', '02', '03', '04', '05', '99')),
    -- Alamat lengkap agunan (kolom III). Wajib terisi agar baris dapat dibaca.
    collateral_address        VARCHAR(255) NOT NULL CHECK (length(btrim(collateral_address)) > 0),
    -- Tanggal pengambilalihan AYDA (kolom IV, format laporan TT-BB-TTTT).
    acquisition_date          DATE NOT NULL,
    -- Nilai Pengakuan Awal (kolom V): nilai wajar dikurangi estimasi biaya penjualan
    -- saat diambil alih, paling tinggi baki debet. Angka bank, bukan hitungan sistem.
    initial_recognition_value NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (initial_recognition_value >= 0),
    -- Akumulasi Kerugian Penurunan Nilai (kolom VI): disimpan sebagai BESARAN
    -- kerugian (>= 0); laporan mengurangi nilai pengakuan awal dengan angka ini.
    accumulated_impairment    NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (accumulated_impairment >= 0),
    -- Nilai Bersih yang Dapat Direalisasikan (kolom VIII): nilai wajar AYDA setelah
    -- dikurangi estimasi biaya pelepasan. Angka bank.
    net_realizable_value      NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (net_realizable_value >= 0),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                     DATE NOT NULL,
    -- AKTIF = AYDA masih dimiliki dan ikut laporan; NONAKTIF = sudah dilepas/tidak
    -- dilaporkan lagi tetapi tetap tersimpan sebagai riwayat.
    status                    VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                      TEXT NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE ayda_register IS
    'Register Agunan yang Diambil Alih (AYDA) untuk Form 07.00 "Daftar Agunan yang Diambil Alih"; sumber struktur PDF #page 179-181 (hlm. 127-129) SEOJK No. 16/SEOJK.03/2024. Semua nilai diisi bank. Register adalah rincian per kasus; saldo buku besar AYDA tetap akun COA 10500, sehingga total register tidak dipaksa sama dengan saldo COA.';

COMMENT ON COLUMN ayda_register.collateral_type_code IS
    'Sandi jenis agunan baku Form 07.00 - 2: 01 Emas perhiasan, 02 Tanah dan/atau bangunan, 03 Resi Gudang, 04 Tempat usaha (los/kios/lapak), 05 Kendaraan bermotor/kapal/alat berat, 99 Lainnya.';

COMMENT ON COLUMN ayda_register.collateral_address IS
    'Alamat lengkap agunan (kolom III). Wajib terisi agar baris dapat dibaca; tidak ada alamat yang dikarang sistem.';

COMMENT ON COLUMN ayda_register.acquisition_date IS
    'Tanggal pengambilalihan AYDA (kolom IV). Ditulis laporan dalam format TT-BB-TTTT sesuai Form 07.00 - 2.';

COMMENT ON COLUMN ayda_register.initial_recognition_value IS
    'Nilai Pengakuan Awal (kolom V): nilai wajar dikurangi estimasi biaya penjualan saat diambil alih, paling tinggi baki debet. Angka bank; sistem tidak menghitungnya karena nilai wajar, estimasi biaya penjualan, dan baki debet bukan kolom register.';

COMMENT ON COLUMN ayda_register.accumulated_impairment IS
    'Akumulasi Kerugian Penurunan Nilai (kolom VI) sebagai BESARAN kerugian >= 0. Laporan menghitung Jumlah = nilai yang lebih rendah dari NRV dan (nilai pengakuan awal - akumulasi ini).';

COMMENT ON COLUMN ayda_register.net_realizable_value IS
    'Nilai Bersih yang Dapat Direalisasikan (kolom VIII): nilai wajar AYDA setelah dikurangi estimasi biaya pelepasan. Angka bank. Kolom VII Jumlah tidak disimpan karena turunan dari V dan VI.';

COMMENT ON COLUMN ayda_register.as_of IS
    'Tanggal posisi AYDA (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN ayda_register.status IS
    'AKTIF = AYDA masih dimiliki dan ikut laporan; NONAKTIF = sudah dilepas/dihentikan pelaporannya namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_ayda_register_collateral_type
    ON ayda_register (collateral_type_code);
CREATE INDEX IF NOT EXISTS idx_ayda_register_acquisition_date
    ON ayda_register (acquisition_date);
CREATE INDEX IF NOT EXISTS idx_ayda_register_as_of
    ON ayda_register (as_of);
CREATE INDEX IF NOT EXISTS idx_ayda_register_status
    ON ayda_register (status);
