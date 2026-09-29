-- CBS Migration 000119: register aset tetap, inventaris, dan aset tidak berwujud
-- untuk Form 08.00 "Daftar Aset Tetap, Inventaris, dan Aset Tidak Berwujud".
-- Run after: 000118_properti_terbengkalai_register.up.sql (urutan; tidak bergantung
-- isinya). Nomor 000119 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 08.00 "DAFTAR ASET TETAP, INVENTARIS, DAN ASET TIDAK BERWUJUD":
--     PDF #page 182 (hlm. tercetak 130). Sembilan kolom: I Sandi Kantor, II Jenis Aset,
--     III Sumber Perolehan, IV Status Aset, V Biaya Perolehan,
--     VI Akumulasi Penyusutan/Amortisasi, VII Akumulasi Kerugian Penurunan Nilai,
--     VIII Nilai Tercatat, IX Metode Pengukuran. Ada baris JUMLAH.
--   * Sandi: PDF #page 183 (hlm. 131). II 101 Tanah / 102 Bangunan /
--     103 Peralatan dan perlengkapan / 104 Kendaraan / 199 Lainnya (aset tetap dan
--     inventaris), 201 Program aplikasi (software) / 202 Goodwill / 299 Lainnya (aset
--     tidak berwujud). III 01 Sewa Pembiayaan / 02 Modal Disetor / 03 Modal Sumbangan /
--     99 Sumber Perolehan Lainnya. IV 1 Dijaminkan / 2 Tidak Dijaminkan.
--     IX 1 Model Biaya / 2 Model Revaluasi.
--   * Penjelasan: PDF #page 184-186 (hlm. 132-134). "BPR dapat menggabungkan
--     pelaporan aset tetap dan inventaris serta aset tidak berwujud yang memiliki
--     kesamaan sandi rincian dan angka pada kolom I sampai dengan kolom VI."
--     Kolom IV Status Aset "dikosongkan untuk aset tidak berwujud" (PDF #185).
--     Kolom V memakai nilai aset setelah revaluasi bila aset telah direvaluasi.
--     Kolom VIII Nilai Tercatat = biaya perolehan dikurangi akumulasi penyusutan
--     atau amortisasi dan kerugian penurunan nilai.
--
-- KEPUTUSAN REGISTER: register disimpan PER ASET (satu baris = satu item), bukan per
-- kombinasi. Builder laporan (form08_00.go) yang mengelompokkan baris per kombinasi
-- Jenis (II) x Sumber Perolehan (III) x Status (IV) x Metode Pengukuran (IX) lalu
-- menjumlahkan kolom nominal, sesuai aturan penggabungan PDF #184. Bank mengisi per
-- aset; form tetap menyajikan agregat per kombinasi.
--
-- Kolom VIII "Nilai Tercatat" SENGAJA TIDAK disimpan: ia turunan yang dihitung laporan
-- dari V - VI - VII. Menyimpannya akan membuat dua sumber kebenaran yang bisa berbeda.
--
-- Kolom IV "Status Aset" NULLABLE: aturan form mengosongkannya untuk aset tidak
-- berwujud (kode 2xx). Register tidak memaksa 1/2 untuk jenis tidak berwujud; jenis
-- berwujud yang statusnya belum diisi dibiarkan kosong dan laporan menuliskan "-"
-- beserta alasannya, bukan menebak.
--
-- Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
-- tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--
-- Semua NILAI (sandi, nominal) adalah isian bank. Tidak ada seed angka.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS aset_tetap_register (
    id                                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Jenis Aset (kolom II) baku PDF #page 183. 1xx aset tetap dan inventaris,
    -- 2xx aset tidak berwujud.
    jenis_aset_code                     VARCHAR(3) NOT NULL
        CHECK (jenis_aset_code IN ('101', '102', '103', '104', '199',
                                   '201', '202', '299')),
    -- Sumber Perolehan (kolom III) baku PDF #page 183.
    sumber_perolehan_code               VARCHAR(2) NOT NULL
        CHECK (sumber_perolehan_code IN ('01', '02', '03', '99')),
    -- Status Aset (kolom IV) baku PDF #page 183: 1 Dijaminkan, 2 Tidak Dijaminkan.
    -- NULLABLE: form mengosongkan kolom ini untuk aset tidak berwujud, dan register
    -- tidak menebak nilainya untuk aset berwujud yang belum diisi bank.
    status_aset_code                    VARCHAR(1) NULL
        CHECK (status_aset_code IS NULL OR status_aset_code IN ('1', '2')),
    -- Biaya Perolehan (kolom V), rupiah penuh; isian bank. Bila aset telah
    -- direvaluasi, bank mengisi nilai setelah revaluasi (PDF #page 185).
    biaya_perolehan                     NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (biaya_perolehan >= 0),
    -- Akumulasi Penyusutan/Amortisasi (kolom VI), rupiah penuh; isian bank.
    akumulasi_penyusutan_amortisasi     NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (akumulasi_penyusutan_amortisasi >= 0),
    -- Akumulasi Kerugian Penurunan Nilai (kolom VII), rupiah penuh; isian bank.
    -- Hanya berarti bila ada bukti objektif penurunan nilai (PDF #page 185).
    akumulasi_kerugian_penurunan_nilai  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (akumulasi_kerugian_penurunan_nilai >= 0),
    -- Metode Pengukuran (kolom IX) baku PDF #page 183: 1 Model Biaya, 2 Model Revaluasi.
    metode_pengukuran_code              VARCHAR(1) NOT NULL
        CHECK (metode_pengukuran_code IN ('1', '2')),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                               DATE NOT NULL,
    -- AKTIF = aset masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan
    -- lagi tetapi tetap tersimpan sebagai riwayat.
    status                              VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                                TEXT NOT NULL DEFAULT '',
    created_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                          UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                          UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE aset_tetap_register IS
    'Register aset tetap, inventaris, dan aset tidak berwujud untuk Form 08.00 "Daftar Aset Tetap, Inventaris, dan Aset Tidak Berwujud"; sumber struktur PDF #page 182-186 (hlm. 130-134) SEOJK No. 16/SEOJK.03/2024. Disimpan PER ASET; laporan mengelompokkan per kombinasi Jenis x Sumber Perolehan x Status x Metode Pengukuran (PDF #184). Semua nilai diisi bank. Kolom VIII Nilai Tercatat tidak disimpan karena turunan V - VI - VII.';

COMMENT ON COLUMN aset_tetap_register.jenis_aset_code IS
    'Jenis Aset (kolom II) baku PDF #page 183: 101 Tanah, 102 Bangunan, 103 Peralatan dan perlengkapan, 104 Kendaraan, 199 Lainnya (aset tetap dan inventaris); 201 Program aplikasi (software), 202 Goodwill, 299 Lainnya (aset tidak berwujud).';

COMMENT ON COLUMN aset_tetap_register.sumber_perolehan_code IS
    'Sumber Perolehan (kolom III) baku PDF #page 183: 01 Sewa Pembiayaan, 02 Modal Disetor, 03 Modal Sumbangan, 99 Sumber Perolehan Lainnya.';

COMMENT ON COLUMN aset_tetap_register.status_aset_code IS
    'Status Aset (kolom IV) baku PDF #page 183: 1 Dijaminkan, 2 Tidak Dijaminkan. NULLABLE: form mengosongkan kolom ini untuk aset tidak berwujud; untuk aset berwujud yang belum diisi, laporan menulis "-" beserta alasannya.';

COMMENT ON COLUMN aset_tetap_register.biaya_perolehan IS
    'Biaya Perolehan (kolom V), rupiah penuh. Bila aset telah direvaluasi, bank mengisi nilai aset setelah revaluasi (PDF #page 185).';

COMMENT ON COLUMN aset_tetap_register.akumulasi_penyusutan_amortisasi IS
    'Akumulasi Penyusutan/Amortisasi (kolom VI), rupiah penuh; isian bank.';

COMMENT ON COLUMN aset_tetap_register.akumulasi_kerugian_penurunan_nilai IS
    'Akumulasi Kerugian Penurunan Nilai (kolom VII), rupiah penuh; isian bank. Hanya berarti bila ada bukti objektif penurunan nilai (PDF #page 185). Laporan menghitung Nilai Tercatat (VIII) = V - VI - VII.';

COMMENT ON COLUMN aset_tetap_register.metode_pengukuran_code IS
    'Metode Pengukuran (kolom IX) baku PDF #page 183: 1 Model Biaya, 2 Model Revaluasi.';

COMMENT ON COLUMN aset_tetap_register.as_of IS
    'Tanggal posisi aset (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN aset_tetap_register.status IS
    'AKTIF = aset masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_aset_tetap_register_jenis
    ON aset_tetap_register (jenis_aset_code);
CREATE INDEX IF NOT EXISTS idx_aset_tetap_register_sumber
    ON aset_tetap_register (sumber_perolehan_code);
CREATE INDEX IF NOT EXISTS idx_aset_tetap_register_metode
    ON aset_tetap_register (metode_pengukuran_code);
CREATE INDEX IF NOT EXISTS idx_aset_tetap_register_as_of
    ON aset_tetap_register (as_of);
CREATE INDEX IF NOT EXISTS idx_aset_tetap_register_status
    ON aset_tetap_register (status);
