-- 000109_ojk_form06_agunan_ppka_kelonggaran.down.sql
-- Membalik 000109_ojk_form06_agunan_ppka_kelonggaran.up.sql: hapus dua kolom penyimpanan
-- kolom Form 06.00 (XXV nilai agunan PPKA, XXVI kelonggaran tarik). Idempotent (IF EXISTS)
-- dan tidak menyentuh tabel/kolom lain.

ALTER TABLE loans
    DROP COLUMN IF EXISTS ojk_agunan_ppka_amount,
    DROP COLUMN IF EXISTS ojk_kelonggaran_tarik_amount;
