-- 000081_branch_hierarchy.up.sql
-- Run after: 000080_fix_config_unit_and_stale_thresholds.up.sql
--
-- ALASAN (keputusan pemilik sistem, docs/BACKLOG.md item 25/28; W14):
-- sistem hanya mengenal `branches` + kantor pusat. Pemilik memutuskan hierarki
-- organisasi OPSIONAL tiga jenjang (cabang -> area -> wilayah) dan menegaskan
-- area/wilayah KEDUANYA OPSIONAL: bank tanpa area/wilayah harus berjalan persis
-- seperti sekarang, bank yang punya dapat menyalakannya.
--
-- PILIHAN RANCANGAN: memperluas tabel `branches` yang SUDAH ADA dengan dua kolom
-- relasi eksplisit (parent_id + unit_level), bukan membuat tabel jenjang generik
-- atau tabel unit organisasi kedua. Alasan:
--   1. Satu tempat tetap menjadi jawaban "unit apa yang boleh saya lihat/proses":
--      cakupan cabang (branchReadClause / Actor.CanAccessBranch) sudah bertumpu
--      pada `branches`; tabel terpisah akan memaksa rekonsiliasi dua entitas.
--   2. Bank tanpa area/wilayah tidak berubah sama sekali: parent_id NULL dan
--      unit_level 'CABANG' adalah nilai lama, sehingga seluruh query dan uji
--      yang ada tetap berlaku tanpa cabang kode baru.
--   3. Relasi eksplisit (bukan tabel jenjang generik) cukup untuk tiga jenjang
--      tetap; jenjang generik menambah tabel+join tanpa memberi kemampuan yang
--      belum dipakai, dan memperbesar permukaan kebocoran cakupan.
--
-- Konsekuensi terhadap query yang ada: `branches` kini boleh memuat unit non
-- operasional (AREA/WILAYAH). Unit itu tidak dipakai untuk penomoran rekening
-- (kode cabang 3 digit tetap syarat cabang operasional) dan disaring di tempat
-- yang memang menuntut cabang operasional. Operator hanya melihat unit sesuai
-- izinnya; data dengan branch_id ke area tidak diciptakan jalur lama.
--
-- Sifat: aditif, idempotent (dijalankan ulang aman), tidak mengubah angka
-- akuntansi, tidak membuat unit apa pun secara otomatis. Tidak ada DML yang
-- menyentuh baris `branches` lama.

-- 1. Jenjang unit organisasi. CABANG = unit operasional (perilaku lama),
--    AREA membawahi beberapa cabang, WILAYAH membawahi area/cabang.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'org_unit_level') THEN
        CREATE TYPE org_unit_level AS ENUM ('CABANG', 'AREA', 'WILAYAH');
    END IF;
END $$;

-- 2. Relasi atasan opsional + jenjang. parent_id NULL berarti unit puncak;
--    untuk CABANG lama nilainya NULL sehingga tak ada perilaku yang berubah.
ALTER TABLE branches
    ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES branches(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS unit_level org_unit_level NOT NULL DEFAULT 'CABANG';

-- 3. Unit tidak boleh menjadi atasannya sendiri. Constraint terpisah agar pesan
--    galatnya jelas dan tetap idempotent (CREATE CONSTRAINT tidak punya IF NOT EXISTS).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'branches_parent_not_self'
    ) THEN
        ALTER TABLE branches
            ADD CONSTRAINT branches_parent_not_self CHECK (parent_id IS NULL OR parent_id <> id);
    END IF;
END $$;

-- 4. Indeks untuk resolusi cakupan menurun (rekursif) wilayah/area -> cabang.
CREATE INDEX IF NOT EXISTS idx_branches_parent_id ON branches(parent_id);

COMMENT ON COLUMN branches.parent_id IS
    'Unit atasan (nullable). NULL = unit puncak. Dipakai cakupan organisasi menurun: area/wilayah mencakup cabang di bawahnya. Bank tanpa area/wilayah membiarkannya NULL.';
COMMENT ON COLUMN branches.unit_level IS
    'Jenjang unit: CABANG (operasional, penomoran rekening), AREA, atau WILAYAH. DEFAULT CABANG agar baris lama tidak berubah. Area/wilayah opsional.';
COMMENT ON COLUMN branches.code IS
    'Kode unik unit. Untuk unit_level CABANG wajib tepat 3 digit (dipakai awalan nomor rekening); area/wilayah hanya butuh kode unik non-kosong.';
