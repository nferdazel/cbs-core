-- CBS Migration 000036: modul agunan — tabel loan_collaterals
-- Run after: 000035_journal_source.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Mengapa modul ini ada: migrasi 000014 dan 000026 sudah mencatat bahwa PPAP dihitung
-- atas pokok penuh karena bank belum punya modul agunan. Akibatnya penyisihan lebih besar
-- dari kewajiban untuk kredit beragunan: modal tertahan, bukan sekadar angka laporan.
--
-- SATU HAL YANG SENGAJA BELUM DIPUTUSKAN DI SINI: pasal POJK yang mengatur agunan
-- pengurang. Catatan lama proyek ini menyebut Pasal 20, komentar migrasi 000026 menyebut
-- Pasal 17 — salah satu salah, dan tidak boleh ada pasal karangan di migrasi. Karena itu
-- migrasi ini hanya menyediakan datanya, dan pengurangan PPAP tetap MATI
-- (ppap.collateral.enabled = false) sampai teks POJK No. 1 Tahun 2024 diverifikasi.

DO $$ BEGIN
    CREATE TYPE collateral_type AS ENUM (
        'TANAH_BANGUNAN',
        'KENDARAAN',
        'DEPOSIT',
        'MESIN_PERALATAN',
        'LAINNYA'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE collateral_status AS ENUM ('ACTIVE', 'RELEASED', 'EXECUTED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS loan_collaterals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    loan_id UUID NOT NULL REFERENCES loans(id) ON DELETE RESTRICT,
    branch_id UUID REFERENCES branches(id) ON DELETE SET NULL,
    collateral_type collateral_type NOT NULL,

    -- Identitas dan ikatan hukum. Nomor bukti ikatan wajib: agunan yang tidak terikat
    -- secara hukum tidak boleh dihitung sebagai pengurang, sekaya apa pun taksasinya.
    description TEXT NOT NULL,
    document_number VARCHAR(64) NOT NULL,
    owner_name VARCHAR(128) NOT NULL,

    -- Penilaian. Nilai taksasi harus positif; tanggal taksasi divalidasi di domain karena
    -- CHECK tidak boleh memakai ekspresi yang bergantung waktu.
    appraisal_value NUMERIC(18,2) NOT NULL CHECK (appraisal_value > 0),
    appraisal_date DATE NOT NULL,
    appraiser VARCHAR(128),

    -- haircut_percent adalah bagian nilai taksasi yang TIDAK dihitung sebagai pengurang.
    -- 100 berarti agunan tidak mengurangi eksposur sama sekali. Bawaan 100 dipilih supaya
    -- rilis tidak menggeser angka PPAP diam-diam dan bank tidak dipaksa memakai kebijakan
    -- yang bukan miliknya; operator menurunkannya sesuai kebijakan internal, dan setiap
    -- perubahan tercatat di audit log.
    haircut_percent NUMERIC(5,2) NOT NULL DEFAULT 100
        CHECK (haircut_percent >= 0 AND haircut_percent <= 100),

    -- bound_amount dihitung database, bukan aplikasi: nilai pengurang yang tersimpan harus
    -- selalu konsisten dengan taksasi dan haircut saat itu, sehingga tetap dapat diaudit
    -- walau kebijakan berubah di kemudian hari.
    bound_amount NUMERIC(18,2) GENERATED ALWAYS AS
        (ROUND(appraisal_value * (1 - haircut_percent / 100), 2)) STORED,

    status collateral_status NOT NULL DEFAULT 'ACTIVE',
    released_at TIMESTAMPTZ,
    notes TEXT,

    created_by VARCHAR(64) NOT NULL DEFAULT 'SYSTEM',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(64),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_collateral_loan ON loan_collaterals(loan_id);
CREATE INDEX IF NOT EXISTS idx_collateral_active ON loan_collaterals(loan_id) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_collateral_branch ON loan_collaterals(branch_id);
CREATE INDEX IF NOT EXISTS idx_collateral_document ON loan_collaterals(document_number);

COMMENT ON COLUMN loan_collaterals.haircut_percent IS
    'Bagian nilai taksasi yang tidak dihitung sebagai pengurang (100 = agunan tidak mengurangi eksposur).';
COMMENT ON COLUMN loan_collaterals.bound_amount IS
    'Nilai pengurang = appraisal_value x (1 - haircut_percent/100); dihitung database.';

-- Kebijakan haircut per jenis agunan. Bawaan 100 = tanpa pengurangan, sengaja: PPAP
-- berhenti dihitung atas pokok penuh sampai bank mengisi kebijakannya sendiri.
INSERT INTO system_config (key, value, description) VALUES
    ('collateral.haircut.tanah_bangunan', '100', 'Bagian nilai taksasi agunan tanah dan bangunan yang tidak dihitung sebagai pengurang PPAP (100 = tanpa pengurangan).'),
    ('collateral.haircut.kendaraan',      '100', 'Bagian nilai taksasi agunan kendaraan yang tidak dihitung sebagai pengurang PPAP (100 = tanpa pengurangan).'),
    ('collateral.haircut.deposit',        '100', 'Bagian nilai taksasi agunan deposito yang tidak dihitung sebagai pengurang PPAP (100 = tanpa pengurangan).'),
    ('collateral.haircut.mesin_peralatan','100', 'Bagian nilai taksasi agunan mesin/peralatan yang tidak dihitung sebagai pengurang PPAP (100 = tanpa pengurangan).'),
    ('collateral.haircut.lainnya',        '100', 'Bagian nilai taksasi agunan jenis lain yang tidak dihitung sebagai pengurang PPAP (100 = tanpa pengurangan).')
ON CONFLICT (key) DO NOTHING;

-- Saklar utama pengurangan agunan pada perhitungan PPAP. Selama false, agunan boleh
-- dicatat lengkap tetapi tidak mengurangi eksposur: perilaku PPAP identik dengan hari ini.
INSERT INTO system_config (key, value, description) VALUES
    ('ppap.collateral.enabled', 'false', 'Aktifkan pengurangan nilai agunan pada perhitungan PPAP. false = PPAP atas pokok penuh (perilaku sebelum modul agunan).')
ON CONFLICT (key) DO NOTHING;
