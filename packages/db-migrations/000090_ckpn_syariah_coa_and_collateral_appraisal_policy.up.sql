-- 000090_ckpn_syariah_coa_and_collateral_appraisal_policy.up.sql
-- Run after: 000089_staff_employee_id_sequence.up.sql
--
-- Keputusan panel (23 Sep 2026) yang bersifat KONFIGURASI — tidak mengubah angka,
-- jurnal, maupun pesan API:
--
-- 1. Akun CKPN unit syariah diisi bawaan. Migrasi 000042 sudah MENDAFTARKAN akun
--    11950 "CKPN - Pembiayaan" dan 15901 "Beban Kerugian Penurunan Nilai -
--    Pembiayaan", tetapi migrasi 000071 sengaja meng-seed kunci pemetaannya KOSONG
--    (artinya mengikuti kunci global konvensional 50301/10950). Selama kosong,
--    jurnal CKPN pembiayaan syariah jatuh ke akun konvensional dan dana UUS
--    tercampur buku konvensional; peringatan installation_validation.go menyala
--    untuk instalasi SYARIAH/DUAL. Migrasi ini mengisi kedua kunci HANYA bila masih
--    KOSONG sehingga pemetaan yang sudah disesuaikan bank TIDAK ditimpa.
--
-- 2. Kebijakan umur taksasi agunan: kunci `collateral.appraisal.validity_months` = 12.
--    Ini KEPUTUSAN PANEL sebagai praktik industri, BUKAN aturan tertulis: teks SEOJK
--    No. 2/SEOJK.03/2025 Lampiran II tidak mengatur umur taksasi (lihat catatan
--    migrasi 000087). Nilainya dipakai untuk menandai "data agunan tidak lengkap":
--    taksasi yang lebih tua dari umur ini dianggap kedaluwarsa sehingga data agunan
--    dinilai TIDAK LENGKAP dan agunan memakai bobot risiko 100% (tidak mengurangi
--    eksposur) sampai taksasi diperbarui. Status "bukan aturan tertulis" ditulis di
--    deskripsi kunci agar tidak salah dibaca sebagai kewajiban regulator.
--
-- 3. `ckpn.shadow_mode.enabled` TIDAK disentuh di sini. Nilai bawaan benar untuk
--    instalasi baru diperbaiki pada seed migrasi 000066; instalasi yang sudah
--    menjalankannya tidak berubah karena ledger schema_migrations melewatinya.
--    Saklar `ckpn.enabled` juga tidak disentuh: menyalakannya otomatis akan membuat
--    instalasi yang sedang berjalan membentuk CKPN mundur, yang dilarang. Lihat
--    docs/CKPN-SIAP-RILIS.md (langkah onboarding) dan peringatan saat start
--    (CKPNReadinessWarnings).
--
-- Idempotent dan aditif: INSERT ... ON CONFLICT (key) DO UPDATE ... WHERE value = ''.

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.coa.expense.syariah', '15901',
     'Kode COA beban kerugian penurunan nilai CKPN untuk unit syariah (sisi debit pembentukan; sisi kredit pemulihan). Diisi bawaan 15901 "Beban Kerugian Penurunan Nilai - Pembiayaan" (akun syariah migrasi 000042). Nilai hanya diisi bila masih KOSONG; pemetaan yang sudah disesuaikan bank tidak ditimpa.'),
    ('ckpn.coa.reserve.syariah', '11950',
     'Kode COA cadangan CKPN untuk unit syariah (sisi kredit pembentukan; sisi debit pemulihan). Diisi bawaan 11950 "CKPN - Pembiayaan" (akun syariah migrasi 000042). Nilai hanya diisi bila masih KOSONG; pemetaan yang sudah disesuaikan bank tidak ditimpa.'),
    ('collateral.appraisal.validity_months', '12',
     'Umur taksasi agunan dalam bulan (angka bulat). KEPUTUSAN PANEL sebagai praktik industri, BUKAN aturan tertulis: teks SEOJK No. 2/SEOJK.03/2025 Lampiran II tidak mengatur umur taksasi. Taksasi yang lebih tua dari ini dianggap kedaluwarsa sehingga data agunan dinilai TIDAK LENGKAP dan agunan memakai bobot risiko 100% (tidak mengurangi eksposur) sampai taksasi diperbarui.')
ON CONFLICT (key) DO UPDATE
    SET value = EXCLUDED.value,
        description = EXCLUDED.description
    WHERE system_config.value = '';
