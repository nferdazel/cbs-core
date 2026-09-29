-- CBS Migration 000120: register penyertaan modal untuk Form 16.00
-- "Daftar Penyertaan Modal".
-- Run after: 000119_aset_tetap_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000120 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 16.00 "DAFTAR PENYERTAAN MODAL": PDF #page 223 (hlm. tercetak 171), lanjutan
--     PDF #page 224 (hlm. 172). Lima belas kolom: I Sandi Kantor, II No. Register,
--     III ID Pihak Lawan, IV Metode Penyertaan, V Kualitas, VI Tujuan Penyertaan,
--     VII Tanggal Mulai, VIII Persentase Penyertaan, IX Nominal,
--     X Jumlah Bulan Laporan, XI Cadangan Kerugian Penurunan Nilai,
--     XII Cadangan Kerugian Penurunan Nilai Aset Baik,
--     XIII Cadangan Kerugian Penurunan Nilai Aset Kurang Baik,
--     XIV Cadangan Kerugian Penurunan Nilai Aset Tidak Baik, XV Jenis CKPN.
--     TIDAK ADA baris JUMLAH. Header PDF #page 224 salah cetak: romawi "XII" muncul dua
--     kali; tabel sandi PDF #page 225 memakai XIII untuk blok CKPN aset kurang baik,
--     sehingga label di sini mengikuti tabel sandi (docs/CELAH-FORM-OJK.md §7 butir 6).
--   * Sandi: PDF #page 225-226 (hlm. 173-174). IV 1 Biaya Perolehan / 2 Metode Ekuitas;
--     V 1 Lancar / 3 Kurang Lancar / 4 Diragukan / 5 Macet;
--     VI 1 Dalam Rangka Investasi - Penyertaan pada Lembaga Penunjang / 9 Lainnya;
--     XV 1 Individual / 2 Kolektif.
--   * Penjelasan: PDF #page 227-228 (hlm. 175-176). Definisi: semua penyertaan modal BPR
--     sesuai ketentuan perundang-undangan. II No. Register unik, satu register untuk satu
--     penyertaan modal kepada satu pihak lawan, "no reuse atau no recycle", mandatory.
--     VIII adalah persentase penyertaan modal BPR pada pihak lawan terhadap modal
--     Lembaga Penunjang. X: "nilai tercatat penyertaan modal pada bulan laporan"
--     (PDF #page 228) -- sebuah NILAI yang diisi bank, bukan jumlah bulan yang dihitung
--     sistem; tidak ada rumus turunan yang diberikan form.
--
-- KEPUTUSAN KOLOM:
--   * Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
--     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain (17.00).
--   * Kolom III "ID Pihak Lawan" BUKAN sandi Lampiran 02 (itu "Gol. Kreditur" pada form
--     lain): tabel sandi 16.00 tidak memuat sandi untuk kolom III. Ia disimpan sebagai
--     teks apa adanya, seperti ID Pihak Lawan Form 00.07; tidak ada daftar sandi yang
--     dikarang.
--   * Sandi IV, V, VI, XV adalah ISIAN BANK dan ditegakkan CHECK di sini + validasi
--     domain; TIDAK diturunkan dari kolom lain. Aturan historis Bab II "Desember 2024 =
--     Kolektif (2)" TIDAK diterapkan karena masa berlakunya sudah lewat.
--   * Blok CKPN XI-XIV adalah ISIAN BANK (rupiah penuh). Form tidak memberikan rumus
--     yang mengikat XI = XII+XIII+XIV, jadi hubungan itu tidak dipaksakan.
--   * Kolom X disimpan `jumlah_bulan_laporan` mengikuti NAMA kolom resmi, tetapi
--     mengikuti definisi PDF #page 228 ia adalah NILAI TERCATAT (rupiah penuh) yang bank
--     isi; TIDAK dihitung builder. Nama kolom tidak diubah agar jejak ke PDF tetap jelas.
--   * Tidak ada kolom identitas pribadi pada form ini.
--
-- NO-REUSE NOMOR REGISTER: II No. Register unik dan tidak boleh dipakai ulang (PDF
-- #page 227). Kolom no_register diberi UNIQUE penuh (bukan parsial): baris NONAKTIF
-- tetap memegang nomornya sehingga nomor tidak pernah kembali bebas. Penghapusan register
-- adalah soft-delete (status -> NONAKTIF), bukan DELETE fisik.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS penyertaan_modal_register (
    id                              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- No. Register (kolom II): kode unik penyertaan, mandatory, no reuse/no recycle
    -- (PDF #page 227). UNIQUE penuh menjaga nomor tetap terpakai walau baris NONAKTIF.
    no_register                     VARCHAR(64) NOT NULL
        CHECK (length(btrim(no_register)) > 0),
    -- ID Pihak Lawan (kolom III): pengenal pihak lawan yang bank catat, wajib terisi.
    -- Bukan sandi Lampiran 02 (lihat komentar tabel), sehingga tidak ada FK.
    counterparty_id                 VARCHAR(64) NOT NULL
        CHECK (length(btrim(counterparty_id)) > 0),
    -- Metode Penyertaan (kolom IV) baku PDF #page 225: 1 Biaya Perolehan, 2 Metode Ekuitas.
    metode_penyertaan_code          VARCHAR(1) NOT NULL
        CHECK (metode_penyertaan_code IN ('1', '2')),
    -- Kualitas (kolom V) baku PDF #page 225: 1 Lancar, 3 Kurang Lancar, 4 Diragukan,
    -- 5 Macet. Isian bank; tidak diturunkan dari kolom lain.
    kualitas_code                   VARCHAR(1) NOT NULL
        CHECK (kualitas_code IN ('1', '3', '4', '5')),
    -- Tujuan Penyertaan (kolom VI) baku PDF #page 225: 1 Dalam Rangka Investasi -
    -- Penyertaan pada Lembaga Penunjang, 9 Lainnya.
    tujuan_penyertaan_code          VARCHAR(1) NOT NULL
        CHECK (tujuan_penyertaan_code IN ('1', '9')),
    -- Tanggal Mulai (kolom VII), diisi bank.
    tanggal_mulai                   DATE NOT NULL,
    -- Persentase Penyertaan (kolom VIII), persen; isian bank (0..100).
    persentase_penyertaan           NUMERIC(9,4) NOT NULL DEFAULT 0
        CHECK (persentase_penyertaan >= 0 AND persentase_penyertaan <= 100),
    -- Nominal (kolom IX), rupiah penuh; isian bank.
    nominal                         NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (nominal >= 0),
    -- Jumlah Bulan Laporan (kolom X). Nama mengikuti kolom resmi, TETAPI definisi PDF
    -- #page 228 adalah "nilai tercatat penyertaan modal pada bulan laporan" sehingga
    -- bertipe rupiah penuh dan diisi bank, bukan dihitung sistem.
    jumlah_bulan_laporan            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (jumlah_bulan_laporan >= 0),
    -- Cadangan Kerugian Penurunan Nilai (kolom XI), rupiah penuh; isian bank.
    ckpn                            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn >= 0),
    -- Cadangan Kerugian Penurunan Nilai Aset Baik (kolom XII), rupiah penuh; isian bank.
    ckpn_aset_baik                  NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_baik >= 0),
    -- Cadangan Kerugian Penurunan Nilai Aset Kurang Baik (kolom XIII), rupiah penuh.
    ckpn_aset_kurang_baik           NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_kurang_baik >= 0),
    -- Cadangan Kerugian Penurunan Nilai Aset Tidak Baik (kolom XIV), rupiah penuh.
    ckpn_aset_tidak_baik            NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (ckpn_aset_tidak_baik >= 0),
    -- Jenis CKPN (kolom XV) baku PDF #page 225: 1 Individual, 2 Kolektif. Isian bank;
    -- aturan historis "Desember 2024 = Kolektif" tidak diterapkan.
    jenis_ckpn_code                 VARCHAR(1) NOT NULL
        CHECK (jenis_ckpn_code IN ('1', '2')),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                           DATE NOT NULL,
    -- AKTIF = penyertaan masih tercatat dan ikut laporan; NONAKTIF = sudah tidak
    -- dilaporkan lagi tetapi tetap tersimpan DAN memegang nomor register-nya (no reuse).
    status                          VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                            TEXT NOT NULL DEFAULT '',
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                      UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                      UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    CONSTRAINT penyertaan_modal_no_register_unique UNIQUE (no_register)
);

