-- CBS Migration 000052: penanda tanggal bisnis run PPAP terakhir yang berhasil.
-- Run after: 000050_ojk_mapping_review.up.sql (nomor 000051 dipesan untuk gelombang
-- lain; migrasi ini tidak bergantung padanya).
--
-- Aditif & idempotent: hanya menyemai satu baris system_config dengan
-- INSERT ... ON CONFLICT DO NOTHING. Aman dijalankan berulang dan tidak mengubah
-- objek/data yang sudah ada.
--
-- ALASAN kunci ini ada:
-- Perbandingan CKPN (apps/api/internal/service/ckpn_service.go) membandingkan target
-- CKPN dengan loans.required_ppap hasil run PPAP terakhir. Bila PPAP belum berjalan
-- pada tanggal bisnis berjalan — mis. pemanggilan manual di luar tutup hari — angka
-- yang dibandingkan adalah milik run PPAP sebelumnya dan laporan tampak sah padahal
-- dasarnya kemarin. Gerbang di dalam tutup hari saja tidak menutup jalur manual itu.
--
-- Tanggal bisnis run PPAP yang berhasil dicatat di system_config, bukan tabel baru,
-- karena ia satu fakta global per bank pada satu waktu (bukan per kredit dan bukan
-- riwayat). CKPN membaca kunci ini dan menolak perbandingan bila tanggalnya berbeda
-- dari tanggal bisnis berjalan. Nilai kosong berarti belum pernah ada run PPAP yang
-- berhasil, sehingga perbandingan juga ditolak.
--
-- Nilai diisi oleh jalur produksi (PPAPService.RunDaily) setelah seluruh kredit
-- selesai diproses; baris di bawah hanya menyemai kuncinya agar terlihat dan dapat
-- diaudit sebelum run pertama. Format nilai: YYYY-MM-DD.
INSERT INTO system_config (key, value, description) VALUES
    ('ppap.last_run_business_date', '',
     'Tanggal bisnis (YYYY-MM-DD) run PPAP terakhir yang berhasil. Dibaca perbandingan CKPN; kosong berarti belum ada run.')
ON CONFLICT (key) DO NOTHING;
