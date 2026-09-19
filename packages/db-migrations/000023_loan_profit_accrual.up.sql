-- CBS Migration 000023: Akrual pendapatan bunga kredit berbasis jadwal angsuran
-- Run after: 000022_maker_checker_branch.up.sql
-- Idempotent: aman dijalankan ulang.

-- Akrual bunga kredit mengikuti jadwal angsuran, bukan harian: begitu angsuran
-- jatuh tempo dan belum dibayar, porsi bunganya diakui sebagai pendapatan
-- (DEBIT 10400 piutang bunga / CREDIT 40100 pendapatan bunga). Nilai akruan persis
-- sama dengan porsi bunga jadwal, sehingga tidak ada selisih pembulatan.
--
-- profit_accrued_at          : penanda idempoten "angsuran ini sudah diakru".
-- profit_accrued_amount      : SISA akruan yang belum diselesaikan lewat pembayaran,
--                              bukan total akruan. Saat angsuran dibayar, kolom ini
--                              dikurangi sebesar porsi yang sudah diakru agar jurnal
--                              penyelesaian (CREDIT 10400) tidak pernah melebihi
--                              saldo piutang yang terbentuk dan 10400 tidak negatif.
--
-- Tidak ada backfill: migrasi ini tidak membuat jurnal akrual untuk periode yang
-- sudah lewat dan tidak mengisi penanda baris lama. Akruan hanya dibentuk oleh EOD
-- yang berjalan maju, dan tanggal jurnalnya adalah tanggal bisnis saat itu, bukan
-- tanggal jatuh tempo yang lampau. Dengan begitu pendapatan tidak pernah diakui surut
-- ke periode yang jurnalnya sudah ditutup.
ALTER TABLE loan_schedules
    ADD COLUMN IF NOT EXISTS profit_accrued_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS profit_accrued_amount NUMERIC(28,4) NOT NULL DEFAULT 0;

-- Melayani pencarian kandidat akrual EOD: angsuran yang sudah jatuh tempo dan belum
-- pernah diakru. Predikat parsial menjaga index tetap kecil seiring bertambahnya
-- angsuran yang sudah diakru.
CREATE INDEX IF NOT EXISTS idx_loan_schedules_profit_accrual
    ON loan_schedules(due_date)
    WHERE profit_accrued_at IS NULL AND status <> 'PAID';
