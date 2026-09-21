-- CBS Migration 000043: kerugian restrukturisasi kredit (Pasal 32 POJK No. 1 Tahun 2024)
-- Run after: 000042_ckpn.up.sql (nomor dipesan; migrasi ini tidak bergantung isinya).
-- Aditif dan idempotent: ADD VALUE IF NOT EXISTS, ADD COLUMN IF NOT EXISTS,
-- ON CONFLICT DO NOTHING. Aman dijalankan ulang.
--
-- Dasar dokumen (dibaca dari berkas resmi, bukan ingatan):
--   - POJK No. 1 Tahun 2024 Pasal 32: "BPR wajib menerapkan perlakuan akuntansi
--     Restrukturisasi Kredit sesuai dengan standar akuntansi keuangan dan pedoman
--     akuntansi bagi BPR."
--   - SEOJK No. 21/SEOJK.03/2024 (Panduan Akuntansi Perbankan BPR, PA BPR) Bab 5.2
--     hlm. 60: "Selisih kurang antara perubahan estimasi arus kas atas Restrukturisasi
--     Kredit dibandingkan dengan nilai tercatat diperhitungkan sebagai kerugian kredit."
--   - PA BPR hlm. 60-61: nilai kini dihitung dengan tingkat diskonto suku bunga efektif
--     orisinal (mengacu SAK EP paragraf 11.20); ilustrasi jurnalnya
--     Db. Beban kerugian penurunan nilai; Kr. Kredit yang diberikan.
--   - PA BPR hlm. 42-43: biaya transaksi dan provisi ikut diperhitungkan dalam estimasi
--     arus kas saat menghitung suku bunga efektif. Catatan penting: DisburseLoan sistem
--     ini TIDAK memotong biaya apa pun, sehingga EIR tersimpan murni berasal dari jadwal
--     angsuran (dicatat pada jsonb basis, apa adanya, bukan dikarang).
--   - PA BPR hlm. 144-145 butir 22.2.2.a.2: "beban kerugian restrukturisasi kredit"
--     disajikan sebagai pos tersendiri, terpisah dari "beban kerugian penurunan nilai"
--     (CKPN). Karena itu COA-nya pun dipisah dari 50200 (PPAP) dan 50301 (CKPN).
--
-- PENTING soal ALTER TYPE ... ADD VALUE: migrate.sh menjalankan satu file dalam SATU
-- transaksi (-1). Sejak PostgreSQL 12 ADD VALUE diperbolehkan di dalam transaksi,
-- TETAPI nilai barunya tidak boleh dipakai sebelum transaksi itu commit. Karena itu file
-- ini TIDAK menanam baris product_journal_mapping untuk LOAN_RESTRUCTURE_LOSS; pemetaan
-- itu menjadi tugas bank (lewat UI/konfigurasi atau migrasi berikutnya). Service mencoba
-- pemetaan produk lebih dulu, lalu jatuh ke COA konfigurasi yang di-seed di bawah.
-- Preseden ALTER TYPE yang sama dipakai migrasi 000004, 000037, 000040, dan 000042.

-- 1. Peristiwa jurnal kerugian restrukturisasi (Pasal 32 POJK 1/2024 jo. PA BPR Bab 5.2).
ALTER TYPE posting_event ADD VALUE IF NOT EXISTS 'LOAN_RESTRUCTURE_LOSS';

