-- CBS Migration 000113: register rekening administratif (pos komitmen/kontinjensi
-- off-balance) untuk Form 01.01 "Rekening Administratif".
-- Run after: 000112_ojk_kelembagaan.up.sql (urutan; tidak bergantung isinya). Nomor
-- 000113 dipesan khusus pekerjaan ini.
--
-- Menutup satu laporan K2 (docs/CELAH-LAPORAN-OJK.md §3): Form 01.01 sebelumnya
-- ditandai belum buildable karena "pos komitmen/kontinjensi belum ada di bagan akun".
-- Table office akun TIDAK ditambah: rekening administratif bukan saldo buku besar,
-- melainkan register pos off-balance yang bank catat. Satu tabel ADITIF di bawah
-- menjadi sumbernya; tidak ada tabel/kolom yang diubah.
--
-- Sumber struktur pos (verifikasi ke PDF resmi SEOJK No. 16/SEOJK.03/2024, 528 hlm.;
-- salinan di luar repo, tidak di-commit):
--   * Form 01.01 - 1 "REKENING ADMINISTRATIF": PDF #page 110 (hlm. tercetak 58).
--     Kolom: Sandi Kantor, Nama Rekening, Sandi, Jumlah. Posisi resmi:
--       Tagihan Komitmen      : 6101010000 Fasilitas Pinjaman yang Diterima yang
--                               Belum Ditarik; 6101990000 Tagihan Komitmen Lainnya.
--       Kewajiban Komitmen    : 6102010000 Fasilitas Kredit kepada Nasabah yang
--                               Belum Ditarik; 6102020000 Penerusan Kredit;
--                               6102990000 Kewajiban Komitmen Lainnya.
--       Tagihan Kontinjensi   : 6201010100/6201010200/6201010300/6201010900
--                               Pendapatan Bunga Dalam Penyelesaian (kredit,
--                               penempatan bank lain, surat berharga, lainnya);
--                               6201020100/6201020200/6201020300/6201020400 Aset
--                               Produktif yang Dihapus Buku; 6201030000 Agunan
--                               dalam Proses Penyelesaian Kredit; 6201990000
--                               Tagihan Kontinjensi Lainnya.
--       Kewajiban Kontinjensi : 6202000000.
--       Rekening Administratif Lainnya: 6900000000.
--     Daftar sandi pos yang sama diulang pada penjelasan Lampiran "D. LAPORAN
--     KOMITMEN DAN KONTINJENSI" PDF #page 488.
--
-- Sandi pos di atas adalah sandi resmi dari PDF, bukan karangan. Meski begitu kolom
-- position_code pada register DIISI BANK dan boleh kosong: bank yang belum tahu
-- sandinya tetap dapat mencatat posnya lewat `description`; laporan menulis "-" bila
-- sandi kosong.
--
-- Semua NILAI (sandi pos, uraian, nominal, lawan, referensi, tanggal) adalah DATA
-- BANK. Tidak ada seed angka, dan tidak ada perhitungan/penurunan otomatis dari
-- jurnal: sistem memakai basis kas untuk kredit NPL dan tidak mereklasifikasi bunga
-- terakru (docs/KEPUTUSAN-OJK.md §4), sehingga pos seperti "Pendapatan Bunga Dalam
-- Penyelesaian" hanya terisi bila bank mencatatnya di sini.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. CREATE TABLE/INDEX IF NOT EXISTS; tidak ada
-- seed data dan tidak mengubah objek yang sudah ada.

