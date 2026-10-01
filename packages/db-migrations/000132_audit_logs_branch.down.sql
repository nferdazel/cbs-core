-- 000132_audit_logs_branch.down.sql
-- Membalik 000132_audit_logs_branch.up.sql: hapus indeks dan kolom branch_code pada
-- audit_logs. Hanya membatalkan objek yang dibuat up; tidak menyentuh data lain.

DROP INDEX IF EXISTS idx_audit_branch_code;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS branch_code;
