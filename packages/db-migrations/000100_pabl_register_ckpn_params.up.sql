-- 000100_pabl_register_ckpn_params.up.sql
-- Run after: 000099_pabl_ckpn.up.sql
--
-- Dua hal:
--   (1) Register lengkap penempatan pada bank lain: tanggal mulai, tanggal jatuh
--       tempo, dan suku bunga tahunan. Sumber kolom VI (Jangka Waktu) dan VIII
--       (Suku Bunga) Form 05.00 menurut SEOJK No. 16/SEOJK.03/2024.
--   (2) Parameter mesin kolektif PD/LGD PABL: CKPN = EAD x PD x LGD (PA BPR Bab XII
--       butir 12.4.g.2 dan 12.6-12.9). Nilai sengaja KOSONG seperti CKPN kredit:
--       bank wajib menghitungnya dari data historisnya lalu mengisinya. Run kolektif
--       menolak (bukan memakai nol) bila parameter belum diisi.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. Kolom hanya ditambahkan bila belum ada dan
-- seed konfigurasi tidak menimpa nilai bank (INSERT ... ON CONFLICT (key) DO NOTHING).

ALTER TABLE lps_placements
    ADD COLUMN IF NOT EXISTS start_date DATE,
    ADD COLUMN IF NOT EXISTS maturity_date DATE,
    ADD COLUMN IF NOT EXISTS interest_rate_annual NUMERIC(9,4);

COMMENT ON COLUMN lps_placements.start_date IS
    'Tanggal mulai penempatan; sumber kolom VI (Jangka Waktu) Form 05.00. NULL = belum diisi, ditulis "-".';
COMMENT ON COLUMN lps_placements.maturity_date IS
    'Tanggal jatuh tempo penempatan; sumber kolom VI (Jangka Waktu) Form 05.00. NULL = tanpa jatuh tempo sehingga kolom VI hanya memuat tanggal mulai.';
COMMENT ON COLUMN lps_placements.interest_rate_annual IS
    'Suku bunga tahunan penempatan dalam persen (%); sumber kolom VIII (Suku Bunga) Form 05.00, sampai 2 digit desimal. NULL = belum diisi, ditulis "-".';

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.pabl.pd_frac.gol_1', '',
     'Probability of Default kolektif PABL golongan 1 - Lancar (fraksi, mis. 0.009). Wajib diisi bank dari data historis; kosong = run kolektif ditolak.'),
    ('ckpn.pabl.pd_frac.gol_3', '',
     'Probability of Default kolektif PABL golongan 3 - Kurang Lancar (fraksi). Wajib diisi bank dari data historis; kosong = run kolektif ditolak.'),
    ('ckpn.pabl.pd_frac.gol_5', '',
     'Probability of Default kolektif PABL golongan 5 - Macet (fraksi). Wajib diisi bank dari data historis; kosong = run kolektif ditolak.'),
    ('ckpn.pabl.lgd_frac', '',
     'Loss Given Default kolektif PABL (fraksi). Wajib diisi bank; kosong = run kolektif ditolak.'),
    ('ckpn.pabl.aset_baik_bentuk_ckpn', 'false',
     'Bila true, penempatan ber-metode COLLECTIVE tetap membentuk CKPN atas outstanding penuh (ECL tahap-1). Bawaan false = dasar dikurangi bagian yang dijamin LPS yang memenuhi kriteria aset baik butir 12.3.a.1.b (12.3.a.2).')
ON CONFLICT (key) DO NOTHING;
