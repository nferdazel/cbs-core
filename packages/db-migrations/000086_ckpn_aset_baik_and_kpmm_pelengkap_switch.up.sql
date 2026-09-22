-- 000086_ckpn_aset_baik_and_kpmm_pelengkap_switch.up.sql
-- Run after: 000085_write_off_amount_and_recovery_threshold.up.sql
--
-- Dua saklar kebijakan yang diminta keputusan auditor KPMM/CKPN:
--
-- 1. `ckpn.aset_baik.bentuk_ckpn` — aset baik (tunggakan <= ambang dan tidak pernah
--    direstrukturisasi; PA BPR butir 12.3.a.1.c) BOLEH tidak dibentuk CKPN
--    (butir 12.3.a.2.a). Namun auditor/bank yang menuntut ECL tahap-1 SAK EP dapat
--    memilih tetap membentuknya. Nilai awal `false` = perilaku sekarang persis
--    (aset baik dikecualikan); `true` = CKPN tahap-1 tetap dibentuk memakai PD/LGD
--    golongan lancar.
--
-- 2. `kpmm.modal_pelengkap_instrumen_max_frac` — sub-batas modal pelengkap
--    ber-instrumen (komponen dengan persetujuan OJK) paling tinggi 50% modal inti
--    (POJK No. 5/POJK.03/2015 Pasal 10 ayat (2)). Terpisah dari batas total modal
--    pelengkap 100% modal inti pada `kpmm.modal_pelengkap_max_frac` (Pasal 3 ayat (2)).
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT (INSERT ... ON CONFLICT (key) DO NOTHING).
-- Tidak mengubah nilai kunci apa pun, tidak menyentuh tabel, jurnal, maupun
-- `ckpn.enabled`.

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.aset_baik.bentuk_ckpn', 'false',
     'Saklar kebijakan aset baik (boolean): false = aset baik boleh tidak dibentuk CKPN (PA BPR butir 12.3.a.2.a; perilaku sekarang, nilai awal); true = CKPN tahap-1 tetap dibentuk atas aset baik memakai PD golongan lancar dan ckpn.lgd_frac (ECL tahap 1 SAK EP).'),
    ('kpmm.modal_pelengkap_instrumen_max_frac', '0.50',
     'Batas atas komponen modal pelengkap ber-instrumen (persetujuan OJK) terhadap modal inti (fraksi 0..1; 0.50 = 50%). Sumber: POJK No. 5/POJK.03/2015 Pasal 10 ayat (2). Batas total modal pelengkap 100% modal inti ada pada kpmm.modal_pelengkap_max_frac (Pasal 3 ayat (2)).')
ON CONFLICT (key) DO NOTHING;
