-- 000104_ojk_reference_links.down.sql
-- Membalik 000104_ojk_reference_links.up.sql: hapus tiga kolom penaut ke tabel
-- referensi sandi OJK 000102. Foreign key ikut terhapus bersama kolomnya. Tabel
-- referensi ojk_* TIDAK dihapus di sini (itu urusan 000102.down.sql).

ALTER TABLE lps_placements
    DROP COLUMN IF EXISTS ojk_kabupaten_code;

ALTER TABLE customers
    DROP COLUMN IF EXISTS ojk_sektor_ekonomi_code,
    DROP COLUMN IF EXISTS ojk_pihak_lawan_code;
