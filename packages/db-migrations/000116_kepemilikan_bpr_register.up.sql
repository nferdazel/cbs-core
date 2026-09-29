-- CBS Migration 000116: register pemegang saham BPR untuk Form 00.01
-- "Data Kepemilikan BPR".
-- Run after: 000115_ayda_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000116 dipesan khusus pekerjaan ini.
--
-- Menutup pendaftaran Form 00.01 yang sebelumnya "BELUM DIMODELKAN"
-- (definitions.go): dulu hanya ada tiga kunci teks pemegang saham Form 00.00
-- (migrasi 000096), tanpa komposisi kepemilikan. Tabel ADITIF di bawah menjadi
-- register per pemegang saham; tidak ada tabel/kolom yang diubah.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo,
-- tidak di-commit):
--   * Form 00.01 "DATA KEPEMILIKAN BPR": PDF #page 72 (hlm. tercetak 20). Delapan
--     kolom: I Nama, II Alamat, III Jenis (01/02/03/04), IV No. Identitas,
--     V Status Pemegang Saham (01/02), VI Jumlah Nominal, VII Persentase Kepemilikan,
--     VIII Status Perubahan (1/2/3/9). TIDAK ADA baris JUMLAH; satu baris = satu
--     pemegang saham.
--   * Sandi: PDF #page 73 (hlm. 21). III 01 Perorangan, 02 Badan Hukum,
--     03 Pemerintah Daerah, 04 Publik; V 01 PSP, 02 Non PSP.
--   * Penjelasan: PDF #page 74 (hlm. 22). VIII 1 pemegang saham baru, 2 perubahan
--     yang mengakibatkan perubahan PSP, 3 perubahan yang tidak mengakibatkan
--     perubahan PSP, 9 tidak ada perubahan. Alamat (II) dan No. Identitas (IV) dapat
--     dikosongkan untuk kepemilikan kurang dari 2%.
--
-- Kolom IV "No. Identitas" SENGAJA TIDAK DISIMPAN. Ini keputusan privasi yang
-- berlaku: NIK/NPWP tidak dibuka ke keluaran laporan (docs/KEPUTUSAN-OJK.md §4 dan
-- §7 butir 6). Sistem TIDAK menyimpan identitas terenkripsi pun; laporan menulis
-- kolom IV "-" beserta alasannya. Ini bukan celah kode yang perlu ditambal.
--
-- Kolom VII "Persentase Kepemilikan" TIDAK divalidasi agar seluruh baris berjumlah
-- 100%: jumlah pemegang saham dan komposisinya adalah urusan bank, bukan sistem.
-- Setiap baris tetap dijaga >= 0 dan <= 100 karena satu pemegang saham tidak dapat
-- memiliki lebih dari 100%.
--
-- Kolom II "Alamat" boleh kosong: form mengizinkannya untuk kepemilikan kurang dari
-- 2% (PDF #page 74). Bank mengisi apa adanya; sistem TIDAK menghapus/mengisi data
-- dan laporan mengikuti isi bank.
--
-- Semua NILAI adalah isian bank. Tidak ada seed angka.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS kepemilikan_bpr_register (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Nama pemegang saham (kolom I). Wajib terisi agar baris dapat dibaca.
    shareholder_name         VARCHAR(255) NOT NULL CHECK (length(btrim(shareholder_name)) > 0),
    -- Alamat pemegang saham (kolom II). Boleh kosong: form mengizinkannya untuk
    -- kepemilikan kurang dari 2% (PDF #page 74). Bank mengisi apa adanya.
    shareholder_address      VARCHAR(255) NOT NULL DEFAULT '',
    -- Jenis pemegang saham (kolom III): 01 Perorangan, 02 Badan Hukum,
    -- 03 Pemerintah Daerah, 04 Publik (PDF #page 73).
    shareholder_type_code    VARCHAR(2) NOT NULL
        CHECK (shareholder_type_code IN ('01', '02', '03', '04')),
    -- Status pemegang saham (kolom V): 01 PSP, 02 Non PSP (PDF #page 73).
    shareholder_status_code  VARCHAR(2) NOT NULL
        CHECK (shareholder_status_code IN ('01', '02')),
    -- Jumlah nominal kepemilikan (kolom VI), rupiah penuh. Angka bank.
    nominal_amount           NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (nominal_amount >= 0),
    -- Persentase kepemilikan (kolom VII), 0-100. TIDAK divalidasi berjumlah 100%.
    ownership_percentage     NUMERIC(9,4) NOT NULL DEFAULT 0
        CHECK (ownership_percentage >= 0 AND ownership_percentage <= 100),
    -- Status perubahan (kolom VIII): 1 pemegang saham baru, 2 perubahan yang
    -- mengakibatkan perubahan PSP, 3 perubahan yang tidak mengakibatkan perubahan PSP,
    -- 9 tidak ada perubahan (PDF #page 74).
    change_status_code       VARCHAR(1) NOT NULL
        CHECK (change_status_code IN ('1', '2', '3', '9')),
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                    DATE NOT NULL,
    -- AKTIF = pemegang saham masih tercatat dan ikut laporan; NONAKTIF = sudah tidak
    -- dicatat lagi tetapi tetap tersimpan sebagai riwayat.
    status                   VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                     TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by               UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by               UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE kepemilikan_bpr_register IS
    'Register pemegang saham BPR untuk Form 00.01 "Data Kepemilikan BPR"; sumber struktur PDF #page 72-74 (hlm. 20-22) SEOJK No. 16/SEOJK.03/2024. Semua nilai diisi bank. Kolom IV No. Identitas sengaja tidak disimpan (keputusan privasi, docs/KEPUTUSAN-OJK.md §4); kolom VII tidak divalidasi berjumlah 100%.';

COMMENT ON COLUMN kepemilikan_bpr_register.shareholder_name IS
    'Nama pemegang saham (kolom I). Wajib terisi agar baris dapat dibaca.';

COMMENT ON COLUMN kepemilikan_bpr_register.shareholder_address IS
    'Alamat pemegang saham (kolom II). Boleh kosong karena form mengizinkannya untuk kepemilikan kurang dari 2% (PDF #page 74); bank mengisi apa adanya dan sistem tidak mengarang alamat.';

COMMENT ON COLUMN kepemilikan_bpr_register.shareholder_type_code IS
    'Jenis pemegang saham (kolom III) baku PDF #page 73: 01 Perorangan, 02 Badan Hukum, 03 Pemerintah Daerah, 04 Publik.';

COMMENT ON COLUMN kepemilikan_bpr_register.shareholder_status_code IS
    'Status pemegang saham (kolom V) baku PDF #page 73: 01 PSP (pemegang saham pengendali), 02 Non PSP.';

COMMENT ON COLUMN kepemilikan_bpr_register.nominal_amount IS
    'Jumlah nominal kepemilikan (kolom VI), rupiah penuh. Angka bank; sistem tidak menurunkannya dari persentase kali modal.';

COMMENT ON COLUMN kepemilikan_bpr_register.ownership_percentage IS
    'Persentase kepemilikan (kolom VII), 0-100. TIDAK divalidasi agar seluruh baris berjumlah 100%: komposisi kepemilikan adalah urusan bank, bukan sistem.';

COMMENT ON COLUMN kepemilikan_bpr_register.change_status_code IS
    'Status perubahan (kolom VIII) baku PDF #page 74: 1 pemegang saham baru, 2 perubahan yang mengakibatkan perubahan PSP, 3 perubahan yang tidak mengakibatkan perubahan PSP, 9 tidak ada perubahan.';

COMMENT ON COLUMN kepemilikan_bpr_register.as_of IS
    'Tanggal posisi kepemilikan (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN kepemilikan_bpr_register.status IS
    'AKTIF = pemegang saham masih tercatat dan ikut laporan; NONAKTIF = sudah tidak dicatat namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_kepemilikan_bpr_register_type
    ON kepemilikan_bpr_register (shareholder_type_code);
CREATE INDEX IF NOT EXISTS idx_kepemilikan_bpr_register_status_code
    ON kepemilikan_bpr_register (shareholder_status_code);
CREATE INDEX IF NOT EXISTS idx_kepemilikan_bpr_register_as_of
    ON kepemilikan_bpr_register (as_of);
CREATE INDEX IF NOT EXISTS idx_kepemilikan_bpr_register_status
    ON kepemilikan_bpr_register (status);
