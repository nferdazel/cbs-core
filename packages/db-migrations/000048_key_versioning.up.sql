-- CBS Migration 000048: Versi kunci untuk data terenkripsi/berindeks
-- Run after: 000047 (nomor dipesan agen lain; migrasi ini tidak bergantung padanya
-- selain urutan penomoran).
--
-- LATAR BELAKANG
-- Sebelum ini, satu master key dipakai untuk dua tujuan: enkripsi data pribadi
-- (envelope encryption) dan pembentukan blind index (HMAC nilai utuh) serta token
-- nama. Dua tujuan itu punya konsekuensi kebocoran yang berbeda: kunci indeks yang
-- bocor memungkinkan kamus nilai dicocokkan untuk menebak NIK/email, sedangkan kunci
-- enkripsi bocor membuka isi data. Karena itu indeks sebaiknya memakai kunci sendiri
-- (ENCRYPTION_INDEX_KEY), dan kunci enkripsi harus bisa dirotasi tanpa mematikan
-- indeks.
--
-- Migrasi ini hanya MENAMBAH metadata versi kunci; belum ada rotasi aktual dan tidak
-- ada data lama yang ditulis ulang. Nilai indeks untuk data lama tidak berubah.
--
-- ARTI KOLOM
-- index_key_version mencatat versi kunci INDEKS yang membentuk id_card_index,
-- email_index, dan customer_name_tokens.token_index pada baris itu. Versi kunci
-- ENKRIPSI tidak perlu kolom terpisah: id-nya sudah tertanam di dalam blob ciphertext
-- (format v1:keyID:...), sehingga dekripsi data lama sudah bekerja lintas versi.
--
-- NILAI UNTUK BARIS LAMA: 'k1'
-- Sebelum migrasi ini, semua baris diindeks dengan master key enkripsi dan id kunci
-- bawaan "k1" (lihat crypto.NewCipher; id kosong selalu menjadi "k1", dan konfigurasi
-- bawaan ENCRYPTION_KEY_ID juga "k1"). Karena itu DEFAULT 'k1' adalah penanda yang
-- benar untuk baris lama pada deployment standar. Bila ENCRYPTION_KEY_ID dikonfigurasi
-- ke nilai lain, perbaiki penanda baris lama satu kali setelah migrasi, TANPA mengubah
-- nilai indeksnya:
--
--     UPDATE customers SET index_key_version = '<id-lama>'
--     WHERE index_key_version = 'k1';   -- sesuaikan bila id lamanya bukan k1
--     UPDATE customer_name_tokens SET index_key_version = '<id-lama>'
--     WHERE index_key_version = 'k1';
--
-- Idempotent: ADD COLUMN IF NOT EXISTS + CREATE INDEX IF NOT EXISTS, aman dijalankan
-- ulang (mis. setelah rollback yang tidak menyentuh tabel pelacak migrasi).

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS index_key_version TEXT NOT NULL DEFAULT 'k1';

ALTER TABLE customer_name_tokens
    ADD COLUMN IF NOT EXISTS index_key_version TEXT NOT NULL DEFAULT 'k1';

COMMENT ON COLUMN customers.index_key_version IS
    'Versi kunci INDEKS yang membentuk id_card_index/email_index; id kunci enkripsi tertanam di blob. k1 = baris lama sebelum pemisahan kunci';
COMMENT ON COLUMN customer_name_tokens.index_key_version IS
    'Versi kunci indeks yang membentuk token_index; k1 = token lama sebelum pemisahan kunci';

-- Rencana rotasi membutuhkan pencarian cepat "baris mana yang masih memakai kunci
-- lama" (mis. WHERE index_key_version <> 'ik1'). Indeks ini hanya membantu pemindaian
-- rotasi, bukan pencarian aplikasi.
CREATE INDEX IF NOT EXISTS idx_customers_index_key_version
    ON customers (index_key_version);
CREATE INDEX IF NOT EXISTS idx_customer_name_tokens_index_key_version
    ON customer_name_tokens (index_key_version);

-- RENCANA ROTASI KUNCI INDEKS (dijalankan manual, tidak otomatis)
-- Tujuan: pindah dari kunci indeks lama (di sini "k1", kunci enkripsi) ke kunci
-- indeks terpisah "ik1" tanpa memutus pencarian NIK/email/nama.
--
-- 1. PERSIAPAN (dapat dibatalkan, tidak mengubah data)
--    a. Set ENCRYPTION_INDEX_KEY_ID=ik1 dan ENCRYPTION_INDEX_KEY=<base64 32 byte>.
--    b. Jangan hapus kunci lama. Selama transisi, kandidat pencarian otomatis memuat
--       indeks "k1" dan "ik1", jadi baris lama tetap ditemukan.
--    c. Verifikasi sebelum perubahan: hitung berapa baris per versi.
--         SELECT index_key_version, COUNT(*) FROM customers GROUP BY 1;
--       Indeks pencarian yang sudah ada harus tetap sama: bandingkan contoh
--       id_card_index untuk NIK yang diketahui sebelum dan sesudah konfigurasi.
-- 2. GANTI KUNCI TULIS (titik yang belum dapat dibatalkan)
--    Setelah langkah 1, nasabah BARU ditulis dengan indeks "ik1" dan versi ik1. Mulai
--    titik ini dua versi hidup berdampingan; pencarian tetap menyatukan keduanya,
--    tetapi jangan buru-buru menghapus kunci lama.
-- 3. ISI ULANG INDEKS BARIS LAMA (idempoten, jalankan bertahap)
--    Untuk tiap baris dengan index_key_version <> ik1: dekripsi field terkait, hitung
--    ulang blind index + token nama memakai kunci indeks aktif, lalu UPDATE
--    id_card_index/email_index/index_key_version dan ganti baris
--    customer_name_tokens dalam SATU transaksi. Karena hanya menyentuh baris yang
--    belum ber-versi ik1, perintah aman diulang dan boleh dijalankan per batch.
--    Kerjakan saat beban rendah; blind index unik parsial tetap utuh karena NIK/email
--    yang sama tetap menghasilkan satu baris.
--    Verifikasi setelah tiap batch:
--         SELECT index_key_version, COUNT(*) FROM customers GROUP BY 1;
-- 4. SELESAI (titik yang belum dapat dibatalkan kedua)
--    Setelah seluruh baris ber-versi ik1 dan pencarian NIK/email/nama diuji terhadap
--    data nyata, baris lama sudah tidak memakai kunci "k1" untuk INDEKS. Baru pada
--    titik ini kunci enkripsi lama boleh dipensiunkan dari ENCRYPTION_PREVIOUS_KEYS —
--    dan itu pun hanya bila tidak ada lagi ciphertext ber-keyID lama (cek blob).
--    Jangan menghapus kunci indeks lama dari konfigurasi selama masih ada baris
--    ber-versi lama, karena baris itu tidak akan ditemukan lagi.
