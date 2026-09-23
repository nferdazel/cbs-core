-- 000093_ckpn_parameters_provisional_guard.up.sql
-- Run after: 000092_recovery_threshold_zero.up.sql
-- (000094 dipakai alur paralel; migrasi ini tidak bergantung isinya.)
--
-- KEPUTUSAN PANEL RISIKO CKPN (23 Sep 2026) butir 1: parameter PD/LGD yang dipakai
-- produksi saat ini DISANDERA dari tarif PPKA/LGD 0,45, sehingga statusnya SEMENTARA
-- dan HANYA untuk internal. Migrasi ini menambahkan PENGAMANNYA, bukan mengubah angka:
--   * ckpn.pd_frac.* dan ckpn.lgd_frac TIDAK disentuh sama sekali;
--   * ckpn.enabled TIDAK dinyalakan (tetap apa adanya).
--
-- Yang ditambahkan:
--   1. ckpn.parameters.status = SEMENTARA (bawaan; enum SEMENTARA | FINAL). Nilai tak
--      dikenal diperlakukan SEMENTARA oleh kode (gagal-aman).
--   2. ckpn.parameters.temporary_since = tanggal mulai parameter sementara (YYYY-MM-DD),
--      diisi CURRENT_DATE pada saat migrasi HANYA bila masih kosong. Maksimum 12 bulan
--      sejak tanggal ini (butir 1.4.2); lewat batas -> peringatan tingkat tinggi.
--   3. ckpn.parameters.ratification_months = 12 (batas ratifikasi).
--   4. ckpn.floor.ppka_enabled = true: lantai wajib target = max(EAD x PD x LGD,
--      required_ppap) (butir 1.3). Selama status SEMENTARA kode menegakkannya tanpa
--      memandang kunci ini; setelah FINAL kunci menjadi pilihan bank.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. Nilai bank TIDAK ditimpa: baris yang sudah
-- berisi nilai selain kosong ditinggalkan apa adanya (WHERE system_config.value = '').
-- Tidak mengubah akun jurnal maupun angka akuntansi apa pun.

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.parameters.status', 'SEMENTARA',
     'Status parameter CKPN: SEMENTARA (belum diratifikasi, bawaan) atau FINAL (sudah diratifikasi Direksi + akuntan, dan DPS untuk BPRS). Selama SEMENTARA angka PD/LGD HANYA untuk internal dan DILARANG menjadi dasar kolom CKPN laporan OJK/APOLO (keputusan panel butir 1.4-1.5). Nilai tak dikenal diperlakukan SEMENTARA oleh kode.'),
    ('ckpn.parameters.temporary_since', '',
     'Tanggal (YYYY-MM-DD) parameter CKPN sementara mulai dipakai. Maksimum 12 bulan sejak tanggal ini untuk ratifikasi (keputusan panel butir 1.4.2); bila lewat, sistem memberi peringatan tingkat tinggi. Diisi CURRENT_DATE saat migrasi bila masih kosong; nilai bank tidak ditimpa.'),
    ('ckpn.parameters.ratification_months', '12',
     'Batas ratifikasi parameter CKPN sementara dalam bulan (keputusan panel butir 1.4.2: maksimum 12 bulan). Perpanjangan hanya lewat berita acara Direksi + akuntan bertanggal.'),
    ('ckpn.floor.ppka_enabled', 'true',
     'Lantai wajib PPKA: CKPN yang dibentuk = max(EAD x PD x LGD, required_ppap) agar tidak di bawah PPKA (keputusan panel butir 1.3). Selama ckpn.parameters.status=SEMENTARA lantai SELALU ditegakkan tanpa memandang kunci ini; setelah FINAL bank boleh mematikannya. Bawaan true (direkomendasikan tetap true).')
ON CONFLICT (key) DO UPDATE
    SET value = EXCLUDED.value,
        description = EXCLUDED.description
    WHERE system_config.value = '';

-- Tanggal mulai diisi hanya bila (masih) kosong, supaya instalasi yang sudah mencatat
-- tanggalnya sendiri tidak diubah. Jalan kedua tidak mengubah apa pun (idempotent).
UPDATE system_config
SET value = TO_CHAR(CURRENT_DATE, 'YYYY-MM-DD')
WHERE key = 'ckpn.parameters.temporary_since'
  AND value = '';
