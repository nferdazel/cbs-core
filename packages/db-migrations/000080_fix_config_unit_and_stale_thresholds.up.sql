-- 000080_fix_config_unit_and_stale_thresholds.up.sql
-- Run after: 000079_user_groups_permissions.up.sql
--
-- Dua temuan kecil yang dilaporkan (commit e78162c) dan diperbaiki di sini:
--
-- 1. SATUAN deskripsi keliru. deposit.mudharabah.yield_annual_pct dideskripsikan
--    "desimal, mis. 0.06 untuk 6%", padahal kode membaca nilainya sebagai PERSEN:
--    domain.DepositDailyProfit membagi yieldRate dengan 100 (deposit.go), sama seperti
--    profitRate. Deskripsi yang menyebut desimal menuntun bank mengisi 0,06 sehingga
--    bagi hasil yang terhitung 100x lebih kecil dari yang dimaksud. Deskripsi
--    diperbaiki menyebut persen; NILAI tidak disentuh.
--
-- 2. Ambang maker-checker ter-seed tetapi TIDAK PERNAH dibaca kode. Threshold()
--    (maker_checker_service.go) memang membangun kunci maker_checker.<aksi>.threshold,
--    tetapi satu-satunya pemanggilnya adalah guardLoanApproval di loan_service.go,
--    dengan aksi LOAN_WRITE_OFF / LOAN_RECOVERY / LOAN_CORRECTION. Untuk setoran,
--    penarikan, dan transfer, keputusan persetujuan dibaca dari
--    limit.<peran>.<jenis>.approval_above (migrasi 000054 menjadikannya SATU sumber
--    kebenaran; kunci maker_checker.*-nya sudah ditandai USANG di sana). Pembatalan
--    lintas hari SELALU menunggu pejabat kedua tanpa melihat nominal, sehingga
--    maker_checker.reverse_transaction.threshold juga tidak pernah dipakai.
--
--    Karena ketiga transaksi itu punya sumber lain dan pembatalan tidak memakai
--    ambang, kunci-kunci basi ini DIHAPUS — sesuai aturan W3 "jangan biarkan alias
--    lama hidup": kunci ter-seed yang tidak dibaca membuat pembaca konfigurasi
--    menyangka ambang itu berlaku, padahal tidak. Penghapusan mengubah perilaku: nol.
--    Ambang tiga aksi kredit yang benar-benar dibaca Threshold() TIDAK dihapus.
--
-- Sifat migrasi: idempotent dan aman dijalankan ulang (UPDATE/DELETE 0 baris bila
-- kunci tidak ada). Tidak mengubah akun jurnal maupun angka akuntansi.

-- 1. Perbaiki satuan deskripsi bagi hasil mudharabah (persen, bukan desimal).
UPDATE system_config
SET description = 'Proyeksi imbal hasil tahunan deposito bagi hasil dalam PERSEN (mis. 6 untuk 6% per tahun; mesin membagi 100). Nilai 0 = bagi hasil nol; bank WAJIB meninjau dan mengisi tarif proyeksinya.'
WHERE key = 'deposit.mudharabah.yield_annual_pct';

-- 2. Hapus ambang maker-checker yang ter-seed tetapi tidak dibaca kode. Satu kunci satu
--    pernyataan DELETE agar penelusuran (dan parser uji invarian seed) sederhana.
DELETE FROM system_config WHERE key = 'maker_checker.deposit.threshold';
DELETE FROM system_config WHERE key = 'maker_checker.withdrawal.threshold';
DELETE FROM system_config WHERE key = 'maker_checker.transfer.threshold';
DELETE FROM system_config WHERE key = 'maker_checker.reverse_transaction.threshold';
