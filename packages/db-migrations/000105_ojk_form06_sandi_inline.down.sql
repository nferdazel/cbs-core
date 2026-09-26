-- 000105_ojk_form06_sandi_inline.down.sql
-- Membalik 000105_ojk_form06_sandi_inline.up.sql: hapus empat kolom penyimpanan sandi
-- inline/field sederhana Form 06.00. Foreign key ojk_kabupaten_code ikut terhapus
-- bersama kolomnya. Tabel referensi ojk_* TIDAK dihapus di sini (itu urusan 000102.down.sql).

ALTER TABLE customers
    DROP COLUMN IF EXISTS ojk_hubungan_bank_code;

ALTER TABLE loans
    DROP COLUMN IF EXISTS ojk_kabupaten_code,
    DROP COLUMN IF EXISTS ojk_periode_pembayaran_code,
    DROP COLUMN IF EXISTS ojk_jenis_penggunaan_code;