CREATE TABLE IF NOT EXISTS off_balance_items (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Sandi pos OJK Form 01.01 yang bank ketahui (mis. 6102010000). Boleh kosong bila
    -- bank belum mengetahuinya; laporan menulis "-". Tidak ada CHECK terhadap daftar
    -- sandi karena register harus menampung pos yang bank catat.
    position_code            VARCHAR(32) NOT NULL DEFAULT '',
    -- Kelompok pos. KOMITMEN/KONTINJENSI mengikuti pembagian resmi Form 01.01;
    -- LAINNYA menampung pos 6900000000 "Rekening Administratif Lainnya".
    category                 VARCHAR(16) NOT NULL
        CHECK (category IN ('KOMITMEN', 'KONTINJENSI', 'LAINNYA')),
    -- Uraian bebas bank (nama rekening/pos). Wajib terisi agar baris dapat dibaca.
    description              VARCHAR(255) NOT NULL CHECK (length(btrim(description)) > 0),
    -- Nominal dalam rupiah penuh (NUMERIC(28,4) mengikuti skala uang tabel lain).
    amount                   NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (amount >= 0),
    -- Nasabah lawan, bila pos terkait satu debitur. NULL = tidak terkait/tanpa nasabah.
    counterparty_customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    -- Nomor referensi dokumen/kontrak (bebas). Kosong = tidak ada.
    reference                VARCHAR(128) NOT NULL DEFAULT '',
    -- Tanggal posisi (diisi bank). Dipakai mencocokkan baris ke periode laporan.
    as_of                    DATE NOT NULL,
    -- AKTIF = pos masih berjalan dan ikut laporan; NONAKTIF = sudah selesai/tidak
    -- dilaporkan lagi tetapi tetap tersimpan sebagai riwayat.
    status                   VARCHAR(16) NOT NULL DEFAULT 'AKTIF'
        CHECK (status IN ('AKTIF', 'NONAKTIF')),
    note                     TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by               UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    updated_by               UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

COMMENT ON TABLE off_balance_items IS
    'Register rekening administratif (pos komitmen/kontinjensi off-balance) untuk Form 01.01 "Rekening Administratif"; sumber struktur pos PDF #page 110 (hlm. 58) SEOJK No. 16/SEOJK.03/2024. Semua nilai diisi bank; laporan hanya mengagregasi per pos/kategori, tidak menurunkan angka dari jurnal.';

COMMENT ON COLUMN off_balance_items.position_code IS
    'Sandi pos OJK Form 01.01 yang bank ketahui (mis. 6102010000 Fasilitas Kredit kepada Nasabah yang Belum Ditarik). Kosong = bank belum mengisi; laporan menulis "-", tidak dikarang dari uraian.';

COMMENT ON COLUMN off_balance_items.category IS
    'Kelompok pos baku: KOMITMEN, KONTINJENSI, atau LAINNYA (pos 6900000000 Rekening Administratif Lainnya). Menentukan pengelompokan Form 01.01.';

COMMENT ON COLUMN off_balance_items.amount IS
    'Nominal pos rekening administratif dalam rupiah penuh. Harus >= 0; baris bank-defined, tidak ada angka seed.';

COMMENT ON COLUMN off_balance_items.counterparty_customer_id IS
    'Nasabah lawan pos (customers.id) bila terkait satu debitur; NULL = tidak terkait/tanpa nasabah. ON DELETE SET NULL agar register tidak memblokir penghapusan nasabah.';

COMMENT ON COLUMN off_balance_items.as_of IS
    'Tanggal posisi pos rekening administratif (diisi bank). Laporan bulanan mengagregasi baris berstatus AKTIF yang as_of-nya pada bulan periode.';

COMMENT ON COLUMN off_balance_items.status IS
    'AKTIF = pos berjalan dan ikut laporan; NONAKTIF = pos selesai/dihentikan pelaporannya namun tetap tersimpan sebagai riwayat.';

CREATE INDEX IF NOT EXISTS idx_off_balance_items_category
    ON off_balance_items (category);
CREATE INDEX IF NOT EXISTS idx_off_balance_items_position_code
    ON off_balance_items (position_code);
CREATE INDEX IF NOT EXISTS idx_off_balance_items_as_of
    ON off_balance_items (as_of);
CREATE INDEX IF NOT EXISTS idx_off_balance_items_status
    ON off_balance_items (status);
CREATE INDEX IF NOT EXISTS idx_off_balance_items_counterparty
    ON off_balance_items (counterparty_customer_id);
