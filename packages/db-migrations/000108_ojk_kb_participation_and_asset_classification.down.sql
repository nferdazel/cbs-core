-- 000108_ojk_kb_participation_and_asset_classification.down.sql
-- Membalik 000108_ojk_kb_participation_and_asset_classification.up.sql: hapus kolom
-- sandi klasifikasi aset, lalu buang kunci saklar partisipasi bank. Idempotent
-- (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

ALTER TABLE lps_placements DROP COLUMN IF EXISTS ojk_klasifikasi_aset_code;
ALTER TABLE loans DROP COLUMN IF EXISTS ojk_klasifikasi_aset_code;

DELETE FROM system_config WHERE key IN (
    'bank.participates.kur',
    'bank.participates.lpbbti',
    'bank.participates.laku_pandai');
