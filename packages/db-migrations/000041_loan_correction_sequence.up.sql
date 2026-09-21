-- CBS Migration 000041: penghitung koreksi nominal kredit untuk kunci idempotensi.
-- Run after: 000040 (nomor dipesan; migrasi ini tidak bergantung isinya selain urutan).
--
-- Mengapa migrasi ini ada: kunci idempotensi jurnal koreksi nominal sebelumnya dibentuk
-- hanya dari nomor kredit dan nominal baru ("CORR-<nomor>-<nominal>"). Rangkaian
-- 10jt -> 12jt -> 10jt -> 12jt memakai kembali kunci langkah pertama pada langkah
-- terakhir. Posting engine mengenali kunci itu sudah pernah dipakai, mengembalikan
-- jurnal lama TANPA menulis selisih baru, padahal state kredit tetap diperbarui —
-- saldo rekening nasabah dan pokok kredit menjadi berbeda. Penghitung per kredit
-- membuat setiap peristiwa koreksi punya kunci tersendiri. Penaikannya dilakukan di
-- dalam transaksi yang sama dengan jurnalnya, sehingga transaksi yang gagal
-- (rollback) tidak meninggalkan nomor yang sudah terpakai.
--
-- Aditif dan idempotent: kolom baru dengan bawaan 0, aman dijalankan ulang. Tidak ada
-- backfill yang diperlukan karena kredit lama yang belum pernah dikoreksi bernilai 0.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS correction_count INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN loans.correction_count IS
    'Jumlah koreksi nominal yang sudah diposting untuk kredit ini; dipakai sebagai bagian kunci idempotensi jurnal koreksi.';
