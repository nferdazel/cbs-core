-- CBS Migration 000013: Deposito Berjangka
-- Tabel kontrak deposito, plus koreksi pemetaan jurnal TIME_DEPOSIT yang belum
-- lengkap (pembayaran bunga dan akrual bagi hasil syariah).
-- Run after: 000012_app_role_grants.up.sql
-- Idempotent: aman dijalankan ulang.

-- ── Status kontrak deposito ─────────────────────────────────────────────────
DO $$ BEGIN
    CREATE TYPE deposit_status AS ENUM ('PLACED', 'MATURED', 'CLOSED', 'BROKEN');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ── Kontrak deposito ────────────────────────────────────────────────────────
-- Satu baris = satu kontrak deposito. account_number menunjuk rekening deposito
-- nasabah (accounts). Saldo jurnal kewajiban tetap berada di akun kontrol GL
-- (20200/12300) karena posting memakai pemetaan produk; kolom di sini adalah
-- catatan kontrak per bilyet (pokok, akrual, pajak, jatuh tempo).
CREATE TABLE IF NOT EXISTS deposits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_number VARCHAR(32) NOT NULL REFERENCES accounts(account_number) ON DELETE RESTRICT,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES banking_products(id) ON DELETE RESTRICT,
    branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
    placement_amount NUMERIC(28,4) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'IDR',
    term_months INT NOT NULL,
    start_date DATE NOT NULL,
    maturity_date DATE NOT NULL,
    -- profit_rate: bunga tahunan (%) untuk INTEREST, atau nisbah (0-1) untuk
    -- MUDHARABAH/MUSYARAKAH.
    profit_rate NUMERIC(8,4) NOT NULL DEFAULT 0.0000,
    -- yield_rate: proyeksi imbal hasil tahunan (%) sebagai dasar bagi hasil syariah.
    -- 0 untuk produk konvensional.
    yield_rate NUMERIC(8,4) NOT NULL DEFAULT 0.0000,
    profit_type VARCHAR(16) NOT NULL DEFAULT 'INTEREST',
    -- tax_rate: PPh final dalam persen (mis. 20). 0 = tidak dipotong (syariah).
    tax_rate NUMERIC(8,4) NOT NULL DEFAULT 0.0000,
    aro BOOLEAN NOT NULL DEFAULT FALSE,
    aro_instruction VARCHAR(24) NOT NULL DEFAULT 'NONE',
    status deposit_status NOT NULL DEFAULT 'PLACED',
    accrued_profit NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    accrued_tax NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    paid_profit NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    paid_tax NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    maturity_proceeds NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    -- last_accrual_date membuat akrual idempotent per hari.
    last_accrual_date DATE,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_deposit_placement_positive CHECK (placement_amount > 0),
    CONSTRAINT chk_deposit_term_positive CHECK (term_months > 0),
    CONSTRAINT chk_deposit_profit_type CHECK (profit_type IN ('INTEREST', 'MARGIN', 'BAGI_HASIL')),
    CONSTRAINT chk_deposit_aro_instruction CHECK (aro_instruction IN ('NONE', 'PRINCIPAL', 'PRINCIPAL_AND_PROFIT')),
    CONSTRAINT chk_deposit_maturity_after_start CHECK (maturity_date >= start_date)
);

CREATE INDEX IF NOT EXISTS idx_deposits_account ON deposits(account_number);
CREATE INDEX IF NOT EXISTS idx_deposits_customer ON deposits(customer_id);
CREATE INDEX IF NOT EXISTS idx_deposits_product ON deposits(product_id);
CREATE INDEX IF NOT EXISTS idx_deposits_status ON deposits(status);
CREATE INDEX IF NOT EXISTS idx_deposits_maturity ON deposits(maturity_date);
-- Pemindaian ARO: deposito yang sudah jatuh tempo dan ber-ARO.
CREATE INDEX IF NOT EXISTS idx_deposits_aro_maturity ON deposits(aro, maturity_date) WHERE aro = TRUE;

-- ── Pemetaan jurnal yang belum ada di seed 000005 ───────────────────────────
-- 000005 hanya memetakan DEPOSIT/WITHDRAWAL/INTEREST_ACCRUAL untuk deposito
-- konvensional dan DEPOSIT/WITHDRAWAL/INTEREST_PAYMENT untuk deposito syariah.
-- Pencairan bunga konvensional dan akrual bagi hasil syariah belum terpetakan.
-- Kode COA di bawah diambil dari bagan akun 000005, bukan ditebak:
--   20400 Bunga Deposito yang Masih Harus Dibayar (konvensional)
--   20500 Utang Pajak
--   10101 Kas Teller
--   12400 Bagi Hasil yang Masih Harus Dibayar (syariah)
--   15100 Bagi Hasil untuk Pemilik Dana (syariah)
--   11100 Kas Syariah
-- 12300 Deposito Mudharabah (dipakai pada perbaikan INTEREST_PAYMENT syariah)

-- Pencairan bunga konvensional: kurangi utang bunga, bayar tunai.
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source) VALUES
    ('e0000000-0000-0000-0000-000000000003', 'INTEREST_PAYMENT', 'DEBIT', '20400', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000003', 'INTEREST_PAYMENT', 'CREDIT', '10101', 'PROFIT')
ON CONFLICT (product_id, event, direction, coa_code) DO NOTHING;

-- Pajak PPh final deposito konvensional: kurangi utang bunga, akui utang pajak.
-- (Sudah ada di 000005, ditulis ulang agar migrasi ini berdiri sendiri.)
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source) VALUES
    ('e0000000-0000-0000-0000-000000000003', 'TAX_WITHHOLDING', 'DEBIT', '20400', 'TAX'),
    ('e0000000-0000-0000-0000-000000000003', 'TAX_WITHHOLDING', 'CREDIT', '20500', 'TAX')
ON CONFLICT (product_id, event, direction, coa_code) DO NOTHING;

-- Akrual bagi hasil syariah: beban bagi hasil -> kewajiban bagi hasil.
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source) VALUES
    ('e0000000-0000-0000-0000-000000000004', 'INTEREST_ACCRUAL', 'DEBIT', '15100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000004', 'INTEREST_ACCRUAL', 'CREDIT', '12400', 'PROFIT')
ON CONFLICT (product_id, event, direction, coa_code) DO NOTHING;

-- Perbaikan pencairan bagi hasil syariah. Seed 000005 memetakan INTEREST_PAYMENT
-- langsung dari beban (15100) ke pokok deposito (12300), sehingga akrual ke 12400
-- tidak akan pernah bersaldo nol. Diganti: kewajiban bagi hasil -> kas syariah.
DELETE FROM product_journal_mapping
WHERE product_id = 'e0000000-0000-0000-0000-000000000004'
  AND event = 'INTEREST_PAYMENT';

INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source) VALUES
    ('e0000000-0000-0000-0000-000000000004', 'INTEREST_PAYMENT', 'DEBIT', '12400', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000004', 'INTEREST_PAYMENT', 'CREDIT', '11100', 'PROFIT')
ON CONFLICT (product_id, event, direction, coa_code) DO NOTHING;

-- ── Hak akses aplikasi ──────────────────────────────────────────────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON deposits TO cbs_app;
