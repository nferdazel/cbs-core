-- CBS Migration 000131: register divisi/satuan kerja (Form 00.19 Struktur Organisasi).
-- Run after: 000130_pihak_lawan_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000131 dipesan khusus pekerjaan ini.
--
-- Sumber struktur (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 00.19 "STRUKTUR ORGANISASI BPR": PDF #page 298 (hlm. tercetak 246); terdaftar di
--     daftar Laporan Gabungan PDF #page 60. Isi peraturan: "Struktur organisasi BPR ...
--     mencakup susunan hierarki seluruh jaringan kantor yang dimiliki oleh BPR, divisi
--     atau satuan kerja, dan nama pegawai tetap atau tidak tetap BPR."
--   * Sebelumnya bagian "divisi atau satuan kerja" hanya ditandai belum tersedia di dalam
--     dokumen karena belum ada sumbernya. Migrasi ini MENYEDIAKAN sumbernya.
--
-- KEPUTUSAN BENTUK:
--   * Tabel baru bank_work_units menyimpan divisi/satuan kerja apa adanya: nama, jenis,
--     unit induk (parent), kepala unit, dan jumlah pegawai. Sistem TIDAK mengarang relasi
--     induk yang tidak bank isi; parent_code boleh kosong.
--   * Hierarki ditulis sebagai teks kode (parent_code), bukan FK ke baris lain, karena
--     bank menatausahakannya sendiri dan boleh mengisi unit sebelum induknya ada. Dokumen
--     00.19 yang menyusun urutan tampil dari hubungan ini.
--   * NIK pegawai TIDAK disimpan di sini (keputusan privasi §4 butir III); dokumen tidak
--     mencetak identitas pribadi, hanya nama dan jabatan.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada seed
-- data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS bank_work_units (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Kode unik unit yang ditetapkan bank (mis. DIV-OPS). Wajib dan unik agar hierarki
    -- bisa dirujuk parent_code.
    code        VARCHAR(32) NOT NULL UNIQUE CHECK (length(btrim(code)) > 0),
    nama        VARCHAR(255) NOT NULL CHECK (length(btrim(nama)) > 0),
    -- Jenis unit (divisi/satuan kerja/bagian/seksi/...). Teks bebas keputusan bank,
    -- bukan enum yang mengarang taksonomi.
    jenis       VARCHAR(64) NOT NULL DEFAULT '',
    -- parent_code menunjuk kode unit induk; kosong = unit puncak. Bukan FK supaya bank
    -- boleh mengisi anak sebelum induk ada dan urutan migrasi tidak terkunci.
    parent_code VARCHAR(32) NOT NULL DEFAULT '',
    -- kepala_unit: nama kepala unit; kosong = belum diisi.
    kepala_unit VARCHAR(255) NOT NULL DEFAULT '',
    -- jumlah_pegawai: jumlah pegawai tetap/tidak tetap pada unit; NULL = belum diisi,
    -- dibedakan dari nol.
    jumlah_pegawai INT CHECK (jumlah_pegawai IS NULL OR jumlah_pegawai >= 0),
    urutan      INT NOT NULL DEFAULT 0,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by  UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE bank_work_units IS
    'Divisi atau satuan kerja untuk Form 00.19 "Struktur Organisasi BPR" (PDF #page 298). Bank menatausahakan sendiri; hierarki ditulis sebagai parent_code, bukan FK. NIK pegawai tidak disimpan (keputusan privasi).';

COMMENT ON COLUMN bank_work_units.code IS
    'Kode unik unit yang ditetapkan bank; dirujuk parent_code. Wajib dan unik.';

COMMENT ON COLUMN bank_work_units.parent_code IS
    'Kode unit induk. Kosong = unit puncak. Bukan FK agar bank boleh mengisi anak sebelum induk ada.';

COMMENT ON COLUMN bank_work_units.jumlah_pegawai IS
    'Jumlah pegawai tetap/tidak tetap pada unit. NULL = belum diisi (dibedakan dari nol).';

CREATE INDEX IF NOT EXISTS idx_bank_work_units_parent
    ON bank_work_units (parent_code);
