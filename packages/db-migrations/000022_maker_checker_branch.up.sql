-- CBS Migration 000022: Atribusi cabang pada permintaan maker-checker
-- Run after: 000021_journal_entry_branch.up.sql
--
-- Latar belakang:
--   Tabel maker_checker_requests tidak menyimpan cabang pengajuan. Akibatnya
--   daftar pending yang dibaca seorang supervisor menampilkan pengajuan dari
--   seluruh bank, dan ia dapat menyetujui pengajuan cabang lain. Penegakan
--   cabang pada jalur tulis dan master data sudah ada (Actor.CanAccessBranch);
--   migrasi ini menambahkan dimensi cabang yang sama ke maker-checker.
--
--   Kolom dibuat nullable: pengajuan lama yang tidak punya dasar cabang yang sah
--   dibiarkan NULL dan TETAP terlihat oleh semua cabang, konsisten kebijakan
--   migrasi 000020/000021. Tidak ada NOT NULL baru dan tidak ada data historis
--   yang dihapus. Seluruh pernyataan idempotent sehingga aman dijalankan ulang.

ALTER TABLE maker_checker_requests
    ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id);

-- Query ListPending memfilter branch_id lalu status PENDING dan mengurutkan
-- created_at. Index gabungan ini melayani pembacaan antrean per cabang.
CREATE INDEX IF NOT EXISTS idx_maker_checker_branch_status_created
    ON maker_checker_requests(branch_id, status, created_at DESC);

-- ── Backfill branch_id dari payload JSONB ───────────────────────────────────
-- Aturan deterministik, dipakai berurutan; cabang pertama yang cocok menang:
--   1. payload->>'account_number' → accounts.branch_id. Seluruh pengajuan
--      maker-checker transaksi rekening (setoran, penarikan, transfer) memuat
--      nomor rekening ini; transfer memakai rekening sumber, yang cabangnya
--      sama dengan cabang transaksi.
--   2. payload->>'loan_id' → loans.branch_id. Menyiapkan pengajuan berbasis
--      kredit yang mungkin ditambahkan kelak.
--   3. payload->>'id' → loans.branch_id. Cadangan bila payload lama memakai
--      kunci "id" alih-alih "loan_id".
--
-- Perbandingan id kredit memakai cast teks (l.id::text) agar payload yang bukan
-- UUID tidak membuat migrasi gagal. Pengajuan yang tidak punya satu pun dasar
-- cabang DIBIARKAN NULL: lebih baik cabang tidak diketahui daripada mengarang
-- cabang. Ini mengikuti kebijakan migrasi 000020/000021; baris NULL tetap
-- terlihat oleh semua cabang, sama seperti semantik Actor.CanAccessBranch.
UPDATE maker_checker_requests mcr
SET branch_id = src.branch_id
FROM (
    SELECT m.id,
           COALESCE(
               (SELECT a.branch_id FROM accounts a
                 WHERE a.account_number = m.payload->>'account_number'
                   AND a.branch_id IS NOT NULL
                 LIMIT 1),
               (SELECT l.branch_id FROM loans l
                 WHERE l.id::text = m.payload->>'loan_id'
                   AND l.branch_id IS NOT NULL
                 LIMIT 1),
               (SELECT l.branch_id FROM loans l
                 WHERE l.id::text = m.payload->>'id'
                   AND l.branch_id IS NOT NULL
                 LIMIT 1)
           ) AS branch_id
    FROM maker_checker_requests m
    WHERE m.branch_id IS NULL
) AS src
WHERE mcr.id = src.id
  AND src.branch_id IS NOT NULL;
