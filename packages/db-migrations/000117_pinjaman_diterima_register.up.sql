-- CBS Migration 000117: register pinjaman yang diterima untuk Form 00.07
-- "Daftar Pinjaman yang Diterima".
-- Run after: 000116_kepemilikan_bpr_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000117 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 00.07 - 1 "DAFTAR PINJAMAN YANG DITERIMA": PDF #page 248 (hlm. tercetak
--     196), lanjutan PDF #page 249 (hlm. 197). Lima belas kolom:
--     I ID Pihak Lawan, II Gol. Kreditur, III Sandi Bank, IV Lokasi Kreditur,
--     V Jenis, VI Hubungan dengan Bank, VII Jangka Waktu (Tanggal Mulai + Tanggal
--     Jatuh Tempo), VIII Suku Bunga (Persentase + Cara Perhitungan), IX Plafon,
--     X Jenis Agunan yang Dijaminkan, XI Nominal Agunan yang Dijaminkan,
--     XII Baki Debet, XIII Biaya Transaksi Belum Diamortisasi,
--     XIV Diskonto Belum Diamortisasi, XV Baki Debet Neto. Ada baris JUMLAH.
--   * Sandi: PDF #page 250 (hlm. 198). V Jenis 10/20/31/32/41/42/99;
--     VI Hubungan 12 Terkait / 20 Tidak Terkait; VIII Cara Perhitungan
--     11/12 (bunga flat tetap/mengambang) dan 21/22 (bunga tidak flat).
--   * Penjelasan: PDF #page 252-254 (hlm. 200-202). Definisi pinjaman yang diterima
--     adalah pinjaman dari bank, Bank Indonesia, dan/atau pihak ketiga bukan bank
--     dengan kewajiban pembayaran kembali berdasarkan persyaratan perjanjian utang
--     piutang. X tanpa agunan = sandi 299; XI tanpa agunan = 0. IX Plafon untuk
--     pinjaman berangsuran pokok = posisi plafon pada tanggal laporan (menurun).
--     XV Baki Debet Neto = baki debet dikurangi diskonto dan biaya transaksi belum
--     diamortisasi.
--
-- Kolom XV "Baki Debet Neto" SENGAJA TIDAK disimpan: ia turunan yang dihitung laporan
-- dari XII - (XIII + XIV). Menyimpannya akan membuat dua sumber kebenaran yang bisa
-- berbeda.
--
-- Sandi rujukan Lampiran memakai tabel referensi yang sudah ada (migrasi 000102):
--   * II Gol. Kreditur -> FK ojk_pihak_lawan(code), Lampiran 02.
--   * IV Lokasi Kreditur -> FK ojk_kabupaten(code), Lampiran 03.
-- Sandi X Jenis Agunan (Lampiran 01) BELUM punya tabel referensi di repo; karena itu
-- disimpan sebagai teks apa adanya, tanpa daftar sandi yang dikarang. Aturan tanpa
-- agunan ditegakkan langsung: sandi 299 hanya sah bila nominal 0, dan nominal 0 hanya
-- sah bila sandinya 299.
--
-- Kolom III "Sandi Bank" adalah sandi bank kreditur (6 digit SPOJK), BUKAN Sandi
-- Kantor pelapor: form 00.07 tidak punya kolom Sandi Kantor (kolom I-nya ID Pihak
-- Lawan), jadi tidak ada kantor pelapor yang perlu dipilih.
--
-- Semua NILAI adalah isian bank. Tidak ada seed angka.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS pinjaman_diterima_register (
    id                              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- ID Pihak Lawan (kolom I): sandi unik kreditur yang bank catat. Wajib terisi
    -- agar baris dapat dibaca.
    counterparty_id                 VARCHAR(64) NOT NULL
        CHECK (length(btrim(counterparty_id)) > 0),
    -- Golongan Kreditur (kolom II, Lampiran 02): FK ojk_pihak_lawan (migrasi 000102).
    -- Wajib terisi; sandi di luar Lampiran 02 ditolak basis data.
    creditor_group_code             VARCHAR(3) NOT NULL
        REFERENCES ojk_pihak_lawan (code),
    -- Sandi Bank kreditur (kolom III, 6 digit SPOJK). Bukan Sandi Kantor pelapor.
    bank_code                       VARCHAR(6) NOT NULL
        CHECK (bank_code ~ '^[0-9]{6}$'),
    -- Lokasi Kreditur (kolom IV, Lampiran 03): FK ojk_kabupaten (migrasi 000102).
    location_code                   VARCHAR(4) NOT NULL
        REFERENCES ojk_kabupaten (code),
    -- Jenis pinjaman (kolom V) baku PDF #page 250.
    jenis_code                      VARCHAR(2) NOT NULL
        CHECK (jenis_code IN ('10', '20', '31', '32', '41', '42', '99')),
    -- Hubungan dengan bank (kolom VI) baku PDF #page 250: 12 Terkait, 20 Tidak Terkait.
    relationship_code               VARCHAR(2) NOT NULL
        CHECK (relationship_code IN ('12', '20')),
    -- Jangka Waktu (kolom VII): dua sub-kolom tanggal.
    start_date                      DATE NOT NULL,
    maturity_date                   DATE NOT NULL,
    -- Suku Bunga (kolom VIII): persentase bank (>= 0) + cara perhitungan baku
    -- 11/12/21/22 (PDF #page 250).
    interest_rate                   NUMERIC(9,4) NOT NULL DEFAULT 0
        CHECK (interest_rate >= 0),
    interest_calc_code              VARCHAR(2) NOT NULL
        CHECK (interest_calc_code IN ('11', '12', '21', '22')),
    -- Plafon (kolom IX), rupiah penuh; untuk pinjaman berangsuran pokok = posisi pada
    -- tanggal laporan (menurun). Angka bank.
    plafon                          NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (plafon >= 0),
    -- Jenis agunan yang dijaminkan (kolom X, Lampiran 01). Sandi Lampiran 01 belum
    -- punya tabel referensi; disimpan sebagai teks. Tanpa agunan = 299.
    collateral_type_code            VARCHAR(8) NOT NULL
        CHECK (length(btrim(collateral_type_code)) > 0),
    -- Nominal agunan yang dijaminkan (kolom XI), rupiah penuh. Tanpa agunan = 0.
    collateral_amount               NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (collateral_amount >= 0),
    -- Baki Debet (kolom XII), rupiah penuh. Angka bank.
    baki_debet                      NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (baki_debet >= 0),
    -- Biaya Transaksi Belum Diamortisasi (kolom XIII), rupiah penuh. Angka bank.
    unamortized_transaction_cost    NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (unamortized_transaction_cost >= 0),
    -- Diskonto Belum Diamortisasi (kolom XIV), rupiah penuh. Angka bank.
    unamortized_discount            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (unamortized_discount >= 0),
    -- Aturan tanpa agunan (PDF #page 253): sandi 299 hanya sah bila nominal 0, dan
    -- nominal 0 hanya sah bila sandinya 299.
    CONSTRAINT pinjaman_no_collateral_consistency CHECK (
        (collateral_type_code = '299' AND collateral_amount = 0)
        OR (collateral_type_code <> '299' AND collateral_amount > 0)
    ),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                           DATE NOT NULL,
    -- AKTIF = pinjaman masih tercatat dan ikut laporan; NONAKTIF = sudah tidak
    -- dilaporkan lagi tetapi tetap tersimpan sebagai riwayat.
    status                          VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                            TEXT NOT NULL DEFAULT '',
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                      UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                      UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE pinjaman_diterima_register IS
    'Register pinjaman yang diterima dari bank/Bank Indonesia/pihak ketiga bukan bank untuk Form 00.07 "Daftar Pinjaman yang Diterima"; sumber struktur PDF #page 248-254 (hlm. 196-202) SEOJK No. 16/SEOJK.03/2024. Semua nilai diisi bank. Kolom XV Baki Debet Neto tidak disimpan karena turunan XII - (XIII + XIV); kolom III Sandi Bank adalah sandi bank kreditur, bukan Sandi Kantor pelapor.';

COMMENT ON COLUMN pinjaman_diterima_register.counterparty_id IS
    'ID Pihak Lawan (kolom I): sandi unik kreditur yang bank catat. Wajib terisi agar baris dapat dibaca.';

COMMENT ON COLUMN pinjaman_diterima_register.creditor_group_code IS
    'Golongan Kreditur (kolom II), Lampiran 02 SEOJK 16/2024; FK ojk_pihak_lawan (migrasi 000102). Sandi di luar Lampiran 02 ditolak.';

COMMENT ON COLUMN pinjaman_diterima_register.bank_code IS
    'Sandi Bank kreditur (kolom III), 6 digit SPOJK. BUKAN Sandi Kantor pelapor: Form 00.07 tidak punya kolom Sandi Kantor.';

COMMENT ON COLUMN pinjaman_diterima_register.location_code IS
    'Lokasi Kreditur (kolom IV), Lampiran 03 SEOJK 16/2024; FK ojk_kabupaten (migrasi 000102).';

COMMENT ON COLUMN pinjaman_diterima_register.jenis_code IS
    'Jenis pinjaman (kolom V) baku PDF #page 250: 10 Bilateral, 20 Sindikasi, 31 Khusus dari Lembaga Pengayom, 32 Khusus Linkage, 41 Tertentu modal inti tambahan, 42 Tertentu modal pelengkap, 99 Lainnya.';

COMMENT ON COLUMN pinjaman_diterima_register.relationship_code IS
    'Hubungan dengan bank (kolom VI) baku PDF #page 250: 12 Terkait, 20 Tidak Terkait.';

COMMENT ON COLUMN pinjaman_diterima_register.start_date IS
    'Jangka Waktu kolom VII sub Tanggal Mulai (diisi bank).';

COMMENT ON COLUMN pinjaman_diterima_register.maturity_date IS
    'Jangka Waktu kolom VII sub Tanggal Jatuh Tempo (diisi bank).';

COMMENT ON COLUMN pinjaman_diterima_register.interest_rate IS
    'Suku Bunga kolom VIII sub Persentase, dalam persen. Angka bank.';

COMMENT ON COLUMN pinjaman_diterima_register.interest_calc_code IS
    'Suku Bunga kolom VIII sub Cara Perhitungan baku PDF #page 250: 11 flat tetap, 12 flat mengambang, 21 tidak flat tetap, 22 tidak flat mengambang.';

COMMENT ON COLUMN pinjaman_diterima_register.plafon IS
    'Plafon (kolom IX), rupiah penuh. Untuk pinjaman berangsuran pokok diisi posisi plafon pada tanggal laporan (menurun). Angka bank.';

COMMENT ON COLUMN pinjaman_diterima_register.collateral_type_code IS
    'Jenis agunan yang dijaminkan (kolom X), sandi Lampiran 01 yang belum punya tabel referensi di repo sehingga disimpan sebagai teks. Tanpa agunan diisi 299 (PDF #page 253); sistem tidak mengarang daftar sandi.';

COMMENT ON COLUMN pinjaman_diterima_register.collateral_amount IS
    'Nominal agunan yang dijaminkan (kolom XI), rupiah penuh. Tanpa agunan diisi 0 (PDF #page 253). Aturan konsistensi: sandi 299 hanya sah bila 0, dan 0 hanya sah bila sandi 299.';

COMMENT ON COLUMN pinjaman_diterima_register.baki_debet IS
    'Baki Debet (kolom XII), rupiah penuh. Angka bank; kolom XV Baki Debet Neto dihitung laporan sebagai XII - (XIII + XIV).';

COMMENT ON COLUMN pinjaman_diterima_register.unamortized_transaction_cost IS
    'Biaya Transaksi Belum Diamortisasi (kolom XIII), rupiah penuh. Angka bank.';

COMMENT ON COLUMN pinjaman_diterima_register.unamortized_discount IS
    'Diskonto Belum Diamortisasi (kolom XIV), rupiah penuh. Angka bank.';

COMMENT ON COLUMN pinjaman_diterima_register.as_of IS
    'Tanggal posisi pinjaman (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN pinjaman_diterima_register.status IS
    'AKTIF = pinjaman masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_pinjaman_diterima_register_creditor_group
    ON pinjaman_diterima_register (creditor_group_code);
CREATE INDEX IF NOT EXISTS idx_pinjaman_diterima_register_location
    ON pinjaman_diterima_register (location_code);
CREATE INDEX IF NOT EXISTS idx_pinjaman_diterima_register_jenis
    ON pinjaman_diterima_register (jenis_code);
CREATE INDEX IF NOT EXISTS idx_pinjaman_diterima_register_as_of
    ON pinjaman_diterima_register (as_of);
CREATE INDEX IF NOT EXISTS idx_pinjaman_diterima_register_status
    ON pinjaman_diterima_register (status);
