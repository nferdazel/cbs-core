-- CBS Migration 000008: Tanggal entri jurnal (entry_date)
-- Run after: 000007_cif_number_sequence.up.sql
--
-- Latar belakang:
--   Modul laporan menghitung periode dari tanggal akuntansi entri jurnal, bukan dari
--   created_at (waktu teknis penulisan baris). Skema awal hanya punya posted_at dan
--   created_at, sehingga ditambahkan kolom entry_date sebagai tanggal entri yang
--   dipakai semua laporan (neraca saldo, laba/rugi, neraca, arus kas).
--
--   Default memakai tanggal UTC agar konsisten dengan posting service yang selalu
--   menyimpan posted_at = time.Now().UTC(). Baris lama di-backfill dari posted_at.

ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS entry_date DATE NOT NULL
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date;

UPDATE journal_entries
   SET entry_date = (posted_at AT TIME ZONE 'UTC')::date;

CREATE INDEX IF NOT EXISTS idx_journal_entry_date ON journal_entries(entry_date);
