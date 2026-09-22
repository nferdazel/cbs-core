-- 000077_eod_orchestration.up.sql
-- Run after: 000075_branding_app_info.up.sql (nomor 000076 dipesan gelombang lain;
-- scripts/migrate.sh memproses urutan glob sehingga gap tidak masalah).
--
-- ALASAN: urutan langkah tutup hari (EOD) sebelumnya di-hardcode di
-- apps/api/internal/service/batch_process_service.go. Pemilik sistem MEMUTUSKAN urutan
-- itu dikelola di database (docs/BACKLOG.md item 7), bukan di kode, supaya bank dapat
-- menyesuaikannya tanpa rilis. Karena salah konfigurasi urutan bisa merusak akuntansi,
-- tabel ini disertai pengaman berlapis:
--   - langkah INTI (core = TRUE) TIDAK BOLEH dinonaktifkan; ditegakkan CHECK (NOT core
--     OR enabled) di database DAN divalidasi ulang di service;
--   - urutan wajib unik dan > 0 (index unik + CHECK);
--   - prasyarat divalidasi di service: tidak menunjuk langkah tak dikenal, tidak
--     bersiklus, dan langkah aktif tidak boleh bergantung pada langkah nonaktif;
--   - perubahan definisi hanya lewat izin system:config dan WAJIB teraudit.
--
-- NILAI AWAL WAJIB mereproduksi urutan & saklar yang berlaku sekarang, sehingga
-- perilaku produksi tidak berubah setelah migrasi:
--   aro -> loan_interest_accrual -> restructure_loss_amortization -> ppap ->
--   ckpn_comparison -> loan_penalty_accrual -> dormant
-- dengan prasyarat yang sama seperti kode lama. Saklar langkah saat ini semuanya AKTIF
-- (ckpn.enabled, ckpn.shadow_mode.enabled, dan loan.restructure.loss.enabled tetap
-- menjadi penentu di dalam layanan masing-masing, bukan kolom enabled di sini).
--
-- eod_step_runs menggantikan ringkasan agregat akhir yang selama ini hanya ada di
-- respons: hasil, status, waktu, dan angka penting TIAP langkah per tanggal bisnis
-- tersimpan sehingga dapat ditelusuri besok.
--
-- eod_triggers menyediakan definisi pemicu manual (yang dipakai sekarang) dan
-- terjadwal. Penjadwal (daemon/cron) BELUM ada; baris terjadwal sengaja nonaktif dan
-- hanya menjadi definisi + titik sambung yang jelas (lihat laporan W6).
--
-- Aditif & idempotent: IF NOT EXISTS, ON CONFLICT DO NOTHING. Nilai yang sudah
-- disesuaikan operator TIDAK ditimpa. Dijalankan scripts/migrate.sh dalam SATU
-- transaksi per berkas (-1), jadi tidak ada BEGIN/COMMIT eksplisit di sini.

-- 1. Definisi langkah EOD.
CREATE TABLE IF NOT EXISTS eod_step_definitions (
    code          TEXT PRIMARY KEY,
    sequence      INTEGER NOT NULL CHECK (sequence > 0),
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    core          BOOLEAN NOT NULL DEFAULT FALSE,
    category      TEXT NOT NULL DEFAULT 'OPERASIONAL',
    prerequisites JSONB NOT NULL DEFAULT '[]'::jsonb,
    description   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by    UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    -- Langkah inti tidak boleh dinonaktifkan: salah satu pengaman terpenting.
    CONSTRAINT chk_eod_step_core_enabled CHECK (NOT core OR enabled),
    -- Prasyarat harus berupa larik JSON (daftar kode), bukan skalar/objek.
    CONSTRAINT chk_eod_step_prerequisites_array CHECK (jsonb_typeof(prerequisites) = 'array'),
    -- Urutan wajib unik. DEFERRABLE supaya penyimpanan urutan baru (mis. tukar posisi)
    -- tidak melanggar di tengah transaksi; pemeriksaan terjadi saat COMMIT.
    CONSTRAINT uq_eod_step_definitions_sequence UNIQUE (sequence) DEFERRABLE INITIALLY IMMEDIATE
);

COMMENT ON TABLE eod_step_definitions IS
    'Definisi & urutan langkah tutup hari (EOD) yang dikelola bank. Sumber kebenaran urutan; kode hanya mengeksekusi. Diubah lewat izin system:config dan teraudit.';
COMMENT ON COLUMN eod_step_definitions.core IS
    'TRUE = langkah inti akuntansi: tidak boleh dihapus maupun dinonaktifkan. Ditegakkan CHECK database + validasi service.';
COMMENT ON COLUMN eod_step_definitions.prerequisites IS
    'Kode langkah yang harus berstatus RAN pada run yang sama sebelum langkah ini dijalankan. Divalidasi: harus ada, tidak bersiklus, dan (untuk langkah aktif) harus aktif.';

