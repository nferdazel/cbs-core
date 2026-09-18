-- CBS Migration 000004: Loan OJK Fields, WRITTEN_OFF status & missing COA accounts
-- Run after: 000003_loans_schema.up.sql
--
-- Latar belakang:
--   1. loan_service.go memakai status 'WRITTEN_OFF' yang belum ada di enum loan_status.
--   2. loan_service.go menyimpan field OJK (collectibility, dpd, accrual_status, required_ppap,
--      is_restructured, restructured_count, restructured_at, restructuring_reason) di domain,
--      tapi tabel loans belum punya kolomnya.
--   3. loan_service.go memposting jurnal ke COA 10300 (Loan Receivable), 10900 (PPAP Reserve),
--      dan 40900 (Recovery Income) yang belum di-seed di 000001.

-- 1. Tambah status WRITTEN_OFF pada enum loan_status (idempotent)
ALTER TYPE loan_status ADD VALUE IF NOT EXISTS 'WRITTEN_OFF';

-- 2. Kolom OJK & restrukturisasi pada tabel loans
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS collectibility     VARCHAR(32)  NOT NULL DEFAULT '1_LANCAR',
    ADD COLUMN IF NOT EXISTS dpd                INT          NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS accrual_status     VARCHAR(32)  NOT NULL DEFAULT 'ACCRUAL_PERFORMING',
    ADD COLUMN IF NOT EXISTS required_ppap      NUMERIC(28,4) NOT NULL DEFAULT 0.0000,
    ADD COLUMN IF NOT EXISTS is_restructured    BOOLEAN      NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS restructured_count INT          NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS restructured_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS restructuring_reason TEXT;

-- 3. Seed akun COA yang dipakai modul kredit/pembiayaan (idempotent)
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance) VALUES
    ('a0000000-0000-0000-0000-000000000006', '10300', 'Loan Receivable Portfolio',          'ASSET',   'DEBIT'),
    ('a0000000-0000-0000-0000-000000000007', '10900', 'Allowance for Impairment (PPAP)',    'ASSET',   'DEBIT'),
    ('a0000000-0000-0000-0000-000000000008', '40900', 'Recovery Income on Written-Off Loan','REVENUE', 'CREDIT')
ON CONFLICT (code) DO NOTHING;
