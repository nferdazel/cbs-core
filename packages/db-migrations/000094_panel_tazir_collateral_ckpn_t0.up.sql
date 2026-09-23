-- 000094_panel_tazir_collateral_ckpn_t0.up.sql
-- Run after: 000092_recovery_threshold_zero.up.sql
-- (000093 dipakai alur paralel; migrasi ini tidak bergantung isinya.)
--
-- Keputusan panel risiko (23 Sep 2026), butir 2 (ta'zir/dana kebajikan), butir 3
-- (gerbang aktivasi bobot agunan), dan TAHAP T0 butir 4 (kerangka CKPN individual).
-- Nol perubahan perilaku: seluruh saklar tetap mati, bobot agunan tetap 100%,
-- required_ckpn tidak disentuh.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. Tidak ada nilai bank yang ditimpa
-- (INSERT ... ON CONFLICT (key) DO NOTHING), dan kolom penanda aktivasi hanya
-- ditambahkan bila belum ada.

-- ---------------------------------------------------------------------------
-- A. Ta'zir / dana kebajikan (keputusan panel butir 2)
-- ---------------------------------------------------------------------------
-- Panel: sistem MENOLAK mengakru ta'zir bila klausul akad belum dinyatakan
-- tercantum di akad. Bawaan `false` = ta'zir tidak diakru; penetapan `true` hanya
-- oleh pejabat berizin (kunci konfigurasi yang sama dipakai izin system:config),
-- dengan bukti nomor akad yang sudah tercatat per kredit (loans.akad_number).
INSERT INTO system_config (key, value, description) VALUES
    ('loan.penalty.syariah.akad_disclosed', 'false',
     'Penanda bahwa klausul ta''zir telah dicantumkan pada akad pembiayaan syariah (POJK 24/2024 kewajiban pengungkapan sebelum akad; Fatwa DSN-MUI 17/2000). Bawaan false: sistem TIDAK mengakru ta''zir sampai pejabat berizin menyetelnya true. Selain kunci ini, kredit syariah juga harus memiliki nomor akad (loans.akad_number) sebagai bukti pencatatan akad.')
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- C. CKPN individual — TAHAP T0 SAJA (keputusan panel butir 4)
-- ---------------------------------------------------------------------------
-- Kerangka konfigurasi + kolom/tabel T0. TIDAK ada harga yang dihitung, TIDAK ada
-- perilaku yang berubah. `ckpn.individual.enabled=false` pada rilis pertama.
-- Ambang kuantitatif (Rp1.000.000.000 dan/atau 20 debitur terbesar) adalah
-- PRAKTIK INDUSTRI, bukan aturan tertulis: PA BPR butir 12.4.e.1.(1) menyerahkan
-- tingkat signifikansi ke bank. Pemicu non-nominal diturunkan dari bukti objektif
-- PA BPR 12.2.b/12.3.c dan kriteria aset baik 12.3.a.1.c.
INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.individual.enabled', 'false',
     'Saklar utama CKPN individual. Bawaan false = perilaku sekarang (hanya CKPN kolektif); tidak mengubah required_ckpn. Menyalakannya adalah langkah bank, bukan bawaan rilis.'),
    ('ckpn.individual.significance_amount', '1000000000',
     'Ambang signifikansi individual (Rp per debitur). KEPUTUSAN PANEL sebagai praktik industri, BUKAN aturan tertulis (PA BPR 12.4.e.1.(1) menyerahkan angka ke bank). Dipakai bersama ckpn.individual.significance_top_n.'),
    ('ckpn.individual.significance_top_n', '20',
     'Jumlah debitur eksposur terbesar yang wajib dinilai individual, di samping ambang nominal. Praktik industri (PA BPR 12.4.e.1.(1)); bank boleh menyesuaikan.'),
    ('ckpn.individual.method', 'MAX',
     'Metode target individual: DCF (nilai kini arus kas), COLLATERAL (nilai realisasi agunan bersih), atau MAX (yang lebih konservatif). Bawaan MAX memenuhi aturan pemilihan PA BPR 12.4.g.1.c (CKPN agunan minimal sama dengan CKPN sebelumnya).'),
    ('ckpn.individual.discount_rate_annual_pct', '',
     'Override tingkat diskonto individual (% per tahun) HANYA bila EIR orisinal kredit belum tersimpan. Kosong = wajib EIR orisinal; kredit tanpa EIR GAGAL dengan ErrEIRMissing, bukan dihitung nol dan bukan memakai suku bunga kontraktual (PA BPR 12.4.g.1.a).'),
    ('ckpn.individual.mandatory_on_macet', 'true',
     'Pemicu individual wajib tanpa memandang nominal: kolektibilitas Macet (golongan 5). Diturunkan dari bukti objektif PA BPR 12.2.b dan kriteria aset baik 12.3.a.1.c.'),
    ('ckpn.individual.mandatory_on_restructured', 'true',
     'Pemicu individual wajib tanpa memandang nominal: kredit pernah direstrukturisasi (konsesi = bukti objektif, PA BPR 12.2.b butir 3; sekaligus gugur kriteria aset baik 12.3.a.1.c).'),
    ('ckpn.individual.mandatory_dpd_days', '90',
     'Ambang hari tunggakan (DPD) yang memicu penilaian individual wajib, di luar golongan Macet. Bawaan 90 hari; bank boleh menyesuaikan.'),
    ('ckpn.individual.mandatory_on_collateral_drop', 'true',
     'Pemicu individual wajib: penurunan nilai agunan signifikan. Diturunkan dari bukti objektif PA BPR 12.2.b; besar penurunan ditentukan bank.'),
    ('ckpn.individual.mandatory_on_objective_evidence', 'true',
     'Pemicu individual wajib: ada bukti objektif lain yang ditandai pengelola kredit (PA BPR 12.2.b/12.3.c), mis. kesulitan keuangan penerbit/obligor atau pelanggaran kontrak.')
