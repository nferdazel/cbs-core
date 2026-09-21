-- CBS Migration 000057: nilai enum DEPOSIT_PLACEMENT pada transaction_type
-- Run after: 000056_loan_number_sequence.up.sql (nomor dipesan; tidak bergantung isi).
--
-- Mengapa migrasi ini ada: jurnal penempatan deposito berjangka memakai
-- TransactionType = 'DEPOSIT', sama dengan setoran tunai teller, sehingga di laporan
-- keduanya tidak dapat dibedakan. Jenis tersendiri memisahkan penempatan deposito dari
-- setoran tunai tanpa menyentuh jurnal lama.
--
-- Jurnal lama SENGAJA tidak diubah: transaction_type = 'DEPOSIT' dipakai bersama oleh
-- setoran tunai teller dan penempatan deposito, dan tidak ada penanda pasti di baris
-- lama untuk membedakannya. Mengubahnya berdasarkan deskripsi/prefix berisiko salah
-- kelompok, jadi keputusan yang aman adalah membiarkan data lama apa adanya.
--
-- Catatan operasional: ALTER TYPE ... ADD VALUE tidak dapat dijalankan di dalam blok
-- transaksi pada PostgreSQL < 12. Migrasi ini hanya berisi satu pernyataan ADD VALUE
-- dan tidak memakai nilai barunya di file yang sama, sehingga aman pada PostgreSQL 12+.
-- Idempotent: IF NOT EXISTS membuat migrasi aman dijalankan ulang.

ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'DEPOSIT_PLACEMENT';
