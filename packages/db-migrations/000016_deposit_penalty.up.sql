-- CBS Migration 000016: Denda pencairan lebih awal deposito berjangka
-- Run after: 000015_batch_closing_interest.up.sql
-- Idempotent: aman dijalankan ulang.

-- Kolom pencatat denda yang dipotong saat deposito dicairkan sebelum jatuh tempo.
-- Nilai 0 berarti pencairan normal (saat/setelah jatuh tempo) atau produk tanpa
-- tarif denda.
ALTER TABLE deposits
    ADD COLUMN IF NOT EXISTS early_withdrawal_penalty NUMERIC(28,4) NOT NULL DEFAULT 0.0000;

-- Akun pendapatan denda dapat ditimpa lewat konfigurasi. Migrasi 000005 hanya
-- menyediakan 40500 "Pendapatan Denda" untuk buku konvensional; buku syariah
-- belum punya akun denda baku, sehingga operator UUS wajib mengisi key ini agar
-- denda syariah dapat diposting (lihat service depositPenaltyCOA).
INSERT INTO system_config (key, value, description) VALUES
    ('deposit.penalty.income.coa', '', 'Override COA pendapatan denda pencairan deposito; kosong = default 40500 (konvensional)')
ON CONFLICT (key) DO NOTHING;
