-- 000054_single_approval_threshold_source.up.sql
-- Menjadikan SATU sumber kebenaran untuk ambang persetujuan transaksi yang dijaga
-- penjaga batas: setoran, penarikan, transfer, dan penempatan deposito.
--
-- ALASAN: dua kunci menyimpan ambang untuk hal yang sama:
--   maker_checker.<jenis>.threshold      (dibaca maker_checker.Threshold)
--   limit.<peran>.<jenis>.approval_above (dibaca penjaga batas limit_service.Check)
-- Selama kode menulis payload.threshold dari kunci pertama sementara keputusannya
-- memakai kunci kedua, pembaca API melihat angka yang BUKAN angka yang dipakai
-- memutuskan. Bila kedua nilai berbeda, dana dapat lolos lewat jalur yang salah.
-- Kode kini memakai limit.<peran>.<jenis>.approval_above sebagai sumber tunggal
-- untuk penjaga batas; maker_checker.Threshold hanya dipakai aksi yang memang tidak
-- punya kunci limit (hapus buku, recovery, koreksi kredit, pembatalan transaksi).
--
-- Kunci lama TIDAK dihapus: baris dan nilainya dipertahankan agar klien/pembaca lama
-- tidak patah, tetapi deskripsinya ditandai usang dan menunjuk sumber yang berlaku.
-- Aditif dan idempotent: aman dijalankan ulang dan tidak menimpa nilai operator.
UPDATE system_config
SET description = 'USANG untuk penjaga batas transaksi: sumber kebenaran ambang persetujuan setoran/penempatan deposito adalah limit.<peran>.deposit.approval_above. Nilai dipertahankan agar pembaca lama tidak patah; tidak lagi dipakai memutuskan.'
WHERE key = 'maker_checker.deposit.threshold';

UPDATE system_config
SET description = 'USANG untuk penjaga batas transaksi: sumber kebenaran ambang persetujuan penarikan adalah limit.<peran>.withdrawal.approval_above. Nilai dipertahankan agar pembaca lama tidak patah; tidak lagi dipakai memutuskan.'
WHERE key = 'maker_checker.withdrawal.threshold';

UPDATE system_config
SET description = 'USANG untuk penjaga batas transaksi: sumber kebenaran ambang persetujuan transfer adalah limit.<peran>.transfer.approval_above. Nilai dipertahankan agar pembaca lama tidak patah; tidak lagi dipakai memutuskan.'
WHERE key = 'maker_checker.transfer.threshold';
