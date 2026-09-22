-- CBS Migration 000087: persiapan data agunan untuk bobot risiko Lampiran II
-- SEOJK No. 2/SEOJK.03/2025 (KPMM BPR).
-- Run after: 000085 (nomor 000086 dipesan alur lain; migrasi ini tidak bergantung isinya).
--
-- MENGAPA MIGRASI INI ADA, DAN MENGAPA BOBOTNYA TIDAK DIPASANG:
-- Lampiran II SEOJK No. 2/SEOJK.03/2025 membedakan bobot risiko kredit menurut
-- agunan (mis. butir 8 emas perhiasan 15%; butir 12 tanah/bangunan ber-hak
-- tanggungan/fidusia 30%; butir 17 tanah/bangunan bersertipikat tanpa beban 50%;
-- butir 18 UMK 70%; butir 19 kendaraan ber-hipotek/fidusia 70%; butir 21 lainnya
-- 100%; butir 22 jatuh tempo/macet 100%). Perhitungan ATMR sekarang memakai SATU
-- bobot kredit agregat 100% (kpmm.rwa_frac.kredit, migrasi 000082) karena data
-- agunan belum layak dipetakan per butir.
--
-- Memetakan data yang belum layak akan menurunkan ATMR dan MENAIKKAN rasio KPMM
-- tampak, padahal kenyataannya tidak: itu pelanggaran, bukan konservatif. Karena
-- itu migrasi ini HANYA menyiapkan tempat datanya. Berkas kode tidak membaca tabel
-- bobot di bawah sama sekali; bobot `applied_weight_frac` untuk SEMUA baris tetap
-- 1.00000 (=100%) dan `enabled` tetap FALSE, sehingga angka ATMR/KPMM tidak bergeser
-- sedikit pun. Pengaktifan kelak cukup mengubah data tabel ini, bukan kode.
--
-- Informasi Lampiran II yang sebelumnya tidak dapat dimuat:
--   * ikatan hukum agunan (hak tanggungan/fidusia/hipotek vs tanpa beban) —
--     pemisah butir 12/19 (berikatan) dari butir 17/21 (tanpa beban);
--   * penanda agunan terbukti sengketa/kepemilikan ganda — angka 8 perhitungan ATMR
--     SEOJK 2/2025 menetapkan bagian kredit itu 100%;
--   * masa berlaku asuransi (butir 15 asuransi kredit; butir 16 asuransi jiwa);
--   * masa berlaku taksasi terakhir. Tanggal taksasi terakhir sudah ada
--     (loan_collaterals.appraisal_date); yang ditambahkan adalah batas berlakunya.
--     Teks SEOJK yang dibaca TIDAK mengatur umur taksasi, jadi kolomnya dibiarkan
--     kosong sampai bank menetapkan kebijakannya sendiri.
--
-- Semua penambahan bersifat aditif: kolom baru boleh NULL atau berdefault aman,
-- sehingga baris lama tetap valid tanpa backfill. Tidak ada nilai lama yang diubah.
--
-- Idempotent: aman dijalankan ulang (CREATE TYPE/ADD COLUMN/CREATE TABLE IF NOT
-- EXISTS, INSERT ... ON CONFLICT DO NOTHING).