COMMENT ON TABLE penyertaan_modal_register IS
    'Register penyertaan modal untuk Form 16.00 "Daftar Penyertaan Modal"; sumber struktur PDF #page 223-228 (hlm. 171-176) SEOJK No. 16/SEOJK.03/2024. Satu baris = satu penyertaan modal kepada satu pihak lawan; TIDAK ada baris JUMLAH. Semua nilai dan sandi diisi bank. Kolom X "Jumlah Bulan Laporan" menurut definisi PDF #page 228 adalah nilai tercatat (rupiah) pada bulan laporan, bukan jumlah bulan, dan diisi bank. No. Register unik dan tidak boleh dipakai ulang; penghapusan adalah soft-delete NONAKTIF sehingga UNIQUE penuh tetap memegang nomornya.';

COMMENT ON COLUMN penyertaan_modal_register.no_register IS
    'No. Register (kolom II): kode unik penyertaan, mandatory, "no reuse atau no recycle" (PDF #page 227). UNIQUE penuh: baris NONAKTIF tetap memegang nomor sehingga tidak dapat dipakai ulang.';

COMMENT ON COLUMN penyertaan_modal_register.counterparty_id IS
    'ID Pihak Lawan (kolom III): pengenal pihak lawan yang bank catat, wajib terisi. BUKAN sandi Lampiran 02 (form 16.00 tidak memuat sandi untuk kolom ini), sehingga disimpan sebagai teks tanpa FK.';

