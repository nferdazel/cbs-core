-- CBS Migration 000011: Akun GL internal untuk seluruh akun COA
-- Run after: 000010_audit_log_correlation.up.sql
--
-- Latar belakang: posting hanya bisa menyentuh baris `accounts`. Sebelumnya hanya
-- sebagian akun COA punya akun GL internal, sehingga transaksi pada akun lain
-- (terutama buku syariah) gagal dengan "akun kontrol GL belum ada".
--
-- Migrasi ini membuat satu akun GL internal untuk setiap COA non-header yang belum
-- punya. Nomor akun GL memakai kode COA apa adanya agar mudah ditelusuri; kode COA
-- dengan nomor akun khusus (mis. GL-VAULT-001) tidak diubah karena sudah punya akun.
-- Idempotent: aman dijalankan ulang.

INSERT INTO accounts (
    id, account_number, customer_id, coa_id, account_type, currency,
    balance, available_balance, hold_balance, status, version, created_at, updated_at
)
SELECT
    uuid_generate_v4(),
    coa.code,
    NULL,
    coa.id,
    'INTERNAL_GL',
    'IDR',
    0.0000,
    0.0000,
    0.0000,
    'ACTIVE',
    1,
    NOW(),
    NOW()
FROM chart_of_accounts coa
WHERE coa.is_header = FALSE
  AND NOT EXISTS (
      SELECT 1 FROM accounts a
      WHERE a.coa_id = coa.id AND a.account_type = 'INTERNAL_GL'
  );
