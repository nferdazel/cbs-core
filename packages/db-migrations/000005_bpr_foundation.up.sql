-- CBS Migration 000005: Fondasi BPR
-- Bagan akun baku, cabang, produk bank, pemetaan jurnal produk, dan skema nomor rekening.
-- Run after: 000004_loan_ojk_fields_and_coa.up.sql

-- ── Cabang ──────────────────────────────────────────────────────────────────
CREATE TABLE branches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(8) NOT NULL UNIQUE,          -- 3 digit, mis. 001 = Kantor Pusat
    name VARCHAR(128) NOT NULL,
    address TEXT,
    phone VARCHAR(32),
    is_head_office BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO branches (id, code, name, is_head_office) VALUES
    ('d0000000-0000-0000-0000-000000000001', '001', 'Kantor Pusat', TRUE);

-- ── Bagan akun: perbaikan struktur ──────────────────────────────────────────
-- COA perlu membedakan konvensional dan syariah karena jurnalnya berbeda
-- (bunga vs margin/bagi hasil) dan laporan harus dapat dipisah per UUS.
CREATE TYPE coa_book AS ENUM ('CONVENTIONAL', 'SYARIAH');

ALTER TABLE chart_of_accounts
    ADD COLUMN IF NOT EXISTS book coa_book NOT NULL DEFAULT 'CONVENTIONAL',
    ADD COLUMN IF NOT EXISTS parent_code VARCHAR(32),
    ADD COLUMN IF NOT EXISTS is_header BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS description TEXT;

-- Akun header tidak boleh diposting; hanya akun daun. Constraint ditegakkan di aplikasi
-- lewat product mapping, karena Postgres tidak bisa menyatakan FK ke diri sendiri secara
-- kondisional untuk aturan bisnis ini.

-- ── Produk bank ─────────────────────────────────────────────────────────────
-- Definisi produk menggantikan hardcode rate/limit/COA di kode.
CREATE TYPE product_family AS ENUM (
    'SAVINGS',            -- Tabungan
    'TIME_DEPOSIT',       -- Deposito berjangka
    'LOAN',               -- Kredit (konvensional) / Pembiayaan (syariah)
    'CURRENT_ACCOUNT'     -- Giro (bila BPR berizin)
);

CREATE TYPE profit_scheme AS ENUM (
    'INTEREST',           -- Bunga (konvensional)
    'MURABAHAH',          -- Jual beli, margin tetap
    'MUDHARABAH',         -- Bagi hasil, nisbah
    'MUSYARAKAH',         -- Bagi hasil, nisbah, modal bersama
    'IJARAH',             -- Sewa manfaat
    'WADIAH'              -- Titipan, tanpa imbalan atau bonus
);

CREATE TYPE schedule_method AS ENUM (
    'FLAT',               -- Bunga/margin proporsional tetap
    'ANNUITY',            -- Anuitas
    'SLIDING',            -- Bunga menurun dari sisa pokok
    'BAGI_HASIL',         -- Bagi hasil atas pendapatan
    'NONE'                -- Tabungan, deposito
);

CREATE TABLE banking_products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(16) NOT NULL UNIQUE,          -- mis. TAB-CONV-01, DEP-SYAR-01
    name VARCHAR(128) NOT NULL,
    family product_family NOT NULL,
    book coa_book NOT NULL,

    profit_scheme profit_scheme NOT NULL,
    schedule_method schedule_method NOT NULL DEFAULT 'NONE',

    -- Tarif: bunga/margin tahunan dalam persen, atau nisbah bagi hasil (0-1).
    rate_annual NUMERIC(8,4) NOT NULL DEFAULT 0.0000,
    profit_sharing_ratio NUMERIC(8,4) NOT NULL DEFAULT 0.0000,

    min_amount NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    max_amount NUMERIC(28,4) NOT NULL DEFAULT 0.0000,   -- 0 = tanpa batas
    min_term_months INT NOT NULL DEFAULT 0,             -- 0 = tidak berlaku (tabungan)
    max_term_months INT NOT NULL DEFAULT 0,
    allow_partial_payment BOOLEAN NOT NULL DEFAULT FALSE,
    early_withdrawal_penalty_rate NUMERIC(8,4) NOT NULL DEFAULT 0.0000,

    admin_fee NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    tax_rate NUMERIC(8,4) NOT NULL DEFAULT 0.0000,      -- PPh final, mis. 20

    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_rate_non_negative CHECK (rate_annual >= 0),
    CONSTRAINT chk_min_max_amount CHECK (max_amount = 0 OR max_amount >= min_amount)
);