-- 2. Suku bunga efektif orisinal. Dihitung saat pencairan dari arus kas nyata dan
--    dipakai sebagai tingkat diskonto nilai kini arus kas hasil restrukturisasi.
--    Bawaan 0 = belum tersimpan; perhitungan kerugian MENOLAK dengan pesan jelas
--    daripada memakai suku bunga kontraktual.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS original_eir_monthly NUMERIC(18,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS original_eir_method TEXT,
    ADD COLUMN IF NOT EXISTS original_eir_basis JSONB,
    ADD COLUMN IF NOT EXISTS original_eir_calculated_at TIMESTAMPTZ;

COMMENT ON COLUMN loans.original_eir_monthly IS
    'Suku bunga efektif orisinal per bulan (fraksi) dari arus kas nyata saat pencairan; tingkat diskonto nilai kini arus kas restrukturisasi (PA BPR Bab 5.2, SAK EP 11.20). 0 = belum tersimpan.';
COMMENT ON COLUMN loans.original_eir_method IS
    'Metode perhitungan EIR, mis. IRR_ACTUAL_CASHFLOW. Disimpan agar hasil dapat diaudit.';
COMMENT ON COLUMN loans.original_eir_basis IS
    'Dasar audit perhitungan EIR (jsonb): jumlah cair, biaya yang dipotong, jadwal angsuran, dan waktu perhitungan.';
COMMENT ON COLUMN loans.original_eir_calculated_at IS
    'Waktu perhitungan EIR orisinal; bagian dari dasar audit.';

-- 3. Saldo kerugian restrukturisasi yang belum diamortisasi. Berperan sebagai contra
--    nilai tercatat: PPKA dihitung atas pokok terutang dikurangi saldo ini (Pasal 32
--    POJK 1/2024 jo. PA BPR Bab 5.2 hlm. 60-61). Bawaan 0 = belum ada kerugian.
--    Amortisasi ke laba belum diimplementasikan pada rilis ini; saldo bertambah saat
--    restrukturisasi berikutnya dan berkurang hanya bila kelak amortisasi dibangun.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS restructure_loss_balance NUMERIC(28,4) NOT NULL DEFAULT 0.0000;

COMMENT ON COLUMN loans.restructure_loss_balance IS
    'Saldo kerugian restrukturisasi yang belum diamortisasi; pengurang nilai tercatat untuk perhitungan PPKA (Pasal 32 POJK 1/2024 jo. PA BPR Bab 5.2).';

-- 4. COA beban kerugian restrukturisasi, terpisah dari PPAP (50200) dan CKPN (50301)
--    karena PA BPR hlm. 144-145 menyajikannya sebagai pos tersendiri. Akun beban bersifat
--    DEBIT; sisi kredit jurnal adalah akun kredit yang diberikan (10301/11310/11400).
INSERT INTO chart_of_accounts (id, code, name, type, normal_balance, book, is_header) VALUES
    ('a5000000-0000-0000-0000-000000000009', '50401', 'Beban Kerugian Restrukturisasi Kredit',    'EXPENSE', 'DEBIT', 'CONVENTIONAL', FALSE),
    ('b5000000-0000-0000-0000-000000000006', '15902', 'Beban Kerugian Restrukturisasi Pembiayaan', 'EXPENSE', 'DEBIT', 'SYARIAH',      FALSE)
ON CONFLICT (code) DO NOTHING;

-- 5. Akun GL internal untuk COA di atas. Konvensi repo: nomor akun sama dengan kode COA.
INSERT INTO accounts (id, account_number, customer_id, coa_id, account_type, currency, balance, available_balance, status, opened_at) VALUES
    ('c0000000-0000-0000-0000-000000000005', '50401', NULL, 'a5000000-0000-0000-0000-000000000009', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW()),
    ('c0000000-0000-0000-0000-000000000006', '15902', NULL, 'b5000000-0000-0000-0000-000000000006', 'INTERNAL_GL', 'IDR', 0.0000, 0.0000, 'ACTIVE', NOW())
ON CONFLICT (account_number) DO NOTHING;

-- 6. Konfigurasi kebijakan. Saklar MATI secara bawaan: tidak ada perhitungan, jurnal,
--    atau perubahan perilaku restrukturisasi sampai bank mengaktifkannya. Suku bunga
--    diskonto sengaja KOSONG: sumber utamanya EIR orisinal tersimpan; nilai ini hanya
--    override kebijakan bank bila diisi (persen per tahun), dan nilai salah format atau
--    di luar rentang 0 < x <= 100 ditolak dengan pesan jelas, bukan menjadi nol.
INSERT INTO system_config (key, value, description) VALUES
    ('loan.restructure.loss.enabled', 'false',
     'Aktifkan perhitungan & jurnal kerugian restrukturisasi kredit (Pasal 32 POJK 1/2024). false = tidak ada perhitungan, jurnal, atau perubahan perilaku.'),
    ('loan.restructure.loss.discount_rate_annual', '',
     'Override suku bunga diskonto tahunan dalam persen (0 < x <= 100) untuk nilai kini arus kas restrukturisasi. Kosong = pakai EIR orisinal tersimpan.'),
    ('loan.restructure.loss.coa.expense', '',
     'Kode COA beban kerugian restrukturisasi. Kosong = fallback per buku: 50401 konvensional, 15902 syariah (di-seed migrasi 000043).'),
    ('loan.restructure.loss.coa.loan', '',
     'Kode COA kredit yang diberikan (sisi kredit jurnal). Kosong = fallback per buku: 10301 konvensional, 11400 syariah. Produk syariah murabahah sebaiknya memakai pemetaan produk ke 11310.')
ON CONFLICT (key) DO NOTHING;
