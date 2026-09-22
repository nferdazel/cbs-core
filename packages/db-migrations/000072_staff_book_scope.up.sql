-- 000072_staff_book_scope.up.sql
-- Run after: 000071_ckpn_coa_syariah.up.sql
--
-- Menambahkan sumber kebenaran buku COA (KONVENSIONAL/SYARIAH) milik pengguna di
-- staff_users.book. Sebelum ini buku hanya difilter di klien web
-- (LoanWorkspace.tsx), sehingga pemanggil API langsung dapat membaca data buku
-- lain. Filter di klien bukan batas keamanan; kolom ini dipakai lapisan
-- repository untuk menyaring kredit, rekening, deposito, dan laporan.
--
-- Nilai:
--   - book = 'CONVENTIONAL'/'SYARIAH' -> pengguna hanya melihat buku itu.
--   - book NULL (belum ditentukan)    -> TIDAK dibatasi. Ini disengaja agar
--     pengguna lama tidak mendadak kehilangan akses; operator dapat menetapkan
--     buku menyusul.
--
-- Nilai awal untuk data produksi yang ada: peran lintas buku (SUPERADMIN dan
-- AUDITOR; SYSTEM tidak tersimpan di tabel ini) dibiarkan NULL karena memang
-- melayani kedua buku lewat Actor.IsCrossBook. Sisanya diberi 'CONVENTIONAL',
-- buku mayoritas BPR, supaya kebocoran silang buku langsung tertutup. Bank yang
-- seluruhnya syariah dapat mengubah baris itu ke 'SYARIAH' lewat operasi biasa
-- setelah migrasi; pilihan ini TIDAK mengunci peran lintas buku dan dapat
-- dibalik, sehingga bila ragu arah amannya adalah buku (bukan lintas) lalu
-- dilaporkan.
--
-- Idempotent: kolom hanya ditambahkan sekali dan pengisian awal hanya berjalan
-- pada saat kolom baru dibuat. Menjalankan ulang berkas ini tidak menimpa buku
-- yang sudah diubah operator, termasuk yang sengaja dikosongkan.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'staff_users'
          AND column_name = 'book'
    ) THEN
        ALTER TABLE staff_users ADD COLUMN book coa_book;
        UPDATE staff_users
           SET book = 'CONVENTIONAL'
         WHERE role NOT IN ('SUPERADMIN', 'AUDITOR');
    END IF;
END $$;
