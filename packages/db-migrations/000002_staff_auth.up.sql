-- CBS Staff User Management, Sessions, and System Configuration Migration
-- Run after: 000001_init_cbs_schema.up.sql

-- 1. Staff Roles Enum
CREATE TYPE staff_role AS ENUM (
    'SUPERADMIN',
    'ADMIN',
    'SUPERVISOR',
    'TELLER',
    'CS',
    'AO',
    'AUDITOR'
);

-- 2. Staff Users (Bank internal employees — TERPISAH dari customers)
CREATE TABLE staff_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    employee_id VARCHAR(32) NOT NULL UNIQUE,    -- e.g. EMP-2026-001
    username VARCHAR(64) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,                -- bcrypt cost 12
    role staff_role NOT NULL DEFAULT 'TELLER',
    branch_code VARCHAR(16) NOT NULL DEFAULT 'HO', -- HO = Head Office; multi-branch ready
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    failed_login_count INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,                   -- auto-lock after 5 failed attempts
    created_by UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_staff_username ON staff_users(username);
CREATE INDEX idx_staff_employee_id ON staff_users(employee_id);
CREATE INDEX idx_staff_role ON staff_users(role);

-- 3. Staff Sessions (Refresh token store for invalidation)
CREATE TABLE staff_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES staff_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,    -- SHA-256 of the actual token
    ip_address VARCHAR(45),
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,                     -- NULL = still valid
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user ON staff_sessions(user_id);
CREATE INDEX idx_sessions_token ON staff_sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires ON staff_sessions(expires_at);

-- 4. System Configuration (Maker-Checker thresholds, role limits, etc.)
CREATE TABLE system_config (
    key VARCHAR(128) PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_by UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default system configuration values
INSERT INTO system_config (key, value, description) VALUES
-- Maker-checker thresholds per transaction type (in IDR)
('maker_checker.deposit.threshold',    '100000000',  'Deposit amount (IDR) requiring supervisor approval'),
('maker_checker.withdrawal.threshold', '50000000',   'Withdrawal amount (IDR) requiring supervisor approval'),
('maker_checker.transfer.threshold',   '50000000',   'Transfer amount (IDR) requiring supervisor approval'),

-- Transaction limits per role (in IDR; 0 = unlimited)
('role_limit.TELLER.per_transaction',     '50000000',   'Max single transaction amount for TELLER'),
('role_limit.AO.per_transaction',         '250000000',  'Max loan/financing origination amount for AO'),
('role_limit.SUPERVISOR.per_transaction', '500000000',  'Max single transaction amount for SUPERVISOR'),
('role_limit.ADMIN.per_transaction',      '0',          'Max single transaction amount for ADMIN (0 = unlimited)'),

-- Session configuration
('auth.access_token_ttl_minutes',  '15',  'JWT access token TTL in minutes'),
('auth.refresh_token_ttl_hours',   '8',   'Refresh token TTL in hours (= 1 work shift)'),
('auth.max_failed_logins',         '5',   'Failed login attempts before account lock'),
('auth.lockout_minutes',           '15',  'Account lockout duration in minutes'),

-- Password policy
('auth.password_expiry_days', '90', 'Days before password must be changed');

-- 5. Add staff_user_id to audit_logs for richer trail
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS staff_user_id UUID REFERENCES staff_users(id) ON DELETE SET NULL;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS staff_role VARCHAR(32);

-- 6. Seed: first SUPERADMIN user
-- Password: "Admin@CBS2026!" (bcrypt hash — MUST BE CHANGED on first login)
-- Generated with: bcrypt.GenerateFromPassword([]byte("Admin@CBS2026!"), 12)
INSERT INTO staff_users (
    id, employee_id, username, full_name, email,
    password_hash, role, branch_code, is_active
) VALUES (
    'c0000000-0000-0000-0000-000000000001',
    'EMP-2026-001',
    'superadmin',
    'System Super Administrator',
    'superadmin@cbs.local',
    '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TiGMamgYlFNZzaKWGfWTjVv.IIJq',
    'SUPERADMIN',
    'HO',
    TRUE
);
