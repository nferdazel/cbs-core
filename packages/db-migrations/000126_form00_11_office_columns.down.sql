-- CBS Migration 000126 (down): hapus kolom Form 00.11 pada bank_offices.
-- Simetris dengan 000126_form00_11_office_columns.up.sql. Hanya membatalkan kolom/indeks
-- yang ditambahkan up; tidak menyentuh kolom atau data lain.

DROP INDEX IF EXISTS idx_bank_offices_ojk_kind;

ALTER TABLE bank_offices
    DROP COLUMN IF EXISTS ojk_approval_date,
    DROP COLUMN IF EXISTS control_office_code,
    DROP COLUMN IF EXISTS implementation_date,
    DROP COLUMN IF EXISTS ojk_change_code,
    DROP COLUMN IF EXISTS phone_number,
    DROP COLUMN IF EXISTS head_name,
    DROP COLUMN IF EXISTS coordinates,
    DROP COLUMN IF EXISTS previous_office_code,
    DROP COLUMN IF EXISTS parent_office_code,
    DROP COLUMN IF EXISTS ojk_office_kind_code;
