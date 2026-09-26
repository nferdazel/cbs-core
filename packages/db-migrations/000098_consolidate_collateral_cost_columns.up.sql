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
--
-- Idempotensi: blok copy dijaga cek keberadaan kolom. Bila `selling_cost_amount` sudah
-- tidak ada — mis. migrasi ini pernah dijalankan tetapi barisnya hilang dari
-- `schema_migrations` (apply dan rekam di migrate.sh tidak satu transaksi), sehingga akan
-- dijalankan ulang — UPDATE dilewati tanpa error, karena tidak ada lagi yang perlu
-- disalin. Tanpa penjaga ini, menjalankan ulang gagal dengan "column does not exist" dan
-- `ON_ERROR_STOP=1` memblokir seluruh migrasi sesudahnya.
ALTER TABLE loan_collaterals
    ADD COLUMN IF NOT EXISTS disposal_cost_amount NUMERIC(18,2)
        CHECK (disposal_cost_amount IS NULL OR disposal_cost_amount >= 0);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'loan_collaterals'
          AND column_name = 'selling_cost_amount'
    ) THEN
        UPDATE loan_collaterals
        SET disposal_cost_amount = selling_cost_amount
        WHERE disposal_cost_amount IS NULL
          AND selling_cost_amount IS DISTINCT FROM 0;
    END IF;
END $$;

ALTER TABLE loan_collaterals
    DROP COLUMN IF EXISTS selling_cost_amount;
