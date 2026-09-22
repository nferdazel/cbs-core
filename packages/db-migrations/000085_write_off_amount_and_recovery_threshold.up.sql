-- 000085_write_off_amount_and_recovery_threshold.up.sql
-- Run after: 000084_kpmm_config_verify.up.sql
--
-- Dua keputusan pemilik sistem yang membutuhkan skema:
--
-- 1. NILAI HAPUS BUKU DISIMPAN DI BARIS KREDIT (lubang nyata).
--    Eksekusi hapus buku menolkan outstanding_principal, sehingga sesudahnya nilai
--    yang pernah dilepas hanya tersisa di audit_logs (WRITE_OFF_LOAN) dan jurnal
--    WOFF-. Akibatnya pemulihan (recovery) tidak punya batas: pencatatan recovery
--    10x lipat dari nilai hapus buku diterima. Kolom written_off_amount menyimpan
--    angka itu pada baris kredit; pemulihan menolak bila akumulasinya melebihi.
--
--    Backfill data lama memakai sumber paling andal lebih dulu:
--      a. audit_logs aksi WRITE_OFF_LOAN (changes->>'amount' = pokok + bunga + denda);
--      b. jurnal WOFF-<nomor kredit>, jumlah kaki DEBIT, bila auditnya tidak ada.
--    Baris WRITTEN_OFF yang tidak punya keduanya dibiarkan 0 dan karena itu pemulihan
--    atasnya DITOLAK oleh kode (fail closed), bukan dibiarkan tanpa batas.
--
-- 2. AMBANG RECOVERY. Pemulihan di bawah ambang boleh dicatat langsung (mis. oleh
--    TELLER/AO), di atas ambang wajib lewat maker-checker. Nilai awal Rp10.000.000
--    mengikuti pola kunci maker_checker.<aksi>.threshold yang sudah ada. Nilai hanya
--    diubah bila masih '0' (bawaan migrasi 000029) supaya kebijakan bank yang sudah
--    disesuaikan tidak tertimpa.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT (ADD COLUMN IF NOT EXISTS; UPDATE ber-klausa
-- "= 0"). Tidak mengubah akun jurnal, angka akuntansi, maupun ckpn.enabled.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS written_off_amount NUMERIC(18,2) NOT NULL DEFAULT 0;

-- Backfill (a): dari audit eksekusi hapus buku. Ambil catatan TERBARU per kredit bila
-- ada lebih dari satu. changes->>'amount' dijamin berbentuk angka oleh kode penulisnya,
-- tetapi pola regex dipasang agar baris non-angka (mis. data rusak) tidak menggagalkan
-- migrasi — baris seperti itu dibiarkan 0 dan dilaporkan tidak pasti.
UPDATE loans l
SET written_off_amount = a.written_off
FROM (
    SELECT resource_id,
           (changes->>'amount')::numeric AS written_off,
           ROW_NUMBER() OVER (PARTITION BY resource_id ORDER BY created_at DESC, id DESC) AS rn
    FROM audit_logs
    WHERE action = 'WRITE_OFF_LOAN'
      AND resource_type = 'loan'
      AND changes ? 'amount'
      AND changes->>'amount' ~ '^[0-9]+(\.[0-9]+)?$'
) a
WHERE a.rn = 1
  AND a.resource_id = l.id::text
  AND l.status = 'WRITTEN_OFF'
  AND l.written_off_amount = 0;

-- Backfill (b): cadangan untuk kredit yang auditnya tidak ada. Jurnal hapus buku
-- memakai idempotency_key 'WOFF-<nomor kredit>' dan melepas total pada kaki DEBIT
-- (cadangan pokok + beban bunga + beban denda), sehingga jumlah kaki DEBIT = nilai
-- yang dilepas.
UPDATE loans l
SET written_off_amount = COALESCE((
    SELECT SUM(jl.amount)
    FROM journal_entries je
    JOIN journal_lines jl ON jl.journal_entry_id = je.id
    WHERE je.idempotency_key = 'WOFF-' || l.loan_number
      AND jl.direction = 'DEBIT'
), 0)
WHERE l.status = 'WRITTEN_OFF'
  AND l.written_off_amount = 0
  AND EXISTS (
    SELECT 1 FROM journal_entries je
    WHERE je.idempotency_key = 'WOFF-' || l.loan_number
  );

-- Ambang recovery: nilai awal 10 juta, HANYA bila masih bawaan 0.
UPDATE system_config
SET value = '10000000',
    description = 'Ambang nominal pemulihan kredit hapus buku dalam Rupiah penuh. Di bawah ambang boleh dicatat langsung; pada/setelah ambang wajib disetujui pejabat kedua lewat maker-checker. 0 berarti setiap pemulihan wajib disetujui.'
WHERE key = 'maker_checker.loan_recovery.threshold'
  AND value = '0';
