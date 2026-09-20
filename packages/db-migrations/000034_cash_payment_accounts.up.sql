-- CBS Migration 000034: akun kas untuk penerimaan angsuran tunai
-- Run after: 000033_savings_tax_config.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Pembayaran angsuran sebelumnya selalu mendebit rekening nasabah, sehingga teller yang
-- menerima uang tunai tidak punya cara mencatatnya tanpa menyetor dulu ke rekening
-- nasabah. Padahal penerimaan tunai di kas adalah transaksi yang berbeda: uangnya masuk
-- kas, bukan berpindah dari rekening nasabah.
--
-- Akun kas dipisah per buku supaya dana UUS tidak tercampur dengan konvensional,
-- mengikuti bagan COA migrasi 000005 (10101 Kas Teller, 11100 Kas Syariah).
INSERT INTO system_config (key, value, description)
VALUES (
    'payment.cash.coa.conventional',
    '10101',
    'Kode COA kas teller untuk penerimaan angsuran kredit tunai buku konvensional.'
)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_config (key, value, description)
VALUES (
    'payment.cash.coa.syariah',
    '11100',
    'Kode COA kas untuk penerimaan angsuran pembiayaan tunai buku syariah.'
)
ON CONFLICT (key) DO NOTHING;
