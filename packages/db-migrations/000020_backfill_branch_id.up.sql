-- CBS Migration 000020: Backfill branch_id untuk nasabah dan kredit
-- Run after: 000019_syariah_financing_mapping.up.sql
--
-- Sebelum ini, customers.branch_id dan loans.branch_id ada di skema tetapi tidak
-- pernah diisi service, sehingga selalu NULL. Migrasi ini mengisinya hanya bila
-- ada dasar yang jelas. Baris yang tidak punya dasar cabang yang dapat
-- dipertanggungjawabkan DIBIARKAN NULL: lebih baik cabang tidak diketahui
-- daripada mengarang cabang. Karena itu tidak ada NOT NULL constraint baru dan
-- data historis tidak dihapus. Seluruh pernyataan idempotent (WHERE ... IS NULL)
-- sehingga aman dijalankan ulang.

-- ── customers.branch_id ─────────────────────────────────────────────────────
-- Sumber 1: kode cabang CIF (customers.cif_branch_code) bila cocok dengan
-- branches.code. Nilai ini berasal dari dokumen pembukaan CIF sehingga paling
-- dapat dipercaya.
UPDATE customers c
SET branch_id = b.id,
    updated_at = NOW()
FROM branches b
WHERE c.branch_id IS NULL
  AND c.cif_branch_code IS NOT NULL
  AND btrim(c.cif_branch_code) <> ''
  AND b.code = btrim(c.cif_branch_code);

-- Sumber 2: cabang rekening pertama nasabah (accounts.branch_id paling awal
-- berdasarkan created_at). Dipakai hanya bila sumber 1 tidak mengisi, yaitu
-- nasabah tanpa cif_branch_code yang cocok. Rekening pertama dipilih
-- deterministik lewat ORDER BY created_at, id.
UPDATE customers c
SET branch_id = first_account.branch_id,
    updated_at = NOW()
FROM (
    SELECT DISTINCT ON (a.customer_id)
        a.customer_id,
        a.branch_id
    FROM accounts a
    WHERE a.customer_id IS NOT NULL
      AND a.branch_id IS NOT NULL
    ORDER BY a.customer_id, a.created_at ASC, a.id ASC
) AS first_account
WHERE c.branch_id IS NULL
  AND c.id = first_account.customer_id;

-- ── loans.branch_id ─────────────────────────────────────────────────────────
-- Kredit mengikuti cabang rekening pencairannya (disbursement_account_id).
-- Inilah cabang tempat akad dan pencairan terjadi, sehingga tidak diragukan.
UPDATE loans l
SET branch_id = a.branch_id,
    updated_at = NOW()
FROM accounts a
WHERE l.branch_id IS NULL
  AND l.disbursement_account_id = a.id
  AND a.branch_id IS NOT NULL;

-- Catatan: kredit tanpa rekening pencairan bercabang, nasabah tanpa
-- cif_branch_code yang cocok dan tanpa rekening bercabang, tetap NULL sampai ada
-- dasar cabang yang sah. Service memperlakukan baris NULL sebagai data
-- pra-migrasi: operasional atasnya TIDAK diblokir oleh pemeriksaan cabang.
