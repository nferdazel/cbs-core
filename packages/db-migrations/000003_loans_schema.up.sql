-- CBS Loans & Financing Schema (Kredit BPR & Pembiayaan BMT)
-- Run after: 000002_staff_auth.up.sql

CREATE TYPE loan_type AS ENUM (
    'CONVENTIONAL_FLAT',      -- BPR Kredit Bunga Flat
    'CONVENTIONAL_ANNUITY',   -- BPR Kredit Bunga Anuitas
    'SYARIAH_MURABAHAH',      -- BMT Pembiayaan Murabahah (Jual Beli Margin)
    'SYARIAH_MUDHARABAH'      -- BMT Pembiayaan Mudharabah (Bagi Hasil)
);

CREATE TYPE loan_status AS ENUM (
    'PENDING_APPROVAL',       -- Inisiasi oleh AO (Maker)
    'APPROVED',               -- Disetujui oleh Supervisor/Komite Kredit (Checker)
    'DISBURSED',              -- Dana sudah dicairkan ke rekening nasabah
    'REJECTED',               -- Ditolak komite
    'PAID_OFF',               -- Lunas
    'DEFAULTED'               -- Macet / NPL
);

CREATE TYPE installment_status AS ENUM (
    'PENDING',
    'PAID',
    'OVERDUE',
    'PARTIAL'
);

-- 1. Loans / Pembiayaan Table
CREATE TABLE loans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    loan_number VARCHAR(64) NOT NULL UNIQUE,     -- e.g. KRD-2026-00001 / PMB-2026-00001
    customer_id UUID NOT NULL REFERENCES customers(id),
    disbursement_account_id UUID NOT NULL REFERENCES accounts(id), -- Rekening pencairan & debet angsuran
    loan_type loan_type NOT NULL,
    status loan_status NOT NULL DEFAULT 'PENDING_APPROVAL',
    
    principal_amount NUMERIC(28,4) NOT NULL,       -- Plafond pinjaman / harga pokok (IDR)
    interest_rate_annual NUMERIC(8,4) NOT NULL DEFAULT 0.00, -- Suku bunga tahunan (%) atau 0 untuk Syariah
    margin_amount NUMERIC(28,4) NOT NULL DEFAULT 0.00,       -- Keuntungan/Margin Murabahah (IDR)
    total_payable NUMERIC(28,4) NOT NULL,          -- Total yang harus dibayar = Principal + Interest/Margin
    term_months INT NOT NULL,                      -- Jangka waktu (bulan)
    monthly_installment NUMERIC(28,4) NOT NULL,    -- Nominal angsuran per bulan
    
    ao_id UUID REFERENCES staff_users(id),         -- Account Officer pengusul
    approved_by UUID REFERENCES staff_users(id),   -- Pejabat / Komite pemberi persetujuan
    approved_at TIMESTAMPTZ,
    disbursed_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loans_customer ON loans(customer_id);
CREATE INDEX idx_loans_ao ON loans(ao_id);
CREATE INDEX idx_loans_status ON loans(status);
CREATE INDEX idx_loans_number ON loans(loan_number);

-- 2. Loan Schedules / Jadwal Angsuran Table
CREATE TABLE loan_schedules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    loan_id UUID NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
    installment_no INT NOT NULL,
    due_date DATE NOT NULL,
    
    principal_amount NUMERIC(28,4) NOT NULL,       -- Porsi pokok bulan ini
    interest_amount NUMERIC(28,4) NOT NULL,        -- Porsi bunga / margin bulan ini
    total_installment NUMERIC(28,4) NOT NULL,      -- Total angsuran bulan ini (principal + interest)
    
    paid_principal NUMERIC(28,4) NOT NULL DEFAULT 0,
    paid_interest NUMERIC(28,4) NOT NULL DEFAULT 0,
    status installment_status NOT NULL DEFAULT 'PENDING',
    paid_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loan_schedules_loan ON loan_schedules(loan_id);
CREATE INDEX idx_loan_schedules_due ON loan_schedules(due_date);
CREATE INDEX idx_loan_schedules_status ON loan_schedules(status);
