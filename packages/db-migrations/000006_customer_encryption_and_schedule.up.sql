-- CBS Migration 000006: Enkripsi data pribadi & penyelarasan jadwal angsuran syariah
-- Run after: 000005_bpr_foundation.up.sql
--
-- Data produksi belum berisi nasabah/kredit, sehingga kolom sensitif langsung
-- dipindahkan ke bentuk terenkripsi tanpa backfill.

-- ── Customers: kolom terenkripsi + blind index ──────────────────────────────
-- NIK dan email tetap punya kolom unik untuk pencarian cepat, tetapi nilainya adalah
-- blind index (HMAC), bukan nilai asli. Nilai asli disimpan terenkripsi di kolom _enc.
ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS full_name_enc TEXT,
    ADD COLUMN IF NOT EXISTS id_card_number_enc TEXT,
    ADD COLUMN IF NOT EXISTS email_enc TEXT,
    ADD COLUMN IF NOT EXISTS phone_number_enc TEXT,
    ADD COLUMN IF NOT EXISTS address_enc TEXT,
    ADD COLUMN IF NOT EXISTS id_card_index TEXT,
    ADD COLUMN IF NOT EXISTS email_index TEXT;

-- Kolom lama tidak dipakai lagi untuk data pribadi. Dibuat nullable agar tidak
-- menghalangi insert baru, dan tetap ada untuk kompatibilitas baca sementara.
ALTER TABLE customers ALTER COLUMN full_name DROP NOT NULL;
ALTER TABLE customers ALTER COLUMN id_card_number DROP NOT NULL;
ALTER TABLE customers ALTER COLUMN email DROP NOT NULL;
ALTER TABLE customers ALTER COLUMN phone_number DROP NOT NULL;

-- Indeks pencarian berbasis blind index.
CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_id_card_index ON customers(id_card_index) WHERE id_card_index IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_email_index ON customers(email_index) WHERE email_index IS NOT NULL;

-- ── Loan schedules: netral terhadap konvensional & syariah ──────────────────
-- "interest_amount" hanya cocok untuk konvensional. Untuk murabahah itu margin, untuk
-- mudharabah itu bagi hasil. Satu nama netral: profit_amount.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='loan_schedules' AND column_name='interest_amount') THEN
        ALTER TABLE loan_schedules RENAME COLUMN interest_amount TO profit_amount;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='loan_schedules' AND column_name='paid_interest') THEN
        ALTER TABLE loan_schedules RENAME COLUMN paid_interest TO paid_profit;
    END IF;
END $$;

-- Jenis imbal hasil pada jadwal, mengikuti produk induknya.
ALTER TABLE loan_schedules ADD COLUMN IF NOT EXISTS profit_type VARCHAR(32) NOT NULL DEFAULT 'INTEREST';
ALTER TABLE loan_schedules ADD COLUMN IF NOT EXISTS outstanding_principal NUMERIC(28,4) NOT NULL DEFAULT 0.0000;

-- ── Loans: kolom akad syariah & produk ──────────────────────────────────────
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS acquisition_cost NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS deferred_margin NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS profit_sharing_ratio NUMERIC(8,4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS akad_number VARCHAR(64),
    ADD COLUMN IF NOT EXISTS akad_date TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS purpose TEXT;

-- Produk lama (000003 era) tidak punya product_id; produk default diisi saat service
-- membuat kredit baru. Kolom di bawah memudahkan pemetaan tipe lama bila ada.
CREATE INDEX IF NOT EXISTS idx_loan_schedules_due_status ON loan_schedules(due_date, status);
