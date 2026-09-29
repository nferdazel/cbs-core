-- CBS Migration 000121: register aset keuangan lainnya untuk Form 18.00
-- "Daftar Aset Keuangan Lainnya".
-- Run after: 000120_penyertaan_modal_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000121 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 18.00 "DAFTAR ASET KEUANGAN LAINNYA": PDF #page 233 (hlm. tercetak 181),
--     lanjutan PDF #page 234 (hlm. 182). Lima belas kolom: I Sandi Kantor,
--     II No. Rekening, III ID Pihak Lawan, IV Jenis, V Tanggal Mulai,
--     VI Tanggal Jatuh Tempo, VII Suku Bunga, VIII Nominal,
--     IX Nilai Agunan yang Dapat Diperhitungkan, X Cadangan Kerugian Penurunan Nilai,
--     XI CKPN Aset Baik, XII CKPN Aset Kurang Baik, XIII CKPN Aset Tidak Baik,
--     XIV Klasifikasi Aset Keuangan, XV Jenis CKPN.
--     TIDAK ADA baris JUMLAH (PDF #page 233-234).
--   * Sandi: PDF #page 235 (hlm. 183). IV 10 Tagihan fraud / 99 Tagihan lainnya;
--     XIV 1 Nilai wajar melalui laba rugi / 2 Nilai wajar melalui penghasilan
--     komprehensif lain / 3 Biaya perolehan diamortisasi; XV 1 Individual / 2 Kolektif.
--   * Penjelasan: PDF #page 237-238 (hlm. 185-186). Definisi: semua aset keuangan
--     lainnya yang dimiliki BPR sesuai standar akuntansi keuangan. II setiap rekening
--     diisi dengan 1 (satu) nomor rekening yang unik (tidak boleh sama) untuk setiap
--     rekening. IX: nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA.
--     Format tanggal form ini TT-MM-TTTT (bukan TT-BB-TTTT) -- PDF #page 235.
--
-- KEPUTUSAN KOLOM:
--   * Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
--     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--   * Kolom II "No. Rekening" diisi bank, satu rekening unik per baris. Karena form
--     menyatakan nomor rekening "tidak boleh sama", kolom ini diberi UNIQUE penuh dan
--     penghapusan adalah soft-delete (status -> NONAKTIF) sehingga nomor rekening tetap
--     terpakai -- analog aturan no reuse/no recycle nomor register Form 16.00/17.00.
--   * Sandi IV, XIV, XV adalah ISIAN BANK dan ditegakkan CHECK di sini + validasi domain;
--     TIDAK diturunkan dari kolom lain. Aturan historis Bab II "Desember 2024 =
--     Kolektif (2)" TIDAK diterapkan karena masa berlakunya sudah lewat.
--   * Seluruh nilai (VII-XIII) adalah ISIAN BANK. Form tidak memberikan rumus yang
--     mengikat X = XI+XII+XIII, jadi hubungan itu tidak dipaksakan.
--   * Kolom III "ID Pihak Lawan" bukan sandi Lampiran 02 (form 18.00 tidak memuat sandi
--     untuk kolom III): disimpan sebagai teks apa adanya, tanpa daftar sandi karangan.
--   * Tidak ada kolom identitas pribadi pada form ini.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS aset_keuangan_lainnya_register (
    id                              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- No. Rekening (kolom II): unik per rekening, "tidak boleh sama" (PDF #page 237).
    -- UNIQUE penuh menjaga nomor tetap terpakai walau baris NONAKTIF.
    no_rekening                     VARCHAR(64) NOT NULL
        CHECK (length(btrim(no_rekening)) > 0),
    -- ID Pihak Lawan (kolom III): pengenal pihak lawan yang bank catat, wajib terisi.
    -- Bukan sandi Lampiran 02 (lihat komentar tabel), sehingga tidak ada FK.
    counterparty_id                 VARCHAR(64) NOT NULL
        CHECK (length(btrim(counterparty_id)) > 0),
    -- Jenis (kolom IV) baku PDF #page 235: 10 Tagihan fraud, 99 Tagihan lainnya.
    jenis_code                      VARCHAR(2) NOT NULL
        CHECK (jenis_code IN ('10', '99')),
    -- Tanggal Mulai (kolom V), diisi bank; form memakai format TT-MM-TTTT.
    tanggal_mulai                   DATE NOT NULL,
    -- Tanggal Jatuh Tempo (kolom VI), diisi bank; form memakai format TT-MM-TTTT.
    tanggal_jatuh_tempo             DATE NOT NULL,
    -- Suku Bunga (kolom VII), persen; isian bank.
    suku_bunga                      NUMERIC(9,4) NOT NULL DEFAULT 0
        CHECK (suku_bunga >= 0),
    -- Nominal (kolom VIII), rupiah penuh; isian bank.
    nominal                         NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (nominal >= 0),
    -- Nilai Agunan yang Dapat Diperhitungkan (kolom IX), rupiah penuh; isian bank.
    -- PDF #page 237: nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA.
    nilai_agunan_diperhitungkan     NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (nilai_agunan_diperhitungkan >= 0),
    -- Cadangan Kerugian Penurunan Nilai (kolom X), rupiah penuh; isian bank.
    ckpn                            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn >= 0),
    -- Cadangan Kerugian Penurunan Nilai Aset Baik (kolom XI), rupiah penuh; isian bank.
    ckpn_aset_baik                  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_baik >= 0),
    -- Cadangan Kerugian Penurunan Nilai Aset Kurang Baik (kolom XII), rupiah penuh.
    ckpn_aset_kurang_baik           NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_kurang_baik >= 0),
    -- Cadangan Kerugian Penurunan Nilai Aset Tidak Baik (kolom XIII), rupiah penuh.
    ckpn_aset_tidak_baik            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_tidak_baik >= 0),
    -- Klasifikasi Aset Keuangan (kolom XIV) baku PDF #page 235: 1 nilai wajar melalui
    -- laba rugi, 2 nilai wajar melalui penghasilan komprehensif lain, 3 biaya perolehan
    -- diamortisasi.
    klasifikasi_aset_keuangan_code  VARCHAR(1) NOT NULL
        CHECK (klasifikasi_aset_keuangan_code IN ('1', '2', '3')),
    -- Jenis CKPN (kolom XV) baku PDF #page 235: 1 Individual, 2 Kolektif. Isian bank;
    -- aturan historis "Desember 2024 = Kolektif" tidak diterapkan.
    jenis_ckpn_code                 VARCHAR(1) NOT NULL
        CHECK (jenis_ckpn_code IN ('1', '2')),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                           DATE NOT NULL,
    -- AKTIF = rekening masih tercatat dan ikut laporan; NONAKTIF = sudah tidak
    -- dilaporkan lagi tetapi tetap tersimpan DAN memegang nomor rekeningnya.
    status                          VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                            TEXT NOT NULL DEFAULT '',
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                      UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                      UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    CONSTRAINT aset_keuangan_lainnya_no_rekening_unique UNIQUE (no_rekening)
);

