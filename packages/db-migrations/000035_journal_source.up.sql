-- CBS Migration 000035: penanda alur jurnal (journal_entries.source)
-- Run after: 000034_cash_payment_accounts.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- transaction_type tidak dapat dipakai memisahkan alur bisnis: DEPOSIT dan WITHDRAWAL
-- dipakai bersama oleh transaksi teller (setoran/penarikan rekening) dan oleh penempatan
-- serta pencairan deposito berjangka, dan jurnal kredit memakai label yang sama. Akibatnya
-- pembatalan transaksi tidak bisa membedakan jurnal yang aman dibatalkan dari jurnal yang
-- membawa state domain (deposito berstatus CLOSED, jadwal angsuran, akrual).
--
-- Kolom ini mencatat alur yang membuat jurnal, diisi saat posting. Nilai kosong berarti
-- alur belum ditandai; pemakaiannya bersifat menolak secara bawaan: jurnal tanpa penanda
-- alur tidak boleh dibatalkan, sehingga jurnal lama dan alur yang belum ditandai tetap
-- aman tanpa perlu ditebak.
--
-- Kolom ini juga menjadi dasar pemisahan wewenang berikutnya (mis. pembatalan jurnal
-- kredit harus lewat fitur koreksi kredit, bukan pembatalan transaksi rekening).
ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS source VARCHAR(16) NOT NULL DEFAULT '';

COMMENT ON COLUMN journal_entries.source IS
    'Alur pembuat jurnal: TELLER, LOAN, DEPOSIT, BATCH. Kosong = belum ditandai, tidak boleh dibatalkan.';
