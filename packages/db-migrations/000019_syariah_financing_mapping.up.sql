-- CBS Migration 000019: Perbaikan pemetaan jurnal pembiayaan syariah
-- Run after: 000018_loan_penalty_last_accrued.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Ditemukan lewat verifikasi end-to-end pada DB nyata: pencairan pembiayaan syariah
-- mengkredit buku konvensional dan dana tidak pernah masuk rekening nasabah.

-- ─────────────────────────────────────────────────────────────────────────────
-- MASALAH 1: kaki rekening nasabah menunjuk COA buku konvensional.
--
-- Pemetaan PMB-MURABAHAH dan PMB-MUDHARABAH memakai COA 20100 (Third-Party Savings
-- Deposits, buku KONVENSIONAL) untuk kaki rekening nasabah. Posting engine tidak
-- dapat mengarahkannya ke rekening nasabah karena override rekening dicocokkan
-- berdasarkan kode COA rekening (12100 Tabungan Wadiah), sehingga diam-diam jatuh
-- ke akun kontrol GL dan dana syariah tercampur buku konvensional. Dana UUS tidak
-- boleh bercampur dengan konvensional.
UPDATE product_journal_mapping m
SET coa_code = '12100'
FROM banking_products p
WHERE m.product_id = p.id
  AND p.book = 'SYARIAH'
  AND p.family = 'LOAN'
  AND m.coa_code = '20100';

-- ─────────────────────────────────────────────────────────────────────────────
-- MASALAH 2: akun pendapatan denda dan pendapatan lain buku syariah belum ada.
-- Tanpa akun ini pemetaan LOAN_PENALTY dan LOAN_RECOVERY tidak dapat dibentuk untuk
-- produk syariah, sehingga denda dan recovery pembiayaan gagal diposting. Nomor
-- mengikuti seri 14000 (PENDAPATAN SYARIAH) yang sudah ada.
INSERT INTO chart_of_accounts (code, name, type, normal_balance, book, is_header, created_at, updated_at)
VALUES
    ('14600', 'Pendapatan Denda Syariah',      'REVENUE', 'CREDIT', 'SYARIAH', FALSE, NOW(), NOW()),
    ('14900', 'Pendapatan Lainnya Syariah',    'REVENUE', 'CREDIT', 'SYARIAH', FALSE, NOW(), NOW())
ON CONFLICT (code) DO NOTHING;

-- Akun GL internal untuk COA baru, mengikuti pola migrasi 000011.
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
WHERE coa.code IN ('14600', '14900')
  AND NOT EXISTS (
      SELECT 1 FROM accounts a
      WHERE a.coa_id = coa.id AND a.account_type = 'INTERNAL_GL'
  );

-- ─────────────────────────────────────────────────────────────────────────────
-- MASALAH 3: recovery pembiayaan syariah mengkredit pendapatan konvensional.
UPDATE product_journal_mapping m
SET coa_code = '14900'
FROM banking_products p
WHERE m.product_id = p.id
  AND p.book = 'SYARIAH'
  AND p.family = 'LOAN'
  AND m.event = 'LOAN_RECOVERY'
  AND m.direction = 'CREDIT'
  AND m.coa_code = '40900';

-- ─────────────────────────────────────────────────────────────────────────────
-- MASALAH 4: produk pembiayaan syariah belum punya pemetaan denda dan PPAP,
-- sehingga akrual denda dan PPAP harian gagal untuk pembiayaan.
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source, created_at)
SELECT p.id, v.event::posting_event, v.direction::entry_direction, v.coa_code, v.amount_source, NOW()
FROM banking_products p
CROSS JOIN (VALUES
    ('LOAN_PENALTY',   'DEBIT',  '12100', 'PENALTY'),
    ('LOAN_PENALTY',   'CREDIT', '14600', 'PENALTY'),
    ('PPAP_PROVISION', 'DEBIT',  '15200', 'PRINCIPAL'),
    ('PPAP_PROVISION', 'CREDIT', '11900', 'PRINCIPAL')
) AS v(event, direction, coa_code, amount_source)
WHERE p.book = 'SYARIAH'
  AND p.family = 'LOAN'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping m
      WHERE m.product_id = p.id
        AND m.event = v.event::posting_event
        AND m.direction = v.direction::entry_direction
  );