COMMENT ON TABLE aset_keuangan_lainnya_register IS
    'Register aset keuangan lainnya untuk Form 18.00 "Daftar Aset Keuangan Lainnya"; sumber struktur PDF #page 233-238 (hlm. 181-186) SEOJK No. 16/SEOJK.03/2024. Satu baris = satu rekening aset keuangan lainnya yang unik; TIDAK ada baris JUMLAH. Semua nilai dan sandi diisi bank. No. Rekening unik dan tidak boleh sama; penghapusan adalah soft-delete NONAKTIF sehingga UNIQUE penuh tetap memegang nomornya.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.no_rekening IS
    'No. Rekening (kolom II): nomor rekening unik untuk setiap rekening aset keuangan lainnya (PDF #page 237: "tidak boleh sama"). UNIQUE penuh: baris NONAKTIF tetap memegang nomornya.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.counterparty_id IS
    'ID Pihak Lawan (kolom III): pengenal pihak lawan yang bank catat, wajib terisi. BUKAN sandi Lampiran 02 (form 18.00 tidak memuat sandi untuk kolom ini), sehingga disimpan sebagai teks tanpa FK.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.jenis_code IS
    'Jenis (kolom IV) baku PDF #page 235: 10 Tagihan fraud, 99 Tagihan lainnya. Isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.tanggal_mulai IS
    'Tanggal Mulai (kolom V), diisi bank. Form 18.00 menulis tanggal dengan format TT-MM-TTTT (PDF #page 235), bukan TT-BB-TTTT.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.tanggal_jatuh_tempo IS
    'Tanggal Jatuh Tempo (kolom VI), diisi bank. Format laporan TT-MM-TTTT (PDF #page 235).';

COMMENT ON COLUMN aset_keuangan_lainnya_register.suku_bunga IS
    'Suku Bunga (kolom VII), persen; isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.nominal IS
    'Nominal (kolom VIII), rupiah penuh; isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.nilai_agunan_diperhitungkan IS
    'Nilai Agunan yang Dapat Diperhitungkan (kolom IX), rupiah penuh; isian bank. PDF #page 237: nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.ckpn IS
    'Cadangan Kerugian Penurunan Nilai (kolom X), rupiah penuh; isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.ckpn_aset_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Baik (kolom XI), rupiah penuh; isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.ckpn_aset_kurang_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Kurang Baik (kolom XII), rupiah penuh; isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.ckpn_aset_tidak_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Tidak Baik (kolom XIII), rupiah penuh; isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.klasifikasi_aset_keuangan_code IS
    'Klasifikasi Aset Keuangan (kolom XIV) baku PDF #page 235: 1 nilai wajar melalui laba rugi, 2 nilai wajar melalui penghasilan komprehensif lain, 3 biaya perolehan diamortisasi. Isian bank.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.jenis_ckpn_code IS
    'Jenis CKPN (kolom XV) baku PDF #page 235: 1 Individual, 2 Kolektif. Isian bank; aturan historis Desember 2024 = Kolektif tidak diterapkan karena masa berlakunya sudah lewat.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.as_of IS
    'Tanggal posisi aset keuangan lainnya (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN aset_keuangan_lainnya_register.status IS
    'AKTIF = rekening masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat dan memegang nomor rekeningnya.';

CREATE INDEX IF NOT EXISTS idx_aset_keuangan_lainnya_jenis
    ON aset_keuangan_lainnya_register (jenis_code);
CREATE INDEX IF NOT EXISTS idx_aset_keuangan_lainnya_klasifikasi
    ON aset_keuangan_lainnya_register (klasifikasi_aset_keuangan_code);
CREATE INDEX IF NOT EXISTS idx_aset_keuangan_lainnya_jenis_ckpn
    ON aset_keuangan_lainnya_register (jenis_ckpn_code);
CREATE INDEX IF NOT EXISTS idx_aset_keuangan_lainnya_as_of
    ON aset_keuangan_lainnya_register (as_of);
CREATE INDEX IF NOT EXISTS idx_aset_keuangan_lainnya_status
    ON aset_keuangan_lainnya_register (status);
