-- Konsolidasi dua kolom biaya pelepasan agunan.
--
-- Latar: rancangan T0 (000094) membuat `selling_cost_amount` NOT NULL DEFAULT 0, tetapi
-- tidak pernah dibaca kode apa pun. T2 (000097) membuat `disposal_cost_amount` NULL-able
-- yang dipakai penilaian NRV (NRV = max(0, bound_amount − biaya)) dengan semantik yang
-- lebih jujur: NULL = "belum diisi bank" (NRV tanpa pengurangan, dilaporkan sebagai data
-- belum lengkap), sedangkan 0 = "bank menetapkan biaya nol". Dua kolom untuk satu konsep
-- membuat dua sumber kebenaran; yang tidak dipakai kode dihapus.
--
-- Data: produksi belum memiliki nilai pada keduanya (kolom baru), tetapi migrasi tetap
-- meng-copy nilai bila ada, agar aman untuk instalasi mana pun.
ALTER TABLE loan_collaterals
    ADD COLUMN IF NOT EXISTS disposal_cost_amount NUMERIC(18,2)
        CHECK (disposal_cost_amount IS NULL OR disposal_cost_amount >= 0);

UPDATE loan_collaterals
SET disposal_cost_amount = selling_cost_amount
WHERE disposal_cost_amount IS NULL
  AND selling_cost_amount IS DISTINCT FROM 0;

ALTER TABLE loan_collaterals
    DROP COLUMN IF EXISTS selling_cost_amount;
