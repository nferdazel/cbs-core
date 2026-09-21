-- CBS Migration 000039: Blind index per kata nama nasabah
-- Run after: 000038 (nomor dipesan agen lain; migrasi ini tidak bergantung padanya
-- selain urutan penomoran).
--
-- LATAR BELAKANG
-- Nama lengkap disimpan terenkripsi (envelope encryption), sehingga pencarian
-- potongan nama tidak mungkin dilakukan dengan LIKE atas kolom terenkripsi. Yang
-- tersedia hanya blind index nilai utuh (HMAC deterministik), dan itu pun menuntut
-- pencocokan persis. Tabel ini menyimpan satu baris per KATA nama, sehingga
-- pencarian "siti" dapat menemukan "Siti Rahayu" tanpa membuka enkripsi.
--
-- Token dihitung aplikasi (HMAC dengan master key enkripsi, awalan domain
-- "name-token:"), BUKAN oleh SQL, karena master key hanya ada di aplikasi. Karena
-- itu kolom token_index di sini tidak dapat diisi lewat migrasi; nasabah lama
-- di-backfill dengan perintah:
--
--     cd apps/api && go run ./cmd/backfill-name-tokens
--
-- Perintah itu aman diulang (token yang sudah ada tidak ditimpa).

CREATE TABLE IF NOT EXISTS customer_name_tokens (
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    -- HMAC(master key, "name-token:" + kata ternormalisasi), base64. Tidak dapat
    -- dibalik menjadi nama: tanpa master key, kamus tidak membantu.
    token_index TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Satu kata hanya sekali per nasabah: nama seperti "Siti Siti" tidak membuat
    -- baris ganda, dan pencarian tidak pernah menghitung baris yang sama dua kali.
    PRIMARY KEY (customer_id, token_index)
);

-- Pencarian berjalan dari token ke nasabah, sedangkan PRIMARY KEY di atas
-- berawalan customer_id sehingga tidak melayani arah itu. Indeks terpisah ini yang
-- membuat "WHERE token_index = ANY(...)" tetap cepat.
CREATE INDEX IF NOT EXISTS idx_customer_name_tokens_token_index
    ON customer_name_tokens (token_index);

COMMENT ON TABLE customer_name_tokens IS
    'Blind index per kata nama untuk pencarian potongan nama tanpa membuka enkripsi; token dihitung aplikasi, lihat cmd/backfill-name-tokens untuk data lama';
COMMENT ON COLUMN customer_name_tokens.token_index IS
    'HMAC(master key, "name-token:" + kata nama huruf kecil tanpa gelar); tidak dapat dibalik tanpa master key';
