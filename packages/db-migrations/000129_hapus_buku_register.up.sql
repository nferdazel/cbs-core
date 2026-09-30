-- CBS Migration 000129: register aset produktif yang dihapus buku (Form 15.00).
-- "Daftar Aset Produktif yang Dihapus Buku".
-- Run after: 000128_modal_register.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000129 dipesan khusus pekerjaan ini.
--
-- Sumber struktur form (PDF resmi SEOJK No. 16/SEOJK.03/2024, salinan di luar repo, tidak
-- di-commit):
--   * Form 15.00 "DAFTAR ASET PRODUKTIF YANG DIHAPUS BUKU": susunan PDF #page 218
--     (hlm. tercetak 166) kolom I-VI, PDF #page 219 (hlm. 167) kolom VII-X + baris JUMLAH,
--     sandi PDF #page 220 (hlm. 168), penjelasan PDF #page 221-222 (hlm. 169-170).
--   * Sepuluh kolom logis: I Sandi Kantor, II ID Pihak Lawan/Sandi Bank, III No. Rekening,
--     IV Jenis Aset, V Jenis Debitur, VI Hubungan dengan Bank, VII Tanggal Hapus Buku,
--     VIII Saldo Pokok (Saat Hapus Buku / Akumulasi Tertagih / Per Posisi Laporan),
--     IX Tunggakan Bunga (Saat Hapus Buku / Akumulasi Tertagih / Akumulasi Tambahan Bunga
--     Berjalan / Per Posisi Laporan), X Agunan (Jenis / Alamat / Nilai). ADA baris JUMLAH.
--   * Sandi IV Jenis Aset: 10 Kredit yang Diberikan, 20 Penempatan pada Bank Lain
--     (PDF #220). VI Hubungan dengan Bank: 11 Terkait Dalam Rangka Kesejahteraan,
--     12 Terkait Lainnya, 20 Tidak Terkait (PDF #220). X Jenis Agunan mengacu Lampiran 01;
--     tanpa agunan = 299 dan Nilai = 0 (PDF #222).
--
-- MENGAPA TABEL BARU:
--   * Form ini memuat SEMUA aset produktif yang dihapus buku, baik KREDIT (jenis aset 10)
--     maupun PENEMPATAN PADA BANK LAIN (jenis aset 20). Penempatan bukan kredit sehingga
--     tidak dapat disimpan pada tabel loans.
--   * Baris `loans` yang sudah ada menyimpan written_off_amount (migrasi 000085) untuk
--     agenda PEMULIHAN, tetapi TIDAK menyimpan dimensi pelaporan Form 15.00: tanggal hapus
--     buku, jenis aset, jenis debitur, hubungan dengan bank, pemisahan pokok vs bunga pada
--     saat hapus buku, akumulasi tertagih, dan agunan saat hapus buku. Menurunkannya dari
--     loans akan MENGARANG dimensi yang tidak ada.
--   * Karena itu bank mencatat baris aset produktif yang dihapus buku apa adanya pada
--     register ini; seluruh nominal adalah isian bank.
--
-- KEPUTUSAN KOLOM:
--   * Kolom V Jenis Debitur hanya bermakna untuk jenis aset 10 (kredit); untuk 20 boleh
--     kosong. Sandi mengacu Lampiran 02 dan disimpan apa adanya.
--   * Kolom X Agunan: tanpa agunan diserahkan -> jenis 299 dan nilai 0, sesuai PDF #222;
--     alamat tanda hubung berarti kosong. Nilai boleh 0.
--   * VIII/IX adalah nominal rupiah penuh isian bank; nol sah (mis. per posisi laporan
--     yang sudah tertagih penuh), TIDAK dianggap "belum diisi".
--   * Tanggal Hapus Buku wajib (inti form); disimpan sebagai DATE.
--   * Tidak ada nomor register unik pada form, sehingga penghapusan adalah DELETE fisik.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada seed
-- data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS hapus_buku_register (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Jenis Aset (kolom IV) baku PDF #220: 10 Kredit, 20 Penempatan pada Bank Lain.
    jenis_aset_code             VARCHAR(2) NOT NULL CHECK (jenis_aset_code IN ('10', '20')),
    -- ID Pihak Lawan / Sandi Bank (kolom II). Untuk kredit = ID pihak lawan; untuk
    -- penempatan = sandi bank. Disimpan apa adanya sebagai teks.
    pihak_lawan_id              VARCHAR(64) NOT NULL DEFAULT '',
    -- No. Rekening (kolom III).
    nomor_rekening              VARCHAR(64) NOT NULL DEFAULT '',
    -- Jenis Debitur (kolom V, Lampiran 02): hanya untuk jenis aset 10. Boleh kosong.
    jenis_debitur_code          VARCHAR(16) NOT NULL DEFAULT '',
    -- Hubungan dengan Bank (kolom VI) baku PDF #220: 11/12/20.
    hubungan_bank_code          VARCHAR(2) NOT NULL
        CHECK (hubungan_bank_code IN ('11', '12', '20')),
    -- Tanggal Hapus Buku (kolom VII), inti form; wajib.
    tanggal_hapus_buku          DATE NOT NULL,
    -- VIII Saldo Pokok: saat hapus buku / akumulasi tertagih / per posisi laporan.
    saldo_pokok_saat_hapus      NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (saldo_pokok_saat_hapus >= 0),
    saldo_pokok_akum_tertagih   NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (saldo_pokok_akum_tertagih >= 0),
    saldo_pokok_per_posisi      NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (saldo_pokok_per_posisi >= 0),
    -- IX Tunggakan Bunga: saat hapus buku / akumulasi tertagih / akumulasi tambahan bunga
    -- berjalan / per posisi laporan.
    bunga_saat_hapus            NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (bunga_saat_hapus >= 0),
    bunga_akum_tertagih         NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (bunga_akum_tertagih >= 0),
    bunga_akum_tambahan         NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (bunga_akum_tambahan >= 0),
    bunga_per_posisi            NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (bunga_per_posisi >= 0),
    -- X Agunan: jenis (Lampiran 01; 299 = tanpa agunan), alamat, nilai (0 bila tanpa agunan).
    agunan_jenis_code           VARCHAR(8) NOT NULL DEFAULT '',
    agunan_alamat               TEXT NOT NULL DEFAULT '',
    agunan_nilai                NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (agunan_nilai >= 0),
    note                        TEXT NOT NULL DEFAULT '',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                  UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by                  UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE hapus_buku_register IS
    'Register aset produktif yang dihapus buku untuk Form 15.00 (PDF #page 218-222). Memuat kredit maupun penempatan pada bank lain; berbeda dari kolom loans.written_off_amount yang hanya menopang pemulihan. Seluruh nominal isian bank.';

COMMENT ON COLUMN hapus_buku_register.jenis_aset_code IS
    'Form 15.00 kolom IV Jenis Aset (baku PDF #220): 10 Kredit yang Diberikan, 20 Penempatan pada Bank Lain.';

COMMENT ON COLUMN hapus_buku_register.pihak_lawan_id IS
    'Form 15.00 kolom II ID Pihak Lawan/Sandi Bank: ID pihak lawan debitur untuk kredit, sandi bank untuk penempatan pada bank lain.';

COMMENT ON COLUMN hapus_buku_register.jenis_debitur_code IS
    'Form 15.00 kolom V Jenis Debitur (Lampiran 02). Hanya diisi untuk jenis aset 10 (kredit); boleh kosong untuk penempatan.';

COMMENT ON COLUMN hapus_buku_register.hubungan_bank_code IS
    'Form 15.00 kolom VI Hubungan dengan Bank (baku PDF #220): 11 Terkait Dalam Rangka Kesejahteraan, 12 Terkait Lainnya, 20 Tidak Terkait.';

COMMENT ON COLUMN hapus_buku_register.tanggal_hapus_buku IS
    'Form 15.00 kolom VII Tanggal Hapus Buku (TT-BB-TTTT pada form). Wajib.';

COMMENT ON COLUMN hapus_buku_register.agunan_jenis_code IS
    'Form 15.00 kolom X.a Jenis Agunan (Lampiran 01). Tanpa agunan diserahkan = 299 (PDF #222).';

COMMENT ON COLUMN hapus_buku_register.agunan_nilai IS
    'Form 15.00 kolom X.c Nilai Agunan: nominal nilai pasar hasil penilaian terakhir. Tanpa agunan = 0 (PDF #222).';

CREATE INDEX IF NOT EXISTS idx_hapus_buku_jenis_aset
    ON hapus_buku_register (jenis_aset_code);
CREATE INDEX IF NOT EXISTS idx_hapus_buku_tanggal
    ON hapus_buku_register (tanggal_hapus_buku);
