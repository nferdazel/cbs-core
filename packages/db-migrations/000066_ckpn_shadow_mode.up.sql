-- CBS Migration 000066: saklar mode bayangan CKPN
-- Run after: 000064_audit_metadata_backfill.up.sql (nomor dipesan; tidak bergantung isinya,
-- jadi gap nomor 000065 tidak masalah karena scripts/migrate.sh memproses urutan glob).
--
-- Latar belakang:
--   Mesin CKPN sudah ada dan teruji, tetapi ckpn.enabled masih 'false' sehingga langkah
--   ckpn_comparison pada tutup hari (EOD) dilewati. Menyalakan ckpn.enabled sekarang akan
--   menghasilkan angka yang tampak resmi padahal parameter PD/LGD belum dimatangkan bank.
--   Mode bayangan menjembatani keadaan itu: CKPN dihitung dan ditampilkan beserta
--   perbandingannya dengan PPKA TANPA menjurnal dan TANPA mengubah state, dengan asumsi
--   yang dilabeli jelas sebagai sementara.
--
-- Kunci ini dibaca apps/api/internal/service/ckpn_service.go (cfgCKPNShadowEnabled) dan
-- WAJIB ada di seed. Uji invarian seed konfigurasi
-- (internal/service/config_seed_invariant_test.go) menggagalkan build bila kunci yang
-- dibaca kode tidak di-seed, jadi jangan hapus baris ini tanpa menghapus pembacanya.
--
-- Hubungan dengan ckpn.enabled:
--   - ckpn.enabled = true  -> perilaku resmi (penjurnalan bila Run dipanggil). Bila kedua
--     saklar menyala, mode resmi yang berlaku dan mode bayangan diabaikan; service
--     melaporkan shadow_mode=false sehingga EOD memakai jalur resmi tanpa mengisi
--     bidang bayangan (lihat domain.CKPNComparisonSummary.ShadowMode).
--   - ckpn.enabled = false dan ckpn.shadow_mode.enabled = true -> EOD menghitung dan
--     melaporkan CKPN bayangan, tetapi TIDAK menulis jurnal dan TIDAK menghitung
--     pengurangan modal inti. Bila parameter PD/LGD kosong, yang dilaporkan adalah daftar
--     kunci yang harus diisi, bukan angka karangan.
--   - keduanya false -> langkah ckpn_comparison dilewati seperti sebelumnya (perilaku
--     sebelum migrasi ini).
--
-- Aditif dan idempotent: ON CONFLICT DO NOTHING; aman dijalankan ulang.

-- NILAI BAWAAN = 'true' (mode bayangan MENYALA) untuk instalasi baru.
--   Panel memutuskan bayangan menyala karena ia alat verifikasi yang TIDAK menjurnal:
--   bank dapat melihat angka CKPN vs PPKA dan daftar parameter yang belum diisi tanpa
--   satu pun jurnal tercipta. Instalasi yang SUDAH menjalankan migrasi ini tidak
--   berubah: ledger schema_migrations melewati berkas yang sudah tercatat, sehingga
--   nilai yang sudah disesuaikan operator (produksi sekarang true) tetap utuh dan
--   tidak pernah diturunkan menjadi false. Karena itu nilai TIDAK di-UPDATE berkas
--   migrasi berikutnya — cukup seed di sini yang hanya dibaca pemasangan baru.
--   `ckpn.enabled` sengaja tetap false: menyalakannya adalah langkah onboarding bank
--   setelah daftar periksa docs/CKPN-SIAP-RILIS.md tuntas, bukan bawaan otomatis.
INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.shadow_mode.enabled', 'true',
     'Mode bayangan CKPN: bila ckpn.enabled masih false, EOD menghitung dan melaporkan CKPN beserta perbandingan PPKA TANPA menjurnal dan tanpa mengubah state. Bawaan TRUE untuk instalasi baru (alat verifikasi yang tidak menjurnal). Angka ini BUKAN kewajiban akuntansi, belum disetujui bank, dan pengurangan modal inti belum dilakukan; asumsinya sementara. Bila ckpn.enabled true, mode resmi yang berlaku.')
ON CONFLICT (key) DO NOTHING;
