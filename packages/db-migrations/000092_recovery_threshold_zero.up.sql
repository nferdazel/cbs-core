-- 000092_recovery_threshold_zero.up.sql
-- Run after: 000091_recover_permission_and_account_freeze.up.sql
--
-- KEPUTUSAN PANEL (23 Sep 2026): setiap pemulihan kredit hapus buku menuntut
-- persetujuan pejabat kedua; TIDAK ADA nominal yang boleh lolos langsung. Ambang
-- `maker_checker.loan_recovery.threshold` karena itu disetel 0. Menurut
-- maker_checker.Threshold, nilai 0 berarti setiap nominal wajib disetujui.
--
-- Nilai 10.000.000 berasal dari migrasi 000085, yang mengisinya HANYA bila nilainya
-- masih bawaan 0. Setelah itu bank boleh menyesuaikan sendiri (kolom system_config
-- memang disediakan untuk itu, mis. melonggarkan recovery kecil). Karena migrasi ini
-- TIDAK boleh menimpa kebijakan bank yang sudah diubah, UPDATE di bawah hanya
-- menyentuh baris yang MASIH PERSIS memuat nilai bawaan 000085 ('10000000'). Nilai
-- lain apa pun — termasuk '0' bila bank sudah menyetelnya sendiri — ditinggalkan apa
-- adanya. Deskripsi pun hanya diperbarui pada baris yang benar-benar diubah.
--
-- Keterbatasan yang disadari: bank yang sengaja mengisi ULANG 10.000.000 (sama persis
-- dengan bawaan migrasi) tidak dapat dibedakan dari baris bawaan karena system_config
-- tidak menyimpan penanda versi. Baris seperti itu IKUT disetel 0. Dampaknya adalah
-- MEMPERKETAT (bukan melonggarkan) kendali, jadi tetap searah dengan keputusan panel;
-- bila kelak dibutuhkan pembedaan yang pasti, tambahkan kolom penanda asal nilai.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT (UPDATE ber-klausa nilai bawaan; jalan kedua
-- tidak menemukan baris dan tidak mengubah apa pun). Tidak mengubah akun jurnal,
-- angka akuntansi, maupun ckpn.enabled.

UPDATE system_config
SET value = '0',
    description = 'Ambang nominal pemulihan kredit hapus buku (Rp). 0 = SETIAP pemulihan wajib disetujui pejabat kedua (keputusan panel 23 Sep 2026); isi nilai positif untuk mengizinkan nominal di bawahnya berjalan langsung.'
WHERE key = 'maker_checker.loan_recovery.threshold'
  AND value = '10000000';
