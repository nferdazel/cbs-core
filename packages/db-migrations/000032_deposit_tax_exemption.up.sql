-- CBS Migration 000032: ambang pembebasan PPh final atas bunga deposito
-- Run after: 000031_write_off_receivable_release.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- PP 131/2000 Pasal 3 huruf a membebaskan pemotongan PPh atas bunga deposito dan
-- tabungan "sepanjang jumlah deposito dan tabungan ... tidak melebihi Rp 7.500.000,00
-- dan bukan merupakan jumlah yang dipecah-pecah"; Pasal 2 hanya mengenakan tarif 20%
-- dari bruto bila jumlah deposito MELAMPAUI angka itu. Yang dibandingkan adalah JUMLAH
-- DEPOSITONYA, bukan bunganya.
--
-- Tanpa kunci ini, aplikasi memotong 20% untuk setiap deposito tanpa kecuali: nasabah
-- deposito Rp 5 juta kehilangan haknya dan bank mencatat utang pajak yang tidak
-- seharusnya, yang harus dikembalikan lewat mekanisme koreksi.
--
-- Nilai 0 pada tax.deposit.exempt_amount mematikan pembebasan (seluruh deposito
-- dikenakan tarif), untuk bank yang memutuskan ambang ini tidak berlaku.
INSERT INTO system_config (key, value, description)
VALUES (
    'tax.deposit.exempt_amount',
    '7500000',
    'Ambang jumlah deposito (Rp) yang dibebaskan dari pemotongan PPh final bunga deposito (PP 131/2000 Pasal 3 huruf a). 0 = tanpa pembebasan.'
)
ON CONFLICT (key) DO NOTHING;

-- Tarif ditulis eksplisit agar terlihat di konfigurasi, bukan hanya tersembunyi sebagai
-- default kode. Nilainya sama dengan default aplikasi (20%).
INSERT INTO system_config (key, value, description)
VALUES (
    'tax.deposit.rate_pct',
    '20',
    'Tarif PPh final atas bunga deposito dalam persen dari bruto (PP 131/2000 Pasal 2).'
)
ON CONFLICT (key) DO NOTHING;
