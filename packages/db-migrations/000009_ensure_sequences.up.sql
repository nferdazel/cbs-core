-- CBS Migration 000009: Pastikan sequence inti ada
-- Run after: 000008_journal_entry_date.up.sql
--
-- Latar belakang: migrasi 000005 di environment live dijalankan tanpa ON_ERROR_STOP
-- sehingga berhenti di tengah dan menyisakan journal_reference_seq tidak terbuat.
-- Akibatnya generator referensi jurnal jatuh ke fallback timestamp yang tidak dijamin
-- unik. Migrasi ini memastikan seluruh sequence inti ada dan idempotent, sehingga aman
-- dijalankan berulang.

CREATE SEQUENCE IF NOT EXISTS journal_reference_seq START 1 INCREMENT 1;
CREATE SEQUENCE IF NOT EXISTS cif_number_seq START 1 INCREMENT 1;
