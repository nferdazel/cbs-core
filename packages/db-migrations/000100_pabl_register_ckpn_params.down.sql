-- 000100_pabl_register_ckpn_params.down.sql
-- Membalik 000100_pabl_register_ckpn_params.up.sql: hapus seed parameter mesin
-- kolektif PABL dan kolom register (tanggal mulai/jatuh tempo, suku bunga tahunan).

DELETE FROM system_config WHERE key IN (
    'ckpn.pabl.pd_frac.gol_1',
    'ckpn.pabl.pd_frac.gol_3',
    'ckpn.pabl.pd_frac.gol_5',
    'ckpn.pabl.lgd_frac',
    'ckpn.pabl.aset_baik_bentuk_ckpn'
);

ALTER TABLE lps_placements
    DROP COLUMN IF EXISTS interest_rate_annual,
    DROP COLUMN IF EXISTS maturity_date,
    DROP COLUMN IF EXISTS start_date;