-- ── Pemetaan jurnal per produk dan peristiwa ────────────────────────────────
-- Satu baris = satu sisi jurnal. Peristiwa yang sama bisa punya beberapa sisi,
-- sehingga posting engine tinggal membaca mapping lalu menulis jurnal.
CREATE TYPE posting_event AS ENUM (
    'ACCOUNT_OPEN',            -- setoran awal / biaya pembukaan
    'DEPOSIT',                 -- setoran
    'WITHDRAWAL',              -- penarikan
    'ACCOUNT_CLOSE',           -- penutupan rekening
    'ACCOUNT_DORMANT',         -- rekening dormant
    'INTEREST_ACCRUAL',        -- akrual bunga/margin/bagi hasil
    'INTEREST_PAYMENT',        -- pembayaran bunga/margin/bagi hasil
    'TAX_WITHHOLDING',         -- potong PPh final
    'LOAN_DISBURSEMENT',       -- pencairan kredit/pembiayaan
    'LOAN_PRINCIPAL_PAYMENT',  -- angsuran pokok
    'LOAN_PROFIT_PAYMENT',     -- angsuran bunga/margin/bagi hasil
    'LOAN_PENALTY',            -- denda keterlambatan
    'LOAN_WRITE_OFF',          -- hapus buku
    'LOAN_RECOVERY',           -- penerimaan kembali hapus buku
    'PPAP_PROVISION',          -- pembentukan cadangan PPAP
    'PPAP_REVERSAL',           -- pemulihan cadangan PPAP
    'FEE_INCOME',              -- pendapatan biaya administrasi
    'CASH_IN',                 -- kas masuk (teller)
    'CASH_OUT'                 -- kas keluar (teller)
);

CREATE TABLE product_journal_mapping (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES banking_products(id) ON DELETE CASCADE,
    event posting_event NOT NULL,
    direction entry_direction NOT NULL,
    coa_code VARCHAR(32) NOT NULL REFERENCES chart_of_accounts(code) ON DELETE RESTRICT,
    amount_source VARCHAR(32) NOT NULL DEFAULT 'PRINCIPAL',
    -- amount_source memberi nama sisi mana dari transaksi yang dipakai:
    -- PRINCIPAL (pokok), PROFIT (bunga/margin/bagi hasil), FEE (biaya), TAX (pajak),
    -- PENALTY (denda), TOTAL (jumlah penuh). Dipakai posting engine untuk memilih nominal.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (product_id, event, direction, coa_code)
);

CREATE INDEX idx_product_mapping_event ON product_journal_mapping(product_id, event);

-- ── Skema nomor rekening ────────────────────────────────────────────────────
-- Nomor rekening = kode cabang (3) + kode produk (2) + nomor seri (9) + cek digit (1).
-- Cek digit dihitung dengan algoritma yang sama seperti NIK/VA, disimpan agar validasi
-- tidak bergantung pada pembangkitan ulang.
CREATE TABLE account_number_sequences (
    product_id UUID NOT NULL REFERENCES banking_products(id) ON DELETE CASCADE,
    branch_code VARCHAR(8) NOT NULL REFERENCES branches(code) ON DELETE RESTRICT,
    last_serial BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, branch_code)
);

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS product_id UUID REFERENCES banking_products(id),
    ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id),
    ADD COLUMN IF NOT EXISTS opened_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS dormant_at TIMESTAMPTZ;

CREATE INDEX idx_accounts_product ON accounts(product_id);
CREATE INDEX idx_accounts_branch ON accounts(branch_id);

-- ── Perbaikan customers & loans untuk multi-cabang ──────────────────────────
ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id),
    ADD COLUMN IF NOT EXISTS cif_branch_code VARCHAR(8);

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS product_id UUID REFERENCES banking_products(id),
    ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id),
    ADD COLUMN IF NOT EXISTS outstanding_principal NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS penalty_accrued NUMERIC(28,4) NOT NULL DEFAULT 0.0000;

CREATE INDEX idx_loans_product ON loans(product_id);
CREATE INDEX idx_loans_branch ON loans(branch_id);