COMMENT ON COLUMN penyertaan_modal_register.metode_penyertaan_code IS
    'Metode Penyertaan (kolom IV) baku PDF #page 225: 1 Biaya Perolehan, 2 Metode Ekuitas. Isian bank.';

COMMENT ON COLUMN penyertaan_modal_register.kualitas_code IS
    'Kualitas (kolom V) baku PDF #page 225: 1 Lancar, 3 Kurang Lancar, 4 Diragukan, 5 Macet. Isian bank; tidak diturunkan dari kolom lain.';

COMMENT ON COLUMN penyertaan_modal_register.tujuan_penyertaan_code IS
    'Tujuan Penyertaan (kolom VI) baku PDF #page 225: 1 Dalam Rangka Investasi - Penyertaan pada Lembaga Penunjang, 9 Lainnya.';

COMMENT ON COLUMN penyertaan_modal_register.tanggal_mulai IS
    'Tanggal Mulai (kolom VII), diisi bank. Ditulis laporan dalam format TT-BB-TTTT mengikuti mayoritas form.';

COMMENT ON COLUMN penyertaan_modal_register.persentase_penyertaan IS
    'Persentase Penyertaan (kolom VIII), persen 0..100; persentase penyertaan modal BPR pada pihak lawan terhadap modal Lembaga Penunjang (PDF #page 228). Isian bank.';

COMMENT ON COLUMN penyertaan_modal_register.nominal IS
    'Nominal (kolom IX), rupiah penuh; isian bank.';

COMMENT ON COLUMN penyertaan_modal_register.jumlah_bulan_laporan IS
    'Jumlah Bulan Laporan (kolom X). Nama mengikuti kolom resmi, tetapi definisi PDF #page 228 adalah "nilai tercatat penyertaan modal pada bulan laporan": NILAI rupiah penuh yang diisi bank, bukan jumlah bulan dan bukan turunan yang dihitung sistem.';

COMMENT ON COLUMN penyertaan_modal_register.ckpn IS
    'Cadangan Kerugian Penurunan Nilai (kolom XI), rupiah penuh; isian bank.';

COMMENT ON COLUMN penyertaan_modal_register.ckpn_aset_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Baik (kolom XII), rupiah penuh; isian bank.';

COMMENT ON COLUMN penyertaan_modal_register.ckpn_aset_kurang_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Kurang Baik (kolom XIII), rupiah penuh; isian bank. Tabel sandi PDF #page 225 memakai XIII (header PDF #page 224 salah cetak "XII" dua kali).';

COMMENT ON COLUMN penyertaan_modal_register.ckpn_aset_tidak_baik IS
    'Cadangan Kerugian Penurunan Nilai Aset Tidak Baik (kolom XIV), rupiah penuh; isian bank.';

COMMENT ON COLUMN penyertaan_modal_register.jenis_ckpn_code IS
    'Jenis CKPN (kolom XV) baku PDF #page 225: 1 Individual, 2 Kolektif. Isian bank; aturan historis Desember 2024 = Kolektif tidak diterapkan karena masa berlakunya sudah lewat.';

COMMENT ON COLUMN penyertaan_modal_register.as_of IS
    'Tanggal posisi penyertaan (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN penyertaan_modal_register.status IS
    'AKTIF = penyertaan masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat dan memegang nomor register-nya.';

CREATE INDEX IF NOT EXISTS idx_penyertaan_modal_metode
    ON penyertaan_modal_register (metode_penyertaan_code);
CREATE INDEX IF NOT EXISTS idx_penyertaan_modal_kualitas
    ON penyertaan_modal_register (kualitas_code);
CREATE INDEX IF NOT EXISTS idx_penyertaan_modal_jenis_ckpn
    ON penyertaan_modal_register (jenis_ckpn_code);
CREATE INDEX IF NOT EXISTS idx_penyertaan_modal_as_of
    ON penyertaan_modal_register (as_of);
CREATE INDEX IF NOT EXISTS idx_penyertaan_modal_status
    ON penyertaan_modal_register (status);
