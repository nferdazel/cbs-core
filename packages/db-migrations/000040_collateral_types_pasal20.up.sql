-- CBS Migration 000040: melengkapi pemetaan agunan Pasal 20 POJK No. 1 Tahun 2024
-- dan pemisahan agunan tunai Pasal 17.
-- Run after: 000039 (nomor dipesan; migrasi ini tidak bergantung isinya selain urutan).
--
-- Mengapa migrasi ini ada: pemetaan tarif Pasal 20 ayat (1) di migrasi 000038 baru
-- mencakup sebagian jenis agunan. Lima jenis pada teks resmi belum punya padanan enum,
-- sehingga jatuh ke tarif konservatif (nol) — penyisihan lebih besar dari kewajiban.
-- Selain itu Pasal 17 jo. Pasal 19 ayat (4) huruf b membedakan perlakuan agunan tunai
-- dari pengurang Pasal 20, dan pita umur penilaian resi gudang (huruf c/h/j) butuh
-- tanggal penilaiannya sendiri bila berbeda dari tanggal taksasi agunan.
--
-- Saklar ppap.collateral.enabled TIDAK disentuh: bawaannya tetap false. Migrasi ini
-- hanya melengkapi jenis dan kolom, bukan mengaktifkan pengurangan.
--
-- Idempotent: aman dijalankan ulang (ADD VALUE IF NOT EXISTS, ADD COLUMN IF NOT EXISTS).
--
-- Catatan ALTER TYPE ... ADD VALUE: dijalankan di dalam transaksi milik migrate.sh.
-- Sejak PostgreSQL 12 hal itu diperbolehkan selama nilai barunya belum dipakai di
-- transaksi yang sama; migrasi ini hanya menambahkannya dan tidak memakai nilainya.
-- Preseden yang sama sudah dipakai migrasi 000004 dan 000037.

-- 1. Jenis agunan Pasal 20 ayat (1) yang belum punya padanan.
--    Tarifnya ditegakkan domain (Pasal20Rate), bukan di database, agar kebijakan bank
--    yang lebih konservatif tetap dapat menurunkannya dan tetap dapat diaudit.
--    - EMAS_PERHIASAN    : huruf a, paling tinggi 85% dari nilai pasar.
--    - RESI_GUDANG       : huruf c/h/j, 70/50/30% menurut umur penilaian.
--    - TANAH_ADAT        : huruf e, 50% dari NJOP untuk surat pengakuan tanah adat.
--    - TEMPAT_USAHA      : huruf f, 50% untuk tempat usaha dengan bukti kepemilikan/
--                          izin pakai dan surat kuasa menjual.
--    - JAMINAN_BUMN_BUMD : huruf i, 50% bagian kredit yang dijamin BUMN/BUMD penjamin
--                          yang memenuhi kriteria KPMM.
ALTER TYPE collateral_type ADD VALUE IF NOT EXISTS 'EMAS_PERHIASAN';
ALTER TYPE collateral_type ADD VALUE IF NOT EXISTS 'RESI_GUDANG';
ALTER TYPE collateral_type ADD VALUE IF NOT EXISTS 'TANAH_ADAT';
ALTER TYPE collateral_type ADD VALUE IF NOT EXISTS 'TEMPAT_USAHA';
ALTER TYPE collateral_type ADD VALUE IF NOT EXISTS 'JAMINAN_BUMN_BUMD';

-- 2. Penanda agunan tunai Pasal 17 dan rekening tempat dananya diblokir.
--    Agunan tunai BUKAN pengurang PPKA khusus (bukan daftar Pasal 20 ayat (1)); perannya
--    membuat bagian yang dijamin berkualitas Lancar sehingga dikecualikan dari PPKA umum
--    (Pasal 19 ayat (4) huruf b jo. Pasal 17). Karena itu ia butuh penanda tersendiri,
--    terpisah dari kolom pengurang Pasal 20.
--    Bawaan FALSE bersifat konservatif: selama operator tidak menyatakan Pasal 17(3)
--    dipenuhi, tidak ada pengecualian PPKA umum yang diakui.
ALTER TABLE loan_collaterals
    ADD COLUMN IF NOT EXISTS is_cash BOOLEAN NOT NULL DEFAULT FALSE,
    -- Rekening tempat tabungan/deposito/logam mulia disimpan dan diblokir (Pasal 17(3)
    -- huruf a dan d). Dibiarkan NULL hanya untuk data lama; agunan tunai baru ditolak
    -- domain bila tidak dikaitkan ke rekening, agar pengecualian PPKA umum dapat diaudit.
    ADD COLUMN IF NOT EXISTS cash_account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
    -- Tanggal penilaian resi gudang bila berbeda dari appraisal_date agunan. Dipakai
    -- menetapkan pita umur huruf c (<=12 bulan), h (>12 s/d 18), dan j (>18 s/d 24);
    -- kosong berarti appraisal_date yang dipakai sebagai penilaian terakhir.
    ADD COLUMN IF NOT EXISTS warehouse_receipt_valued_at DATE;

COMMENT ON COLUMN loan_collaterals.is_cash IS
    'Agunan tunai Pasal 17 (tabungan/deposito/logam mulia/surat berharga BI-Pemerintah); dikecualikan dari PPKA umum, bukan pengurang PPKA khusus.';
COMMENT ON COLUMN loan_collaterals.cash_account_id IS
    'Rekening tempat agunan tunai diblokir (Pasal 17 ayat (3) huruf a dan d); sumber dana pengecualian PPKA umum.';
COMMENT ON COLUMN loan_collaterals.warehouse_receipt_valued_at IS
    'Tanggal penilaian resi gudang untuk pita umur Pasal 20 ayat (1) huruf c/h/j; kosong berarti memakai appraisal_date.';

-- Indeks pendukung foreign key cash_account_id, agar penghapusan/pembaruan rekening
-- tidak memindai seluruh tabel agunan.
CREATE INDEX IF NOT EXISTS idx_collateral_cash_account
    ON loan_collaterals (cash_account_id) WHERE cash_account_id IS NOT NULL;
