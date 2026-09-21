-- CBS Migration 000045: Pasal 23 POJK No. 1 Tahun 2024 — pengurang PPKA umum dan khusus
-- untuk bagian Penempatan pada Bank Lain (PABL) yang dijamin Lembaga Penjamin Simpanan.
-- Run after: 000044 (nomor dipesan; migrasi ini tidak bergantung isinya selain urutan).
-- Idempotent: CREATE TABLE/INDEX IF NOT EXISTS, ON CONFLICT DO NOTHING.
--
-- Dasar pasal:
--   Pasal 23: "Bagian Penempatan pada Bank Lain yang memenuhi persyaratan kriteria
--   penjaminan Lembaga Penjamin Simpanan dapat dijadikan sebagai faktor pengurang dalam
--   pembentukan perhitungan PPKA umum dan khusus."
--   Penjelasan Pasal 23 memberi contoh: penempatan Rp10.000.000.000 pada satu bank
--   dijamin LPS paling tinggi Rp2.000.000.000 (per nasabah pada satu bank), sehingga
--   PPKA = tarif x (Rp10.000.000.000 - Rp2.000.000.000).
--   Pasal 1 angka 4: PABL berbentuk giro, tabungan, deposito, sertifikat deposito,
--   kredit yang diberikan, dan penempatan dana lain yang sejenis.
--   Pasal 15 ayat (1): kualitas PABL hanya Lancar, Kurang Lancar, atau Macet.
--
-- Mengapa tabel baru: sistem ini TIDAK memodelkan PABL sebagai instrumen tersendiri;
-- yang ada hanya COA 10200 (konvensional) dan 11200 (syariah) serta label laporan OJK.
-- Nilai yang dijamin LPS tidak boleh disimpulkan dari nama akun dengan pencocokan teks,
-- sehingga penandanya disimpan eksplisit di sini. Bank mengisi barisnya; modul menghitung
-- pengurangnya. Saklar ppap.lps.enabled tetap 'false' supaya tidak ada perubahan perilaku
-- apa pun sampai bank memutuskan bagaimana penempatannya dicatat dan dijamin.
--
-- TIDAK ada nilai posting_event baru di migrasi ini: Pasal 23 hanya mengurangi DASAR
-- pengenaan tarif, bukan jenis jurnalnya. Pembentukan/pemulihan tetap memakai
-- PPAP_PROVISION/PPAP_REVERSAL yang sudah ada, bila kelak bank memutuskan mempostingnya.

CREATE TABLE IF NOT EXISTS lps_placements (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- coa_code menautkan penempatan ke akun tempatnya tercatat pada bagan baku, sehingga
    -- rekonsiliasi dengan GL mungkin dilakukan. Bukan pencocokan nama akun.
    coa_code          VARCHAR(16) NOT NULL REFERENCES chart_of_accounts(code),
    counterparty_bank VARCHAR(150) NOT NULL,
    placement_type    VARCHAR(24) NOT NULL
        CHECK (placement_type IN ('GIRO', 'TABUNGAN', 'DEPOSITO', 'SERTIFIKAT_DEPOSITO', 'KREDIT', 'LAINNYA')),
    outstanding       NUMERIC(28,4) NOT NULL DEFAULT 0 CHECK (outstanding >= 0),
    -- Bagian penempatan yang memenuhi kriteria penjaminan LPS (Pasal 23). Tidak boleh
    -- melebihi nilai penempatan; plafon per bank lawan ditegakkan service memakai
    -- konfigurasi ppap.lps.guarantee_cap.
    lps_guaranteed    NUMERIC(28,4) NOT NULL DEFAULT 0
        CHECK (lps_guaranteed >= 0 AND lps_guaranteed <= outstanding),
    collectibility    VARCHAR(16) NOT NULL DEFAULT 'LANCAR'
        CHECK (collectibility IN ('LANCAR', 'KURANG_LANCAR', 'MACET')),
    as_of             DATE NOT NULL,
    branch_id         UUID REFERENCES branches(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lps_placements_as_of ON lps_placements (as_of);
CREATE INDEX IF NOT EXISTS idx_lps_placements_branch ON lps_placements (branch_id);
CREATE INDEX IF NOT EXISTS idx_lps_placements_counterparty ON lps_placements (counterparty_bank);

COMMENT ON TABLE lps_placements IS
    'Penempatan pada Bank Lain (Pasal 1 angka 4 POJK 1/2024) beserta bagian yang dijamin LPS; dasar pengurang PPKA umum dan khusus Pasal 23. Bank mengisi barisnya.';
COMMENT ON COLUMN lps_placements.coa_code IS
    'Kode COA tempat penempatan dicatat (mis. 10200 konvensional, 11200 syariah). Penautan eksplisit, bukan tebakan dari nama akun.';
COMMENT ON COLUMN lps_placements.lps_guaranteed IS
    'Bagian penempatan yang memenuhi kriteria penjaminan LPS (Pasal 23 POJK 1/2024). Tidak boleh melebihi outstanding; jumlah per bank lawan dibatasi plafon LPS.';
COMMENT ON COLUMN lps_placements.collectibility IS
    'Kualitas PABL menurut Pasal 15 ayat (1) POJK 1/2024: LANCAR, KURANG_LANCAR, atau MACET.';

-- Konfigurasi. Saklar bawaan 'false' agar modul tidak berjalan sampai bank mengisi data.
-- ppap.lps.guarantee_cap boleh diubah bila plafon penjaminan LPS berubah; nilai yang salah
-- ditolak service, bukan dijatuhkan menjadi nol.
INSERT INTO system_config (key, value, description) VALUES
    ('ppap.lps.enabled', 'false',
     'Aktifkan pengurang PPKA Pasal 23 POJK 1/2024 untuk penempatan pada bank lain yang dijamin LPS. false = tidak ada query tambahan dan tidak ada perubahan angka PPKA.'),
    ('ppap.lps.guarantee_cap', '2000000000',
     'Plafon penjaminan LPS per nasabah pada satu bank (rupiah penuh, Penjelasan Pasal 23 POJK 1/2024). Kosong = bawaan Rp2.000.000.000; nilai salah format atau tidak positif ditolak.')
ON CONFLICT (key) DO NOTHING;
