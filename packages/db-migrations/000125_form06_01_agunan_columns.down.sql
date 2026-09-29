-- CBS Migration 000125 (down): hapus kolom Form 06.01 pada loan_collaterals.
-- Simetris dengan 000125_form06_01_agunan_columns.up.sql. Hanya membatalkan kolom/indeks
-- yang ditambahkan up; tidak menyentuh kolom atau data lain.

DROP INDEX IF EXISTS idx_loan_collaterals_ojk_register_number;
DROP INDEX IF EXISTS uq_loan_collaterals_ojk_register_number;

ALTER TABLE loan_collaterals
    DROP COLUMN IF EXISTS ojk_ppka_amount,
    DROP COLUMN IF EXISTS ojk_appraiser_code,
    DROP COLUMN IF EXISTS ojk_bound_value,
    DROP COLUMN IF EXISTS ojk_collateral_type_code,
    DROP COLUMN IF EXISTS ojk_collateral_address,
    DROP COLUMN IF EXISTS ojk_register_number;