ON CONFLICT (key) DO NOTHING;

-- Kolom/tabel T0 dari docs/CKPN-INDIVIDUAL-RANCANGAN.md §6.1. Semua aditif; nilai
-- bawaan aman sehingga baris lama tetap 'COLLECTIVE' dan saklar tetap mati.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS ckpn_method TEXT NOT NULL DEFAULT 'COLLECTIVE',
    ADD COLUMN IF NOT EXISTS ckpn_significant BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ckpn_objective_evidence BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ckpn_individual_target NUMERIC(28,4);

COMMENT ON COLUMN loans.ckpn_method IS
    'Metode CKPN per kredit (T0): COLLECTIVE (bawaan, perilaku sekarang), INDIVIDUAL_DCF, INDIVIDUAL_COLLATERAL, INDIVIDUAL_MAX, EXCLUDED_ASET_BAIK. Belum ada perhitungan yang menulis kolom ini.';
COMMENT ON COLUMN loans.ckpn_significant IS
    'Hasil Langkah Kedua penilaian signifikansi (T0). Bawaan FALSE; belum ada perhitungan.';
COMMENT ON COLUMN loans.ckpn_objective_evidence IS
    'Hasil Langkah Ketiga bukti objektif penurunan nilai (T0). Bawaan FALSE; belum ada perhitungan.';
COMMENT ON COLUMN loans.ckpn_individual_target IS
    'Target CKPN individual terakhir (jejak T0). Angka resmi tetap loans.required_ckpn; kolom ini belum diisi kode apa pun.';

CREATE TABLE IF NOT EXISTS loan_ckpn_individual_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id UUID NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
    as_of DATE NOT NULL,
    method TEXT NOT NULL,
    carrying_amount NUMERIC(28,4) NOT NULL DEFAULT 0,
    present_value NUMERIC(28,4) NOT NULL DEFAULT 0,
    collateral_nrv NUMERIC(28,4) NOT NULL DEFAULT 0,
    target NUMERIC(28,4) NOT NULL DEFAULT 0,
    basis JSONB,
    decided_by VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE loan_ckpn_individual_assessments IS
    'Jejak audit penilaian CKPN individual (T0). Belum ada kode yang menulis tabel ini.';

