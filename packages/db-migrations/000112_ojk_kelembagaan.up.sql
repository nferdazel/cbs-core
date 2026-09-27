-- CBS Migration 000112: data kelembagaan bank untuk LAPORAN_KELEMBAGAAN (jaringan
-- kantor, direksi/komisaris, pejabat eksekutif).
-- Run after: 000111_ojk_form06_amortisasi_restrukturisasi.up.sql (urutan; tidak
-- bergantung isinya). Nomor 000112 dipesan khusus pekerjaan ini.
--
-- Menutup laporan K2 LAPORAN_KELEMBAGAAN (docs/CELAH-LAPORAN-OJK.md §3): laporan ini
-- sebelumnya ditandai belum buildable karena tidak ada sumber data jaringan kantor,
-- direksi, komisaris, dan pejabat eksekutif. Dua tabel ADITIF di bawah menjadi
-- sumbernya; tidak ada tabel/kolom yang diubah.
--
-- Sumber struktur kolom (verifikasi ke PDF resmi SEOJK No. 16/SEOJK.03/2024, 528 hlm.;
-- salinan di luar repo, tidak di-commit):
--   * Ruang lingkup Laporan Kelembagaan BPR: "status jaringan kantor, anggota direksi,
--     anggota dewan komisaris, dan pejabat eksekutif serta dokumen pendukung"
--     (PDF #page 2).
--   * Form 00.04 - 1/2/3 "Data Kantor BPR": PDF #page 88-95, hlm. 36-43 (kolom I-XV:
--     sandi kantor, nama, koordinat, alamat, pimpinan, telepon, jumlah pegawai,
--     jumlah kantor kas, status kepemilikan gedung, kas keliling, EDC, ATM, perubahan,
--     jumlah SKK).
--   * Form 00.02 - 1/2/3 "Data Anggota Direksi dan Anggota Dewan Komisaris BPR":
--     PDF #page 75-82, hlm. 23-30 (kolom I-XVII; sandi jabatan 110/120 Direksi,
--     210/220 Komisaris).
--   * Form 00.03 - 1/2/3 "Data Pejabat Eksekutif BPR": PDF #page 83-87, hlm. 31-35
--     (kolom I-VIII; jabatan 00/01/02, keanggotaan komite 00/01/02).
--
-- Mengapa HANYA dua tabel dan bukan salinan seluruh form: yang diminta adalah seam data
-- minimal yang membuat laporan dapat dibangun, bukan menyalin ratusan kolom. Data yang
-- belum punya sumber (mis. NIK, koordinat, jumlah pegawai per jenjang, jumlah EDC/ATM)
-- TIDAK dikarang dan ditandai belum tersedia oleh perakit laporan
-- (apps/api/internal/ojkreport/kelembagaan.go) dengan alasan eksplisit.
--
-- Semua nilai adalah DATA BANK. Kategori orang (DIREKSI/KOMISARIS/PEJABAT_EKSEKUTIF)
-- dan status kantor/orang dibatasi CHECK karena maknanya baku, tetapi jabatan, tipe
-- kantor, dan sandi jabatan OJK disimpan sebagai teks yang bank isi sendiri. NIK
-- SENGAJA tidak disimpan: keputusan panel menetapkan NIK tidak dipaparkan sebagai
-- keluaran laporan (docs/KEPUTUSAN-OJK.md §4).
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS bank_offices (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Tipe/status jaringan kantor menurut bank (mis. KANTOR_PUSAT, KANTOR_CABANG,
    -- KANTOR_KAS, SENTRA_KEUANGAN_KHUSUS, TERMINAL_PERBANKAN_ELEKTRONIK). Teks bebas:
    -- daftar sandi "Jenis" resmi Form 00.11 divariasikan per format, jadi tidak dikarang.
    office_type        VARCHAR(32) NOT NULL CHECK (length(btrim(office_type)) > 0),
    -- Sandi kantor 3 angka yang bank pakai (Form 00.04 kolom I). Boleh kosong bila bank
    -- belum mengisinya; laporan menulis "-".
    code               VARCHAR(16) NOT NULL DEFAULT '',
    name               VARCHAR(255) NOT NULL CHECK (length(btrim(name)) > 0),
    address            TEXT NOT NULL DEFAULT '',
    city               VARCHAR(128) NOT NULL DEFAULT '',
    -- Sandi Kabupaten/Kota (Lampiran 03 SEOJK 16/2024, tabel ojk_kabupaten migrasi
    -- 000102). Nullable: NULL = bank belum mengisi; laporan menulis "-".
    ojk_kabupaten_code TEXT REFERENCES ojk_kabupaten(code) ON DELETE SET NULL,
    opened_at          DATE,
    closed_at          DATE,
    -- AKTIF = kantor beroperasi; TUTUP = kantor sudah ditutup (bukan dihapus, agar
    -- riwayat jaringan kantor tetap terbaca laporan).
    status             VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'TUTUP')),
    note               TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by         UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE bank_offices IS
    'Jaringan kantor bank (kantor pusat/cabang/kas/SKK/TPE) untuk LAPORAN_KELEMBAGAAN; sumber Form 00.04 "Data Kantor BPR" (PDF #page 88-95, hlm. 36-43). Semua nilai diisi bank; hanya kolom yang dipetakan yang disimpan, sisanya ditandai belum tersedia laporan.';

