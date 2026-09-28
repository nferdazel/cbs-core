-- CBS Migration 000114: enam akun COA Aset Lainnya untuk Form 09.00 "Rincian Aset
-- Lainnya".
-- Run after: 000113_ojk_off_balance.up.sql (urutan; tidak bergantung isinya). Nomor
-- 000114 dipesan khusus pekerjaan ini.
--
-- Menutup satu laporan K2 (docs/CELAH-LAPORAN-OJK.md §3): Form 09.00 sebelumnya
-- ditandai belum buildable karena bagan akun belum punya akun untuk pos Premi
-- Penjaminan LPS Dibayar di Muka, Uang Muka Pajak, Aset Pajak Tangguhan, Biaya
-- Dibayar di Muka, Tagihan kepada Perusahaan Asuransi, dan Uang Muka untuk Kegiatan
-- Operasional. Enam akun daun ADITIF di bawah menjadi sumbernya; tidak ada tabel/kolom
-- yang diubah.
--
-- Sumber struktur pos (verifikasi ke PDF resmi SEOJK No. 16/SEOJK.03/2024, 528 hlm.;
-- salinan di luar repo, tidak di-commit):
--   * Form 09.00 - 1 "RINCIAN ASET LAINNYA": PDF #page 187 (hlm. tercetak 135).
--     Kolom: Sandi Kantor, Nama Rekening, Sandi, Jumlah. Posisi resmi (sandi dan nama
--     persis dari PDF, bukan karangan):
--       1299020000 Premi Penjaminan LPS Dibayar di Muka
--       1299030000 Uang Muka Pajak
--       1299040000 Aset Pajak Tangguhan
--       1299050000 Biaya Dibayar di Muka
--       1299060000 Tagihan kepada Perusahaan Asuransi
--       1299070000 Uang Muka untuk Kegiatan Operasional
--     Penjelasan tiap pos ada pada Form 09.00 - 2 "PENJELASAN RINCIAN ASET LAINNYA",
--     PDF #page 188 (hlm. tercetak 136).
--
-- Nama akun COA di bawah SAMA PERSIS dengan nama pos resmi Form 09.00 agar pemetaan
-- COA -> pos (coa_mapping.go) terbaca tanpa terjemahan. Akun bertipe ASSET, saldo
-- normal DEBIT, dan bukan akun induk (is_header FALSE).
--
-- Aditif dan idempotent: ON CONFLICT (code) DO NOTHING dan NOT EXISTS pada akun GL.

INSERT INTO chart_of_accounts (code, name, type, normal_balance, book, is_header, created_at, updated_at)
VALUES
    ('10310', 'Premi Penjaminan LPS Dibayar di Muka', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW()),
    ('10320', 'Uang Muka Pajak',                      'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW()),
    ('10330', 'Aset Pajak Tangguhan',                 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW()),
    ('10340', 'Biaya Dibayar di Muka',                'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW()),
    ('10350', 'Tagihan kepada Perusahaan Asuransi',   'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW()),
    ('10360', 'Uang Muka untuk Kegiatan Operasional', 'ASSET', 'DEBIT', 'CONVENTIONAL', FALSE, NOW(), NOW())
ON CONFLICT (code) DO NOTHING;

-- Akun GL internal untuk tiap COA baru, mengikuti pola migrasi 000024. Sumber saldo
-- Form 01.00/09.00 adalah jurnal (journal_lines), jadi akun inilah yang menampung
-- postur saldonya; tanpa akun GL, COA baru tidak akan muncul pada laporan.
INSERT INTO accounts (
    id, account_number, customer_id, coa_id, account_type, currency,
    balance, available_balance, hold_balance, status, version, created_at, updated_at
)
SELECT
    uuid_generate_v4(),
    coa.code,
    NULL,
    coa.id,
    'INTERNAL_GL',
    'IDR',
    0.0000,
    0.0000,
    0.0000,
    'ACTIVE',
    1,
    NOW(),
    NOW()
FROM chart_of_accounts coa
WHERE coa.code IN ('10310', '10320', '10330', '10340', '10350', '10360')
  AND NOT EXISTS (
      SELECT 1 FROM accounts a
      WHERE a.coa_id = coa.id AND a.account_type = 'INTERNAL_GL'
  );
