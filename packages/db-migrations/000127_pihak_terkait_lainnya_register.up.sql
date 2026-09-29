-- CBS Migration 000127: register pihak terkait lainnya untuk Form 00.05
-- "Data Pihak Terkait Lainnya".
-- Run after: 000126_form00_11_office_columns.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000127 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 00.05 "DATA PIHAK TERKAIT LAINNYA": PDF #page 96 (hlm. tercetak 44) susunan,
--     sandi PDF #page 97 (hlm. 45), penjelasan PDF #page 98 (hlm. 46). Lima kolom:
--     I Nama Pihak Terkait, II No. Identitas, III Alamat Pihak Terkait,
--     IV Jenis Pihak Terkait, V Hubungan Pihak Terkait. TIDAK ada baris JUMLAH.
--   * Sandi IV Jenis: 01 Perorangan, 02 Perusahaan/Badan, 03 Pemerintah Daerah/Pusat
--     (PDF #97). Sandi V Hubungan: 01-06 (PDF #97), masing-masing dengan definisi rinci
--     di PDF #98 (pengendali/keluarga, perusahaan bukan bank >=25%, BPR lain >=10%,
--     rangkap komisaris >=50%, dst.).
--
-- MENGAPA TABEL BARU, BUKAN MENAMBAH KOLOM DI bmpk_related_parties:
--   * bmpk_related_parties (migrasi 000103) menandai NASABAH sebagai pihak terkait BMPK:
--     kuncinya customer_id (FK NOT NULL ke customers) dan dipakai bersama bmpk_limits
--     untuk BATAS PAPARAN. Semantiknya BMPK, bukan pelaporan.
--   * Form 00.05 adalah daftar pihak terkait untuk PELAPORAN dan memuat pihak yang
--     "selain pemegang saham, anggota direksi, anggota dewan komisaris, dan pejabat
--     eksekutif" — pihak yang bisa jadi BUKAN nasabah (mis. pemerintah daerah, perusahaan
--     pengendali, keluarga pengendali). Memaksa mereka masuk bmpk_related_parties akan
--     menuntut baris customers palsu dan mencampur dua maksud.
--   * Taksonomi resmi IV/V sekarang SUDAH ADA (PDF #97), sehingga triase lama "kurang
--     sandi Jenis & Hubungan" terjawab: sandi disimpan di register ini, bukan dikarang
--     di bmpk_related_parties.
--
-- KEPUTUSAN KOLOM:
--   * Kolom II No. Identitas (NIK/NPWP) SENGAJA TIDAK DISIMPAN mengikuti keputusan
--     privasi (docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6, pola Form 00.01/06.02/form
--     berhenti). Laporan menulis "-" beserta alasannya.
--   * Kolom I Nama Pihak Terkait adalah data yang memang dipaparkan (bukan identitas
--     rahasia); disimpan apa adanya.
--   * Sandi IV dan V adalah ISIAN BANK, ditegakkan CHECK di sini + validasi domain.
--     Definisi rinci tiap sandi V TIDAK disalin ke kode; cukup dinukil sebagai catatan.
--   * Tidak ada nomor register unik pada form, sehingga penghapusan adalah DELETE fisik.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada seed
-- data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS pihak_terkait_lainnya_register (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Nama Pihak Terkait (kolom I).
    nama        VARCHAR(255) NOT NULL CHECK (length(btrim(nama)) > 0),
    -- Alamat Pihak Terkait (kolom III), nullable: bank boleh belum melengkapi.
    alamat      TEXT NOT NULL DEFAULT '',
    -- Jenis Pihak Terkait (kolom IV) baku PDF #97: 01 Perorangan, 02 Perusahaan/Badan,
    -- 03 Pemerintah Daerah/Pusat.
    jenis_code  VARCHAR(2) NOT NULL
        CHECK (jenis_code IN ('01', '02', '03')),
    -- Hubungan Pihak Terkait (kolom V) baku PDF #97: 01-06.
    hubungan_code VARCHAR(2) NOT NULL
        CHECK (hubungan_code IN ('01', '02', '03', '04', '05', '06')),
    note        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by  UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE pihak_terkait_lainnya_register IS
    'Register pihak terkait lainnya untuk Form 00.05 "Data Pihak Terkait Lainnya" (PDF #page 96-98). Berisi pihak terkait BPR selain pemegang saham/direksi/komisaris/pejabat eksekutif; bisa bukan nasabah. Kolom II No. Identitas (NIK/NPWP) sengaja tidak disimpan (keputusan privasi).';

COMMENT ON COLUMN pihak_terkait_lainnya_register.nama IS
    'Form 00.05 kolom I Nama Pihak Terkait: nama lengkap pihak terkait selain pemegang saham/direksi/komisaris/pejabat eksekutif.';

COMMENT ON COLUMN pihak_terkait_lainnya_register.alamat IS
    'Form 00.05 kolom III Alamat Pihak Terkait. Kosong = bank belum melengkapi; laporan menulis "-".';

COMMENT ON COLUMN pihak_terkait_lainnya_register.jenis_code IS
    'Form 00.05 kolom IV Jenis Pihak Terkait (baku PDF #97): 01 Perorangan, 02 Perusahaan atau Badan, 03 Pemerintah Daerah atau Pemerintah Pusat. Isian bank.';

COMMENT ON COLUMN pihak_terkait_lainnya_register.hubungan_code IS
    'Form 00.05 kolom V Hubungan Pihak Terkait (baku PDF #97): 01-06. Definisi rinci di PDF #98; tidak disalin ke kode. Isian bank.';

COMMENT ON COLUMN pihak_terkait_lainnya_register.note IS
    'Catatan internal bank; tidak ikut menjadi kolom form.';

CREATE INDEX IF NOT EXISTS idx_pihak_terkait_jenis
    ON pihak_terkait_lainnya_register (jenis_code);
CREATE INDEX IF NOT EXISTS idx_pihak_terkait_hubungan
    ON pihak_terkait_lainnya_register (hubungan_code);
