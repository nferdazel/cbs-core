-- CBS Migration 000015: Penanda idempotensi batch akrual bunga, biaya admin, dan tutup buku
-- Run after: 000014 (pekerjaan lain).
--
-- Latar belakang:
--   EOM akrual bunga/ bagi hasil dan pemotongan biaya administrasi harus idempotent per
--   periode: menjalankan ulang batch pada bulan yang sama tidak boleh memposting jurnal
--   dua kali. EOY tutup buku juga tidak boleh dijalankan dua kali untuk satu tahun buku.
--   Tabel di bawah menjadi penanda sekaligus jejak jumlah yang benar-benar diposting.
--   Unique key-nya yang menegakkan idempotensi, bukan pengecekan aplikasi.

-- Akrual bunga tabungan konvensional / bagi hasil mudharabah per rekening per bulan.
CREATE TABLE IF NOT EXISTS interest_accruals (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id        UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    account_number    VARCHAR(32) NOT NULL,
    period            VARCHAR(7) NOT NULL,          -- YYYY-MM
    book              coa_book NOT NULL,
    profit_scheme     VARCHAR(32) NOT NULL,
    amount            NUMERIC(28,4) NOT NULL,
    average_balance   NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    expense_coa       VARCHAR(32) NOT NULL,
    payable_coa       VARCHAR(32) NOT NULL,
    journal_entry_id  UUID REFERENCES journal_entries(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, period)
);

CREATE INDEX IF NOT EXISTS idx_interest_accruals_period ON interest_accruals(period);

-- Pemotongan biaya administrasi bulanan per rekening.
CREATE TABLE IF NOT EXISTS admin_fee_charges (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id        UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    account_number    VARCHAR(32) NOT NULL,
    period            VARCHAR(7) NOT NULL,          -- YYYY-MM
    amount            NUMERIC(28,4) NOT NULL,
    revenue_coa       VARCHAR(32) NOT NULL,
    journal_entry_id  UUID REFERENCES journal_entries(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, period)
);

CREATE INDEX IF NOT EXISTS idx_admin_fee_charges_period ON admin_fee_charges(period);

-- Jurnal penutup tahun buku per buku (konvensional dan syariah terpisah).
CREATE TABLE IF NOT EXISTS year_end_closings (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    fiscal_year           INT NOT NULL,
    book                  coa_book NOT NULL,
    total_revenue         NUMERIC(28,4) NOT NULL,
    total_expense         NUMERIC(28,4) NOT NULL,
    net_income            NUMERIC(28,4) NOT NULL,
    retained_earnings_coa VARCHAR(32) NOT NULL,
    journal_entry_id      UUID REFERENCES journal_entries(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (fiscal_year, book)
);