-- 2. Riwayat hasil per langkah per tanggal bisnis.
CREATE TABLE IF NOT EXISTS eod_step_runs (
    id            BIGSERIAL PRIMARY KEY,
    run_id        UUID NOT NULL,
    business_date DATE NOT NULL,
    step_code     TEXT NOT NULL,
    sequence      INTEGER NOT NULL,
    status        TEXT NOT NULL CHECK (status IN ('RAN', 'FAILED', 'SKIPPED')),
    trigger       TEXT NOT NULL DEFAULT 'MANUAL' CHECK (trigger IN ('MANUAL', 'SCHEDULED')),
    reason        TEXT NOT NULL DEFAULT '',
    summary       JSONB NOT NULL DEFAULT '{}'::jsonb,
    executed_by   UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE eod_step_runs IS
    'Riwayat hasil TIAP langkah EOD per tanggal bisnis (status, waktu, angka penting). Menggantikan ringkasan agregat akhir yang tidak dapat ditelusuri per langkah.';

CREATE INDEX IF NOT EXISTS idx_eod_step_runs_date
    ON eod_step_runs (business_date, sequence);
CREATE INDEX IF NOT EXISTS idx_eod_step_runs_run
    ON eod_step_runs (run_id);

-- 3. Definisi pemicu EOD (manual sekarang, terjadwal menyusul).
CREATE TABLE IF NOT EXISTS eod_triggers (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL UNIQUE,
    trigger_type  TEXT NOT NULL CHECK (trigger_type IN ('MANUAL', 'SCHEDULED')),
    schedule_cron TEXT NOT NULL DEFAULT '',
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    description   TEXT NOT NULL DEFAULT '',
    last_run_at   TIMESTAMPTZ,
    next_run_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE eod_triggers IS
    'Definisi pemicu EOD. MANUAL = tombol POST /batch/eod (dipakai sekarang). SCHEDULED = pemicu terjadwal; penjadwal (daemon) belum ada sehingga barisnya nonaktif dan next_run_at diisi operator/penjadwal saat itu tersedia.';

-- 4. Seed definisi: WAJIB sama dengan urutan & prasyarat kode lama.
INSERT INTO eod_step_definitions (code, sequence, enabled, core, category, prerequisites, description) VALUES
    ('aro', 10, TRUE, FALSE, 'OPERASIONAL', '[]',
     'Perpanjangan otomatis (ARO) deposito yang jatuh tempo.'),
    ('loan_interest_accrual', 20, TRUE, TRUE, 'AKUNTANSI', '[]',
     'Akrual pendapatan bunga kredit konvensional berbasis jadwal angsuran. INTI: menentukan pendapatan periode.'),
    ('restructure_loss_amortization', 30, TRUE, TRUE, 'AKUNTANSI', '["loan_interest_accrual"]',
     'Amortisasi saldo kerugian restrukturisasi (suku bunga efektif). INTI; berjalan setelah akrual agar berdiri di atas pendapatan yang sudah diakui.'),
    ('ppap', 40, TRUE, TRUE, 'AKUNTANSI', '["restructure_loss_amortization"]',
     'Perhitungan kolektibilitas & cadangan PPAP harian. INTI: dasar angka CKPN dan penyisihan.'),
    ('ckpn_comparison', 50, TRUE, TRUE, 'PELAPORAN', '["ppap"]',
     'Perbandingan CKPN vs required_ppap. INTI (SAK EP); baca-saja, hanya boleh berjalan bila PPAP RAN pada tanggal bisnis yang sama.'),
    ('loan_penalty_accrual', 60, TRUE, FALSE, 'AKUNTANSI', '[]',
     'Akrual denda kredit.'),
    ('dormant', 70, TRUE, FALSE, 'OPERASIONAL', '[]',
     'Penandaan rekening nasabah yang tidak ada aktivitas sebagai DORMANT.')
ON CONFLICT (code) DO NOTHING;

-- 5. Seed pemicu. Manual aktif (perilaku sekarang); terjadwal nonaktif sampai
-- penjadwal benar-benar dipasang agar tidak ada tutup hari tak terduga.
INSERT INTO eod_triggers (name, trigger_type, schedule_cron, enabled, description) VALUES
    ('manual', 'MANUAL', '', TRUE,
     'Pemicu manual lewat POST /api/v1/batch/eod. Pemicu yang berlaku sekarang.'),
    ('scheduled-daily', 'SCHEDULED', '0 23 * * *', FALSE,
     'Pemicu terjadwal harian pukul 23:00. NONAKTIF: penjadwal (daemon/cron yang memanggil jalur terjadwal) belum dibangun; aktifkan hanya setelah penjadwal tersedia dan jamnya disetujui bank.')
ON CONFLICT (name) DO NOTHING;
