-- CBS Migration 000014: Dukungan PPAP harian & stop accrual
-- Run after: 000013 (dipakai pekerjaan lain)
--
-- Menambah kolom state yang dibutuhkan proses PPAP dan nilai konfigurasi tarif PPAP
-- serta ambang DPD. Idempotent: aman dijalankan ulang.
--
-- Catatan: kolom required_ppap sudah ada sejak 000004. Kolom last_accrual_date tidak
-- ditambahkan karena proses PPAP tidak membutuhkannya; yang perlu hanya penanda
-- stop_accrual. Kode COA PPAP sengaja TIDAK di-seed di sini: service memakai fallback
-- per buku produk (50200/15200 beban, 10900/11900 cadangan) bila key konfigurasi kosong.

-- 1. Penanda penghentian akrual bunga/margin untuk kredit tidak lancar (NPL).
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS stop_accrual BOOLEAN NOT NULL DEFAULT FALSE;

-- 2. Indeks pemindaian kredit aktif untuk proses PPAP harian.
CREATE INDEX IF NOT EXISTS idx_loans_ppap_active
    ON loans(status, outstanding_principal)
    WHERE outstanding_principal > 0;

-- 3. Nilai awal konfigurasi. Tarif dalam fraksi (0.005 = 0,5%).
INSERT INTO system_config (key, value, description) VALUES
    ('ppap.dpd.dpk',           '30',    'Batas atas DPD (hari) kolektibilitas 2 - Dalam Perhatian Khusus'),
    ('ppap.dpd.kurang_lancar', '90',    'Batas atas DPD (hari) kolektibilitas 3 - Kurang Lancar'),
    ('ppap.dpd.diragukan',     '180',   'Batas atas DPD (hari) kolektibilitas 4 - Diragukan'),
    ('ppap.rate.1',            '0.005', 'Tarif PPAP minimum kolektibilitas 1 - Lancar'),
    ('ppap.rate.2',            '0.1',   'Tarif PPAP minimum kolektibilitas 2 - Dalam Perhatian Khusus'),
    ('ppap.rate.3',            '0.15',  'Tarif PPAP minimum kolektibilitas 3 - Kurang Lancar'),
    ('ppap.rate.4',            '0.5',   'Tarif PPAP minimum kolektibilitas 4 - Diragukan'),
    ('ppap.rate.5',            '1.0',   'Tarif PPAP minimum kolektibilitas 5 - Macet')
ON CONFLICT (key) DO NOTHING;
