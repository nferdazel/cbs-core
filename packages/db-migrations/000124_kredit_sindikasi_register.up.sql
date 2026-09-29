-- CBS Migration 000124: register kredit sindikasi untuk Form 06.02
-- "Daftar Kredit Sindikasi".
-- Run after: 000123_kas_valas_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000124 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 06.02 "DAFTAR KREDIT SINDIKASI": PDF #page 173 (hlm. tercetak 121) kolom I-VII,
--     PDF #page 174 (hlm. 122) kolom VIII-XIII. Tiga belas kolom: I Sandi Kantor,
--     II ID Pihak Lawan, III No. Identitas, IV No. Rekening,
--     V Jumlah Pendanaan Sindikasi, VI Bagian Pendanaan,
--     VII Sandi Bank Peserta Sindikasi (Plafon + Baki Debet),
--     VIII Status Kepesertaan, IX Nomor Perjanjian Kredit Sindikasi,
--     X Pendanaan di Bank Pelapor, XI Kualitas,
--     XII Nominal Tunggakan (Pokok + Bunga), XIII Hari Tunggakan (Pokok + Bunga).
--     TIDAK ADA baris JUMLAH.
--   * Sandi: PDF #page 175-176 (hlm. 123-124). VIII 1 Arranger / 2 Anggota sindikasi;
--     X 1 Ya / 2 Tidak; XI 1 Lancar / 2 Dalam Perhatian Khusus / 3 Kurang Lancar /
--     4 Diragukan / 5 Macet.
--   * Penjelasan: PDF #page 177-178 (hlm. 125-126). Kredit sindikasi = kredit yang
--     diberikan bersama-sama dua bank atau lebih (atau perusahaan pembiayaan lain) dengan
--     pembagian dana, risiko, dan pendapatan sesuai porsi kepesertaan. IV setiap rekening
--     fasilitas kredit unik dan "tidak boleh sama", harus sama dengan nomor rekening SLIK,
--     dan DIKOSONGKAN bila X diisi sandi 2 (tidak). V adalah jumlah pendanaan SELURUH
--     anggota sindikasi; VI adalah bagian BPR pelapor. IX diisi nomor perjanjian INDUK awal
--     (perjanjian pertama) tanpa spasi. XIII paling singkat 0.
--
-- KEPUTUSAN KOLOM:
--   * Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
--     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--   * Kolom III "No. Identitas" (NIK/NPWP debitur) SENGAJA TIDAK DISIMPAN mengikuti
--     keputusan privasi docs/KEPUTUSAN-OJK.md (§4 dan §7 butir 6, pola yang sama dengan
--     Form 00.01 dan form berhenti). Kolom ini tidak ada di tabel; laporan selalu menulis
--     "-" beserta alasannya.
--   * Kolom IV "No. Rekening" unik dan "tidak boleh sama" (PDF #page 177). UNIQUE penuh
--     menjaga nomor tetap terpakai walau baris NONAKTIF, sehingga penghapusan adalah
--     soft-delete. Karena IV dikosongkan bila X = 2, kolom ini NULLABLE; keunikan hanya
--     berlaku untuk nilai yang terisi (UNIQUE pada kolom nullable sudah memperlakukan NULL
--     sebagai berbeda di PostgreSQL, jadi beberapa baris "tanpa rekening" diperbolehkan).
--   * Kolom V, VI, XII, XIII serta sub-kolom Plafon/Baki Debet pada VII dan Pokok/Bunga pada
--     XII/XIII adalah ISIAN BANK. Form tidak memberikan rumus pengikat, jadi laporan tidak
--     menghitungnya.
--   * Sandi VIII, X, XI adalah ISIAN BANK dan ditegakkan CHECK di sini + validasi domain;
--     TIDAK diturunkan dari kolom lain. Aturan historis Bab II "Desember 2024 = Kolektif"
--     tidak relevan untuk form ini (tidak ada kolom Jenis CKPN).
--   * Kolom VII "Sandi Bank Peserta Sindikasi" adalah teks sandi 6 digit mengacu SPOJK;
--     daftar sandi tidak disalin ke kode, disimpan apa adanya.
--   * Tidak ada kolom identitas pribadi lain pada tabel ini.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS kredit_sindikasi_register (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- ID Pihak Lawan (kolom II): pengenal pihak lawan/debitur, teks apa adanya.
    counterparty_id             VARCHAR(64) NOT NULL
        CHECK (length(btrim(counterparty_id)) > 0),
    -- No. Rekening (kolom IV): unik bila terisi, "tidak boleh sama" (PDF #page 177).
    -- Dikosongkan (NULL) bila pendanaan di bank pelapor diisi sandi 2 (tidak).
    no_rekening                 VARCHAR(64),
    -- Jumlah Pendanaan Sindikasi (kolom V): seluruh anggota sindikasi, rupiah penuh.
    jumlah_pendanaan_sindikasi  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (jumlah_pendanaan_sindikasi >= 0),
    -- Bagian Pendanaan (kolom VI): bagian BPR pelapor, rupiah penuh.
    bagian_pendanaan            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (bagian_pendanaan >= 0),
    -- Sandi Bank Peserta Sindikasi (kolom VII): teks sandi bank peserta mengacu SPOJK.
    sandi_bank_peserta          VARCHAR(16) NOT NULL DEFAULT '',
    -- Sub-kolom Plafon dan Baki Debet pada kolom VII.
    plafon                      NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (plafon >= 0),
    baki_debet                  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (baki_debet >= 0),
    -- Status Kepesertaan (kolom VIII) baku PDF #page 175: 1 Arranger, 2 Anggota sindikasi.
    status_kepesertaan_code     VARCHAR(1) NOT NULL
        CHECK (status_kepesertaan_code IN ('1', '2')),
    -- Nomor Perjanjian Kredit Sindikasi (kolom IX): nomor perjanjian INDUK awal, tanpa spasi.
    nomor_perjanjian_induk      VARCHAR(64) NOT NULL
        CHECK (length(btrim(nomor_perjanjian_induk)) > 0),
    -- Pendanaan di Bank Pelapor (kolom X) baku PDF #page 176: 1 Ya, 2 Tidak.
    pendanaan_di_bank_pelapor_code VARCHAR(1) NOT NULL
        CHECK (pendanaan_di_bank_pelapor_code IN ('1', '2')),
    -- Kualitas (kolom XI) baku PDF #page 175: 1-5.
    kualitas_code               VARCHAR(1) NOT NULL
        CHECK (kualitas_code IN ('1', '2', '3', '4', '5')),
    -- Nominal Tunggakan (kolom XII): pokok dan bunga, rupiah penuh.
    tunggakan_pokok             NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (tunggakan_pokok >= 0),
    tunggakan_bunga             NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (tunggakan_bunga >= 0),
    -- Hari Tunggakan (kolom XIII): pokok dan bunga, paling singkat 0 (PDF #page 176).
    hari_tunggakan_pokok        INTEGER NOT NULL DEFAULT 0 CHECK (hari_tunggakan_pokok >= 0),
    hari_tunggakan_bunga        INTEGER NOT NULL DEFAULT 0 CHECK (hari_tunggakan_bunga >= 0),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                       DATE NOT NULL,
    -- AKTIF = masih dilaporkan; NONAKTIF = tidak lagi, tetapi memegang nomor rekeningnya.
    status                      VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                        TEXT NOT NULL DEFAULT '',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                  UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                  UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    CONSTRAINT kredit_sindikasi_no_rekening_unique UNIQUE (no_rekening)
);

COMMENT ON TABLE kredit_sindikasi_register IS
    'Register kredit sindikasi untuk Form 06.02 "Daftar Kredit Sindikasi"; sumber struktur PDF #page 173-178 (hlm. 121-126) SEOJK No. 16/SEOJK.03/2024. Satu baris = satu rekening fasilitas kredit sindikasi; TIDAK ada baris JUMLAH. Kolom III No. Identitas (NIK/NPWP) sengaja tidak disimpan (keputusan privasi). Nomor rekening unik dan tidak boleh sama; penghapusan soft-delete NONAKTIF sehingga nomor tetap terkunci.';

COMMENT ON COLUMN kredit_sindikasi_register.counterparty_id IS
    'ID Pihak Lawan (kolom II): pengenal pihak lawan yang bank catat, teks apa adanya (mengacu Bab II tentang ID Pihak Lawan).';

COMMENT ON COLUMN kredit_sindikasi_register.no_rekening IS
    'No. Rekening fasilitas kredit (kolom IV): unik dan "tidak boleh sama" (PDF #page 177), harus sama dengan nomor rekening SLIK. DIKOSONGKAN (NULL) bila pendanaan di bank pelapor = sandi 2 (tidak). UNIQUE penuh: baris NONAKTIF tetap memegang nomornya, dan NULL boleh berulang.';

COMMENT ON COLUMN kredit_sindikasi_register.jumlah_pendanaan_sindikasi IS
    'Jumlah Pendanaan Sindikasi (kolom V): jumlah pendanaan yang dibiayai SELURUH anggota sindikasi menurut perjanjian kredit, rupiah penuh; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.bagian_pendanaan IS
    'Bagian Pendanaan (kolom VI): bagian pendanaan yang dibiayai BPR pelapor menurut perjanjian kredit, rupiah penuh; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.sandi_bank_peserta IS
    'Sandi Bank Peserta Sindikasi (kolom VII): sandi bank peserta mengacu Sistem Pelaporan OJK (SPOJK), teks apa adanya; daftar sandi tidak disalin ke kode.';

COMMENT ON COLUMN kredit_sindikasi_register.plafon IS
    'Plafon pada kolom VII, rupiah penuh; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.baki_debet IS
    'Baki Debet pada kolom VII, rupiah penuh; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.status_kepesertaan_code IS
    'Status Kepesertaan (kolom VIII) baku PDF #page 175: 1 Arranger, 2 Anggota sindikasi. Isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.nomor_perjanjian_induk IS
    'Nomor Perjanjian Kredit Sindikasi (kolom IX): nomor perjanjian INDUK awal (perjanjian pertama) tanpa spasi (PDF #page 178).';

COMMENT ON COLUMN kredit_sindikasi_register.pendanaan_di_bank_pelapor_code IS
    'Pendanaan di Bank Pelapor (kolom X) baku PDF #page 176: 1 Ya, 2 Tidak. Bila 2, kolom IV No. Rekening dikosongkan.';

COMMENT ON COLUMN kredit_sindikasi_register.kualitas_code IS
    'Kualitas (kolom XI) baku PDF #page 175: 1 Lancar, 2 Dalam Perhatian Khusus, 3 Kurang Lancar, 4 Diragukan, 5 Macet. Isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.tunggakan_pokok IS
    'Nominal Tunggakan Pokok (kolom XII), rupiah penuh; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.tunggakan_bunga IS
    'Nominal Tunggakan Bunga (kolom XII), rupiah penuh; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.hari_tunggakan_pokok IS
    'Hari Tunggakan Pokok (kolom XIII), jumlah hari, paling singkat 0 (PDF #page 176); isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.hari_tunggakan_bunga IS
    'Hari Tunggakan Bunga (kolom XIII), jumlah hari, paling singkat 0; isian bank.';

COMMENT ON COLUMN kredit_sindikasi_register.as_of IS
    'Tanggal posisi kredit sindikasi (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN kredit_sindikasi_register.status IS
    'AKTIF = rekening masih dilaporkan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan dan memegang nomor rekeningnya.';

CREATE INDEX IF NOT EXISTS idx_kredit_sindikasi_kualitas
    ON kredit_sindikasi_register (kualitas_code);
CREATE INDEX IF NOT EXISTS idx_kredit_sindikasi_kepesertaan
    ON kredit_sindikasi_register (status_kepesertaan_code);
CREATE INDEX IF NOT EXISTS idx_kredit_sindikasi_as_of
    ON kredit_sindikasi_register (as_of);
CREATE INDEX IF NOT EXISTS idx_kredit_sindikasi_status
    ON kredit_sindikasi_register (status);
