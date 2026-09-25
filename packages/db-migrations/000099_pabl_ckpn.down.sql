-- 000099_pabl_ckpn.down.sql
-- Membalik 000099_pabl_ckpn.up.sql: hapus jejak asesmen, kolom CKPN pada
-- lps_placements, dan kunci konfigurasi ckpn.pabl.enabled.

DROP TABLE IF EXISTS pabl_ckpn_assessments;

ALTER TABLE lps_placements
    DROP COLUMN IF EXISTS ckpn_assessed_at,
    DROP COLUMN IF EXISTS ckpn_individual_target,
    DROP COLUMN IF EXISTS required_ckpn,
    DROP COLUMN IF EXISTS ckpn_objective_evidence,
    DROP COLUMN IF EXISTS ckpn_significant,
    DROP COLUMN IF EXISTS ckpn_method;

DELETE FROM system_config WHERE key = 'ckpn.pabl.enabled';
