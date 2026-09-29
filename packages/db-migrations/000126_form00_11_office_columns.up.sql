-- CBS Migration 000126: kolom Form 00.11 "Data Kantor selain Kantor Pusat dan Kantor
-- Cabang dan Terminal Perbankan Elektronik (TPE)" pada bank_offices.
-- Run after: 000125_form06_01_agunan_columns.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000126 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 00.11 "DATA KANTOR SELAIN KANTOR PUSAT DAN KANTOR CABANG DAN TERMINAL
--     PERBANKAN ELEKTRONIK (TPE)": PDF #page 270 (hlm. tercetak 218) susunan kolom,
--     sandi PDF #page 271-272 (hlm. 219-220), penjelasan PDF #page 273-275 (hlm. 221-223).
--     Empat belas kolom: I Jenis, II Kode Kantor, III Sandi Kantor Induk,
--     IV Sandi Kantor Sebelumnya, V Nama Kantor, VI Koordinat, VII Alamat,
--     VIII Nama Pimpinan, IX No. Telepon, X Keterangan Data Kantor dan TPE,
--     XI Tanggal Pelaksanaan, XII Sandi Kantor Kendali, XIII Tanggal Persetujuan,
--     XIV Lokasi. TIDAK ada baris JUMLAH.
--   * Sandi I Jenis (PDF #271): 02 Kantor Kas, 03 Kas Keliling, 04 Titik Pembayaran,
--     05 ATM, 06 EDC, 07 Kantor Wilayah, 08 Sentra Keuangan Khusus, 99 Lainnya.
--   * Sandi X Keterangan (PDF #272): 1-7 (pembukaan/pemindahan/perubahan status/tidak
--     berubah).
--   * XIII Tanggal Persetujuan: tanggal surat persetujuan OJK.
--   * XIV Lokasi mengacu Lampiran 03 — daftar sandi kabupaten/kota, sudah ada sebagai
--     tabel ojk_kabupaten (migrasi 000102) dan kolom bank_offices.ojk_kabupaten_code.
--
-- KEPUTUSAN KOLOM:
--   * bank_offices DIPERLUAS, bukan tabel baru: kantor kas/SKK/kantor wilayah/TPE adalah
--     jaringan kantor yang sama dan sudah dimodelkan. Form 00.04 memakai kantor
--     pusat/cabang; Form 00.11 memakai SELAIN pusat/cabang + TPE. Pemisahan jenis ada di
--     kolom I (sandi baku), bukan di tabel berbeda.
--   * Kolom I Jenis: kolom baru `ojk_office_kind_code` (sandi baku 02-08/99), TERPISAH
--     dari `office_type` yang teks bebas kebijakan bank. `office_type` tidak diubah agar
--     tidak menggeser perilaku Form 00.04 dan data lama.
--   * Kolom II Kode Kantor memakai `code` yang sudah ada (Form 00.04 kolom I).
--   * Kolom V Nama memakai `name`; VII Alamat memakai `address`; XIV Lokasi memakai
--     `ojk_kabupaten_code`. Tidak diduplikasi.
--   * Kolom III/IV Sandi Kantor Induk/Sebelumnya, VI Koordinat, VIII Nama Pimpinan,
--     IX No. Telepon, X Keterangan (sandi 1-7), XI Tanggal Pelaksanaan, XII Sandi Kantor
--     Kendali, XIII Tanggal Persetujuan: kolom baru. Seluruhnya isian bank; tidak
--     diturunkan.
--   * Kolom X Keterangan HANYA diisi bila ada perubahan; sandi 4 = tidak berubah.
--   * Tidak ada kolom identitas pribadi selain nama pimpinan (jabatan struktural, bukan
--     konsumen) — tidak ada keputusan privasi yang berlaku di sini.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS. Tidak ada seed data dan
-- tidak mengubah kolom yang sudah ada.

ALTER TABLE bank_offices
    ADD COLUMN IF NOT EXISTS ojk_office_kind_code VARCHAR(2)
        CHECK (ojk_office_kind_code IS NULL OR ojk_office_kind_code IN ('02','03','04','05','06','07','08','99')),
    ADD COLUMN IF NOT EXISTS parent_office_code VARCHAR(16),
    ADD COLUMN IF NOT EXISTS previous_office_code VARCHAR(16),
    ADD COLUMN IF NOT EXISTS coordinates VARCHAR(64),
    ADD COLUMN IF NOT EXISTS head_name VARCHAR(128),
    ADD COLUMN IF NOT EXISTS phone_number VARCHAR(32),
    ADD COLUMN IF NOT EXISTS ojk_change_code VARCHAR(1)
        CHECK (ojk_change_code IS NULL OR ojk_change_code IN ('1','2','3','4','5','6','7')),
    ADD COLUMN IF NOT EXISTS implementation_date DATE,
    ADD COLUMN IF NOT EXISTS control_office_code VARCHAR(16),
    ADD COLUMN IF NOT EXISTS ojk_approval_date DATE;

CREATE INDEX IF NOT EXISTS idx_bank_offices_ojk_kind
    ON bank_offices (ojk_office_kind_code);

COMMENT ON COLUMN bank_offices.ojk_office_kind_code IS
    'Form 00.11 kolom I Jenis: sandi baku PDF #271 — 02 Kantor Kas, 03 Kas Keliling, 04 Titik Pembayaran, 05 ATM, 06 EDC, 07 Kantor Wilayah, 08 Sentra Keuangan Khusus, 99 Lainnya. Terpisah dari office_type (teks bebas bank, dipakai Form 00.04).';

COMMENT ON COLUMN bank_offices.parent_office_code IS
    'Form 00.11 kolom III Sandi Kantor Induk: 3 angka sandi kantor pusat/cabang induk, atau sandi kantor penanggung jawab TPE (PDF #273). Isian bank.';

COMMENT ON COLUMN bank_offices.previous_office_code IS
    'Form 00.11 kolom IV Sandi Kantor Sebelumnya: 3 angka sandi kantor sebelumnya bila ada perubahan status jaringan kantor (PDF #274). Isian bank.';

COMMENT ON COLUMN bank_offices.coordinates IS
    'Form 00.11 kolom VI Koordinat kantor kas/kantor wilayah/SKK, atau koordinat kantor induk untuk kas keliling, atau lokasi TPE (PDF #274). Isian bank, teks apa adanya.';

COMMENT ON COLUMN bank_offices.head_name IS
    'Form 00.11 kolom VIII Nama Pimpinan: nama pimpinan/kepala kantor kas/kantor wilayah/SKK (PDF #274). Kosong untuk TPE. Isian bank.';

COMMENT ON COLUMN bank_offices.phone_number IS
    'Form 00.11 kolom IX No. Telepon kantor kas/kantor wilayah/SKK (PDF #274). Kosong untuk TPE. Isian bank.';

COMMENT ON COLUMN bank_offices.ojk_change_code IS
    'Form 00.11 kolom X Keterangan Data Kantor dan TPE: sandi baku PDF #272/#274 — 1 pembukaan, 2 pemindahan (induk), 3 pemindahan, 4 tidak berubah, 5-7 perubahan status. Isian bank.';

COMMENT ON COLUMN bank_offices.implementation_date IS
    'Form 00.11 kolom XI Tanggal Pelaksanaan: tanggal pembukaan/penggunaan/pemindahan/perubahan status jaringan kantor (PDF #274). Isian bank.';

COMMENT ON COLUMN bank_offices.control_office_code IS
    'Form 00.11 kolom XII Sandi Kantor Kendali: sandi kantor di bawah kendali langsung kantor yang dilaporkan; HANYA diisi untuk Kantor Wilayah dan Sentra Keuangan Khusus (PDF #274). Isian bank.';

COMMENT ON COLUMN bank_offices.ojk_approval_date IS
    'Form 00.11 kolom XIII Tanggal Persetujuan: tanggal surat persetujuan OJK atas pembukaan/penutupan/pemindahan/perubahan status jaringan kantor (PDF #274). Isian bank.';
