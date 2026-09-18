-- CBS Migration 000012: Hak akses aplikasi pada tabel baru
-- Run after: 000011_gl_accounts_for_all_coa.up.sql
--
-- Latar belakang: tabel yang dibuat migrasi 000005 (branches, banking_products,
-- product_journal_mapping, account_number_sequences) dibuat tanpa GRANT ke role
-- aplikasi, sehingga runtime gagal dengan "permission denied for table ...".
-- Migrasi ini memberi hak yang diperlukan aplikasi dan memastikan objek sequence
-- ikut dapat diakses. Idempotent: aman dijalankan ulang.

-- Role aplikasi. Bila role tidak ada, migrasi ini akan gagal dengan pesan jelas,
-- lebih baik daripada runtime gagal tanpa petunjuk.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO cbs_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO cbs_app;

-- Berlaku juga untuk objek yang dibuat kemudian oleh owner ini.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO cbs_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO cbs_app;
