-- CBS Migration 000033: konfigurasi PPh final atas bunga tabungan
-- Run after: 000032_deposit_tax_exemption.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Bank adalah pemotong PPh final atas bunga tabungan yang dibayarkannya (PP 131/2000
-- Pasal 1). Pembayaran bunga tabungan bulanan sebelumnya memposting bruto tanpa
-- potongan sama sekali, sehingga tidak ada utang pajak yang diakui dan nasabah
-- menerima bunga yang seharusnya sudah dipotong.
--
-- Ambang pembebasan membandingkan SALDO tabungan, bukan bunga yang dibayarkan:
-- Pasal 3 huruf a membebaskan pemotongan sepanjang "jumlah deposito dan tabungan ...
-- tidak melebihi Rp 7.500.000,00 dan bukan merupakan jumlah yang dipecah-pecah".
--
-- Bagi hasil mudharabah (buku syariah) belum dipotong karena bukan bunga; perlakuan
-- pajaknya menunggu keputusan bank dan dicatat sebagai pekerjaan terbuka di backlog.
INSERT INTO system_config (key, value, description)
VALUES (
    'tax.savings.rate',
    '20',
    'Tarif PPh final atas bunga tabungan dalam persen dari bruto (PP 131/2000 Pasal 2).'
)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_config (key, value, description)
VALUES (
    'tax.savings.exempt_amount',
    '7500000',
    'Ambang saldo tabungan (Rp) yang dibebaskan dari pemotongan PPh final bunga tabungan (PP 131/2000 Pasal 3 huruf a). 0 = tanpa pembebasan.'
)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_config (key, value, description)
VALUES (
    'tax.savings.payable.coa',
    '20500',
    'Kode COA utang pajak (20500 Utang Pajak) untuk PPh final yang dipotong dari bunga tabungan.'
)
ON CONFLICT (key) DO NOTHING;
