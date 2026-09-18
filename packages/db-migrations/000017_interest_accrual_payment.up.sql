-- CBS Migration 000017: Pembayaran akrual bunga tabungan ke rekening nasabah
-- Run after: 000016_deposit_penalty.up.sql
-- Idempotent: aman dijalankan ulang.

-- EOM mengakru bunga tabungan sebagai utang (payable) tanpa memindahkannya ke
-- rekening nasabah, sehingga bunga tidak pernah diterima nasabah. Kolom penanda
-- berikut memungkinkan langkah pembayaran: debit utang bunga, kredit rekening
-- nasabah, lalu tandai akrual sudah dibayar.
ALTER TABLE interest_accruals
    ADD COLUMN IF NOT EXISTS payment_journal_entry_id UUID REFERENCES journal_entries(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;

-- Pencarian akrual yang belum dibayar per periode.
CREATE INDEX IF NOT EXISTS idx_interest_accruals_unpaid
    ON interest_accruals(period) WHERE paid_at IS NULL;
