-- CBS Migration 000128: register modal (Form 00.06) "Daftar Modal Disetor, Modal
-- Sumbangan, dan Dana Setoran Modal - Ekuitas".
-- Run after: 000127_pihak_terkait_lainnya_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000128 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 00.06 "DAFTAR MODAL DISETOR, MODAL SUMBANGAN, DAN DANA SETORAN MODAL -
--     EKUITAS": susunan PDF #page 245 (hlm. tercetak 193), sandi PDF #page 246 (hlm. 194),
--     penjelasan PDF #page 247 (hlm. 195). Empat kolom: I Jenis, II Tanggal Persetujuan
--     Otoritas, III Jenis Modal, IV Jumlah. ADA baris JUMLAH.
--   * Sandi I Jenis (PDF #246): 01 Dana, 02 Tanah dan bangunan yang dapat diperhitungkan
--     sebagai modal inti, 03 Tanah dan bangunan yang tidak dapat diperhitungkan sebagai
--     modal inti.
--   * Sandi III Jenis Modal (PDF #246): 01 Modal Disetor, 02 Modal Sumbangan,
--     03 Dana Setoran Modal - Ekuitas.
--
-- MENGAPA TABEL BARU, BUKAN MENURUNKAN DARI SALDO COA:
--   * coa_mapping.go memetakan saldo 30100/13100 (Modal Disetor) ke kelas modal inti
--     Form 01.00. Saldo bagan akun TIDAK menyimpan dimensi yang diminta Form 00.06:
--     bentuk setoran (dana vs tanah/bangunan yang dapat/tidak dapat diperhitungkan
--     sebagai modal inti), tanggal persetujuan otoritas, dan pemisahan Modal Sumbangan /
--     Dana Setoran Modal - Ekuitas. Menurunkan form ini dari saldo tunggal akan MENGARANG
--     dimensi yang tidak ada di data.
--   * Karena itu bank mencatat peristiwa modal satu per satu; register menyimpannya apa
--     adanya. Jumlah per jenis adalah isian bank, bukan hasil hitung sistem.
--
-- KEPUTUSAN KOLOM:
--   * Sandi I dan III adalah ISIAN BANK, ditegakkan CHECK di sini + validasi domain.
--   * II Tanggal Persetujuan Otoritas boleh kosong (mis. modal yang belum disetujui
--     otoritas); NULL = belum dicatat, BUKAN tanggal nol.
--   * IV Jumlah rupiah penuh; nol sah (mis. baris informasi), TIDAK dianggap belum diisi.
--   * Tidak ada nomor register unik pada form, sehingga penghapusan adalah DELETE fisik.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada seed
-- data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS modal_register (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Jenis (kolom I) baku PDF #246: 01 Dana, 02 Tanah/bangunan (modal inti),
    -- 03 Tanah/bangunan (bukan modal inti).
    jenis_code         VARCHAR(2) NOT NULL CHECK (jenis_code IN ('01', '02', '03')),
    -- Tanggal Persetujuan Otoritas (kolom II), format tanggal. NULL = belum dicatat.
    tanggal_persetujuan DATE,
    -- Jenis Modal (kolom III) baku PDF #246: 01 Modal Disetor, 02 Modal Sumbangan,
    -- 03 Dana Setoran Modal - Ekuitas.
    jenis_modal_code   VARCHAR(2) NOT NULL
        CHECK (jenis_modal_code IN ('01', '02', '03')),
    -- Jumlah (kolom IV) rupiah penuh; nol sah.
    jumlah             NUMERIC(28,4) NOT NULL CHECK (jumlah >= 0),
    note               TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by         UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE modal_register IS
    'Register peristiwa modal untuk Form 00.06 "Daftar Modal Disetor, Modal Sumbangan, dan Dana Setoran Modal - Ekuitas" (PDF #page 245-247). Bank mencatat satu baris per setoran/sumbangan; Jumlah adalah isian bank, bukan turunan saldo bagan akun.';

COMMENT ON COLUMN modal_register.jenis_code IS
    'Form 00.06 kolom I Jenis (baku PDF #246): 01 Dana, 02 Tanah dan bangunan yang dapat diperhitungkan sebagai modal inti, 03 Tanah dan bangunan yang tidak dapat diperhitungkan sebagai modal inti.';

COMMENT ON COLUMN modal_register.tanggal_persetujuan IS
    'Form 00.06 kolom II Tanggal Persetujuan Otoritas (TT-BB-TTTT pada form). NULL = belum dicatat, bukan berarti tanggal nol.';

COMMENT ON COLUMN modal_register.jenis_modal_code IS
    'Form 00.06 kolom III Jenis Modal (baku PDF #246): 01 Modal Disetor, 02 Modal Sumbangan, 03 Dana Setoran Modal - Ekuitas.';

COMMENT ON COLUMN modal_register.jumlah IS
    'Form 00.06 kolom IV Jumlah (rupiah penuh): nominal yang diakui sebagai modal. Nol sah, bukan berarti belum diisi.';

CREATE INDEX IF NOT EXISTS idx_modal_jenis_modal
    ON modal_register (jenis_modal_code);
CREATE INDEX IF NOT EXISTS idx_modal_jenis
    ON modal_register (jenis_code);
