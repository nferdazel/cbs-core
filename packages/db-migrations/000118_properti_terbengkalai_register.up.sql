-- CBS Migration 000118: register properti terbengkalai untuk Form 17.00
-- "Daftar Properti Terbengkalai".
-- Run after: 000117_pinjaman_diterima_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000118 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 17.00 "DAFTAR PROPERTI TERBENGKALAI": PDF #page 229 (hlm. tercetak 177).
--     Sepuluh kolom: I Sandi Kantor, II No. Register, III Jenis Properti Terbengkalai,
--     IV Alamat Properti Terbengkalai, V Koordinat, VI Tanggal Penetapan,
--     VII Biaya Perolehan atau Nilai Wajar, VIII Akumulasi Penyusutan atau Amortisasi,
--     IX Jumlah, X Metode Pengukuran. TIDAK ADA baris JUMLAH.
--   * Sandi: PDF #page 230 (hlm. 178). III 1 Tanah / 2 Bangunan / 3 Tanah dan Bangunan;
--     X 1 Model Biaya / 2 Model Revaluasi (Nilai Wajar).
--   * Penjelasan: PDF #page 231-232 (hlm. 179-180). II No. Register unik, satu register
--     untuk satu properti, "no reuse atau no recycle", mandatory. VIII memuat akumulasi
--     penurunan nilai bila ada. IX Jumlah = nilai pengakuan awal dikurangi akumulasi
--     penyusutan/amortisasi termasuk akumulasi kerugian penurunan nilai.
--
-- Kolom IX "Jumlah" SENGAJA TIDAK disimpan: ia turunan yang dihitung laporan dari
-- VII - VIII. Menyimpannya akan membuat dua sumber kebenaran yang bisa berbeda.
--
-- NO-REUSE NOMOR REGISTER: II No. Register bersifat unik dan tidak boleh dipakai ulang
-- (PDF #page 231). Karena itu kolom no_register diberi UNIQUE constraint penuh (bukan
-- parsial): baris NONAKTIF tetap memegang nomornya sehingga nomor tidak pernah kembali
-- bebas. Penghapusan register dilakukan sebagai soft-delete (status -> NONAKTIF), bukan
-- DELETE fisik; DELETE fisik akan membebaskan nomor dan melanggar aturan no-reuse.
--
-- Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
-- tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain (07.00).
--
-- Semua NILAI (nomor, sandi, alamat, koordinat, tanggal, nominal) adalah isian bank.
-- Tidak ada seed angka.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS properti_terbengkalai_register (
    id                                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- No. Register (kolom II): kode unik properti, mandatory, no reuse/no recycle
    -- (PDF #page 231). UNIQUE penuh menjaga nomor tetap terpakai walau baris NONAKTIF.
    no_register                         VARCHAR(64) NOT NULL
        CHECK (length(btrim(no_register)) > 0),
    -- Jenis Properti Terbengkalai (kolom III) baku PDF #page 230.
    jenis_properti_code                 VARCHAR(1) NOT NULL
        CHECK (jenis_properti_code IN ('1', '2', '3')),
    -- Alamat Properti Terbengkalai (kolom IV). Wajib terisi agar baris dapat dibaca.
    alamat_properti                     VARCHAR(255) NOT NULL
        CHECK (length(btrim(alamat_properti)) > 0),
    -- Koordinat (kolom V), teks bebas (mis. "-6.200000, 106.816666"). Boleh kosong.
    koordinat                           VARCHAR(64) NOT NULL DEFAULT '',
    -- Tanggal Penetapan (kolom VI), diisi bank.
    tanggal_penetapan                   DATE NOT NULL,
    -- Biaya Perolehan atau Nilai Wajar (kolom VII), rupiah penuh; isian bank.
    biaya_perolehan_atau_nilai_wajar    NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (biaya_perolehan_atau_nilai_wajar >= 0),
    -- Akumulasi Penyusutan atau Amortisasi (kolom VIII), rupiah penuh; isian bank.
    -- Menurut PDF #page 231, bila properti mengalami penurunan nilai, jumlah penurunan
    -- itu juga dilaporkan pada kolom ini.
    akumulasi_penyusutan_atau_amortisasi NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (akumulasi_penyusutan_atau_amortisasi >= 0),
    -- Metode Pengukuran (kolom X) baku PDF #page 230: 1 Model Biaya, 2 Model Revaluasi.
    metode_pengukuran_code              VARCHAR(1) NOT NULL
        CHECK (metode_pengukuran_code IN ('1', '2')),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                               DATE NOT NULL,
    -- AKTIF = properti masih tercatat dan ikut laporan; NONAKTIF = sudah tidak
    -- dilaporkan lagi tetapi tetap tersimpan sebagai riwayat DAN memegang nomor
    -- register-nya (no reuse).
    status                              VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                                TEXT NOT NULL DEFAULT '',
    created_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                          UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                          UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    CONSTRAINT properti_terbengkalai_no_register_unique UNIQUE (no_register)
);

COMMENT ON TABLE properti_terbengkalai_register IS
    'Register properti terbengkalai untuk Form 17.00 "Daftar Properti Terbengkalai"; sumber struktur PDF #page 229-232 (hlm. 177-180) SEOJK No. 16/SEOJK.03/2024. Semua nilai diisi bank. Kolom IX Jumlah tidak disimpan karena turunan VII - VIII. No. Register unik dan tidak boleh dipakai ulang (no reuse/no recycle); penghapusan adalah soft-delete NONAKTIF sehingga UNIQUE penuh tetap memegang nomornya.';

COMMENT ON COLUMN properti_terbengkalai_register.no_register IS
    'No. Register (kolom II): kode unik properti, mandatory, "no reuse atau no recycle" (PDF #page 231). UNIQUE penuh: baris NONAKTIF tetap memegang nomor sehingga tidak dapat dipakai ulang.';

COMMENT ON COLUMN properti_terbengkalai_register.jenis_properti_code IS
    'Jenis Properti Terbengkalai (kolom III) baku PDF #page 230: 1 Tanah, 2 Bangunan, 3 Tanah dan Bangunan.';

COMMENT ON COLUMN properti_terbengkalai_register.alamat_properti IS
    'Alamat Properti Terbengkalai (kolom IV). Wajib terisi agar baris dapat dibaca; tidak ada alamat yang dikarang sistem.';

COMMENT ON COLUMN properti_terbengkalai_register.koordinat IS
    'Koordinat (kolom V), teks bebas (mis. lintang, bujur). Boleh kosong.';

COMMENT ON COLUMN properti_terbengkalai_register.tanggal_penetapan IS
    'Tanggal Penetapan (kolom VI), diisi bank. Ditulis laporan dalam format TT-BB-TTTT mengikuti mayoritas form.';

COMMENT ON COLUMN properti_terbengkalai_register.biaya_perolehan_atau_nilai_wajar IS
    'Biaya Perolehan atau Nilai Wajar (kolom VII), rupiah penuh. Angka bank.';

COMMENT ON COLUMN properti_terbengkalai_register.akumulasi_penyusutan_atau_amortisasi IS
    'Akumulasi Penyusutan atau Amortisasi (kolom VIII) sebagai BESARAN penyusutan >= 0; termasuk akumulasi penurunan nilai bila ada (PDF #page 231). Laporan menghitung Jumlah (IX) = VII - VIII.';

COMMENT ON COLUMN properti_terbengkalai_register.metode_pengukuran_code IS
    'Metode Pengukuran (kolom X) baku PDF #page 230: 1 Model Biaya, 2 Model Revaluasi (Nilai Wajar).';

COMMENT ON COLUMN properti_terbengkalai_register.as_of IS
    'Tanggal posisi properti (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN properti_terbengkalai_register.status IS
    'AKTIF = properti masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat dan memegang nomor register-nya.';

CREATE INDEX IF NOT EXISTS idx_properti_terbengkalai_jenis
    ON properti_terbengkalai_register (jenis_properti_code);
CREATE INDEX IF NOT EXISTS idx_properti_terbengkalai_metode
    ON properti_terbengkalai_register (metode_pengukuran_code);
CREATE INDEX IF NOT EXISTS idx_properti_terbengkalai_as_of
    ON properti_terbengkalai_register (as_of);
CREATE INDEX IF NOT EXISTS idx_properti_terbengkalai_status
    ON properti_terbengkalai_register (status);
