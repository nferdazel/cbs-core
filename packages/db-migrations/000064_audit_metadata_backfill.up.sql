-- CBS Migration 000064: backfill metadata alasan penolakan kredit
-- Run after: 000063_product_parameters_writable.up.sql
--
-- Latar belakang:
--   Migrasi 000061 menambahkan audit_logs.metadata dan penolakan kredit BARU menulis
--   alasan ke sana, tetapi penolakan yang terjadi SEBELUM migrasi tetap
--   metadata = NULL padahal alasannya sudah ada di changes->>'reason'. Pembaca audit
--   lama (mis. handler audit yang memetakan metadata) tetap melihat null untuk
--   penolakan lama, justru masalah yang ingin diperbaiki 000061. Migrasi ini
--   menyalin changes.reason ke metadata untuk baris penolakan pra-migrasi.
--
-- Kriteria baris (hanya bila ketiganya terpenuhi):
--   metadata IS NULL
--   AND resource_type = 'loan'
--   AND action = 'REJECT_LOAN'
--   AND changes ? 'reason'
--   Nilai action/resource_type ini diambil dari penulisan audit produksi
--   (loan_service.go RejectLoan: writeAuditWithMetadata(..., "REJECT_LOAN", "loan", ...)
--   dengan changes memuat kunci "reason"), bukan tebakan. Aksi lain yang kebetulan
--   punya changes.reason (mis. WRITE_OFF_LOAN, CORRECT_LOAN_AMOUNT) TIDAK ikut karena
--   bukan penolakan kredit.
--
-- Bentuk metadata disamakan dengan penulisan baru: {"reason": changes->>'reason'}.
--
-- Jumlah baris yang diperkirakan terdampak di produksi TIDAK diketahui dari sini;
-- bisa nol dan bisa sangat banyak. Karena itu migrasi tidak boleh mengandalkan
-- perkiraan jumlah dan harus aman untuk jumlah berapa pun.
--
-- Idempotent: penjaga metadata IS NULL membuat jalan kedua tidak mengubah apa pun,
-- dan baris yang metadata-nya sudah terisi tidak pernah ditimpa.
--
-- Catatan trigger append-only:
--   audit_logs dijaga trigger audit_logs_append_only_row (migrasi 000030) yang menolak
--   setiap UPDATE, termasuk dari owner tabel. Backfill ini menonaktifkan trigger baris
--   sementara, menjalankan UPDATE, lalu menyalakannya kembali DI DALAM SATU TRANSAKSI
--   (scripts/migrate.sh memakai psql -1). Bila terjadi galat di tengah, transaksi
--   di-rollback utuh dan status trigger ikut kembali, sehingga trigger tidak pernah
--   tertinggal mati.

ALTER TABLE audit_logs DISABLE TRIGGER audit_logs_append_only_row;

UPDATE audit_logs
SET metadata = jsonb_build_object('reason', changes->>'reason')
WHERE metadata IS NULL
  AND resource_type = 'loan'
  AND action = 'REJECT_LOAN'
  AND changes ? 'reason';

ALTER TABLE audit_logs ENABLE TRIGGER audit_logs_append_only_row;