-- 1. Jenis ikatan hukum agunan. Pemisah utama bobot Lampiran II: agunan tanah/
--    bangunan yang dibebani hak tanggungan/fidusia (butir 12, 30%) berbeda dari
--    yang bersertipikat tanpa beban (butir 17, 50%), dan kendaraan yang terikat
--    hipotek/fidusia (butir 19, 70%) berbeda dari yang tanpa ikatan (butir 21, 100%).
DO $$ BEGIN
    CREATE TYPE collateral_binding AS ENUM (
        'HAK_TANGGUNGAN',
        'FIDUSIA',
        'HIPOTEK',
        'TANPA_BEBAN',
        'LAINNYA'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE loan_collaterals
    -- Ikatan hukum. NULL = belum diisi operator; TIDAK ditafsirkan sebagai
    -- "tanpa beban", karena menebak ikatan menggeser bobot risiko.
    ADD COLUMN IF NOT EXISTS binding_type collateral_binding,
    -- Butir 8 perhitungan ATMR: agunan yang terbukti sengketa/kepemilikan ganda
    -- membuat bagian kredit berbobot 100%. Bawaan FALSE = belum ditandai sengketa;
    -- bukan pernyataan bahwa agunan pasti bersih.
    ADD COLUMN IF NOT EXISTS disputed BOOLEAN NOT NULL DEFAULT FALSE,
    -- Bukti/keterangan sengketa agar penanda dapat diaudit; boleh kosong.
    ADD COLUMN IF NOT EXISTS dispute_evidence TEXT,
    -- Masa berlaku asuransi (butir 15 asuransi kredit / butir 16 asuransi jiwa).
    -- NULL = belum diisi; tanggal yang sudah lewat berarti polis kedaluwarsa, dan
    -- itu fakta yang sah, bukan masukan tidak masuk akal.
    ADD COLUMN IF NOT EXISTS insurance_expiry_date DATE,
    -- Nomor polis untuk menelusuri asuransi; boleh kosong.
    ADD COLUMN IF NOT EXISTS insurance_policy_number VARCHAR(64),
    -- Masa berlaku taksasi terakhir. appraisal_date sudah mencatat tanggal taksasi;
    -- kolom ini mencatat sampai kapan taksasi itu berlaku menurut kebijakan bank.
    -- NULL = kebijakan umur taksasi bank belum diisi (teks SEOJK tidak mengaturnya).
    ADD COLUMN IF NOT EXISTS appraisal_valid_until DATE;

COMMENT ON COLUMN loan_collaterals.binding_type IS
    'Ikatan hukum agunan; pemisah bobot Lampiran II butir 12/19 (berikatan) dari butir 17/21 (tanpa beban). NULL berarti belum diisi, bukan tanpa beban.';
COMMENT ON COLUMN loan_collaterals.disputed IS
    'Agunan terbukti sengketa/kepemilikan ganda; bagian kredit berbobot 100% menurut butir 8 perhitungan ATMR SEOJK 2/2025. Bawaan FALSE = belum ditandai.';
COMMENT ON COLUMN loan_collaterals.dispute_evidence IS
    'Bukti/keterangan sengketa atau kepemilikan ganda; boleh kosong, hanya untuk auditabilitas.';
COMMENT ON COLUMN loan_collaterals.insurance_expiry_date IS
    'Tanggal berakhir asuransi agunan; sumber bobot Lampiran II butir 15/16. NULL berarti belum diisi.';
COMMENT ON COLUMN loan_collaterals.insurance_policy_number IS
    'Nomor polis asuransi agunan; pendukung penelusuran, boleh kosong.';
COMMENT ON COLUMN loan_collaterals.appraisal_valid_until IS
    'Batas berlaku taksasi terakhir menurut kebijakan bank; teks SEOJK tidak mengatur umur taksasi sehingga boleh kosong sampai bank menetapkannya.';

-- Indeks untuk melihat agunan yang masa berlaku asuransi/taksasinya sudah lewat.
CREATE INDEX IF NOT EXISTS idx_collateral_insurance_expiry
    ON loan_collaterals (insurance_expiry_date) WHERE insurance_expiry_date IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_collateral_appraisal_valid
    ON loan_collaterals (appraisal_valid_until) WHERE appraisal_valid_until IS NOT NULL;

-- 2. Pemetaan kategori agunan -> butir Lampiran II beserta bobotnya.
--    Tabel ini SENGAJA tidak dibaca berkas kode mana pun pada fase persiapan:
--    `applied_weight_frac` = 1.00000 dan `enabled` = FALSE untuk semua baris, jadi
--    ATMR tetap memakai 100%. Angka resmi Lampiran II disimpan terpisah pada
--    `official_weight_frac` supaya pengisian bobot nanti cukup menyalin nilai resmi
--    ke `applied_weight_frac` dan menyalakan `enabled`, tanpa menebak butirnya.
CREATE TABLE IF NOT EXISTS collateral_lampiran_ii_weights (
    category_code TEXT PRIMARY KEY,
    lampiran_ii_item SMALLINT NOT NULL,
    label TEXT NOT NULL,
    -- Petunjuk jenis/ikatan agunan yang masuk kategori ini; NULL berarti kategori
    -- tidak ditentukan oleh jenis agunan (mis. jatuh tempo/macet atau UMK).
    collateral_type collateral_type,
    binding_type collateral_binding,
    -- Angka resmi Lampiran II SEOJK 2/2025 (fraksi 0..1). Dokumentasi saat angka itu
    -- belum boleh dipakai; JANGAN dianggap bobot aktif.
    official_weight_frac NUMERIC(6,5) NOT NULL CHECK (official_weight_frac >= 0),
    -- Bobot yang dipakai sistem. Fase persiapan: SELALU 1.00000 (100%).
    applied_weight_frac NUMERIC(6,5) NOT NULL DEFAULT 1.00000
        CHECK (applied_weight_frac >= 0),
    -- Saklar aktivasi. Fase persiapan: SELALU FALSE.
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    notes TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE collateral_lampiran_ii_weights IS
    'Pemetaan kategori agunan ke butir Lampiran II SEOJK 2/2025. Fase persiapan: applied_weight_frac 1.00000 dan enabled FALSE untuk semua baris, sehingga ATMR tidak berubah. Tidak dibaca kode sampai bobot diaktifkan.';
COMMENT ON COLUMN collateral_lampiran_ii_weights.official_weight_frac IS
    'Angka bobot resmi Lampiran II (fraksi 0..1), sumber untuk pengisian nanti; bukan bobot yang dipakai sekarang.';
COMMENT ON COLUMN collateral_lampiran_ii_weights.applied_weight_frac IS
    'Bobot yang dipakai sistem; pada fase persiapan selalu 1.00000 (100%).';
COMMENT ON COLUMN collateral_lampiran_ii_weights.enabled IS
    'Saklar aktivasi per kategori; pada fase persiapan selalu FALSE.';

CREATE INDEX IF NOT EXISTS idx_lampiran_ii_weights_type
    ON collateral_lampiran_ii_weights (collateral_type);

INSERT INTO collateral_lampiran_ii_weights
    (category_code, lampiran_ii_item, label, collateral_type, binding_type,
     official_weight_frac, applied_weight_frac, enabled, notes)
VALUES
    ('EMAS_PERHIASAN', 8, 'Kredit dengan agunan emas perhiasan',
     'EMAS_PERHIASAN', NULL, 0.15000, 1.00000, FALSE,
     'Butir 8 Lampiran II: emas perhiasan 15%. Aktifkan hanya bila nilai emas dapat dinilai andal.'),
    ('TANAH_BANGUNAN_HAK_TANGGUNGAN', 12, 'Tanah/bangunan bersertifikat ber-hak tanggungan',
     'TANAH_BANGUNAN', 'HAK_TANGGUNGAN', 0.30000, 1.00000, FALSE,
     'Butir 12 Lampiran II: tanah/bangunan bersertifikat yang dibebani hak tanggungan 30%.'),
    ('TANAH_BANGUNAN_FIDUSIA', 12, 'Tanah/bangunan bersertifikat ber-fidusia',
     'TANAH_BANGUNAN', 'FIDUSIA', 0.30000, 1.00000, FALSE,
     'Butir 12 Lampiran II: tanah/bangunan bersertifikat yang dibebani fidusia 30%.'),
    ('TANAH_BANGUNAN_TANPA_BEBAN', 17, 'Tanah/bangunan bersertipikat tanpa beban',
     'TANAH_BANGUNAN', 'TANPA_BEBAN', 0.50000, 1.00000, FALSE,
     'Butir 17 Lampiran II: tanah/bangunan bersertipikat tanpa hak tanggungan/fidusia 50%.'),
    ('KENDARAAN_HIPOTEK', 19, 'Kendaraan/kapal/alat berat/mesin ber-hipotek',
     'KENDARAAN', 'HIPOTEK', 0.70000, 1.00000, FALSE,
     'Butir 19 Lampiran II: kendaraan bermotor/kapal/alat berat/mesin dengan bukti kepemilikan dan pengikatan hipotek 70%.'),
    ('KENDARAAN_FIDUSIA', 19, 'Kendaraan/kapal/alat berat/mesin ber-fidusia',
     'KENDARAAN', 'FIDUSIA', 0.70000, 1.00000, FALSE,
     'Butir 19 Lampiran II: kendaraan bermotor/kapal/alat berat/mesin dengan bukti kepemilikan dan pengikatan fidusia 70%.'),
    ('UMK', 18, 'Kredit usaha mikro dan kecil yang memenuhi seluruh kriteria',
     NULL, NULL, 0.70000, 1.00000, FALSE,
     'Butir 18 Lampiran II: UMK (plafon paling tinggi Rp500 juta dan tidak memenuhi kriteria agunan tanah/bangunan) 70%. Tidak ditentukan jenis agunan.'),
    ('LAINNYA', 21, 'Tagihan/kredit lain yang tidak memenuhi kriteria di atas',
     NULL, NULL, 1.00000, 1.00000, FALSE,
     'Butir 21 Lampiran II: tagihan/kredit lain 100%. Kategori jatuh (fallback) untuk jenis agunan yang belum dipetakan, termasuk deposito/resi gudang/tanah adat/tempat usaha/jaminan BUMN-BUMD sampai butirnya ditetapkan.'),
    ('JATUH_TEMPO_MACET', 22, 'Tagihan/kredit jatuh tempo atau kualitas macet',
     NULL, NULL, 1.00000, 1.00000, FALSE,
     'Butir 22 Lampiran II: tagihan/kredit jatuh tempo atau macet 100%, berlaku sebagai pengganti kategori mana pun berdasarkan kualitas kolektibilitas, bukan jenis agunan.')
ON CONFLICT (category_code) DO NOTHING;
