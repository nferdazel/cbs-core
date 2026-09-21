-- CBS Migration 000056: sequence nomor kredit tersendiri (loan_number_seq)
-- Run after: 000055_loan_rejection_reason.up.sql (nomor dipesan; tidak bergantung isi).
--
-- Mengapa migrasi ini ada: pembangkit nomor kredit memakai Next(TxTypeTransferInternal),
-- sehingga nomor kredit berformat TRF-YYYYMMDD-NNNNNN dan setiap pencairan memakan
-- urutan referensi transaksi (journal_reference_seq). Dua domain berbeda berbagi satu
-- urutan, sehingga nomor kredit dan nomor referensi jurnal saling menggeser.
--
-- Sequence ini terpisah dari journal_reference_seq dan dipakai khusus nomor kredit
-- (KRD- untuk konvensional, PMB- untuk syariah). Nomor kredit lama yang sudah terbit
-- TIDAK di-backfill dan TIDAK diubah: urutan mulai dari 1 lagi, dan baris lama tetap
-- memakai nomornya masing-masing karena UNIQUE hanya melarang duplikat persis.
--
-- Aditif dan idempotent: CREATE SEQUENCE IF NOT EXISTS, aman dijalankan ulang.

CREATE SEQUENCE IF NOT EXISTS loan_number_seq START 1 INCREMENT 1;
