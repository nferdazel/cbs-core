-- CBS Migration 000042: kerangka CKPN (Cadangan Kerugian Penurunan Nilai)
-- Run after: 000041 (nomor dipesan; migrasi ini tidak bergantung isinya selain urutan).
--
-- Dasar dokumen: SEOJK No. 21/SEOJK.03/2024 tentang Panduan Akuntansi Perbankan bagi
-- Bank Perekonomian Rakyat (PA BPR), yang mulai berlaku 1 Januari 2025.
--   - Bab XII butir 12.5 dan 12.10: jurnal pembentukan CKPN adalah
--     Db. Beban kerugian penurunan nilai; Kr. CKPN. Karena itu posting_event
--     CKPN_PROVISION/CKPN_REVERSAL ditambahkan, terpisah dari PPAP_PROVISION/
--     PPAP_REVERSAL.
--   - Butir 1.1.6: PPKA dihitung menurut POJK kualitas aset; bila PPKA lebih besar
--     dari CKPN, selisihnya menjadi pengurang modal inti. Itu sebabnya COA CKPN
--     dibuat terpisah dari COA PPAP (10900/50200) agar keduanya dapat dibandingkan.
--
-- PENTING soal ALTER TYPE ... ADD VALUE: migrate.sh menjalankan satu file dalam SATU
-- transaksi (-1). Sejak PostgreSQL 12 ADD VALUE diperbolehkan di dalam transaksi,
-- TETAPI nilai barunya tidak boleh dipakai sebelum transaksi itu commit. Karena itu
-- file ini tidak menanam baris product_journal_mapping untuk peristiwa CKPN_*;
-- pemetaan itu menjadi tugas bank (lewat UI/konfigurasi atau migrasi berikutnya).
-- Service CKPN mencoba pemetaan produk lebih dulu, lalu jatuh ke COA konfigurasi
-- ckpn.coa.expense/ckpn.coa.reserve yang di-seed di bawah. Preseden ALTER TYPE yang
-- sama dipakai migrasi 000004, 000037, dan 000040.
--
-- Aditif dan idempotent: ADD VALUE IF NOT EXISTS, ADD COLUMN IF NOT EXISTS,
-- ON CONFLICT DO NOTHING. Saklar ckpn.enabled dibiarkan 'false' agar tidak ada
-- perubahan perilaku sampai bank mengisi parameter kebijakannya.

-- 1. Peristiwa jurnal CKPN (SAK EP via PA BPR Bab XII).
ALTER TYPE posting_event ADD VALUE IF NOT EXISTS 'CKPN_PROVISION';
ALTER TYPE posting_event ADD VALUE IF NOT EXISTS 'CKPN_REVERSAL';

-- 2. Target CKPN per kredit. Analog loans.required_ppap: menyimpan target terakhir
--    yang diakui agar selisih target-harapan (pembentukan/pemulihan) dapat dihitung
--    idempoten antar run. Bawaan 0 = belum ada CKPN yang diakui.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS required_ckpn NUMERIC(28,4) NOT NULL DEFAULT 0.0000;

COMMENT ON COLUMN loans.required_ckpn IS
    'Target CKPN per kredit yang terakhir diakui (SAK EP via SEOJK 21/2024 Bab XII). Terpisah dari required_ppap karena CKPN dan PPKA adalah dua dasar yang berbeda.';

-- 3. COA CKPN, terpisah dari COA PPAP. Akun CKPN adalah contra-asset (normal CREDIT)
--    dan bebannya adalah beban operasional (normal DEBIT), sesuai butir 12.5.
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a1000000-0000-0000-0000-000000000014', '10950', 'CKPN - Kredit',                              'ASSET',   'CREDIT', 'CONVENTIONAL', FALSE),
    ('a5000000-0000-0000-0000-000000000008', '50301', 'Beban Kerugian Penurunan Nilai - Kredit',   'EXPENSE', 'DEBIT',  'CONVENTIONAL', FALSE),
    ('b1000000-0000-0000-0000-000000000011', '11950', 'CKPN - Pembiayaan',                         'ASSET',   'CREDIT', 'SYARIAH',      FALSE),
    ('b5000000-0000-0000-0000-000000000005', '15901', 'Beban Kerugian Penurunan Nilai - Pembiayaan','EXPENSE','DEBIT',  'SYARIAH',      FALSE)
ON CONFLICT (code) DO NOTHING;

-- 4. Akun GL internal untuk COA CKPN. Konvensi repo: nomor akun sama dengan kode COA.
INSERT INTO accounts (id, account_number, customer_id, coa_id, account_type, currency, balance, available_balance, status, opened_at) VALUES
    ('c0000000-0000-0000-0000-000000000001', '10950', NULL, 'a1000000-0000-0000-0000-000000000014', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('c0000000-0000-0000-0000-000000000002', '50301', NULL, 'a5000000-0000-0000-0000-000000000008', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('c0000000-0000-0000-0000-000000000003', '11950', NULL, 'b1000000-0000-0000-0000-000000000011', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('c0000000-0000-0000-0000-000000000004', '15901', NULL, 'b5000000-0000-0000-0000-000000000005', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW())
ON CONFLICT (account_number) DO NOTHING;

-- 5. Konfigurasi kebijakan CKPN. PD dan LGD sengaja KOSONG: bank wajib menghitungnya
--    dari data historisnya (PA BPR butir 12.4.g.2.c dan 12.6-12.7) lalu mengisinya.
--    Service menolak menghitung kredit yang membutuhkan parameter kosong ini dengan
--    ErrCKPNParameterMissing, bukan memakai nol diam-diam.
INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.enabled', 'false',
     'Aktifkan perhitungan CKPN. false = tidak ada perhitungan, posting, atau query tambahan (perilaku sebelum modul CKPN).'),
    ('ckpn.pd_frac.gol_1', '',
     'Probability of Default golongan 1 - Lancar (fraksi, mis. 0.009). Wajib diisi bank dari data historis; kosong = perhitungan kredit itu ditolak.'),
    ('ckpn.pd_frac.gol_2', '',
     'Probability of Default golongan 2 - Dalam Perhatian Khusus (fraksi). Wajib diisi bank dari data historis.'),
    ('ckpn.pd_frac.gol_3', '',
     'Probability of Default golongan 3 - Kurang Lancar (fraksi). Wajib diisi bank dari data historis.'),
    ('ckpn.pd_frac.gol_4', '',
     'Probability of Default golongan 4 - Diragukan (fraksi). Wajib diisi bank dari data historis.'),
    ('ckpn.pd_frac.gol_5', '',
     'Probability of Default golongan 5 - Macet (fraksi). Wajib diisi bank dari data historis.'),
    ('ckpn.lgd_frac', '',
     'Loss Given Default (fraksi). Boleh satu nilai all account bila data tidak mendukung pengelompokan (PA BPR butir 12.7.a). Wajib diisi bank; kosong = perhitungan ditolak.'),
    ('ckpn.aset_baik.max_dpd', '7',
     'Batas tunggakan hari agar aset memenuhi kriteria aset baik (PA BPR butir 12.3.a.1.c): tidak menunggak lebih dari 7 hari dan tidak pernah direstrukturisasi.'),
    ('ckpn.coa.expense', '',
     'Kode COA beban kerugian penurunan nilai CKPN. Kosong = fallback per buku: 50301 konvensional, 15901 syariah (di-seed migrasi 000042).'),
    ('ckpn.coa.reserve', '',
     'Kode COA cadangan CKPN. Kosong = fallback per buku: 10950 konvensional, 11950 syariah (di-seed migrasi 000042).')
ON CONFLICT (key) DO NOTHING;
