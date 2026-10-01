-- CBS Migration 000132: kolom cabang pada audit_logs (Form audit lintas-cabang).
--
-- Latar: GET /audit-logs sebelumnya mengembalikan seluruh bank ke peran yang secara
-- desain terbatas cabang (ADMIN/SUPERVISOR; IsCrossBranch hanya SUPERADMIN/AUDITOR/
-- SYSTEM). audit_logs tidak punya kolom cabang, sehingga filter per cabang tidak mungkin.
-- Migrasi ini menambahkan kolom branch_code agar penulisan audit baru mencatat cabang
-- pelaku dan pembacaan dapat difilter memakai cakupan unit aktor (branchCodeReadClause).
--
-- Backfill: TIDAK ada. Log lama tidak menyimpan cabang dan tidak boleh ditebak dari
-- IP/aktor (bisa salah). Baris lama bernilai NULL dan oleh klausa filter tetap
-- ditampilkan (dianggap "cabang tidak diketahui"), sehingga audit lama tidak hilang.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT.

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS branch_code VARCHAR(32);

COMMENT ON COLUMN audit_logs.branch_code IS
    'Kode cabang pelaku saat peristiwa dicatat; NULL untuk log lama (pra-000132) atau aksi sistem yang tidak terikat satu cabang. Filter GET /audit-logs menampilkan baris NULL apa adanya (cabang tidak diketahui), bukan menyembunyikannya.';

CREATE INDEX IF NOT EXISTS idx_audit_branch_code ON audit_logs (branch_code);
