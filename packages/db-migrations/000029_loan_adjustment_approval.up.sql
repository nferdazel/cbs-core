-- CBS Migration 000029: ambang persetujuan hapus buku dan recovery kredit
-- Run after: 000028_restructure_collectibility.up.sql
-- Idempotent: aman dijalankan ulang.
--
-- Hapus buku melepas tagihan kredit dari neraca dan tidak dapat dibatalkan begitu
-- jurnalnya terposting; recovery adalah penyesuaian kas atas kredit yang sudah
-- dihapus buku. Keduanya wajib disetujui pejabat kedua, jadi ambangnya disetel 0:
-- menurut maker_checker.Threshold, nilai 0 berarti setiap nominal wajib disetujui.
--
-- Nilai dibiarkan di system_config, bukan di kode, supaya bank dapat melonggarkan
-- aturannya sendiri (mis. recovery kecil oleh teller) tanpa menunggu rilis baru:
-- isi ambang positif untuk mengizinkan nominal di bawahnya berjalan langsung.
INSERT INTO system_config (key, value, description)
VALUES (
    'maker_checker.loan_write_off.threshold',
    '0',
    'Ambang nominal hapus buku kredit (Rp) yang wajib disetujui pejabat kedua. 0 = setiap hapus buku wajib disetujui.'
)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_config (key, value, description)
VALUES (
    'maker_checker.loan_recovery.threshold',
    '0',
    'Ambang nominal recovery kredit hapus buku (Rp) yang wajib disetujui pejabat kedua. 0 = setiap recovery wajib disetujui.'
)
ON CONFLICT (key) DO NOTHING;
