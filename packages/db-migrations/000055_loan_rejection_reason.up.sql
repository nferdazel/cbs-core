-- CBS Migration 000055: alasan penolakan kredit (loans.rejection_reason)
-- Run after: 000051_limits_and_config.up.sql (nomor dipesan; migrasi ini tidak
-- bergantung isinya selain urutan).
--
-- Mengapa migrasi ini ada: endpoint POST /api/v1/loans/{id}/reject tidak pernah
-- membaca body permintaan dan tabel loans tidak punya kolom alasan, sehingga alasan
-- penolakan yang diketik pejabat hilang tanpa jejak — audit hanya mencatat status dan
-- nominal. Penolakan kredit adalah keputusan yang harus bisa dipertanggungjawabkan,
-- jadi alasannya disimpan pada baris kredit sekaligus di audit.
--
-- Aditif dan idempotent: kolom baru dengan bawaan '' sehingga baris lama tidak
-- berubah; batas panjang dijaga constraint (drop-add) agar aman dijalankan ulang.
-- Tidak ada backfill: kredit yang sudah ditolak sebelum migrasi ini memang tidak
-- punya alasan tersimpan dan tidak boleh dikarang.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS rejection_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE loans
    DROP CONSTRAINT IF EXISTS loans_rejection_reason_length;
ALTER TABLE loans
    ADD CONSTRAINT loans_rejection_reason_length CHECK (char_length(rejection_reason) <= 500);

COMMENT ON COLUMN loans.rejection_reason IS
    'Alasan penolakan pengajuan kredit; wajib diisi saat status diubah menjadi REJECTED.';
