-- CBS Migration 000130: register pihak lawan (Form 00.16).
-- "Daftar Pihak Lawan".
-- Run after: 000129_hapus_buku_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000130 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 00.16 "DAFTAR PIHAK LAWAN": susunan PDF #page 286 (hlm. tercetak 234) kolom
--     I-IX, PDF #page 287 (hlm. 235) kolom X-XX, sandi PDF #page 288 (hlm. 236) dan
--     PDF #page 289 (hlm. 237), penjelasan PDF #page 290-291 (hlm. 238-239).
--   * Dua puluh kolom, TANPA baris JUMLAH. Isi: seluruh pihak lawan baik bank maupun
--     bukan bank yang melakukan transaksi dengan BPR pelapor (PDF #290).
--
-- MENGAPA TABEL BARU, BUKAN MENAMBAH KOLOM DI customers:
--   * Form ini memuat SELURUH pihak lawan, termasuk BANK dan bukan bank yang BUKAN
--     nasabah BPR. Kolom-kolom yang ada pada customers (ojk_pihak_lawan_code,
--     ojk_hubungan_bank_code, ojk_kabupaten_code) hanya menutup sebagian kolom; tujuh
--     kolom lainnya tidak punya sumber (jenis identitas, jenis kelamin, kewarganegaraan,
--     negara, jenis kegiatan usaha, pemeringkat+peringkat+tanggalnya, grup, tanggal
--     lahir). Memaksa bank lawan masuk customers akan menuntut baris nasabah palsu.
--   * Karena itu bank mencatat pihak lawan apa adanya pada register ini.
--
-- KEPUTUSAN KOLOM:
--   * Kolom III Nomor Identitas dan VI NPWP SENGAJA TIDAK DISIMPAN (keputusan privasi
--     docs/KEPUTUSAN-OJK.md §4 butir III dan §7 butir 6). Laporan menulis keduanya "-"
--     beralasan.
--   * Kolom II Jenis Identitas, IV Jenis Kelamin, IX Jenis Kegiatan Usaha, X Hubungan
--     dengan Bank memakai sandi baku PDF #288 dan ditegakkan CHECK di sini.
--   * Kolom VII Kewarganegaraan, VIII Negara (Lampiran 10), XI Golongan Pihak Lawan
--     (Lampiran 02), XII Lembaga Pemeringkat (Lampiran 08), XIII Peringkat (Lampiran 09),
--     XVI Lokasi (Lampiran 03) disimpan sebagai teks sandi apa adanya; daftar lampiran
--     tidak disalin ke kode. Referensi yang sudah diseed (pihak lawan, kabupaten) tetap
--     diambil dari tabel referensi di lapisan aplikasi, bukan di-FK-kan di sini, supaya
--     register tidak mengunci urutan migrasi.
--   * Kolom XVIII Nama Grup, XIX No. Telp/HP, XX Alamat, V Nama, I ID Pihak Lawan adalah
--     teks bebas.
--   * Tanggal Pemeringkatan (XIV) dan Tanggal Lahir (XV) boleh NULL (diisi hanya bila
--     ada peringkat / pihak lawan perorangan).
--   * Tidak ada nomor register unik pada form, sehingga penghapusan adalah DELETE fisik.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada seed
-- data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS pihak_lawan_register (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- I ID Pihak Lawan: nomor ID internal yang ditetapkan bank.
    pihak_lawan_id        VARCHAR(64) NOT NULL DEFAULT '',
    -- II Jenis Identitas (PDF #288): 1 KTP, 2 Paspor, 3 KITAS/KITAP, 4 Kartu Keluarga.
    -- Hanya untuk perorangan; kosong bila bukan perorangan.
    jenis_identitas_code  VARCHAR(2) NOT NULL DEFAULT ''
        CHECK (jenis_identitas_code IN ('', '1', '2', '3', '4')),
    -- III Nomor Identitas SENGAJA TIDAK DISIMPAN (keputusan privasi).
    -- IV Jenis Kelamin (PDF #288): 1 Laki-laki, 2 Perempuan; boleh kosong (badan usaha).
    jenis_kelamin_code    VARCHAR(1) NOT NULL DEFAULT ''
        CHECK (jenis_kelamin_code IN ('', '1', '2')),
    -- V Nama Lengkap/Nama Badan Usaha.
    nama                  VARCHAR(255) NOT NULL DEFAULT '',
    -- VI NPWP SENGAJA TIDAK DISIMPAN (keputusan privasi).
    -- VII Kewarganegaraan (Lampiran 10); sandi teks apa adanya.
    kewarganegaraan_code  VARCHAR(8) NOT NULL DEFAULT '',
    -- VIII Negara domisili (Lampiran 10); sandi teks apa adanya.
    negara_code           VARCHAR(8) NOT NULL DEFAULT '',
    -- IX Jenis Kegiatan Usaha (PDF #288): 1 Konvensional, 2 Syariah; hanya untuk LJK.
    jenis_usaha_code      VARCHAR(1) NOT NULL DEFAULT ''
        CHECK (jenis_usaha_code IN ('', '1', '2')),
    -- X Hubungan dengan Bank (PDF #288): 12 Pihak Terkait, 20 Pihak Tidak Terkait.
    hubungan_bank_code    VARCHAR(2) NOT NULL DEFAULT ''
        CHECK (hubungan_bank_code IN ('', '12', '20')),
    -- XI Golongan Pihak Lawan (Lampiran 02); sandi teks apa adanya.
    golongan_code         VARCHAR(8) NOT NULL DEFAULT '',
    -- XII Lembaga Pemeringkat (Lampiran 08); tanpa peringkat = 9.
    lembaga_pemeringkat_code VARCHAR(8) NOT NULL DEFAULT '',
    -- XIII Peringkat Pihak Lawan (Lampiran 09); tanpa peringkat = 99.
    peringkat_code        VARCHAR(8) NOT NULL DEFAULT '',
    -- XIV Tanggal Pemeringkatan; NULL bila belum/tanpa peringkat.
    tanggal_pemeringkatan DATE,
    -- XV Tanggal Lahir; NULL bila pihak lawan bukan perorangan.
    tanggal_lahir         DATE,
    -- XVI Lokasi (Lampiran 03); sandi teks apa adanya.
    lokasi_code           VARCHAR(8) NOT NULL DEFAULT '',
    -- XVII ID Grup; XVIII Nama Grup: ditatausahakan bank.
    grup_id               VARCHAR(64) NOT NULL DEFAULT '',
    grup_nama             VARCHAR(255) NOT NULL DEFAULT '',
    -- XIX No. Telp/No. HP.
    telepon               VARCHAR(32) NOT NULL DEFAULT '',
    -- XX Alamat lengkap.
    alamat                TEXT NOT NULL DEFAULT '',
    note                  TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by            UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by            UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE pihak_lawan_register IS
    'Register pihak lawan untuk Form 00.16 "Daftar Pihak Lawan" (PDF #page 286-291). Memuat seluruh pihak lawan bank maupun bukan bank yang bertransaksi dengan BPR pelapor. Kolom III Nomor Identitas dan VI NPWP sengaja tidak disimpan (keputusan privasi).';

COMMENT ON COLUMN pihak_lawan_register.jenis_identitas_code IS
    'Form 00.16 kolom II Jenis Identitas (baku PDF #288): 1 KTP, 2 Paspor, 3 KITAS/KITAP, 4 Kartu Keluarga. Hanya untuk perorangan.';

COMMENT ON COLUMN pihak_lawan_register.jenis_usaha_code IS
    'Form 00.16 kolom IX Jenis Kegiatan Usaha (baku PDF #288): 1 Konvensional, 2 Syariah. Hanya untuk Lembaga Jasa Keuangan.';

COMMENT ON COLUMN pihak_lawan_register.hubungan_bank_code IS
    'Form 00.16 kolom X Hubungan dengan Bank (baku PDF #288): 12 Pihak Terkait, 20 Pihak Tidak Terkait.';

COMMENT ON COLUMN pihak_lawan_register.lembaga_pemeringkat_code IS
    'Form 00.16 kolom XII Lembaga Pemeringkat (Lampiran 08). Tanpa peringkat / pemeringkat tak diakui OJK = 9 (PDF #291).';

COMMENT ON COLUMN pihak_lawan_register.peringkat_code IS
    'Form 00.16 kolom XIII Peringkat Pihak Lawan (Lampiran 09). Tanpa peringkat = 99 (PDF #291).';

COMMENT ON COLUMN pihak_lawan_register.tanggal_pemeringkatan IS
    'Form 00.16 kolom XIV Tanggal Pemeringkatan (reviu terakhir). NULL = tanpa/belum ada peringkat.';

COMMENT ON COLUMN pihak_lawan_register.tanggal_lahir IS
    'Form 00.16 kolom XV Tanggal Lahir. NULL = pihak lawan bukan perorangan.';

CREATE INDEX IF NOT EXISTS idx_pihak_lawan_golongan
    ON pihak_lawan_register (golongan_code);
CREATE INDEX IF NOT EXISTS idx_pihak_lawan_grup
    ON pihak_lawan_register (grup_id);