COMMENT ON COLUMN bank_offices.office_type IS
    'Tipe kantor menurut bank (teks bebas, mis. KANTOR_PUSAT/KANTOR_CABANG/KANTOR_KAS). Bukan enum yang dikarang: daftar sandi resmi Form 00.11 bervariasi dan belum dipetakan.';

COMMENT ON COLUMN bank_offices.code IS
    'Sandi kantor (Form 00.04 kolom I, 3 angka) yang bank pakai. Kosong = belum diisi; laporan menulis "-".';

COMMENT ON COLUMN bank_offices.ojk_kabupaten_code IS
    'Sandi Kabupaten/Kota lokasi kantor (Lampiran 03 SEOJK 16/2024, FK ojk_kabupaten migrasi 000102). NULL = belum diisi; laporan menulis "-", BUKAN dikarang dari nama kota.';

COMMENT ON COLUMN bank_offices.status IS
    'AKTIF = kantor beroperasi; TUTUP = kantor ditutup. Kantor yang ditutup tidak dihapus agar riwayat jaringan terbaca laporan.';

CREATE INDEX IF NOT EXISTS idx_bank_offices_status
    ON bank_offices (status);
CREATE INDEX IF NOT EXISTS idx_bank_offices_type
    ON bank_offices (office_type);

CREATE TABLE IF NOT EXISTS bank_management (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Kategori baku: DIREKSI (Form 00.02, sandi jabatan 110/120), KOMISARIS (210/220),
    -- PEJABAT_EKSEKUTIF (Form 00.03, sandi jabatan 00/01/02).
    category           VARCHAR(32) NOT NULL
        CHECK (category IN ('DIREKSI', 'KOMISARIS', 'PEJABAT_EKSEKUTIF')),
    name               VARCHAR(255) NOT NULL CHECK (length(btrim(name)) > 0),
    -- Jabatan bebas yang bank isi (mis. Direktur Utama, Komisaris Independen,
    -- Kepala Satuan Kerja Kepatuhan). Laporan mencetaknya apa adanya.
    position           VARCHAR(128) NOT NULL DEFAULT '',
    -- Sandi jabatan resmi OJK (Form 00.02: 110/120 Direksi, 210/220 Komisaris;
    -- Form 00.03: 00/01/02 Pejabat Eksekutif). Bank mengisi; kosong = "-".
    ojk_position_code  VARCHAR(16) NOT NULL DEFAULT '',
    -- Nomor dan tanggal surat persetujuan/ pengangkatan OJK (Form 00.02 kolom VII,
    -- Form 00.03 kolom VI). Kosong = belum diisi.
    license_number     VARCHAR(128) NOT NULL DEFAULT '',
    license_date       DATE,
    started_at         DATE,
    ended_at           DATE,
    -- AKTIF = masih menjabat; NONAKTIF = sudah selesai/berhenti.
    status             VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note               TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by         UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE bank_management IS
    'Direksi, dewan komisaris, dan pejabat eksekutif bank untuk LAPORAN_KELEMBAGAAN; sumber Form 00.02 (PDF #page 75-82, hlm. 23-30) dan Form 00.03 (PDF #page 83-87, hlm. 31-35). NIK tidak disimpan (keputusan privasi docs/KEPUTUSAN-OJK.md §4).';

COMMENT ON COLUMN bank_management.category IS
    'Kategori baku: DIREKSI, KOMISARIS, atau PEJABAT_EKSEKUTIF. Menentukan bagian laporan (Form 00.02 vs 00.03).';

COMMENT ON COLUMN bank_management.ojk_position_code IS
    'Sandi jabatan resmi OJK (Form 00.02 kolom IV: 110/120/210/220; Form 00.03 kolom IV: 00/01/02) yang bank isi. Kosong = belum diisi; laporan menulis "-". Tidak diturunkan dari teks jabatan.';

COMMENT ON COLUMN bank_management.license_number IS
    'Nomor surat persetujuan OJK (Form 00.02 kolom VII) atau surat pengangkatan (Form 00.03 kolom VI) yang bank isi. Kosong = belum diisi; laporan menulis "-".';

CREATE INDEX IF NOT EXISTS idx_bank_management_category
    ON bank_management (category);
CREATE INDEX IF NOT EXISTS idx_bank_management_status
    ON bank_management (status);
