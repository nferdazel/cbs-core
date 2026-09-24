-- CKPN individual TAHAP T2: biaya pelepasan agunan untuk NRV (net realisable value).
--
-- Rancangan: docs/CKPN-INDIVIDUAL-RANCANGAN.md bagian 2.2 dan 3 (baris "Biaya pelepasan
-- agunan (biaya penjualan) | Tidak ada kolom").
--
-- Mengapa kolom baru: haircut_percent adalah KEBIJAKAN PENGURANG PPKA (POJK 1/2024
-- Pasar 20-21), BUKAN biaya untuk menjual agunan. Memakai haircut sebagai biaya
-- pelepasan akan mencampur dua konsep yang berbeda dan membuat NRV tidak dapat
-- diaudit. NRV = bound_amount − disposal_cost (lihat komentar di bawah).
--
-- BAWAAN KOSONG (NULL): tidak ada nilai karangan. NRV tanpa biaya pelepasan memakai
-- bound_amount apa adanya; penilaian individual melaporkan biaya yang belum diisi
-- sebagai bagian dari keterbukaan data, bukan menebak.
ALTER TABLE loan_collaterals
    ADD COLUMN IF NOT EXISTS disposal_cost_amount NUMERIC(18,2)
        CHECK (disposal_cost_amount IS NULL OR disposal_cost_amount >= 0);

COMMENT ON COLUMN loan_collaterals.disposal_cost_amount IS
    'Estimasi biaya pelepasan agunan (biaya penjualan/lelang, pajak, biaya notaris) untuk NRV CKPN individual. KOSONG = belum diisi bank; NRV memakai bound_amount tanpa pengurangan. Bukan haircut: haircut adalah kebijakan pengurang PPKA, bukan biaya penjualan.';
