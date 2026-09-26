-- 000102_ojk_reference_codes.down.sql
-- Membalik 000102_ojk_reference_codes.up.sql: hapus ketiga tabel referensi sandi OJK.
-- Tidak ada foreign key ke tabel lain, jadi urutan drop tidak penting.

DROP TABLE IF EXISTS ojk_sektor_ekonomi;
DROP TABLE IF EXISTS ojk_kabupaten;
DROP TABLE IF EXISTS ojk_pihak_lawan;
