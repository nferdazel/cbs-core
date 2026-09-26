-- CBS Migration 000103: fondasi BMPK (Batas Maksimum Pemberian Kredit) untuk BPR/BPRS.
-- Run after: 000102_ojk_reference_codes.up.sql (urutan; tidak bergantung isinya).
--
-- Ruang lingkup: MENYEDIAKAN SEAM DATA, bukan menegakkan aturan. Nomor/format kolom
-- resmi Laporan BMPK (Lampiran SEOJK) belum ada di repo ini, sehingga migrasi ini
-- SENGAJA tidak meng-hardcode persentase atau plafon regulasi apa pun. Bank mengisi
-- pihak terkait dan batasnya; modul membandingkan paparan dengan batas yang diisi.
--
-- Mengapa tabel baru:
--   * bmpk_related_parties menandai NASABAH yang bank golongkan sebagai pihak terkait
--     beserta jenis hubungannya. Tabel customers TIDAK diubah (dipakai modul lain).
--   * bmpk_limits menyimpan BATAS per pihak terkait yang ditetapkan bank (nominal).
--     Bentuk batas sebagai persentase modal menunggu keputusan bank/OJK; karena itu
--     batas disimpan eksplisit, bukan diturunkan dari angka yang tidak ada rujukannya.
--   * lps_placements.customer_id menautkan penempatan pada bank lain ke nasabah pihak
--     terkait, SUPERSEDING pencocokan nama (teks) yang dilarang di modul lain. Kolom
--     ini nullable: penempatan lama/di luar pihak terkait dibiarkan NULL.
--
-- Saklar bmpk.enabled bawaan 'false' (non-enforcing): selama mati modul tidak menegakkan
-- apa pun dan status batas hanya dilaporkan sebagai informasi. Bank menyalakannya setelah
-- mengisi data.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS, kolom hanya
-- ditambahkan bila belum ada, seed konfigurasi tidak menimpa nilai bank
-- (INSERT ... ON CONFLICT (key) DO NOTHING).

CREATE TABLE IF NOT EXISTS bmpk_related_parties (
    -- Pengisian bank lewat SQL/seed; satu nasabah hanya punya satu penandaan.
    customer_id       UUID PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
    -- Taksonomi jenis hubungan adalah keputusan bank, jadi disimpan sebagai teks bebas
    -- (bukan enum yang mengarang sandi). Wajib tidak kosong.
    relationship_type VARCHAR(64) NOT NULL CHECK (length(btrim(relationship_type)) > 0),
    note              TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE bmpk_related_parties IS
    'Nasabah yang digolongkan bank sebagai pihak terkait BMPK beserta jenis hubungannya. Bank mengisi; tabel customers tidak diubah.';
COMMENT ON COLUMN bmpk_related_parties.relationship_type IS
    'Jenis hubungan pihak terkait menurut kebijakan bank (teks bebas, mis. PEMILIK/PENGURUS/KELUARGA/GRUP). Taksonomi resmi menunggu keputusan bank/OJK.';

CREATE TABLE IF NOT EXISTS bmpk_limits (
    customer_id    UUID PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
    -- Batas nominal (rupiah penuh) yang ditetapkan bank untuk pihak ini. Nol sah
    -- (berarti tidak boleh ada eksposur lagi), karena itu TIDAK dianggap "belum diset".
    max_amount     NUMERIC(28,4) NOT NULL CHECK (max_amount >= 0),
    effective_date DATE,
    note           TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE bmpk_limits IS
    'Batas BMPK per pihak terkait yang ditetapkan bank (nominal rupiah). Baris tidak ada = batas belum diset, status BATAS_BELUM_DISET (bukan nol).';
COMMENT ON COLUMN bmpk_limits.max_amount IS
    'Batas maksimum paparan untuk satu pihak terkait. Nol berarti tidak boleh ada eksposur tambahan; baris tidak ada berarti batas belum ditetapkan.';
COMMENT ON COLUMN bmpk_limits.effective_date IS
    'Tanggal berlaku batas. NULL = belum dicatat, bukan berarti batas berlaku sejak awal waktu.';

-- Tautan penempatan pada bank lain ke nasabah pihak terkait. Nullable dan aditif;
-- repository penempatan lama tidak diubah dan mengabaikan kolom ini.
ALTER TABLE lps_placements
    ADD COLUMN IF NOT EXISTS customer_id UUID REFERENCES customers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_lps_placements_customer_id
    ON lps_placements (customer_id);

COMMENT ON COLUMN lps_placements.customer_id IS
    'Nasabah pihak terkait yang terkait dengan penempatan ini (bila ada). Bank mengisi lewat SQL; NULL = belum ditautkan sehingga tidak masuk agregasi BMPK.';

-- Saklar modul. Bawaan 'false' agar tidak ada perubahan perilaku sampai bank mengisi
-- pihak terkait dan batasnya.
INSERT INTO system_config (key, value, description) VALUES
    ('bmpk.enabled', 'false',
     'Aktifkan modul BMPK (Batas Maksimum Pemberian Kredit). false = tidak menegakkan apa pun; status batas hanya dilaporkan sebagai informasi. Bank mengisi bmpk_related_parties dan bmpk_limits sebelum menyalakan.')
ON CONFLICT (key) DO NOTHING;
