-- CBS Migration 000021: Atribusi cabang pada jurnal (journal_entries.branch_id)
-- Run after: 000020_backfill_branch_id.up.sql
--
-- Latar belakang:
--   PostingMeta.BranchCode sudah diisi pemanggil dan diteruskan ke
--   domain.PostingRequest.BranchCode, tetapi InsertJournal tidak pernah
--   menyimpannya. Akibatnya jurnal tidak dapat difilter per cabang dan pegawai
--   satu cabang (mis. TELLER dengan PermLedgerRead) dapat membaca seluruh
--   transaksi bank lewat daftar jurnal.
--
--   Kolom dibuat nullable: jurnal sistem/batch yang memang bank-wide tetap boleh
--   tanpa cabang, dan baris lama yang tidak punya dasar cabang yang sah dibiarkan
--   NULL. Tidak ada NOT NULL baru. Seluruh pernyataan idempotent sehingga aman
--   dijalankan ulang.

ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id);

-- Query daftar jurnal memfilter branch_id dan mengurutkan posted_at DESC.
-- Index gabungan ini melayani pencarian per cabang beserta urutan waktu posting.
CREATE INDEX IF NOT EXISTS idx_journal_branch_posted_at
    ON journal_entries(branch_id, posted_at DESC);

-- Laporan periode memakai entry_date; index ini menyiapkan pembacaan per cabang
-- pada rentang tanggal akuntansi bila kelak diperlukan.
CREATE INDEX IF NOT EXISTS idx_journal_branch_entry_date
    ON journal_entries(branch_id, entry_date);

-- ── Backfill branch_id dari baris jurnal ────────────────────────────────────
-- Aturan deterministik: untuk setiap entri, pilih SATU baris jurnal yang punya
-- cabang. Prioritas pertama adalah akun non-GL (rekening nasabah), karena akun
-- GL internal sering bank-wide dan cabangnya kosong; bila tidak ada, ambil baris
-- pertama yang punya cabang. Urutan dalam kelompok mengikuti sequence lalu id
-- baris, sehingga hasilnya stabil saat migrasi dijalankan ulang.
--
-- Entri yang tidak punya satu pun baris bercabang DIBIARKAN NULL: lebih baik
-- cabang tidak diketahui daripada mengarang cabang.
UPDATE journal_entries je
SET branch_id = picked.branch_id
FROM (
    SELECT DISTINCT ON (jl.journal_entry_id)
        jl.journal_entry_id,
        a.branch_id
    FROM journal_lines jl
    JOIN accounts a ON a.id = jl.account_id
    WHERE a.branch_id IS NOT NULL
    ORDER BY
        jl.journal_entry_id,
        CASE WHEN a.account_type = 'INTERNAL_GL' THEN 1 ELSE 0 END,
        jl.sequence ASC,
        jl.id ASC
) AS picked
WHERE je.id = picked.journal_entry_id
  AND je.branch_id IS NULL
  AND picked.branch_id IS NOT NULL;
