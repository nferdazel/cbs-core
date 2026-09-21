-- 000053_password_expiry_enforcement.up.sql
-- Mengaktifkan penegakan kebijakan kedaluwarsa kata sandi dengan bawaan yang aman:
-- nilai auth.password_expiry_days diubah menjadi 0 (nonaktif).
--
-- ALASAN: kode sekarang benar-benar membaca auth.password_expiry_days pada jalur
-- login dan refresh (apps/api/internal/service/auth_service.go). Sebelumnya kunci
-- ini hanya di-seed dan tidak pernah dipakai. Bila nilai seed lama (90 hari)
-- dibiarkan aktif saat penegakan menyala, seluruh akun yang kata sandinya lebih tua
-- dari 90 hari akan langsung dibatasi hanya untuk mengganti kata sandi. Untuk
-- mencegah akun produksi terkunci mendadak, bawaan dimatikan. Bank mengisi N hari
-- (> 0) untuk mengaktifkan penegakan; 0 berarti nonaktif.
--
-- Aditif dan idempotent: hanya UPDATE satu baris system_config, tidak menyentuh
-- objek/data lain. Menjalankannya ulang tetap menghasilkan nilai 0 sampai bank
-- sengaja mengubahnya menjadi N > 0. Bila bank sudah sempat menyesuaikan nilai ini
-- sebelum migrasi berjalan, nilai itu ikut direset ke 0 demi mencegah penguncian
-- massal; bank tinggal mengisi N hari setelahnya.
UPDATE system_config
SET value = '0',
    description = 'Masa berlaku kata sandi (hari). 0 = nonaktif (kedaluwarsa tidak ditegakkan). Bank mengisi N hari untuk mengaktifkan.'
WHERE key = 'auth.password_expiry_days';
