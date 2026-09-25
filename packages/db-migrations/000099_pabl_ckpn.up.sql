-- 000099_pabl_ckpn.up.sql
-- Run after: 000098_consolidate_collateral_cost_columns.up.sql
--
-- CKPN per penempatan pada bank lain (PABL) untuk Form 05.00 kolom XII (CKPN) dan
-- XXI (Jenis CKPN). lps_placements (migrasi 000045) hanya menyimpan penempatan dan
-- bagian yang dijamin LPS, tanpa cadangan per penempatan; migrasi ini menambahkan
-- asesmen eksplisit sehingga bank memutuskan sendiri metodenya.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. Kolom hanya ditambahkan bila belum ada,
-- tabel/index memakai IF NOT EXISTS, dan seed konfigurasi tidak menimpa nilai bank
-- (INSERT ... ON CONFLICT (key) DO NOTHING).
--
-- Saklar ckpn.pabl.enabled bawaan 'false': selama mati kolom XII/XXI Form 05.00 tetap
-- dinyatakan belum tersedia dan tidak ada perilaku yang berubah.

ALTER TABLE lps_placements
    ADD COLUMN IF NOT EXISTS ckpn_method TEXT NOT NULL DEFAULT 'COLLECTIVE',
    ADD COLUMN IF NOT EXISTS ckpn_significant BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ckpn_objective_evidence BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS required_ckpn NUMERIC(28,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ckpn_individual_target NUMERIC(28,4),
    ADD COLUMN IF NOT EXISTS ckpn_assessed_at TIMESTAMPTZ;

COMMENT ON COLUMN lps_placements.ckpn_method IS
    'Metode CKPN per penempatan: COLLECTIVE (bawaan), INDIVIDUAL_DCF, INDIVIDUAL_COLLATERAL, INDIVIDUAL_MAX, EXCLUDED_ASET_BAIK. Menentukan sandi kolom XXI Form 05.00.';
COMMENT ON COLUMN lps_placements.ckpn_significant IS
    'Penanda signifikansi asesmen CKPN penempatan; data operasional bank.';
COMMENT ON COLUMN lps_placements.ckpn_objective_evidence IS
    'Penanda bukti objektif penurunan nilai asesmen CKPN penempatan; data operasional bank.';
COMMENT ON COLUMN lps_placements.required_ckpn IS
    'Target CKPN berlaku satu penempatan (kolom XII Form 05.00). Bawaan 0 sampai bank mengasesmen.';
COMMENT ON COLUMN lps_placements.ckpn_individual_target IS
    'Target perhitungan individual terakhir; hanya bermakna untuk metode INDIVIDUAL_*.';
COMMENT ON COLUMN lps_placements.ckpn_assessed_at IS
    'Waktu asesmen CKPN terakhir. NULL = penempatan belum pernah diasesmen; Form 05.00 kolom XII/XXI belum tersedia.';

-- Jejak audit setiap asesmen; tabel menjumlahkan riwayat, bukan mengganti.
CREATE TABLE IF NOT EXISTS pabl_ckpn_assessments (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    placement_id       UUID NOT NULL REFERENCES lps_placements(id) ON DELETE CASCADE,
    as_of              DATE NOT NULL,
    method             TEXT NOT NULL,
    significant        BOOLEAN NOT NULL DEFAULT FALSE,
    objective_evidence BOOLEAN NOT NULL DEFAULT FALSE,
    carrying           NUMERIC(28,4) NOT NULL,
    target             NUMERIC(28,4) NOT NULL,
    basis              JSONB,
    decided_by         TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pabl_ckpn_assessments_placement
    ON pabl_ckpn_assessments (placement_id);

COMMENT ON TABLE pabl_ckpn_assessments IS
    'Jejak audit asesmen CKPN per penempatan pada bank lain (kolom XII/XXI Form 05.00). carrying = outstanding saat asesmen; target = CKPN yang ditetapkan.';

-- Konfigurasi. Bawaan 'false' agar perilaku default tidak berubah: Form 05.00 tetap
-- menyatakan kolom XII/XXI belum tersedia sampai bank mengasesmen minimal satu penempatan.
INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.pabl.enabled', 'false',
     'Saklar asesmen CKPN per penempatan pada bank lain (Form 05.00 kolom XII/XXI). Bawaan false = perilaku sekarang dan kolom tetap belum tersedia; menyimpannya adalah langkah bank, bukan bawaan rilis.')
ON CONFLICT (key) DO NOTHING;
