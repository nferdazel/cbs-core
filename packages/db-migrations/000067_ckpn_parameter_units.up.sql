-- CBS Migration 000067: perjelas satuan parameter CKPN (fraksi 0..1, bukan persen)
-- Run after: 000066_ckpn_shadow_mode.up.sql.
--
-- Latar belakang:
--   Produksi mengisi ckpn.pd_frac.gol_1..5 = 1, 5, 15, 35, 60 dan ckpn.lgd_frac = 45 dengan niat
--   PERSEN, sedangkan mesin CKPN membaca parameter sebagai FRAKSI 0..1. Nilai di atas 1
--   ditolak (dan dilaporkan sebagai kekurangan parameter), tetapi nilai tepat 1 ambigu:
--   dibaca 100%, padahal bank mungkin bermaksud 1%. Deskripsi seed 000042 sudah menyebut
--   "(fraksi)", namun tidak menegaskan bahwa persen harus dibagi 100 lebih dulu.
--
--   Migrasi ini HANYA memperjelas deskripsi kunci; tidak mengubah nilai, saklar, maupun
--   rumus. Satuan yang berlaku tetap FRAKSI 0..1: 1% ditulis 0.01, 45% ditulis 0.45.
--
-- Aditif dan idempotent: hanya UPDATE deskripsi; aman dijalankan ulang.

UPDATE system_config SET description =
    'Probability of Default golongan 1 - Lancar. SATUAN FRAKSI 0..1, BUKAN PERSEN: 1% ditulis 0.01 (bukan 1). Wajib diisi bank dari data historis; kosong = perhitungan kredit itu ditolak.'
    WHERE key = 'ckpn.pd_frac.gol_1';
UPDATE system_config SET description =
    'Probability of Default golongan 2 - Dalam Perhatian Khusus. SATUAN FRAKSI 0..1, BUKAN PERSEN: 5% ditulis 0.05 (bukan 5). Wajib diisi bank dari data historis.'
    WHERE key = 'ckpn.pd_frac.gol_2';
UPDATE system_config SET description =
    'Probability of Default golongan 3 - Kurang Lancar. SATUAN FRAKSI 0..1, BUKAN PERSEN: 15% ditulis 0.15 (bukan 15). Wajib diisi bank dari data historis.'
    WHERE key = 'ckpn.pd_frac.gol_3';
UPDATE system_config SET description =
    'Probability of Default golongan 4 - Diragukan. SATUAN FRAKSI 0..1, BUKAN PERSEN: 35% ditulis 0.35 (bukan 35). Wajib diisi bank dari data historis.'
    WHERE key = 'ckpn.pd_frac.gol_4';
UPDATE system_config SET description =
    'Probability of Default golongan 5 - Macet. SATUAN FRAKSI 0..1, BUKAN PERSEN: 60% ditulis 0.60 (bukan 60). Wajib diisi bank dari data historis.'
    WHERE key = 'ckpn.pd_frac.gol_5';
UPDATE system_config SET description =
    'Loss Given Default. SATUAN FRAKSI 0..1, BUKAN PERSEN: 45% ditulis 0.45 (bukan 45). Boleh satu nilai all account bila data tidak mendukung pengelompokan (PA BPR butir 12.7.a). Wajib diisi bank; kosong = perhitungan ditolak.'
    WHERE key = 'ckpn.lgd_frac';
