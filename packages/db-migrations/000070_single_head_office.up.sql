-- 000070_single_head_office.up.sql
-- Run after: 000068_loan_penalty_cap_and_syariah_social_fund.up.sql
-- Aditif dan idempotent: hanya menambah satu indeks unik parsial; aman dijalankan
-- ulang (IF NOT EXISTS) dan tidak mengubah baris mana pun.
--
-- Pembuatan kantor pusat lewat API sudah ditolak di lapisan service
-- (domain.ErrBranchHeadOfficeNotAllowed, is_head_office dipaksa false), tetapi
-- tidak ada penjaga di lapisan database: INSERT/UPDATE langsung masih dapat
-- menyimpan kantor pusat kedua. Pemilihan kantor pusat memakai
-- `is_head_office = TRUE ORDER BY code LIMIT 1` (customer_service.go,
-- actor_branch.go), sehingga kantor pusat kedua dapat menggeser atribusi cabang
-- nasabah/rekening/jurnal pelaku lintas cabang ke cabang yang salah.
--
-- Indeks unik parsial ini menegakkan paling banyak SATU baris dengan
-- is_head_office = TRUE. Data produksi saat ini hanya memiliki satu kantor pusat
-- (kode 001), jadi tidak melanggar.

CREATE UNIQUE INDEX IF NOT EXISTS idx_branches_single_head_office
    ON branches (is_head_office)
    WHERE is_head_office = TRUE;
