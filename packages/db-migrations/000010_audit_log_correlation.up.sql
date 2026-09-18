-- CBS Migration 000010: Korelasi audit log dengan application log
-- Run after: 000009_ensure_sequences.up.sql
--
-- request_id memungkinkan satu aksi bisnis dilacak dari application log (stdout)
-- ke audit log (database). user_agent membantu investigasi akses mencurigakan.
-- Idempotent, aman dijalankan ulang.

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS user_agent TEXT;

CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_logs(request_id);