-- ── COA baku BPR ────────────────────────────────────────────────────────────
-- Struktur kelompok: 1 Aset, 2 Kewajiban, 3 Ekuitas, 4 Pendapatan,
-- 5 Beban, 6 Beban Pajak, 7 Pendapatan Lain, 8 Beban Lain.
-- Syariah memakai kelompok terpisah (10-12) agar laporan UUS dapat dipisah.
-- Normal balance mengikuti sifat akun, bukan digit nomor rekening.

-- Konvensional: Aset
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a1000000-0000-0000-0000-000000000001', '10000', 'ASET', 'ASSET', 'DEBIT', 'CONVENTIONAL', TRUE),
    ('a1000000-0000-0000-0000-000000000002', '10100', 'Kas', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000003', '10101', 'Kas Teller', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000004', '10200', 'Penempatan pada Bank Lain', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000005', '10300', 'Kredit yang Diberikan', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000006', '10301', 'Kredit - Pokok', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000007', '10400', 'Bunga Kredit yang Masih Akan Diterima', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000008', '10500', 'Agunan yang Diambil Alih', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000009', '10600', 'Aset Tetap', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000010', '10700', 'Akumulasi Penyusutan Aset Tetap', 'ASSET', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000011', '10800', 'Rekening Antar Kantor', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000012', '10900', 'Cadangan Kerugian Penurunan Nilai (PPAP)', 'ASSET', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a1000000-0000-0000-0000-000000000013', '10999', 'Akun Sementara (Suspense)', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Konvensional: Kewajiban
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a2000000-0000-0000-0000-000000000001', '20000', 'KEWAJIBAN', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', TRUE),
    ('a2000000-0000-0000-0000-000000000002', '20100', 'Tabungan', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000003', '20200', 'Deposito Berjangka', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000004', '20300', 'Giro', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000005', '20400', 'Bunga Deposito yang Masih Harus Dibayar', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000006', '20500', 'Utang Pajak', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000007', '20600', 'Utang Bunga', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000008', '20700', 'Utang Lainnya', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a2000000-0000-0000-0000-000000000009', '20800', 'Rekening Antar Kantor - Kredit', 'LIABILITY', 'CREDIT', 'CONVENTIONAL', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Konvensional: Ekuitas
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a3000000-0000-0000-0000-000000000001', '30000', 'EKUITAS', 'EQUITY', 'CREDIT', 'CONVENTIONAL', TRUE),
    ('a3000000-0000-0000-0000-000000000002', '30100', 'Modal Disetor', 'EQUITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a3000000-0000-0000-0000-000000000003', '30200', 'Laba Ditahan', 'EQUITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a3000000-0000-0000-0000-000000000004', '30300', 'Laba Tahun Berjalan', 'EQUITY', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a3000000-0000-0000-0000-000000000005', '30400', 'Cadangan Umum', 'EQUITY', 'CREDIT', 'CONVENTIONAL', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Konvensional: Pendapatan
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a4000000-0000-0000-0000-000000000001', '40000', 'PENDAPATAN', 'REVENUE', 'CREDIT', 'CONVENTIONAL', TRUE),
    ('a4000000-0000-0000-0000-000000000002', '40100', 'Pendapatan Bunga Kredit', 'REVENUE', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a4000000-0000-0000-0000-000000000003', '40200', 'Pendapatan Bunga Penempatan', 'REVENUE', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a4000000-0000-0000-0000-000000000004', '40300', 'Pendapatan Provisi dan Komisi', 'REVENUE', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a4000000-0000-0000-0000-000000000005', '40400', 'Pendapatan Administrasi', 'REVENUE', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a4000000-0000-0000-0000-000000000006', '40500', 'Pendapatan Denda', 'REVENUE', 'CREDIT', 'CONVENTIONAL', FALSE),
    ('a4000000-0000-0000-0000-000000000007', '40900', 'Pendapatan Lainnya', 'REVENUE', 'CREDIT', 'CONVENTIONAL', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Konvensional: Beban
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a5000000-0000-0000-0000-000000000001', '50000', 'BEBAN', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', TRUE),
    ('a5000000-0000-0000-0000-000000000002', '50100', 'Beban Bunga Deposito', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a5000000-0000-0000-0000-000000000003', '50200', 'Beban Penyisihan Kerugian Kredit', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a5000000-0000-0000-0000-000000000004', '50300', 'Beban Gaji dan Tunjangan', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a5000000-0000-0000-0000-000000000005', '50400', 'Beban Umum dan Administrasi', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a5000000-0000-0000-0000-000000000006', '50500', 'Beban Penyusutan', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('a5000000-0000-0000-0000-000000000007', '50900', 'Beban Lainnya', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Konvensional: Beban pajak
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a6000000-0000-0000-0000-000000000001', '60000', 'BEBAN PAJAK', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', TRUE),
    ('a6000000-0000-0000-0000-000000000002', '60100', 'Beban Pajak Penghasilan', 'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Syariah: Aset
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('b1000000-0000-0000-0000-000000000001', '11000', 'ASET SYARIAH', 'ASSET', 'DEBIT', 'SYARIAH', TRUE),
    ('b1000000-0000-0000-0000-000000000002', '11100', 'Kas Syariah', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000003', '11200', 'Penempatan pada Bank Syariah', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000004', '11300', 'Pembiayaan Murabahah', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000005', '11310', 'Pembiayaan Murabahah - Piutang', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000006', '11320', 'Pembiayaan Murabahah - Margin Ditangguhkan', 'ASSET', 'CREDIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000007', '11400', 'Pembiayaan Mudharabah', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000008', '11500', 'Pembiayaan Musyarakah', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000009', '11600', 'Pembiayaan Ijarah', 'ASSET', 'DEBIT', 'SYARIAH', FALSE),
    ('b1000000-0000-0000-0000-000000000010', '11900', 'Cadangan Kerugian Pembiayaan', 'ASSET', 'CREDIT', 'SYARIAH', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Syariah: Kewajiban
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('b2000000-0000-0000-0000-000000000001', '12000', 'KEWAJIBAN SYARIAH', 'LIABILITY', 'CREDIT', 'SYARIAH', TRUE),
    ('b2000000-0000-0000-0000-000000000002', '12100', 'Tabungan Wadiah', 'LIABILITY', 'CREDIT', 'SYARIAH', FALSE),
    ('b2000000-0000-0000-0000-000000000003', '12200', 'Tabungan Mudharabah', 'LIABILITY', 'CREDIT', 'SYARIAH', FALSE),
    ('b2000000-0000-0000-0000-000000000004', '12300', 'Deposito Mudharabah', 'LIABILITY', 'CREDIT', 'SYARIAH', FALSE),
    ('b2000000-0000-0000-0000-000000000005', '12400', 'Bagi Hasil yang Masih Harus Dibayar', 'LIABILITY', 'CREDIT', 'SYARIAH', FALSE),
    ('b2000000-0000-0000-0000-000000000006', '12900', 'Kewajiban Lainnya Syariah', 'LIABILITY', 'CREDIT', 'SYARIAH', FALSE)
ON CONFLICT (code) DO NOTHING;

-- Syariah: Ekuitas & pendapatan
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('b3000000-0000-0000-0000-000000000001', '13000', 'EKUITAS SYARIAH', 'EQUITY', 'CREDIT', 'SYARIAH', TRUE),
    ('b3000000-0000-0000-0000-000000000002', '13100', 'Modal Disetor Syariah', 'EQUITY', 'CREDIT', 'SYARIAH', FALSE),
    ('b3000000-0000-0000-0000-000000000003', '13200', 'Laba Ditahan Syariah', 'EQUITY', 'CREDIT', 'SYARIAH', FALSE),
    ('b4000000-0000-0000-0000-000000000001', '14000', 'PENDAPATAN SYARIAH', 'REVENUE', 'CREDIT', 'SYARIAH', TRUE),
    ('b4000000-0000-0000-0000-000000000002', '14100', 'Pendapatan Margin Murabahah', 'REVENUE', 'CREDIT', 'SYARIAH', FALSE),
    ('b4000000-0000-0000-0000-000000000003', '14200', 'Pendapatan Bagi Hasil Mudharabah', 'REVENUE', 'CREDIT', 'SYARIAH', FALSE),
    ('b4000000-0000-0000-0000-000000000004', '14300', 'Pendapatan Bagi Hasil Musyarakah', 'REVENUE', 'CREDIT', 'SYARIAH', FALSE),
    ('b4000000-0000-0000-0000-000000000005', '14400', 'Pendapatan Ijarah', 'REVENUE', 'CREDIT', 'SYARIAH', FALSE),
    ('b4000000-0000-0000-0000-000000000006', '14500', 'Pendapatan Administrasi Syariah', 'REVENUE', 'CREDIT', 'SYARIAH', FALSE),
    ('b5000000-0000-0000-0000-000000000001', '15000', 'BEBAN SYARIAH', 'EXPENSE', 'DEBIT', 'SYARIAH', TRUE),
    ('b5000000-0000-0000-0000-000000000002', '15100', 'Bagi Hasil untuk Pemilik Dana', 'EXPENSE', 'DEBIT', 'SYARIAH', FALSE),
    ('b5000000-0000-0000-0000-000000000003', '15200', 'Beban Penyisihan Kerugian Pembiayaan', 'EXPENSE', 'DEBIT', 'SYARIAH', FALSE),
    ('b5000000-0000-0000-0000-000000000004', '15900', 'Beban Lainnya Syariah', 'EXPENSE', 'DEBIT', 'SYARIAH', FALSE)
ON CONFLICT (code) DO NOTHING;

-- ── Perbaikan akun lama agar selaras bagan baku ─────────────────────────────
-- Akun 10300/10900/40900/40100 dibuat oleh 000004 dengan nama dan makna yang
-- dipakai kode lama. Bagan baku memberi makna baru pada kode yang sama, jadi nama
-- diselaraskan di sini agar laporan tidak menampilkan label lama.
UPDATE chart_of_accounts SET name = 'Kredit yang Diberikan', book = 'CONVENTIONAL' WHERE code = '10300';
UPDATE chart_of_accounts SET name = 'Cadangan Kerugian Penurunan Nilai (PPAP)', book = 'CONVENTIONAL' WHERE code = '10900';
UPDATE chart_of_accounts SET name = 'Pendapatan Lainnya', book = 'CONVENTIONAL' WHERE code = '40900';
UPDATE chart_of_accounts SET name = 'Pendapatan Bunga Kredit', book = 'CONVENTIONAL' WHERE code = '40100';
UPDATE chart_of_accounts SET name = 'Beban Bunga Deposito', book = 'CONVENTIONAL' WHERE code = '50100';
UPDATE chart_of_accounts SET name = 'Penempatan pada Bank Lain', book = 'CONVENTIONAL' WHERE code = '10200';

-- ── Akun internal (tabel accounts) untuk posting jurnal ─────────────────────
-- Akun GL internal sekarang punya product/branch NULL dan tidak terikat produk.
INSERT INTO accounts (id, account_number, customer_id, coa_id, account_type, currency, balance, available_balance, status, opened_at) VALUES
    ('b0000000-0000-0000-0000-000000000010', '10101', NULL, 'a1000000-0000-0000-0000-000000000003', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000011', '10301', NULL, 'a1000000-0000-0000-0000-000000000006', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000012', '10400', NULL, 'a1000000-0000-0000-0000-000000000007', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000013', '20400', NULL, 'a2000000-0000-0000-0000-000000000005', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000014', '20500', NULL, 'a2000000-0000-0000-0000-000000000006', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000015', '40400', NULL, 'a4000000-0000-0000-0000-000000000005', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000016', '40500', NULL, 'a4000000-0000-0000-0000-000000000006', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000017', '50200', NULL, 'a5000000-0000-0000-0000-000000000003', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000018', '30200', NULL, 'a3000000-0000-0000-0000-000000000003', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000019', '14100', NULL, 'b4000000-0000-0000-0000-000000000002', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('b0000000-0000-0000-0000-000000000020', '11320', NULL, 'b1000000-0000-0000-0000-000000000006', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW())
ON CONFLICT (account_number) DO NOTHING;

-- ── Sequence nomor referensi jurnal ─────────────────────────────────────────
-- Nomor referensi dibangkitkan database agar unik walau posting paralel.
-- Satu sequence global cukup: prefiks tipe transaksi membuat nomor mudah dibaca.
CREATE SEQUENCE IF NOT EXISTS journal_reference_seq START 1 INCREMENT 1;

-- ── Produk contoh ───────────────────────────────────────────────────────────
INSERT INTO banking_products (id, code, name, family, book, profit_scheme, schedule_method, rate_annual, profit_sharing_ratio, min_amount, min_term_months, max_term_months, allow_partial_payment, tax_rate) VALUES
    ('e0000000-0000-0000-0000-000000000001', 'TAB-CONV', 'Tabungan Konvensional', 'SAVINGS', 'CONVENTIONAL', 'INTEREST', 'NONE', 2.0000, 0, 10000, 0, 0, FALSE, 20),
    ('e0000000-0000-0000-0000-000000000002', 'TAB-SYAR', 'Tabungan Syariah (Wadiah)', 'SAVINGS', 'SYARIAH', 'WADIAH', 'NONE', 0, 0, 10000, 0, 0, FALSE, 0),
    ('e0000000-0000-0000-0000-000000000003', 'DEP-CONV', 'Deposito Berjangka Konvensional', 'TIME_DEPOSIT', 'CONVENTIONAL', 'INTEREST', 'NONE', 4.0000, 0, 1000000, 1, 24, FALSE, 20),
    ('e0000000-0000-0000-0000-000000000004', 'DEP-SYAR', 'Deposito Mudharabah', 'TIME_DEPOSIT', 'SYARIAH', 'MUDHARABAH', 'NONE', 0, 0.6000, 1000000, 1, 24, FALSE, 0),
    ('e0000000-0000-0000-0000-000000000005', 'KRD-FLAT', 'Kredit Flat Konvensional', 'LOAN', 'CONVENTIONAL', 'INTEREST', 'FLAT', 12.0000, 0, 1000000, 1, 120, TRUE, 0),
    ('e0000000-0000-0000-0000-000000000006', 'KRD-ANUITAS', 'Kredit Anuitas Konvensional', 'LOAN', 'CONVENTIONAL', 'INTEREST', 'ANNUITY', 12.0000, 0, 1000000, 1, 120, TRUE, 0),
    ('e0000000-0000-0000-0000-000000000007', 'PMB-MURABAHAH', 'Pembiayaan Murabahah', 'LOAN', 'SYARIAH', 'MURABAHAH', 'FLAT', 0, 0.5000, 1000000, 1, 120, TRUE, 0),
    ('e0000000-0000-0000-0000-000000000008', 'PMB-MUDHARABAH', 'Pembiayaan Mudharabah', 'LOAN', 'SYARIAH', 'MUDHARABAH', 'BAGI_HASIL', 0, 0.4000, 1000000, 1, 120, TRUE, 0)
ON CONFLICT (code) DO NOTHING;

-- ── Pemetaan jurnal produk contoh ───────────────────────────────────────────
-- Tabungan konvensional: setoran masuk ke kas teller, lawannya kewajiban tabungan.
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source) VALUES
    ('e0000000-0000-0000-0000-000000000001', 'DEPOSIT', 'DEBIT', '10101', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000001', 'DEPOSIT', 'CREDIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000001', 'WITHDRAWAL', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000001', 'WITHDRAWAL', 'CREDIT', '10101', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000001', 'INTEREST_ACCRUAL', 'DEBIT', '50100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000001', 'INTEREST_ACCRUAL', 'CREDIT', '20400', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000001', 'FEE_INCOME', 'DEBIT', '10101', 'FEE'),
    ('e0000000-0000-0000-0000-000000000001', 'FEE_INCOME', 'CREDIT', '40400', 'FEE'),

    ('e0000000-0000-0000-0000-000000000002', 'DEPOSIT', 'DEBIT', '11100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000002', 'DEPOSIT', 'CREDIT', '12100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000002', 'WITHDRAWAL', 'DEBIT', '12100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000002', 'WITHDRAWAL', 'CREDIT', '11100', 'PRINCIPAL'),

    ('e0000000-0000-0000-0000-000000000003', 'DEPOSIT', 'DEBIT', '10101', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000003', 'DEPOSIT', 'CREDIT', '20200', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000003', 'WITHDRAWAL', 'DEBIT', '20200', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000003', 'WITHDRAWAL', 'CREDIT', '10101', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000003', 'INTEREST_ACCRUAL', 'DEBIT', '50100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000003', 'INTEREST_ACCRUAL', 'CREDIT', '20400', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000003', 'TAX_WITHHOLDING', 'DEBIT', '20400', 'TAX'),
    ('e0000000-0000-0000-0000-000000000003', 'TAX_WITHHOLDING', 'CREDIT', '20500', 'TAX'),

    ('e0000000-0000-0000-0000-000000000004', 'DEPOSIT', 'DEBIT', '11100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000004', 'DEPOSIT', 'CREDIT', '12300', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000004', 'WITHDRAWAL', 'DEBIT', '12300', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000004', 'WITHDRAWAL', 'CREDIT', '11100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000004', 'INTEREST_PAYMENT', 'DEBIT', '15100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000004', 'INTEREST_PAYMENT', 'CREDIT', '12300', 'PROFIT'),

    ('e0000000-0000-0000-0000-000000000005', 'LOAN_DISBURSEMENT', 'DEBIT', '10301', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_DISBURSEMENT', 'CREDIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_PRINCIPAL_PAYMENT', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_PRINCIPAL_PAYMENT', 'CREDIT', '10301', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_PROFIT_PAYMENT', 'DEBIT', '20100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_PROFIT_PAYMENT', 'CREDIT', '40100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000005', 'INTEREST_ACCRUAL', 'DEBIT', '10400', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000005', 'INTEREST_ACCRUAL', 'CREDIT', '40100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_PENALTY', 'DEBIT', '20100', 'PENALTY'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_PENALTY', 'CREDIT', '40500', 'PENALTY'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_WRITE_OFF', 'DEBIT', '10900', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_WRITE_OFF', 'CREDIT', '10301', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_RECOVERY', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'LOAN_RECOVERY', 'CREDIT', '40900', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'PPAP_PROVISION', 'DEBIT', '50200', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000005', 'PPAP_PROVISION', 'CREDIT', '10900', 'PRINCIPAL'),

    ('e0000000-0000-0000-0000-000000000006', 'LOAN_DISBURSEMENT', 'DEBIT', '10301', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_DISBURSEMENT', 'CREDIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_PRINCIPAL_PAYMENT', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_PRINCIPAL_PAYMENT', 'CREDIT', '10301', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_PROFIT_PAYMENT', 'DEBIT', '20100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_PROFIT_PAYMENT', 'CREDIT', '40100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000006', 'INTEREST_ACCRUAL', 'DEBIT', '10400', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000006', 'INTEREST_ACCRUAL', 'CREDIT', '40100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_WRITE_OFF', 'DEBIT', '10900', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_WRITE_OFF', 'CREDIT', '10301', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_RECOVERY', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'LOAN_RECOVERY', 'CREDIT', '40900', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'PPAP_PROVISION', 'DEBIT', '50200', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000006', 'PPAP_PROVISION', 'CREDIT', '10900', 'PRINCIPAL'),

    -- Murabahah: aset murabahah (harga perolehan + margin ditangguhkan),
    -- angsuran memindahkan margin ditangguhkan ke pendapatan secara bertahap.
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_DISBURSEMENT', 'DEBIT', '11310', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_DISBURSEMENT', 'CREDIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_PRINCIPAL_PAYMENT', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_PRINCIPAL_PAYMENT', 'CREDIT', '11310', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_PROFIT_PAYMENT', 'DEBIT', '20100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_PROFIT_PAYMENT', 'CREDIT', '14100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_WRITE_OFF', 'DEBIT', '11900', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_WRITE_OFF', 'CREDIT', '11310', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_RECOVERY', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000007', 'LOAN_RECOVERY', 'CREDIT', '40900', 'PRINCIPAL'),

    ('e0000000-0000-0000-0000-000000000008', 'LOAN_DISBURSEMENT', 'DEBIT', '11400', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_DISBURSEMENT', 'CREDIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_PRINCIPAL_PAYMENT', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_PRINCIPAL_PAYMENT', 'CREDIT', '11400', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_PROFIT_PAYMENT', 'DEBIT', '20100', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_PROFIT_PAYMENT', 'CREDIT', '14200', 'PROFIT'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_WRITE_OFF', 'DEBIT', '11900', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_WRITE_OFF', 'CREDIT', '11400', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_RECOVERY', 'DEBIT', '20100', 'PRINCIPAL'),
    ('e0000000-0000-0000-0000-000000000008', 'LOAN_RECOVERY', 'CREDIT', '40900', 'PRINCIPAL')
ON CONFLICT (product_id, event, direction, coa_code) DO NOTHING;
