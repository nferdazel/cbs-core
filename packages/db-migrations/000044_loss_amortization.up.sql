-- CBS Migration 000044: amortisasi saldo kerugian restrukturisasi kredit (metode EIR)
-- Run after: 000043_restructure_loss.up.sql (nomor dipesan; tidak bergantung isinya).
-- Aditif dan idempotent: ADD COLUMN IF NOT EXISTS, CREATE INDEX IF NOT EXISTS,
-- ON CONFLICT DO NOTHING. Aman dijalankan ulang.
--
-- Dasar dokumen (dibaca dari berkas resmi, bukan ingatan):
--   - POJK No. 1 Tahun 2024 Pasal 32: BPR wajib menerapkan perlakuan akuntansi
--     Restrukturisasi Kredit sesuai standar akuntansi keuangan dan pedoman akuntansi BPR.
--   - SEOJK No. 21/SEOJK.03/2024 (Panduan Akuntansi Perbankan BPR, PA BPR) Bab 5.2
--     hlm. 61 (ilustrasi jurnal): saldo kerugian restrukturisasi dipulihkan perlahan
--     melalui pengakuan pendapatan bunga pada suku bunga efektif orisinal, dengan
--     jurnal Db. Kredit yang diberikan / Kr. Pendapatan bunga.
--
-- ALASAN migrasi ini: jurnal kerugian restrukturisasi (migrasi 000043) mengkredit akun
-- piutang kredit, sedangkan jurnal pelunasan mengkredit pokok penuh. Tanpa amortisasi,
-- saat kredit lunas/dihapusbukukan saldo akun piutang menjadi NEGATIF sebesar kerugian
-- dan beban kerugian tidak pernah kembali ke laba. Kolom di bawah menyimpan state
-- per-angsuran agar batch amortisasi idempoten dan dapat dihitung ulang dari state
-- tersimpan, bukan dari asumsi urutan pemanggilan.

-- 1. State per-angsuran amortisasi saldo kerugian.
--    restructure_loss_amortized_at     : penanda "angsuran ini sudah diamortisasi".
--    restructure_loss_amortized_amount : nominal kumulatif yang diamortisasi (audit).
--    Bawaan NULL/0 = belum diamortisasi. Kredit lama tidak terpengaruh karena saldo
--    kerugiannya juga nol.
ALTER TABLE loan_schedules
    ADD COLUMN IF NOT EXISTS restructure_loss_amortized_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS restructure_loss_amortized_amount NUMERIC(28,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN loan_schedules.restructure_loss_amortized_at IS
    'Penanda angsuran ini saldo kerugian restrukturisasinya sudah diamortisasi ke pendapatan bunga (PA BPR Bab 5.2 hlm. 61).';
COMMENT ON COLUMN loan_schedules.restructure_loss_amortized_amount IS
    'Nominal kumulatif amortisasi saldo kerugian restrukturisasi pada angsuran ini, untuk audit.';

-- 2. Index parsial melayani pencarian kandidat batch: kredit aktif yang punya angsuran
--    jatuh tempo dan belum diamortisasi. Predikat parsial menjaga index tetap kecil.
CREATE INDEX IF NOT EXISTS idx_loan_schedules_loss_amortization
    ON loan_schedules(due_date)
    WHERE restructure_loss_amortized_at IS NULL;

-- 3. Konfigurasi sisi KREDIT jurnal amortisasi (pendapatan bunga). Saklar hidup/mati
--    tetap memakai loan.restructure.loss.enabled yang di-seed migrasi 000043 (bawaan
--    false). Kosong = fallback per buku: 40100 konvensional, 14100 syariah.
INSERT INTO system_config (key, value, description) VALUES
    ('loan.restructure.loss.amortization.coa.income', '',
     'Kode COA pendapatan bunga sisi kredit jurnal amortisasi saldo kerugian restrukturisasi. Kosong = fallback per buku: 40100 konvensional, 14100 syariah.')
ON CONFLICT (key) DO NOTHING;
