-- 000073_unify_cash_coa_config.up.sql
-- Menyatukan DUA kunci konfigurasi untuk konsep yang sama: akun kas teller per buku.
--
-- ALASAN: dua kunci menyimpan akun yang sama dan dibaca alur berbeda:
--   cash.coa.conventional / cash.coa.syariah
--       (migrasi 000051; dibaca ledger_service.resolveCashAccount untuk setoran,
--        penarikan, transfer, dan pembatalannya)
--   payment.cash.coa.conventional / payment.cash.coa.syariah
--       (migrasi 000034; dibaca loan_service.fundingSource untuk angsuran tunai)
-- Selama keduanya berbeda, setoran tunai dan angsuran tunai bisa mengarah ke akun kas
-- yang BERBEDA padahal keduanya uang fisik di teller, sehingga rekonsiliasi kas per
-- buku tidak lagi menjelaskan saldo sebenarnya. Sumber kebenaran yang berlaku adalah
-- cash.coa.*; kunci payment.cash.coa.* dihapus.
--
-- Nilai TIDAK diubah bila kedua kunci sudah sama (keduanya 10101/11100 di seed).
-- Bila keduanya ada dan BERBEDA, migrasi ini gagal dengan pesan jelas: bank harus
-- memutuskan akun mana yang benar, bukan dibiarkan berpindah diam-diam. Aditif dan
-- idempotent: aman dijalankan ulang dan tidak menimpa nilai operator yang sudah ada.

-- 1. Bila cash.coa.* belum ada (mis. hanya kunci lama yang terisi pada lingkungan
--    yang di-provision khusus), salin nilainya agar alur tidak jatuh ke bawaan kode.
INSERT INTO system_config (key, value, description)
SELECT 'cash.coa.conventional', value,
       'Kode COA kas teller buku konvensional sebagai lawan jurnal transaksi rekening. Disatukan dari payment.cash.coa.conventional (migrasi 000073).'
FROM system_config
WHERE key = 'payment.cash.coa.conventional'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_config (key, value, description)
SELECT 'cash.coa.syariah', value,
       'Kode COA kas buku syariah sebagai lawan jurnal transaksi rekening. Disatukan dari payment.cash.coa.syariah (migrasi 000073).'
FROM system_config
WHERE key = 'payment.cash.coa.syariah'
ON CONFLICT (key) DO NOTHING;

-- 2. Penjaga: hentikan migrasi bila kedua kunci ada dan nilainya berbeda, supaya
--    perbedaan itu diputuskan bank, bukan disamarkan dengan menghapus salah satunya.
DO $$
DECLARE
    cash_conv    text;
    payment_conv text;
    cash_syar    text;
    payment_syar text;
BEGIN
    SELECT value INTO cash_conv    FROM system_config WHERE key = 'cash.coa.conventional';
    SELECT value INTO payment_conv FROM system_config WHERE key = 'payment.cash.coa.conventional';
    IF cash_conv IS NOT NULL AND payment_conv IS NOT NULL AND cash_conv <> payment_conv THEN
        RAISE EXCEPTION 'Akun kas konvensional berbeda: cash.coa.conventional=% dan payment.cash.coa.conventional=%. Satukan nilainya lebih dulu sebelum migrasi 000073 dijalankan.', cash_conv, payment_conv;
    END IF;

    SELECT value INTO cash_syar    FROM system_config WHERE key = 'cash.coa.syariah';
    SELECT value INTO payment_syar FROM system_config WHERE key = 'payment.cash.coa.syariah';
    IF cash_syar IS NOT NULL AND payment_syar IS NOT NULL AND cash_syar <> payment_syar THEN
        RAISE EXCEPTION 'Akun kas syariah berbeda: cash.coa.syariah=% dan payment.cash.coa.syariah=%. Satukan nilainya lebih dulu sebelum migrasi 000073 dijalankan.', cash_syar, payment_syar;
    END IF;
END $$;

-- 3. Hapus kunci lama. Setelah ini hanya cash.coa.* yang tersisa dan dibaca kode.
DELETE FROM system_config WHERE key = 'payment.cash.coa.conventional';
DELETE FROM system_config WHERE key = 'payment.cash.coa.syariah';
