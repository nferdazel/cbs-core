-- Core Banking System (CBS) Initial Database Schema
-- Strict Double-Entry Ledger, CIF, Accounts, Idempotency, Maker-Checker & Audit Trail

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Chart of Accounts (COA)
CREATE TYPE coa_type AS ENUM ('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE');
CREATE TYPE balance_type AS ENUM ('DEBIT', 'CREDIT');

CREATE TABLE chart_of_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    type coa_type NOT NULL,
    normal_balance balance_type NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Customer Information File (CIF)
CREATE TYPE customer_status AS ENUM ('PENDING_KYC', 'ACTIVE', 'BLOCKED', 'CLOSED');

CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cif_number VARCHAR(32) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    id_card_number VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone_number VARCHAR(32) NOT NULL,
    address TEXT,
    status customer_status NOT NULL DEFAULT 'ACTIVE',
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_customers_cif ON customers(cif_number);
CREATE INDEX idx_customers_id_card ON customers(id_card_number);

-- 3. Accounts (Customer Savings & Internal GL Accounts)
CREATE TYPE account_type AS ENUM ('SAVINGS', 'CHECKING', 'LOAN', 'INTERNAL_GL');
CREATE TYPE account_status AS ENUM ('ACTIVE', 'DORMANT', 'FROZEN', 'CLOSED');

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_number VARCHAR(32) NOT NULL UNIQUE,
    customer_id UUID REFERENCES customers(id) ON DELETE RESTRICT,
    coa_id UUID NOT NULL REFERENCES chart_of_accounts(id) ON DELETE RESTRICT,
    account_type account_type NOT NULL DEFAULT 'SAVINGS',
    currency VARCHAR(3) NOT NULL DEFAULT 'IDR',
    balance NUMERIC(28, 4) NOT NULL DEFAULT 0.0000,
    available_balance NUMERIC(28, 4) NOT NULL DEFAULT 0.0000,
    hold_balance NUMERIC(28, 4) NOT NULL DEFAULT 0.0000,
    status account_status NOT NULL DEFAULT 'ACTIVE',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_positive_hold CHECK (hold_balance >= 0)
);

CREATE INDEX idx_accounts_number ON accounts(account_number);
CREATE INDEX idx_accounts_customer ON accounts(customer_id);

-- 4. Journal Entries (Transaction Header)
CREATE TYPE transaction_type AS ENUM (
    'DEPOSIT',
    'WITHDRAWAL',
    'TRANSFER_INTERNAL',
    'FEE_CHARGE',
    'INTEREST_ACCRUAL',
    'REVERSAL',
    'ADJUSTMENT'
);

CREATE TYPE journal_status AS ENUM ('POSTED', 'REVERSED', 'FAILED', 'PENDING_APPROVAL');

CREATE TABLE journal_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reference_number VARCHAR(64) NOT NULL UNIQUE,
    idempotency_key VARCHAR(128) UNIQUE,
    transaction_type transaction_type NOT NULL,
    description TEXT NOT NULL,
    status journal_status NOT NULL DEFAULT 'POSTED',
    posted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(64) NOT NULL DEFAULT 'SYSTEM',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journal_ref ON journal_entries(reference_number);
CREATE INDEX idx_journal_idempotency ON journal_entries(idempotency_key);
CREATE INDEX idx_journal_posted_at ON journal_entries(posted_at DESC);

-- 5. Journal Lines (Double-Entry Posting Lines)
CREATE TYPE entry_direction AS ENUM ('DEBIT', 'CREDIT');

CREATE TABLE journal_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    direction entry_direction NOT NULL,
    amount NUMERIC(28, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'IDR',
    balance_after NUMERIC(28, 4) NOT NULL,
    sequence INT NOT NULL DEFAULT 1,
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_positive_amount CHECK (amount > 0)
);

CREATE INDEX idx_lines_entry ON journal_lines(journal_entry_id);
CREATE INDEX idx_lines_account ON journal_lines(account_id);
CREATE INDEX idx_lines_account_created ON journal_lines(account_id, created_at DESC);

-- 6. Idempotency Records
CREATE TYPE idempotency_status AS ENUM ('PROCESSING', 'COMPLETED', 'FAILED');

CREATE TABLE idempotency_keys (
    key VARCHAR(128) PRIMARY KEY,
    request_path VARCHAR(255) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    status idempotency_status NOT NULL DEFAULT 'PROCESSING',
    response_code INT,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- 7. Maker-Checker Workflow
CREATE TYPE maker_checker_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED');

CREATE TABLE maker_checker_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status maker_checker_status NOT NULL DEFAULT 'PENDING',
    maker_id VARCHAR(64) NOT NULL,
    checker_id VARCHAR(64),
    maker_notes TEXT,
    checker_notes TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 8. Audit Logs
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_id VARCHAR(64) NOT NULL,
    actor_role VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    ip_address VARCHAR(45),
    changes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);

-- Default Seed Data for Chart of Accounts (COA)
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance) VALUES
('a0000000-0000-0000-0000-000000000001', '10100', 'Cash and Vault', 'ASSET', 'DEBIT'),
('a0000000-0000-0000-0000-000000000002', '10200', 'Interbank Settlement Clearing (BI-FAST / RTGS)', 'ASSET', 'DEBIT'),
('a0000000-0000-0000-0000-000000000003', '20100', 'Third-Party Savings Deposits', 'LIABILITY', 'CREDIT'),
('a0000000-0000-0000-0000-000000000004', '40100', 'Fee and Commission Income', 'REVENUE', 'CREDIT'),
('a0000000-0000-0000-0000-000000000005', '50100', 'Deposit Interest Expense', 'EXPENSE', 'DEBIT');

-- Default Bank Internal Vault Account
INSERT INTO accounts (id, account_number, customer_id, coa_id, account_type, currency, balance, available_balance, status) VALUES
('b0000000-0000-0000-0000-000000000001', 'GL-VAULT-001', NULL, 'a0000000-0000-0000-0000-000000000001', 'INTERNAL_GL', 'IDR', 10000000000.0000, 10000000000.0000, 'ACTIVE'),
('b0000000-0000-0000-0000-000000000002', 'GL-FEE-INCOME-001', NULL, 'a0000000-0000-0000-0000-000000000004', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE');
