-- CBS Migration 000024: Piutang denda + dana kebajikan syariah
-- Run after: 000023_loan_profit_accrual.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Latar belakang (temuan audit + verifikasi kode):
--
-- 1. Akrual denda kredit mendebit REKENING NASABAH secara langsung lewat
--    AccountOverrides. Pemetaan lama menunjuk akun kontrol kewajiban tabungan
--    (20100 konvensional, 12100 Tabungan Wadiah syariah), lalu override
--    mengarahkannya ke rekening nasabah tertentu. Akibatnya denda terpotong dari
--    tabungan tanpa persetujuan, dan bila saldo tidak cukup saldo dapat menjadi
--    NEGATIF (mesin posting belum punya batas bawah untuk rekening nasabah).
--
--    Denda pada hakikatnya adalah TAGIHAN, bukan pemindahan dana. Karena itu
--    akrual denda dipindahkan ke akun piutang; pemindahan dana dari rekening
--    nasabah baru terjadi saat denda benar-benar dibayar.
--
-- 2. Denda pembiayaan syariah dikreditkan ke 14600 "Pendapatan Denda Syariah".
--    Fatwa DSN-MUI No. 17/DSN-MUI/IX/2000 menetapkan dana yang berasal dari denda
--    diperuntukkan sebagai dana sosial, sehingga tidak boleh diakui sebagai
--    pendapatan bank. Denda syariah kini dikreditkan ke akun kewajiban
--    "Dana Kebajikan" (TBDSP) sampai disalurkan.
--
-- Catatan klasifikasi: Dana Kebajikan digolongkan LIABILITY, bukan EQUITY, karena
-- ia merupakan kewajiban bank untuk menyalurkan dana sosial, bukan bagian modal
-- pemilik. Bank belum mengakui hak atas dana tersebut.

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. Akun piutang denda (konvensional) dan piutang denda syariah.
-- -----------------------------------------------------------------------------
INSERT INTO chart_of_accounts (code, name, type, normal_balance, book, is_header, created_at, updated_at)
VALUES
    ('10305', 'Piutang Denda',         'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW()),
    ('11700', 'Piutang Denda Syariah', 'ASSET', 'DEBIT', 'SYARIAH',      FALSE, NOW(), NOW()),
-- 2. Akun kewajiban dana kebajikan (TBDSP) untuk denda pembiayaan syariah.
    ('12500', 'Dana Kebajikan',        'LIABILITY', 'CREDIT', 'SYARIAH',  FALSE, NOW(), NOW())
ON CONFLICT (code) DO NOTHING;

-- Akun GL internal untuk COA baru, mengikuti pola migrasi 000011/000019.
INSERT INTO accounts (
    id, account_number, customer_id, coa_id, account_type, currency,
    balance, available_balance, hold_balance, status, version, created_at, updated_at
)
SELECT
    uuid_generate_v4(),
    coa.code,
    NULL,
    coa.id,
    'INTERNAL_GL',
    'IDR',
    0.0000,
    0.0000,
    0.0000,
    'ACTIVE',
    1,
    NOW(),
    NOW()
FROM chart_of_accounts coa
WHERE coa.code IN ('10305', '11700', '12500')
  AND NOT EXISTS (
      SELECT 1 FROM accounts a
      WHERE a.coa_id = coa.id AND a.account_type = 'INTERNAL_GL'
  );

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. Denda konvensional: debit piutang denda, bukan rekening nasabah.
-- -----------------------------------------------------------------------------
UPDATE product_journal_mapping m
SET coa_code = '10305'
FROM banking_products p
WHERE m.product_id = p.id
  AND p.book = 'CONVENTIONAL'
  AND p.family = 'LOAN'
  AND m.event = 'LOAN_PENALTY'
  AND m.direction = 'DEBIT'
  AND m.coa_code = '20100';

-- ─────────────────────────────────────────────────────────────────────────────
-- 4. Denda syariah: debit piutang denda syariah, kredit dana kebajikan.
--    Dua baris dipisah agar kegagalan salah satu terlihat, bukan diam-diam.
-- -----------------------------------------------------------------------------
UPDATE product_journal_mapping m
SET coa_code = '11700'
FROM banking_products p
WHERE m.product_id = p.id
  AND p.book = 'SYARIAH'
  AND p.family = 'LOAN'
  AND m.event = 'LOAN_PENALTY'
  AND m.direction = 'DEBIT'
  AND m.coa_code = '12100';

UPDATE product_journal_mapping m
SET coa_code = '12500'
FROM banking_products p
WHERE m.product_id = p.id
  AND p.book = 'SYARIAH'
  AND p.family = 'LOAN'
  AND m.event = 'LOAN_PENALTY'
  AND m.direction = 'CREDIT'
  AND m.coa_code = '14600';

-- ─────────────────────────────────────────────────────────────────────────────
-- 5. Pemetaan denda yang belum ada (produk syariah baru di masa depan) diisi
--    langsung dengan akun yang benar, agar tidak lahir dengan pemetaan lama.
-- -----------------------------------------------------------------------------
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source, created_at)
SELECT p.id, v.event::posting_event, v.direction::entry_direction, v.coa_code, v.amount_source, NOW()
FROM banking_products p
CROSS JOIN (VALUES
    ('LOAN_PENALTY', 'DEBIT',  '10305', 'PENALTY'),
    ('LOAN_PENALTY', 'CREDIT', '40500', 'PENALTY')
) AS v(event, direction, coa_code, amount_source)
WHERE p.book = 'CONVENTIONAL'
  AND p.family = 'LOAN'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping m
      WHERE m.product_id = p.id
        AND m.event = v.event::posting_event
        AND m.direction = v.direction::entry_direction
  );

INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source, created_at)
SELECT p.id, v.event::posting_event, v.direction::entry_direction, v.coa_code, v.amount_source, NOW()
FROM banking_products p
CROSS JOIN (VALUES
    ('LOAN_PENALTY', 'DEBIT',  '11700', 'PENALTY'),
    ('LOAN_PENALTY', 'CREDIT', '12500', 'PENALTY')
) AS v(event, direction, coa_code, amount_source)
WHERE p.book = 'SYARIAH'
  AND p.family = 'LOAN'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping m
      WHERE m.product_id = p.id
        AND m.event = v.event::posting_event
        AND m.direction = v.direction::entry_direction
  );
