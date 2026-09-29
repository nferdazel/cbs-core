-- CBS Migration 000125: kolom Form 06.01 "Daftar Agunan" pada loan_collaterals.
-- Run after: 000124_kredit_sindikasi_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000125 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 06.01 "DAFTAR AGUNAN": PDF #page 169 (hlm. tercetak 117) susunan kolom,
--     sandi PDF #page 170 (hlm. 118), penjelasan PDF #page 171-172 (hlm. 119-120).
--     Delapan kolom: I Sandi Kantor, II Kode Register/Nomor Agunan, III No. Rekening,
--     IV Jenis Agunan, V Alamat Agunan, VI Nilai yang Diagunkan,
--     VII Nilai Agunan (Nominal + Penilai + Tanggal Penilaian Terakhir),
--     VIII Nilai yang Diperhitungkan untuk PPKA. ADA baris JUMLAH (PDF #169).
--   * Sandi kolom VII.b Penilai: 1 Penilai Independen, 2 Internal BPR (PDF #170).
--   * Lampiran 01 "Daftar Sandi Jenis Agunan" (PDF #page 301, hlm. cetak 249): sandi
--     3 digit dengan dua kategori induk — LIKUID (101 SBI/Surat Utang Pemerintah,
--     102 tabungan/deposito diblokir, 103 logam mulia) dan NON LIKUID (201 dst).
--
-- KEPUTUSAN SME 29 Sep 2026 atas inkonsistensi internal PDF (CELAH-FORM-OJK §3 poin 6):
--   * Sel "Likuid | Non Likuid" pada halaman susunan (PDF #169) BUKAN kolom kesembilan.
--     Daftar sandi resmi (PDF #170) berhenti di kolom VIII, dan penjelasan (PDF #171)
--     meletakkan "Likuid/Non Likuid" sebagai rincian DI DALAM kolom IV Jenis Agunan
--     ("IV. Jenis Agunan ... 1. Likuid ... 2. NonLikuid"). Jadi keduanya adalah KATEGORI
--     INDUK dari sandi 3 digit Lampiran 01, bukan kolom terpisah. Konfirmasi OJK tidak
--     lagi diperlukan untuk butir ini.
--   * Kolom IV karena itu menyimpan sandi Lampiran 01 (3 digit, teks apa adanya; daftar
--     lampiran tidak disalin ke kode). Kategori Likuid/Non Likuid TIDAK disimpan
--     terpisah: ia turunan digit pertama sandi dan hanya dijelaskan di laporan.
--
-- KEPUTUSAN KOLOM:
--   * Kolom I "Sandi Kantor" tidak disimpan: diambil dari kantor pelapor tunggal
--     bank_offices (migrasi 000112), sama seperti form bank-wide lain.
--   * Kolom II "Kode Register/Nomor Agunan" wajib, unik, dan tidak boleh dipakai ulang
--     (PDF #171: "no reuse atau no recycle"). UNIQUE penuh; baris yang dilepas tetap
--     memegang nomornya (pelepasan di sini = status RELEASED, baris tidak dihapus).
--   * Kolom III No. Rekening: dari loans.loan_number lewat loan_id; tidak disimpan ulang.
--   * Kolom V Alamat Agunan: kolom baru `ojk_collateral_address` (TEXT, nullable).
--   * Kolom VI Nilai yang Diagunkan: kolom baru `ojk_bound_value` (NUMERIC).
--   * Kolom VII Nominal: memakai `appraisal_value` yang sudah ada. Penilai: kolom baru
--     `ojk_appraiser_code` (1/2). Tanggal Penilaian Terakhir: `appraisal_date` yang ada.
--   * Kolom VIII Nilai yang Diperhitungkan untuk PPKA: kolom baru
--     `ojk_ppka_amount` (NUMERIC, nullable). Isian bank; tidak diturunkan, karena
--     kebijakan PPKA per agunan adalah keputusan bank.
--   * Kolom VII.a "Nominal" dan VI "Nilai yang Diagunkan" berbeda: VI adalah nilai yang
--     diikat sebagai jaminan, VII.a adalah taksasi. Keduanya isian bank, tidak dipaksa
--     sama.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS, CREATE INDEX/UNIQUE
-- IF NOT EXISTS. Tidak ada seed data dan tidak mengubah kolom yang sudah ada.

ALTER TABLE loan_collaterals
    ADD COLUMN IF NOT EXISTS ojk_register_number VARCHAR(64),
    ADD COLUMN IF NOT EXISTS ojk_collateral_address TEXT,
    ADD COLUMN IF NOT EXISTS ojk_collateral_type_code VARCHAR(8),
    ADD COLUMN IF NOT EXISTS ojk_bound_value NUMERIC(28,4)
        CHECK (ojk_bound_value IS NULL OR ojk_bound_value >= 0),
    ADD COLUMN IF NOT EXISTS ojk_appraiser_code VARCHAR(1)
        CHECK (ojk_appraiser_code IS NULL OR ojk_appraiser_code IN ('1', '2')),
    ADD COLUMN IF NOT EXISTS ojk_ppka_amount NUMERIC(28,4)
        CHECK (ojk_ppka_amount IS NULL OR ojk_ppka_amount >= 0);

-- Kolom II unik dan tidak boleh dipakai ulang (PDF #171 "no reuse atau no recycle").
-- UNIQUE parsial: baris lama yang belum diisi (NULL) tidak menghalangi; begitu diisi,
-- nomornya terkunci.
CREATE UNIQUE INDEX IF NOT EXISTS uq_loan_collaterals_ojk_register_number
    ON loan_collaterals (ojk_register_number)
    WHERE ojk_register_number IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_loan_collaterals_ojk_register_number
    ON loan_collaterals (ojk_register_number);

COMMENT ON COLUMN loan_collaterals.ojk_register_number IS
    'Form 06.01 kolom II Kode Register/Nomor Agunan: kode unik 1 agunan = 1 kode, sama dengan SLIK, tidak boleh dipakai ulang (no reuse/no recycle) dan tidak boleh berubah selama fasilitas tercatat (PDF #171). Diisi bank; UNIQUE parsial mengunci begitu terisi.';

COMMENT ON COLUMN loan_collaterals.ojk_collateral_address IS
    'Form 06.01 kolom V Alamat Agunan: alamat lengkap agunan yang dijaminkan debitur kepada BPR (PDF #171). Teks bebas, isian bank.';

COMMENT ON COLUMN loan_collaterals.ojk_collateral_type_code IS
    'Form 06.01 kolom IV Jenis Agunan: sandi Lampiran 01 (3 digit, mis. 101 likuid SBI/Surat Utang Pemerintah, 202 tanah/bangunan). Kategori Likuid (1xx)/Non Likuid (2xx) adalah turunan digit pertama, tidak disimpan terpisah. Daftar lampiran tidak disalin ke kode; sandi ditulis apa adanya.';

COMMENT ON COLUMN loan_collaterals.ojk_bound_value IS
    'Form 06.01 kolom VI Nilai yang Diagunkan (rupiah penuh): nilai yang diikat sebagai jaminan. Beda dari taksasi appraisal_value (kolom VII.a); keduanya isian bank.';

COMMENT ON COLUMN loan_collaterals.ojk_appraiser_code IS
    'Form 06.01 kolom VII.b Penilai: 1 Penilai Independen, 2 Internal BPR (PDF #170). Isian bank.';

COMMENT ON COLUMN loan_collaterals.ojk_ppka_amount IS
    'Form 06.01 kolom VIII Nilai yang Diperhitungkan untuk PPKA (rupiah penuh). Isian bank; TIDAK diturunkan dari taksasi/bound, karena kebijakan PPKA per agunan adalah keputusan bank dan form tidak memberi rumus pengikat.';
