-- CBS Migration 000123: register kas valuta asing untuk Form 03.00
-- "Daftar Kas dalam Valuta Asing".
-- Run after: 000122_surat_berharga_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000123 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 03.00 "DAFTAR KAS DALAM VALUTA ASING": PDF #page 126 (hlm. tercetak 74),
--     sandi PDF #page 127 (hlm. 75), penjelasan PDF #page 128 (hlm. 76); cross-ref
--     posisi keuangan PDF #page 102 (hlm. 50). Lima kolom: I Sandi Kantor,
--     II Jenis Valuta Asing, III Nominal, IV Kurs Tengah (Rp), V Nilai Rupiah.
--     Satu baris = satu jenis valas yang diperdagangkan; ADA baris JUMLAH (PDF #126).
--   * Definisi dan aturan (PDF #page 128):
--     - Kas valas = uang kertas/uang logam asing dan cek pelawat (travellers cheque)
--       yang masih berlaku yang dimiliki BPR sebagai pedagang valuta asing.
--     - III: nilai valas (original currency) sebelum dirupiahkan pada tanggal laporan.
--     - IV: kurs tengah yang tersedia di sistem Bank Indonesia pada tanggal laporan;
--       bila kurs tengah tidak tersedia, dipakai rata-rata (kurs beli + kurs jual) / 2.
--     - V: hasil perkalian dari nominal dengan kurs tengah -> TURUNAN.
--
-- KEPUTUSAN KOLOM:
--   * Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
--     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--   * Kolom II "Jenis Valuta Asing" adalah sandi Lampiran 04. Daftar Lampiran 04 tidak
--     disalin ke kode (bukan bagian form yang ditranskrip), sehingga disimpan teks sandi
--     apa adanya + hint; TIDAK ada daftar sandi karangan.
--   * Kolom III "Nominal" dan IV "Kurs Tengah" adalah ISIAN BANK (2 desimal). Form tidak
--     menghitung keduanya; III memakai 2 desimal sesuai transkrip.
--   * Kolom V "Nilai Rupiah" adalah TURUNAN: III x IV, rupiah penuh (dibulatkan 0
--     desimal mengikuti FormatRupiah). Bila III atau IV tidak diisi, V tidak dihitung.
--   * Tidak ada kolom identitas pribadi pada form ini.
--
-- Pemicu: form ini hanya relevan bila BPR berstatus pedagang valuta asing. Status PVA
-- sudah dicatat pada konfigurasi ojk.report.pva_status (migrasi 000096:28); builder tetap
-- memakai aturan yang sama seperti register lain (kosong -> SkippedForms) dan TIDAK
-- mengarang baris bila bank bukan PVA.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS kas_valas_register (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Jenis Valuta Asing (kolom II): sandi Lampiran 04, teks apa adanya (daftar lampiran
    -- tidak disalin ke kode). Wajib terisi.
    jenis_valas_code    VARCHAR(16) NOT NULL
        CHECK (length(btrim(jenis_valas_code)) > 0),
    -- Nominal (kolom III): nilai valas original currency, 2 desimal; isian bank.
    nominal             NUMERIC(28,2) NOT NULL DEFAULT 0
        CHECK (nominal >= 0),
    -- Kurs Tengah (kolom IV) dalam rupiah, 2 desimal; isian bank (kurs BI pada tanggal
    -- laporan, atau rata-rata kurs beli/jual bila kurs tengah tidak tersedia).
    kurs_tengah         NUMERIC(28,2) NOT NULL DEFAULT 0
        CHECK (kurs_tengah >= 0),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of               DATE NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by          UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE kas_valas_register IS
    'Register kas valuta asing untuk Form 03.00 "Daftar Kas dalam Valuta Asing"; sumber struktur PDF #page 126-128 (hlm. 74-76) SEOJK No. 16/SEOJK.03/2024. Satu baris = satu jenis valas yang diperdagangkan; ADA baris JUMLAH di laporan. Kolom V Nilai Rupiah adalah turunan (III x IV). Form hanya relevan bila BPR berstatus pedagang valuta asing (ojk.report.pva_status).';

COMMENT ON COLUMN kas_valas_register.jenis_valas_code IS
    'Jenis Valuta Asing (kolom II): sandi Lampiran 04, teks apa adanya. Daftar lengkap Lampiran 04 tidak disalin ke kode.';

COMMENT ON COLUMN kas_valas_register.nominal IS
    'Nominal (kolom III): nilai valas original currency sebelum dirupiahkan (PDF #page 128), 2 desimal; isian bank.';

COMMENT ON COLUMN kas_valas_register.kurs_tengah IS
    'Kurs Tengah (kolom IV) dalam rupiah, 2 desimal; isian bank. Menurut PDF #page 128 memakai kurs tengah Bank Indonesia pada tanggal laporan, atau rata-rata (kurs beli + kurs jual) / 2 bila kurs tengah tidak tersedia.';

COMMENT ON COLUMN kas_valas_register.as_of IS
    'Tanggal posisi kas valas (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN kas_valas_register.status IS
    'AKTIF = posisi valas masih dilaporkan; NONAKTIF = sudah tidak dilaporkan namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_kas_valas_jenis
    ON kas_valas_register (jenis_valas_code);
CREATE INDEX IF NOT EXISTS idx_kas_valas_as_of
    ON kas_valas_register (as_of);
CREATE INDEX IF NOT EXISTS idx_kas_valas_status
    ON kas_valas_register (status);
