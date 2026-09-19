-- CBS Migration 000025: Profil bank untuk dokumen cetak
-- Run after: 000024_penalty_receivable_and_social_fund.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Dokumen cetak (slip setoran/penarikan, surat perjanjian kredit, struk thermal)
-- sebelumnya menuliskan identitas bank sebagai literal di kode. Identitas bank
-- adalah data konfigurasi: operator harus dapat mengubahnya tanpa membangun ulang
-- aplikasi, dan dokumen tidak boleh mengarang nama bank.
--
-- Tabel ini menyimpan satu baris profil bank. Tidak ada akun GL karena profil bank
-- bukan peristiwa akuntansi.

CREATE TABLE IF NOT EXISTS bank_profile (
    id         SMALLINT PRIMARY KEY DEFAULT 1,
    bank_name  TEXT NOT NULL DEFAULT '',
    address    TEXT NOT NULL DEFAULT '',
    city       TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    npwp       TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT bank_profile_single_row CHECK (id = 1)
);

-- Satu baris kosong agar pengisian profil cukup lewat UPDATE. Nilai bank_name
-- kosong ditafsirkan service sebagai "profil bank belum dikonfigurasi" dan
-- dokumen menandainya secara eksplisit.
INSERT INTO bank_profile (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;
