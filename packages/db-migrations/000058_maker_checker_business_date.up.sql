-- CBS Migration 000058: tanggal bisnis pengajuan maker-checker (business_date)
-- Run after: 000057_transaction_type_deposit_placement.up.sql
--
-- Latar belakang:
--   Akumulasi batas harian menjumlahkan pengajuan maker-checker yang masih PENDING
--   selain jurnal terposting. Sebelum ini penyaring harinya memakai created_at::date
--   (tanggal kalender UTC). Itu salah dua kali:
--     1. UTC bukan zona waktu bank (Asia/Jakarta/WIB), sehingga pengajuan menjelang
--        tengah malam WIB dapat jatuh ke tanggal kalender yang berbeda.
--     2. Yang benar adalah TANGGAL BISNIS, bukan tanggal kalender. Saat tutup hari
--        belum berjalan, seluruh transaksi hari itu tetap bertanggal bisnis kemarin;
--        pengajuan PENDING yang baru dibuat harus ikut dihitung pada hari bisnis itu.
--   Tabel maker_checker_requests tidak menyimpan penanda tanggal bisnis, sehingga
--   ditambahkan kolom business_date: penanda eksplisit, bukan tebakan dari created_at.
--
-- Aditif dan idempotent: aman dijalankan ulang, tidak menimpa nilai yang sudah ada.
-- Baris lama di-backfill dari created_at di zona waktu bank (WIB) sebagai perkiraan
-- terbaik; kode baru selalu mengisi business_date dari BusinessDateRepository.
-- Baris yang tetap NULL (mis. tabel kosong lalu diisi kode lama) dibaca dengan
-- fallback (created_at AT TIME ZONE 'Asia/Jakarta')::date pada kueri akumulasi.

ALTER TABLE maker_checker_requests
    ADD COLUMN IF NOT EXISTS business_date DATE;

UPDATE maker_checker_requests
   SET business_date = (created_at AT TIME ZONE 'Asia/Jakarta')::date
 WHERE business_date IS NULL;

-- Membaca pengajuan PENDING per pembuat/jenis aksi pada satu tanggal bisnis.
CREATE INDEX IF NOT EXISTS idx_maker_checker_business_date
    ON maker_checker_requests(business_date, status, action_type);
