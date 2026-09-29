-- CBS Migration 000122: register surat berharga untuk Form 04.00 "Daftar Surat Berharga".
-- Run after: 000121_aset_keuangan_lainnya_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000122 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 04.00 "DAFTAR SURAT BERHARGA": PDF #page 129 (hlm. tercetak 77) kolom I-VI,
--     lanjutan PDF #page 130 (hlm. 78) kolom VII-XV, lanjutan PDF #page 131 (hlm. 79)
--     kolom XVI-XXV. Dua puluh lima kolom: I Sandi Kantor, II Klasifikasi, III Suku Bunga,
--     IV Jangka Waktu (Tanggal Mulai + Tanggal Jatuh Tempo), V Nominal,
--     VI Nominal yang Dijaminkan, VII Biaya Perolehan,
--     VIII Diskonto/Premium Belum Diamortisasi, IX Biaya Transaksi Belum Diamortisasi,
--     X Laba/Rugi Belum Direalisasi, XI Biaya Perolehan Diamortisasi/Nilai Wajar,
--     XII Nomor Surat Berharga, XIII ID Pihak Lawan, XIV Jenis, XV Kualitas,
--     XVI Cadangan Kerugian Penurunan Nilai, XVII Lembaga Pemeringkat,
--     XVIII Peringkat Surat Berharga, XIX Tanggal Pemeringkatan, XX Tanggal Penerbitan,
--     XXI CKPN Aset Baik, XXII CKPN Aset Kurang Baik, XXIII CKPN Aset Tidak Baik,
--     XXIV Klasifikasi Aset Keuangan, XXV Jenis CKPN. Satu baris = satu surat berharga;
--     ADA baris JUMLAH (PDF #page 129).
--   * Sandi: PDF #page 132-133 (hlm. 80-81). II 1 Tersedia untuk dijual / 2 Dimiliki
--     hingga jatuh tempo; XIV 1 diterbitkan Bank Indonesia / 2 Pemerintah /
--     3 Pemerintah Daerah; XV 1 Lancar / 3 Kurang Lancar / 5 Macet;
--     XXIV 1 nilai wajar melalui laba rugi / 2 nilai wajar melalui penghasilan
--     komprehensif lain / 3 biaya perolehan diamortisasi; XXV 1 Individual / 2 Kolektif.
--   * Penjelasan: PDF #page 134-136 (hlm. 82-84). XI: nilai nominal - diskonto belum
--     diamortisasi + premium belum diamortisasi + biaya transaksi belum diamortisasi,
--     ATAU nilai wajar. XXIV hanya diisi apabila BPR melakukan penawaran umum efek di
--     pasar modal. XVII diisi sandi 9 bila pihak lawan tanpa peringkat / peringkat dari
--     lembaga yang tidak diakui OJK; XVIII diisi sandi 99 pada keadaan sama.
--
-- KEPUTUSAN KOLOM:
--   * Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
--     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--   * Kolom XII "Nomor Surat Berharga" adalah ISIN (teks), disimpan apa adanya sebagai
--     teks bebas + hint; tidak ada tabel sandi ISIN di form.
--   * Kolom XVII/XVIII disimpan sebagai sandi (VARCHAR) mengikuti Lampiran 08/09 dengan
--     nilai sentinel resmi 9 (tanpa peringkat) dan 99 (tanpa peringkat surat berharga).
--     Daftar lengkap Lampiran 08/09 tidak disalin ke kode (bukan bagian form); kolom
--     tidak diberi CHECK whitelist yang akan mengunci sandi yang sah di luar salinan,
--     tetapi panjang dibatasi dan sentinel diizinkan. HINT: bank merujuk Lampiran 08/09.
--   * Kolom XI "Biaya Perolehan Diamortisasi/Nilai Wajar" adalah ISIAN BANK: form
--     memberikan DUA kemungkinan (amortized cost atau nilai wajar) tanpa aturan pengikat
--     tunggal, sehingga laporan tidak menghitungnya; rumus officiaL dicatat di Notes form.
--   * Sandi II/XIV/XV/XXIV/XXV adalah ISIAN BANK dan ditegakkan CHECK di sini + validasi
--     domain; TIDAK diturunkan dari kolom lain. Aturan historis Bab II "Desember 2024 =
--     Kolektif (2)" TIDAK diterapkan karena masa berlakunya sudah lewat.
--   * Seluruh nilai V-XI dan XVI serta XXI-XXIII adalah ISIAN BANK.
--   * Kolom XIII "ID Pihak Lawan" bukan sandi: disimpan sebagai teks apa adanya.
--   * Tidak ada kolom identitas pribadi pada form ini.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS surat_berharga_register (
    id                                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Klasifikasi (kolom II) baku PDF #page 132: 1 Tersedia untuk dijual,
    -- 2 Dimiliki hingga jatuh tempo.
    klasifikasi_code                    VARCHAR(1) NOT NULL
        CHECK (klasifikasi_code IN ('1', '2')),
    -- Suku Bunga (kolom III), persen; isian bank.
    suku_bunga                          NUMERIC(9,4) NOT NULL DEFAULT 0
        CHECK (suku_bunga >= 0),
    -- Jangka Waktu (kolom IV): Tanggal Mulai dan Tanggal Jatuh Tempo.
    tanggal_mulai                       DATE NOT NULL,
    tanggal_jatuh_tempo                 DATE NOT NULL,
    -- Nominal (kolom V), rupiah penuh; isian bank.
    nominal                             NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (nominal >= 0),
    -- Nominal yang Dijaminkan (kolom VI), rupiah penuh; isian bank.
    nominal_dijaminkan                  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (nominal_dijaminkan >= 0),
    -- Biaya Perolehan (kolom VII), rupiah penuh; isian bank.
    biaya_perolehan                     NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (biaya_perolehan >= 0),
    -- Diskonto/Premium Belum Diamortisasi (kolom VIII), rupiah penuh; isian bank.
    diskonto_premium_belum_diamortisasi NUMERIC(28,4) NOT NULL DEFAULT 0,
    -- Biaya Transaksi Belum Diamortisasi (kolom IX), rupiah penuh; isian bank.
    biaya_transaksi_belum_diamortisasi  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (biaya_transaksi_belum_diamortisasi >= 0),
    -- Laba/Rugi Belum Direalisasi (kolom X), rupiah penuh; isian bank (boleh negatif).
    laba_rugi_belum_direalisasi         NUMERIC(28,4) NOT NULL DEFAULT 0,
    -- Biaya Perolehan Diamortisasi/Nilai Wajar (kolom XI), rupiah penuh; ISIAN BANK
    -- (form memberi dua kemungkinan, tidak ada rumus pengikat tunggal).
    biaya_perolehan_diamortisasi        NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (biaya_perolehan_diamortisasi >= 0),
    -- Nomor Surat Berharga (kolom XII): ISIN, teks apa adanya.
    nomor_surat_berharga                VARCHAR(64) NOT NULL DEFAULT '',
    -- ID Pihak Lawan (kolom XIII): pengenal pihak lawan, teks apa adanya.
    counterparty_id                     VARCHAR(64) NOT NULL DEFAULT '',
    -- Jenis (kolom XIV) baku PDF #page 132: 1 Bank Indonesia, 2 Pemerintah,
    -- 3 Pemerintah Daerah.
    jenis_code                          VARCHAR(1) NOT NULL
        CHECK (jenis_code IN ('1', '2', '3')),
    -- Kualitas (kolom XV) baku PDF #page 132: 1 Lancar, 3 Kurang Lancar, 5 Macet.
    kualitas_code                       VARCHAR(1) NOT NULL
        CHECK (kualitas_code IN ('1', '3', '5')),
    -- Cadangan Kerugian Penurunan Nilai (kolom XVI), rupiah penuh; isian bank.
    ckpn                                NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn >= 0),
    -- Lembaga Pemeringkat (kolom XVII) baku Lampiran 08; sentinel resmi 9 = tanpa
    -- peringkat (PDF #page 135). Disimpan teks sandi; daftar lengkap Lampiran 08 tidak
    -- disalin ke kode.
    lembaga_pemeringkat_code            VARCHAR(9) NOT NULL DEFAULT '',
    -- Peringkat Surat Berharga (kolom XVIII) baku Lampiran 09; sentinel resmi 99 = tanpa
    -- peringkat (PDF #page 135).
    peringkat_surat_berharga_code       VARCHAR(9) NOT NULL DEFAULT '',
    -- Tanggal Pemeringkatan (kolom XIX); boleh kosong bila tanpa peringkat.
    tanggal_pemeringkatan               DATE,
    -- Tanggal Penerbitan (kolom XX).
    tanggal_penerbitan                  DATE NOT NULL,
    -- CKPN Aset Baik (kolom XXI), rupiah penuh; isian bank.
    ckpn_aset_baik                      NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_baik >= 0),
    -- CKPN Aset Kurang Baik (kolom XXII), rupiah penuh; isian bank.
    ckpn_aset_kurang_baik               NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_kurang_baik >= 0),
    -- CKPN Aset Tidak Baik (kolom XXIII), rupiah penuh; isian bank.
    ckpn_aset_tidak_baik                NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_tidak_baik >= 0),
    -- Klasifikasi Aset Keuangan (kolom XXIV) baku PDF #page 133: 1 nilai wajar melalui
    -- laba rugi, 2 nilai wajar melalui penghasilan komprehensif lain, 3 biaya perolehan
    -- diamortisasi. Hanya diisi bila BPR melakukan penawaran umum efek di pasar modal
    -- (PDF #page 135); karena itu boleh kosong.
    klasifikasi_aset_keuangan_code      VARCHAR(1)
        CHECK (klasifikasi_aset_keuangan_code IN ('1', '2', '3')),
    -- Jenis CKPN (kolom XXV) baku PDF #page 133: 1 Individual, 2 Kolektif. Isian bank;
    -- aturan historis "Desember 2024 = Kolektif" tidak diterapkan.
    jenis_ckpn_code                     VARCHAR(1) NOT NULL
        CHECK (jenis_ckpn_code IN ('1', '2')),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                               DATE NOT NULL,
    status                              VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                                TEXT NOT NULL DEFAULT '',
    created_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                          UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                          UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE surat_berharga_register IS
    'Register surat berharga untuk Form 04.00 "Daftar Surat Berharga"; sumber struktur PDF #page 129-136 (hlm. 77-84) SEOJK No. 16/SEOJK.03/2024. Satu baris = satu surat berharga yang dimiliki; ADA baris JUMLAH di laporan. Semua nilai dan sandi diisi bank. Form 04.00 tidak menetapkan nomor register unik, sehingga tabel ini tidak memiliki UNIQUE selain PK dan penghapusan boleh DELETE fisik.';

COMMENT ON COLUMN surat_berharga_register.klasifikasi_code IS
    'Klasifikasi (kolom II) baku PDF #page 132: 1 Tersedia untuk dijual, 2 Dimiliki hingga jatuh tempo. Isian bank.';

COMMENT ON COLUMN surat_berharga_register.suku_bunga IS
    'Suku Bunga (kolom III), persen; isian bank.';

COMMENT ON COLUMN surat_berharga_register.tanggal_mulai IS
    'Jangka Waktu - Tanggal Mulai (kolom IV), diisi bank.';

COMMENT ON COLUMN surat_berharga_register.tanggal_jatuh_tempo IS
    'Jangka Waktu - Tanggal Jatuh Tempo (kolom IV), diisi bank.';

COMMENT ON COLUMN surat_berharga_register.nominal IS
    'Nominal (kolom V), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.nominal_dijaminkan IS
    'Nominal yang Dijaminkan (kolom VI), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.biaya_perolehan IS
    'Biaya Perolehan (kolom VII), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.diskonto_premium_belum_diamortisasi IS
    'Diskonto/Premium Belum Diamortisasi (kolom VIII), rupiah penuh; isian bank. Dapat negatif untuk premium.';

COMMENT ON COLUMN surat_berharga_register.biaya_transaksi_belum_diamortisasi IS
    'Biaya Transaksi Belum Diamortisasi (kolom IX), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.laba_rugi_belum_direalisasi IS
    'Laba/Rugi Belum Direalisasi (kolom X), rupiah penuh; isian bank. Dapat negatif (rugi).';

COMMENT ON COLUMN surat_berharga_register.biaya_perolehan_diamortisasi IS
    'Biaya Perolehan Diamortisasi/Nilai Wajar (kolom XI), rupiah penuh; ISIAN BANK. Form (PDF #page 135) memberi dua kemungkinan: nilai nominal - diskonto belum diamortisasi + premium belum diamortisasi + biaya transaksi belum diamortisasi, ATAU nilai wajar; tidak ada rumus pengikat tunggal, sehingga laporan tidak menghitungnya.';

COMMENT ON COLUMN surat_berharga_register.nomor_surat_berharga IS
    'Nomor Surat Berharga (kolom XII): ISIN, teks apa adanya. Form tidak menyediakan tabel sandi untuk kolom ini.';

COMMENT ON COLUMN surat_berharga_register.counterparty_id IS
    'ID Pihak Lawan (kolom XIII): pengenal pihak lawan yang bank catat, teks apa adanya.';

COMMENT ON COLUMN surat_berharga_register.jenis_code IS
    'Jenis (kolom XIV) baku PDF #page 132: 1 diterbitkan Bank Indonesia, 2 Pemerintah, 3 Pemerintah Daerah. Isian bank.';

COMMENT ON COLUMN surat_berharga_register.kualitas_code IS
    'Kualitas (kolom XV) baku PDF #page 132: 1 Lancar, 3 Kurang Lancar, 5 Macet (hanya tiga ini tercetak di form). Isian bank.';

COMMENT ON COLUMN surat_berharga_register.ckpn IS
    'Cadangan Kerugian Penurunan Nilai (kolom XVI), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.lembaga_pemeringkat_code IS
    'Lembaga Pemeringkat (kolom XVII) baku Lampiran 08; sentinel resmi 9 = pihak lawan tanpa peringkat atau peringkat dari lembaga yang tidak diakui OJK (PDF #page 135). Daftar lengkap Lampiran 08 tidak disalin ke kode.';

COMMENT ON COLUMN surat_berharga_register.peringkat_surat_berharga_code IS
    'Peringkat Surat Berharga (kolom XVIII) baku Lampiran 09; sentinel resmi 99 = tanpa peringkat (PDF #page 135).';

COMMENT ON COLUMN surat_berharga_register.tanggal_pemeringkatan IS
    'Tanggal Pemeringkatan (kolom XIX); boleh kosong bila surat berharga tanpa peringkat.';

COMMENT ON COLUMN surat_berharga_register.tanggal_penerbitan IS
    'Tanggal Penerbitan (kolom XX), diisi bank.';

COMMENT ON COLUMN surat_berharga_register.ckpn_aset_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Baik (kolom XXI), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.ckpn_aset_kurang_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Kurang Baik (kolom XXII), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.ckpn_aset_tidak_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Tidak Baik (kolom XXIII), rupiah penuh; isian bank.';

COMMENT ON COLUMN surat_berharga_register.klasifikasi_aset_keuangan_code IS
    'Klasifikasi Aset Keuangan (kolom XXIV) baku PDF #page 133: 1 nilai wajar melalui laba rugi, 2 nilai wajar melalui penghasilan komprehensif lain, 3 biaya perolehan diamortisasi. Hanya diisi bila BPR melakukan penawaran umum efek di pasar modal (PDF #page 135), sehingga boleh kosong.';

COMMENT ON COLUMN surat_berharga_register.jenis_ckpn_code IS
    'Jenis CKPN (kolom XXV) baku PDF #page 133: 1 Individual, 2 Kolektif. Isian bank; aturan historis Desember 2024 = Kolektif tidak diterapkan karena masa berlakunya sudah lewat.';

COMMENT ON COLUMN surat_berharga_register.as_of IS
    'Tanggal posisi surat berharga (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN surat_berharga_register.status IS
    'AKTIF = surat berharga masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_surat_berharga_klasifikasi
    ON surat_berharga_register (klasifikasi_code);
CREATE INDEX IF NOT EXISTS idx_surat_berharga_jenis
    ON surat_berharga_register (jenis_code);
CREATE INDEX IF NOT EXISTS idx_surat_berharga_kualitas
    ON surat_berharga_register (kualitas_code);
CREATE INDEX IF NOT EXISTS idx_surat_berharga_as_of
    ON surat_berharga_register (as_of);
CREATE INDEX IF NOT EXISTS idx_surat_berharga_status
    ON surat_berharga_register (status);
