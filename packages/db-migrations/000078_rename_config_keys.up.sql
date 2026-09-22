-- 000078_rename_config_keys.up.sql
-- Run after: 000077_eod_orchestration.up.sql
--
-- Menyelaraskan nama kunci system_config dengan konvensi W3: huruf kecil, pemisah
-- titik, minimal dua segmen, SATUAN EKSPLISIT untuk nilai bersatuan
-- (_pct / _frac / _per_mille / _amount / _days), dan INDEKS MEMAKAI NAMA
-- (gol_1, bukan 1).
--
-- Latar: nama kunci lama membiarkan satuan tidak eksplisit sehingga bank dapat salah
-- mengisi (mis. tax.deposit.rate diisi 0,2 padahal mesin membaca 20 sebagai persen).
-- Beberapa keluarga juga memakai indeks angka (ckpn.pd.1, ppap.rate.1) yang tidak
-- menjelaskan golongan kolektibilitas yang dimaksud.
--
-- Sifat migrasi:
--   - ADITIF dan IDEMPOTENT: UPDATE mengubah nama kunci saja. Kunci yang belum pernah
--     ada di suatu instalasi tidak menghasilkan baris sampah (UPDATE 0 baris), dan
--     kunci bank yang sudah disesuaikan tidak tertimpa.
--   - SATU kunci satu pernyataan UPDATE. Nilai, deskripsi, updated_by, dan updated_at
--     tidak disentuh: rename key memindahkan seluruh baris apa adanya sehingga
--     deskripsi kunci (termasuk penjelasan FRAKSI dari migrasi 000067) tidak hilang.
--   - Nilai TIDAK diubah dan perilaku perhitungan TIDAK diubah.
--
-- Kode pembaca ikut dirilis bersama migrasi ini; tidak ada alias kunci lama yang
-- dipertahankan. Bila migrasi belum diterapkan, pembaca limit menolak transaksi
-- (gagal berisik), bukan jatuh ke batas bawaan.

-- 1. Keluarga PD/LGD CKPN dan tarif PPAP: satuan FRAKSI 0..1 dan indeks golongan
--    kolektibilitas memakai nama (gol_1..gol_5).
UPDATE system_config SET key = 'ckpn.pd_frac.gol_1' WHERE key = 'ckpn.pd.1';
UPDATE system_config SET key = 'ckpn.pd_frac.gol_2' WHERE key = 'ckpn.pd.2';
UPDATE system_config SET key = 'ckpn.pd_frac.gol_3' WHERE key = 'ckpn.pd.3';
UPDATE system_config SET key = 'ckpn.pd_frac.gol_4' WHERE key = 'ckpn.pd.4';
UPDATE system_config SET key = 'ckpn.pd_frac.gol_5' WHERE key = 'ckpn.pd.5';
UPDATE system_config SET key = 'ppap.rate_frac.gol_1' WHERE key = 'ppap.rate.1';
UPDATE system_config SET key = 'ppap.rate_frac.gol_2' WHERE key = 'ppap.rate.2';
UPDATE system_config SET key = 'ppap.rate_frac.gol_3' WHERE key = 'ppap.rate.3';
UPDATE system_config SET key = 'ppap.rate_frac.gol_4' WHERE key = 'ppap.rate.4';
UPDATE system_config SET key = 'ppap.rate_frac.gol_5' WHERE key = 'ppap.rate.5';
UPDATE system_config SET key = 'ckpn.lgd_frac' WHERE key = 'ckpn.lgd';

-- 2. Tarif/rasio yang nilainya PERSEN per tahun (mesin membagi 100 lebih dulu):
--    satuan _pct ditulis eksplisit.
UPDATE system_config SET key = 'tax.deposit.rate_pct' WHERE key = 'tax.deposit.rate';
UPDATE system_config SET key = 'tax.savings.rate_pct' WHERE key = 'tax.savings.rate';
UPDATE system_config SET key = 'loan.penalty.cap_pct' WHERE key = 'loan.penalty.cap.percent';
UPDATE system_config SET key = 'savings.interest.rate_annual_pct' WHERE key = 'savings.interest.rate_annual';
UPDATE system_config SET key = 'loan.restructure.loss.discount_rate_annual_pct' WHERE key = 'loan.restructure.loss.discount_rate_annual';
UPDATE system_config SET key = 'deposit.mudharabah.yield_annual_pct' WHERE key = 'deposit.mudharabah.yield_annual';

-- 3. Haircut agunan: nilainya persen 0..100 (HaircutPercent), satuan _pct ditulis
--    eksplisit. 100 berarti nilai taksasi tidak menjadi pengurang.
UPDATE system_config SET key = 'collateral.haircut.deposit_pct' WHERE key = 'collateral.haircut.deposit';
UPDATE system_config SET key = 'collateral.haircut.kendaraan_pct' WHERE key = 'collateral.haircut.kendaraan';
UPDATE system_config SET key = 'collateral.haircut.lainnya_pct' WHERE key = 'collateral.haircut.lainnya';
UPDATE system_config SET key = 'collateral.haircut.mesin_peralatan_pct' WHERE key = 'collateral.haircut.mesin_peralatan';
UPDATE system_config SET key = 'collateral.haircut.tanah_bangunan_pct' WHERE key = 'collateral.haircut.tanah_bangunan';