CREATE TABLE IF NOT EXISTS loan_cashflow_projections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id UUID NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
    as_of DATE NOT NULL,
    period INT NOT NULL,
    amount NUMERIC(28,4) NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'MANUAL',
    created_by VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE loan_cashflow_projections IS
    'Proyeksi arus kas per debitur (data operasional bank, T0). Belum ada kode yang menulis tabel ini.';

ALTER TABLE loan_collaterals
    ADD COLUMN IF NOT EXISTS selling_cost_amount NUMERIC(18,2) NOT NULL DEFAULT 0;
COMMENT ON COLUMN loan_collaterals.selling_cost_amount IS
    'Biaya pelepasan agunan untuk nilai realisasi bersih (net proceed) CKPN individual (T0). Bawaan 0; belum dipakai perhitungan.';

-- ---------------------------------------------------------------------------
-- B. Gerbang aktivasi bobot agunan (keputusan panel butir 3)
-- ---------------------------------------------------------------------------
-- Penanda persetujuan Direksi (C8), waktu mulai mode bayangan (minimal 2 bulan
-- bisnis), dan jejak maker/checker (C9). Semua kolom NULL pada fase persiapan;
-- bobot tetap enabled=FALSE dan applied_weight_frac=1.00000.
ALTER TABLE collateral_lampiran_ii_weights
    ADD COLUMN IF NOT EXISTS direksi_policy_number VARCHAR(128),
    ADD COLUMN IF NOT EXISTS direksi_policy_date DATE,
    ADD COLUMN IF NOT EXISTS shadow_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS activated_maker VARCHAR(128),
    ADD COLUMN IF NOT EXISTS activated_by VARCHAR(128),
    ADD COLUMN IF NOT EXISTS activated_at TIMESTAMPTZ;

COMMENT ON COLUMN collateral_lampiran_ii_weights.direksi_policy_number IS
    'Nomor surat kebijakan Direksi yang menyetujui bobot kategori (syarat C8). NULL = belum ada; aktivasi ditolak.';
COMMENT ON COLUMN collateral_lampiran_ii_weights.direksi_policy_date IS
    'Tanggal kebijakan Direksi (syarat C8). NULL = belum ada; aktivasi ditolak.';
COMMENT ON COLUMN collateral_lampiran_ii_weights.shadow_started_at IS
    'Awal mode bayangan kategori. Aktivasi baru boleh setelah minimal 2 bulan bisnis penuh (keputusan panel butir 3.2).';
COMMENT ON COLUMN collateral_lampiran_ii_weights.activated_maker IS
    'Pembuat (maker) usulan aktivasi kategori; wajib berbeda dari penyetuju (C9).';
COMMENT ON COLUMN collateral_lampiran_ii_weights.activated_by IS
    'Penyetujui (checker) aktivasi kategori. Wajib berbeda dari pembuat (maker); tercatat untuk audit (C9).';
COMMENT ON COLUMN collateral_lampiran_ii_weights.activated_at IS
    'Waktu aktivasi kategori. NULL = belum pernah diaktifkan.';

INSERT INTO system_config (key, value, description) VALUES
    ('collateral.weight.activation.shadow_months', '2',
     'Lama minimum mode bayangan bobot agunan (bulan bisnis) sebelum sebuah kategori boleh diaktifkan (keputusan panel butir 3.2). Selama mode bayangan, ATMR dihitung dua kali (100% dan kandidat) dan selisihnya dilaporkan.'),
    ('collateral.weight.activation.coverage_min_frac', '0.90',
     'Cakupan minimum nilai agunan aktif yang harus lolos C1-C4 sebelum sebuah kategori bobot agunan boleh diaktifkan (fraksi 0..1; 0.90 = 90%). Pagar anti-cherry-picking keputusan panel butir 3.3.')
ON CONFLICT (key) DO NOTHING;
