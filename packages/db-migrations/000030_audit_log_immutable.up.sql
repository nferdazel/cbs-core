-- CBS Migration 000030: audit log bersifat append-only
-- Run after: 000029_loan_adjustment_approval.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- audit_logs adalah bukti kepatuhan: ia hanya boleh bertambah, tidak pernah diubah
-- atau dihapus. Repositori aplikasi memang hanya menyediakan Write dan List, tetapi
-- tanpa penjagaan di database satu kredensial aplikasi yang bocor cukup untuk menulis
-- ulang sejarah keputusan kredit, hapus buku, dan perubahan akun staf.
--
-- Trigger dipakai, bukan hanya hak akses: aplikasi terhubung sebagai pemilik tabel
-- sehingga REVOKE dari PUBLIC tidak menghalanginya. Trigger berlaku untuk pemilik
-- sekalipun, kecuali superuser yang memang menguasai database.
CREATE OR REPLACE FUNCTION audit_logs_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs bersifat append-only; % tidak diizinkan', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_logs_append_only_row ON audit_logs;
CREATE TRIGGER audit_logs_append_only_row
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_append_only();

DROP TRIGGER IF EXISTS audit_logs_append_only_truncate ON audit_logs;
CREATE TRIGGER audit_logs_append_only_truncate
    BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_append_only();

REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM PUBLIC;
