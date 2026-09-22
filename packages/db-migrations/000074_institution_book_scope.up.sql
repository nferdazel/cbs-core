-- 000074_institution_book_scope.up.sql
-- Run after: 000073_unify_cash_coa_config.up.sql
--
-- Menambahkan kunci konfigurasi cakupan buku tingkat INSTALASI:
--   institution.book_scope = KONVENSIONAL | SYARIAH | DUAL
--
-- Latar: CBS Core adalah produk yang dijual per bank (satu instalasi per badan
-- hukum). Badan hukum pelanggan berbeda-beda: BPR (konvensional), BPRS (syariah),
-- atau Bank Umum dengan UUS (dua lini usaha). POJK 7/2024 tidak mengenal UUS pada
-- BPR; BPR berprinsip syariah berkonversi menjadi BPRS. Karena itu cakupan buku
-- harus menjadi setelan instalasi, bukan sekadar kolom buku per staf:
--   - KONVENSIONAL -> sisi syariah tidak boleh diakses (modul, produk, data, laporan).
--   - SYARIAH      -> sisi konvensional tidak boleh diakses.
--   - DUAL         -> kedua lini aktif (BUK + UUS).
--
-- Nilai awal wajib DUAL supaya instalasi produksi yang sudah berjalan tidak
-- berubah perilakunya sama sekali. Penegakan per-pengguna yang sudah ada
-- (staff_users.book, Actor.CanAccessBook, bookReadClause) TETAP dipakai sebagai
-- penjaga lapis kedua untuk instalasi DUAL dan tidak dihapus.
--
-- Idempotent: aman dijalankan ulang. Nilai yang sudah ditetapkan operator TIDAK
-- ditimpa; kunci yang sudah ada hanya deskripsinya yang dirapikan.

INSERT INTO system_config (key, value, description) VALUES
    ('institution.book_scope', 'DUAL',
     'Cakupan buku COA tingkat instalasi: KONVENSIONAL (hanya sisi konvensional, mis. BPR), SYARIAH (hanya sisi syariah, mis. BPRS), atau DUAL (kedua lini aktif, mis. Bank Umum dengan UUS). Nilai tidak dikenal/kosong diperlakukan sebagai DUAL. Diisi ke klaim setiap permintaan dan dilaporkan lewat /auth/me agar menu web disaring; penugasan buku per staf (staff_users.book) tetap menjadi penjaga lapis kedua pada instalasi DUAL.')
ON CONFLICT (key) DO UPDATE
    SET description = EXCLUDED.description;
