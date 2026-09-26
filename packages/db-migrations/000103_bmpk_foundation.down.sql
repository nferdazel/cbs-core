-- 000103_bmpk_foundation.down.sql
-- Membalik 000103_bmpk_foundation.up.sql: hapus tautan penempatan, batas, dan
-- penandaan pihak terkait BMPK, lalu buang kunci konfigurasinya.
-- idempotent (IF EXISTS) dan tidak menyentuh tabel/kolom lain.

DROP INDEX IF EXISTS idx_lps_placements_customer_id;
ALTER TABLE lps_placements DROP COLUMN IF EXISTS customer_id;

DROP TABLE IF EXISTS bmpk_limits;
DROP TABLE IF EXISTS bmpk_related_parties;

DELETE FROM system_config WHERE key = 'bmpk.enabled';
