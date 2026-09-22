-- 000069_ckpn_coa_config.up.sql
-- Run after: 000068_loan_penalty_cap_and_syariah_social_fund.up.sql
--
-- Mengisi kunci COA penjurnalan CKPN yang sengaja dibiarkan kosong oleh migrasi
-- 000042, yaitu:
--   - ckpn.coa.expense = 50301 "Beban Kerugian Penurunan Nilai - Kredit"
--     (EXPENSE, DEBIT, buku CONVENTIONAL). Sisi DEBIT jurnal pembentukan CKPN.
--   - ckpn.coa.reserve = 10950 "CKPN - Kredit"
--     (ASSET contra, CREDIT, buku CONVENTIONAL). Sisi KREDIT jurnal pembentukan
--     CKPN, dan sisi DEBIT jurnal pemulihan.
-- Keduanya di-seed (chart_of_accounts + accounts GL internal) oleh migrasi 000042;
-- di sini hanya konfigurasi pemetaannya yang diisi, bukan akun baru.
--
-- Dasar dokumen: SEOJK No. 21/SEOJK.03/2024 Bab XII butir 12.5 dan 12.10 (jurnal
-- pembentukan Db. Beban kerugian penurunan nilai; Kr. CKPN) serta butir 1.1.6
-- (selisih PPKA-CKPN sebagai pengurang modal inti). Kode CKPN sengaja dipisah dari
-- kode PPAP (50200/10900) agar kedua dasar tidak saling mengurangi.
--
-- Latar: ckpn_service.go membaca kedua kunci ini. Selama kosong, ia jatuh ke fallback
-- per buku (50301/10950 konvensional, 15901/11950 syariah). Mengisinya membuat
-- pemetaan eksplisit dan terlihat bank, sesuai maksud seed 000042.
--
-- Idempotent: aman dijalankan ulang. Nilai hanya diisi bila masih KOSONG, sehingga
-- pemetaan yang sudah disesuaikan operator/laporan bank TIDAK ditimpa. Deskripsi
-- ikut diperbarui agar kegunaan akun terbaca.
--
-- Catatan cakupan: kunci ini GLOBAL, bukan per buku. Nilai di atas adalah akun buku
-- KONVENSIONAL. Buku syariah tetap memakai pemetaan produk bila produknya dipetakan;
-- bila tidak, kunci global ini akan menunjuk akun konvensional. Bank yang melayani
-- buku syariah harus meninjau dan (bila perlu) memetakan jurnal CKPN per produknya.

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.coa.expense', '50301',
     'Kode COA beban kerugian penurunan nilai CKPN (sisi debit pembentukan; sisi kredit pemulihan). Diisi 50301 "Beban Kerugian Penurunan Nilai - Kredit" dari seed migrasi 000042. Kunci global (bukan per buku); buku syariah harus memetakan jurnal CKPN per produk bila memakai akun berbeda.'),
    ('ckpn.coa.reserve', '10950',
     'Kode COA cadangan CKPN (sisi kredit pembentukan; sisi debit pemulihan). Diisi 10950 "CKPN - Kredit" dari seed migrasi 000042. Kunci global (bukan per buku); buku syariah harus memetakan jurnal CKPN per produk bila memakai akun berbeda.')
ON CONFLICT (key) DO UPDATE
    SET value = EXCLUDED.value,
        description = EXCLUDED.description
    WHERE system_config.value = '';
