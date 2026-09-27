-- 000107_ojk_form06_k1_columns.down.sql
-- Membalik 000107_ojk_form06_k1_columns.up.sql: hapus tujuh kolom penyimpanan kolom K1
-- Form 06.00. Foreign key ojk_penjamin_code ke ojk_pihak_lawan ikut terhapus bersama
-- kolomnya; tabel referensi ojk_* TIDAK dihapus di sini (itu urusan 000102.down.sql).

ALTER TABLE loans
    DROP COLUMN IF EXISTS ojk_tanggal_mulai_macet,
    DROP COLUMN IF EXISTS ojk_penjamin_bagian_pct,
    DROP COLUMN IF EXISTS ojk_penjamin_code,
    DROP COLUMN IF EXISTS ojk_sifat_kredit_code,
    DROP COLUMN IF EXISTS ojk_kategori_usaha_code,
    DROP COLUMN IF EXISTS ojk_sumber_dana_code,
    DROP COLUMN IF EXISTS ojk_kelompok_kredit_code;
