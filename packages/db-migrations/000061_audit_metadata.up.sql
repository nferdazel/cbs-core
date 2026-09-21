-- CBS Migration 000061: kolom metadata pada audit_logs
-- Run after: 000060_bagi_hasil_range.up.sql
--
-- Latar belakang:
--   Alasan penolakan kredit ditulis ke changes.reason, tetapi pembaca audit lama
--   membaca metadata dan mendapat NULL, sehingga keputusan tampak tanpa alasan.
--   changes tetap menjadi diff terstruktur (status/amount); metadata menyimpan
--   konteks bebas (mis. reason) yang diharapkan pembaca tersebut. Alasan tetap
--   ditulis ke changes.reason agar pembaca yang sudah bergantung padanya tidak
--   rusak, jadi kedua kolom memuat nilai yang sama untuk aksi ini.
--
-- Kolom aditif dan nullable: baris lama tetap NULL dan pemanggil lama tidak
-- berubah. Idempotent dan aman dijalankan ulang.

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS metadata JSONB;

COMMENT ON COLUMN audit_logs.metadata IS
    'Konteks bebas aksi (mis. alasan penolakan kredit) yang dibaca pembaca audit lama; changes tetap diff terstruktur.';
