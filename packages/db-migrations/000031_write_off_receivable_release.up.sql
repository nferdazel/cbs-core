-- CBS Migration 000031: pelepasan piutang bunga dan denda saat hapus buku kredit
-- Run after: 000030_audit_log_immutable.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Jurnal hapus buku hanya melepas pokok kredit. Piutang bunga kredit (10400) dan
-- piutang denda (10305 konvensional, 11700 syariah) tetap tercatat di neraca padahal
-- kreditnya sudah tidak ada lagi, sehingga bank melaporkan aset yang tidak mungkin
-- tertagih dan tidak ada kredit yang menopangnya. Aplikasi kini mengirim nominal
-- PROFIT dan PENALTY pada peristiwa LOAN_WRITE_OFF; tanpa kaki jurnal di bawah ini
-- hapus buku DITOLAK dengan pesan yang menyebut kaki mana yang belum dipetakan.
--
-- Kaki ditambahkan untuk SETIAP produk yang sudah punya pemetaan hapus buku, jadi
-- produk yang dibuat bank sendiri ikut terjaga tanpa perubahan kode.
--
-- Perlakuan akuntansi yang dipilih: MEMBALIK pengakuan pendapatan yang tidak akan
-- tertagih, yaitu cermin dari jurnal akrualnya.
--   konvensional : DEBIT 40100 Pendapatan Bunga Kredit / CREDIT 10400
--                  DEBIT 40500 Pendapatan Denda        / CREDIT 10305
--   syariah      : DEBIT 12500 Dana Kebajikan         / CREDIT 11700
--
-- Pembalikan pendapatan dipilih karena piutang ini tidak pernah dicadangkan: PPAP
-- dihitung dari pokok, sedangkan pokok yang dihapus sudah dibebankan ke cadangan 10900.
-- Pengaruhnya pada laba sama dengan membebankan ke akun beban, tetapi akunnya sudah ada
-- dan tidak perlu menambah COA baru — lagipula pemetaan hanya mengizinkan satu kaki per
-- (produk, peristiwa, arah, akun), sehingga dua beban ke akun yang sama tidak dapat
-- dinyatakan. Bank tetap dapat mengganti akunnya lewat product_journal_mapping.
-- Untuk syariah hanya denda yang punya piutang: margin murabahah sudah ditangguhkan di
-- 11320 dan tidak pernah diakru sebagai piutang, sehingga dana kebajikan 12500 yang
-- dikurangi, bukan pendapatan bank.

-- Bunga kredit yang sudah diakru (piutang 10400) pada buku konvensional.
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source)
SELECT DISTINCT m.product_id, 'LOAN_WRITE_OFF'::posting_event, 'DEBIT'::entry_direction, '40100', 'PROFIT'
FROM product_journal_mapping m
JOIN banking_products p ON p.id = m.product_id
WHERE m.event = 'LOAN_WRITE_OFF'
  AND p.book = 'CONVENTIONAL'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping x
      WHERE x.product_id = m.product_id AND x.event = 'LOAN_WRITE_OFF'
        AND x.direction = 'DEBIT' AND x.amount_source = 'PROFIT'
  );

INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source)
SELECT DISTINCT m.product_id, 'LOAN_WRITE_OFF'::posting_event, 'CREDIT'::entry_direction, '10400', 'PROFIT'
FROM product_journal_mapping m
JOIN banking_products p ON p.id = m.product_id
WHERE m.event = 'LOAN_WRITE_OFF'
  AND p.book = 'CONVENTIONAL'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping x
      WHERE x.product_id = m.product_id AND x.event = 'LOAN_WRITE_OFF'
        AND x.direction = 'CREDIT' AND x.amount_source = 'PROFIT'
  );

-- Denda kredit konvensional (piutang 10305).
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source)
SELECT DISTINCT m.product_id, 'LOAN_WRITE_OFF'::posting_event, 'DEBIT'::entry_direction, '40500', 'PENALTY'
FROM product_journal_mapping m
JOIN banking_products p ON p.id = m.product_id
WHERE m.event = 'LOAN_WRITE_OFF'
  AND p.book = 'CONVENTIONAL'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping x
      WHERE x.product_id = m.product_id AND x.event = 'LOAN_WRITE_OFF'
        AND x.direction = 'DEBIT' AND x.amount_source = 'PENALTY'
  );

INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source)
SELECT DISTINCT m.product_id, 'LOAN_WRITE_OFF'::posting_event, 'CREDIT'::entry_direction, '10305', 'PENALTY'
FROM product_journal_mapping m
JOIN banking_products p ON p.id = m.product_id
WHERE m.event = 'LOAN_WRITE_OFF'
  AND p.book = 'CONVENTIONAL'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping x
      WHERE x.product_id = m.product_id AND x.event = 'LOAN_WRITE_OFF'
        AND x.direction = 'CREDIT' AND x.amount_source = 'PENALTY'
  );

-- Denda pembiayaan syariah (piutang 11700) dikurangi dari dana kebajikan (12500),
-- bukan dari pendapatan bank.
INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source)
SELECT DISTINCT m.product_id, 'LOAN_WRITE_OFF'::posting_event, 'DEBIT'::entry_direction, '12500', 'PENALTY'
FROM product_journal_mapping m
JOIN banking_products p ON p.id = m.product_id
WHERE m.event = 'LOAN_WRITE_OFF'
  AND p.book = 'SYARIAH'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping x
      WHERE x.product_id = m.product_id AND x.event = 'LOAN_WRITE_OFF'
        AND x.direction = 'DEBIT' AND x.amount_source = 'PENALTY'
  );

INSERT INTO product_journal_mapping (product_id, event, direction, coa_code, amount_source)
SELECT DISTINCT m.product_id, 'LOAN_WRITE_OFF'::posting_event, 'CREDIT'::entry_direction, '11700', 'PENALTY'
FROM product_journal_mapping m
JOIN banking_products p ON p.id = m.product_id
WHERE m.event = 'LOAN_WRITE_OFF'
  AND p.book = 'SYARIAH'
  AND NOT EXISTS (
      SELECT 1 FROM product_journal_mapping x
      WHERE x.product_id = m.product_id AND x.event = 'LOAN_WRITE_OFF'
        AND x.direction = 'CREDIT' AND x.amount_source = 'PENALTY'
  );
