-- CBS Migration 000047: dasar NJOP huruf d/e dan penanda kriteria penjamin BUMN/BUMD
-- huruf i pada kepatuhan agunan Pasal 20 POJK No. 1 Tahun 2024.
-- Run after: 000046 (nomor dipesan; migrasi ini tidak bergantung isinya selain urutan).
--
-- Mengapa migrasi ini ada: pemetaan tarif Pasal 20 ayat (1) di migrasi 000038 dan 000040
-- sudah ada, tetapi dua batas kepatuhan masih belum punya datanya.
--
--   1. Pasal 20 ayat (1) huruf d dan e menghitung pengurang dari NJOP — huruf d untuk
--      tanah/bangunan bersertifikat TANPA hak tanggungan (60%), huruf e untuk tanah
--      dengan surat pengakuan tanah adat (50%). Selama ini nilai taksasi agunan dipakai
--      sebagai pendekatan, padahal pasal menetapkan dasar yang berbeda. Kolom NJOP
--      tersendiri beserta tanggal dan asal nilainya membuat dasarnya eksplisit; bila
--      kosong, pengurangnya nol (konservatif), BUKAN nilai taksasi.
--   2. Pasal 20 ayat (1) huruf i menghitung 50% atas bagian Kredit yang dijamin BUMN/BUMD
--      yang memenuhi kriteria penjamin. Sistem tidak dapat menilai kriteria itu, sehingga
--      pemenuhannya harus dinyatakan operator beserta buktinya. Tanpa penanda, kriteria
--      dianggap TIDAK terpenuhi dan nilainya bukan pengurang (Pasal 20 ayat (2)).
--
-- Saklar ppap.collateral.enabled TIDAK disentuh: bawaannya tetap false. Migrasi ini hanya
-- melengkapi kolom, bukan mengaktifkan pengurangan.
--
-- Idempotent: aman dijalankan ulang (ADD COLUMN IF NOT EXISTS).

ALTER TABLE loan_collaterals
    -- Pasal 20 ayat (1) huruf d/e: dasar pengurang dari NJOP (atau nilai pasar penilai
    -- independen). Bawaan 0 berarti belum diisi; nol bukan pengurang, bukan pula nilai
    -- taksasi yang disamarkan sebagai NJOP.
    ADD COLUMN IF NOT EXISTS njop_value NUMERIC(18,2) NOT NULL DEFAULT 0
        CHECK (njop_value >= 0),
    -- Tanggal NJOP/penilaian diambil agar dasar pengurangnya dapat ditelusuri.
    ADD COLUMN IF NOT EXISTS njop_date DATE,
    -- Asal nilai huruf d/e. 'NJOP' = SPPT/PBB; 'PENILAI_INDEPENDEN' = nilai pasar menurut
    -- penilaian penilai independen, pengganti yang diizinkan pasal. NULL berarti belum
    -- dinyatakan operator; dasar pengurangnya tidak boleh ditebak.
    ADD COLUMN IF NOT EXISTS njop_source TEXT
        CHECK (njop_source IN ('NJOP', 'PENILAI_INDEPENDEN')),
    -- Pasal 20 ayat (1) huruf i: penanda eksplisit operator bahwa penjamin BUMN/BUMD
    -- memenuhi kriteria. Bawaan FALSE bersifat konservatif: tanpa pernyataan operator,
    -- kriteria dianggap belum terpenuhi sehingga nilainya bukan pengurang.
    ADD COLUMN IF NOT EXISTS bumn_bumd_criteria_met BOOLEAN NOT NULL DEFAULT FALSE,
    -- Bukti/keterangan pemenuhan kriteria huruf i, supaya penanda dapat diaudit.
    ADD COLUMN IF NOT EXISTS bumn_bumd_evidence TEXT;

COMMENT ON COLUMN loan_collaterals.njop_value IS
    'NJOP atau nilai pasar penilai independen sebagai dasar pengurang Pasal 20 ayat (1) huruf d/e; 0 berarti belum diisi dan pengurangnya nol.';
COMMENT ON COLUMN loan_collaterals.njop_date IS
    'Tanggal NJOP/penilaian penilai independen untuk huruf d/e; dasar penelusuran asal nilai.';
COMMENT ON COLUMN loan_collaterals.njop_source IS
    'Asal nilai huruf d/e: NJOP (SPPT/PBB) atau PENILAI_INDEPENDEN (nilai pasar penilai independen); NULL berarti belum dinyatakan operator.';
COMMENT ON COLUMN loan_collaterals.bumn_bumd_criteria_met IS
    'Penanda operator bahwa penjamin BUMN/BUMD memenuhi kriteria Pasal 20 ayat (1) huruf i; tanpa penanda nilainya bukan pengurang.';
COMMENT ON COLUMN loan_collaterals.bumn_bumd_evidence IS
    'Bukti/keterangan pemenuhan kriteria penjamin BUMN/BUMD (Pasal 20 ayat (1) huruf i).';

-- Tidak ada backfill: data lama tidak pernah mencatat NJOP maupun penanda BUMN/BUMD, dan
-- menebaknya justru membuat dasar pengurang tampak pasti padahal bukan. Agunan lama
-- dengan huruf d/e tetap menghasilkan pengurang nol sampai operator mengisi NJOP.
