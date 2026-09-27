-- CBS Migration 000107: kolom penyimpanan kolom K1 Form 06.00 yang tersisa.
-- Run after: 000106_ojk_form05_k1_columns.up.sql (urutan; tidak bergantung isinya).
-- Nomor 000107 dipesan khusus pekerjaan ini.
--
-- Menutup delapan kolom K1 Form 06.00 (docs/CELAH-LAPORAN-OJK.md §2b). Definisi sandi
-- diambil langsung dari salinan PDF resmi SEOJK No. 16/SEOJK.03/2024 di luar repo
-- (528 hlm., tidak di-commit), dengan nomor halaman PDF dicantumkan pada tiap COMMENT:
--   * loans.ojk_kelompok_kredit_code  -> IV  Kode Kelompok Kredit. BUKAN daftar sandi
--     OJK: BPR mengisi kode unik (angka dan/atau huruf) per kelompok peminjam pihak
--     tidak terkait (PDF #page 158). Karena bank-defined, nilainya disimpan apa adanya.
--   * loans.ojk_sumber_dana_code      -> X   Sumber Dana Pelunasan, sandi inline
--     10/21/22/31/32 (PDF #page 153 daftar; #page 160 penjelasan).
--   * loans.ojk_kategori_usaha_code   -> XXI Kategori Usaha, sandi inline 1/2/3/4
--     (PDF #page 154 daftar; #page 162-163 penjelasan kriteria).
--   * loans.ojk_sifat_kredit_code     -> XXXVIII Sifat Kredit, sandi inline 2/9
--     (PDF #page 155 daftar; #page 166 penjelasan).
--   * loans.ojk_penjamin_code         -> XXIV Penjamin/Golongan Penjamin, mengacu
--     Lampiran 02 Daftar Sandi Pihak Lawan (PDF #page 154; penjelasan #page 163-164).
--     Foreign key ke tabel referensi ojk_pihak_lawan (migrasi 000102).
--   * loans.ojk_penjamin_bagian_pct   -> XXIV Penjamin/Bagian yang Dijamin, persentase
--     0-100 sampai 2 desimal (PDF #page 154/164).
--   * loans.ojk_tanggal_mulai_macet   -> XV  Tanggal Mulai Macet: tanggal kredit mulai
--     dinyatakan macet (PDF #page 153 daftar; #page 161 penjelasan). BUKAN diturunkan
--     dari DPD: DPD menghitung hari sejak jatuh tempo, sedangkan tanggal macet
--     bergantung ambang/riwayat kebijakan yang tidak tersimpan per kredit.
--
-- Kolom II ID Pihak Lawan tidak butuh kolom baru: nilainya adalah nomor CIF internal
-- nasabah (customers.cif_number), sama dengan yang dilaporkan ke SLIK. Kutipan
-- (PDF #page 66, BAB II ID Pihak Lawan): "ID Pihak Lawan harus sama dengan nomor
-- Customer Identification File (CIF) yang dilaporkan melalui Sistem Layanan Informasi
-- Keuangan (SLIK)." Disingkapkan langsung oleh perakit laporan (form06.go).
--
-- Kolom XLII Tanggal Akad Akhir juga tidak butuh kolom baru: diturunkan dari tanggal
-- restrukturisasi terakhir (loans.restructured_at) bila ada, selain itu dari tanggal
-- akad awal (loans.akad_date). Kutipan (PDF #page 167): "Diisi dengan tanggal akad
-- terbaru fasilitas kredit. Apabila terdapat addendum perjanjian kredit akibat
-- restrukturisasi kredit, maka yang dilaporkan pada kolom ini yaitu tanggal akad
-- terbaru pada saat addendum perjanjian kredit." Lihat form06.go.
--
-- Kolom ini NULLABLE dan tidak diisi aplikasi mana pun. Bank mengisi nilainya lewat
-- SQL/seed sampai ada jalur API; baris yang belum diisi ditulis "-" pada laporan,
-- bukan ditebak dari loans.purpose atau nilai lain.
--
-- Tipe kolom sandi/teks TEXT agar seragam dengan kolom sandi OJK lain (000104/000105);
-- persentase memakai NUMERIC(5,2); tanggal memakai TIMESTAMPTZ mengikuti akad_date.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom, CHECK, foreign key, maupun COMMENT kedua (pernyataan berikutnya
-- no-op saat kolom sudah ada). Tidak ada data yang diubah.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS ojk_kelompok_kredit_code TEXT,
    ADD COLUMN IF NOT EXISTS ojk_sumber_dana_code TEXT,
    ADD COLUMN IF NOT EXISTS ojk_kategori_usaha_code TEXT,
    ADD COLUMN IF NOT EXISTS ojk_sifat_kredit_code TEXT,
    ADD COLUMN IF NOT EXISTS ojk_penjamin_code TEXT
        REFERENCES ojk_pihak_lawan (code),
    ADD COLUMN IF NOT EXISTS ojk_penjamin_bagian_pct NUMERIC(5,2)
        CHECK (ojk_penjamin_bagian_pct IS NULL
            OR (ojk_penjamin_bagian_pct >= 0 AND ojk_penjamin_bagian_pct <= 100)),
    ADD COLUMN IF NOT EXISTS ojk_tanggal_mulai_macet TIMESTAMPTZ;

COMMENT ON COLUMN loans.ojk_kelompok_kredit_code IS
    'Kode Kelompok Kredit peminjam pihak tidak terkait. Lampiran II Form 06.00-3, PDF #page 158: "Setiap kode kelompok kredit harus unik untuk setiap kelompok peminjam (1 (satu) nomor kode kelompok kredit untuk setiap 1 (satu) kelompok peminjam)." Bukan daftar sandi OJK: kode unik angka/huruf buatan BPR, tidak boleh dipakai ulang. Sumber Form 06.00 kolom IV. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_sumber_dana_code IS
    'Sandi inline Sumber Dana Pelunasan. Lampiran II Form 06.00-2, PDF #page 153: "1. Gaji/Honor 10; 2. Usaha a. Subsidi 21 b. Nonsubsidi 22; 3. Lainnya a. Subsidi 31 b. Nonsubsidi 32" (penjelasan #page 160). Sumber Form 06.00 kolom X. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_kategori_usaha_code IS
    'Sandi inline Kategori Usaha. Lampiran II Form 06.00-2, PDF #page 154: "1. Mikro 1; 2. Kecil 2; 3. Menengah 3; 4. Selain Mikro, Kecil, dan Menengah 4" (kriteria #page 162-163). Sumber Form 06.00 kolom XXI. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_sifat_kredit_code IS
    'Sandi inline Sifat Kredit. Lampiran II Form 06.00-2, PDF #page 155: "1. Pengalihan piutang 2; 2. Lainnya 9" (penjelasan #page 166). Sumber Form 06.00 kolom XXXVIII. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_penjamin_code IS
    'Sandi golongan Penjamin. Lampiran II Form 06.00-2, PDF #page 154: "1. Golongan Penjamin Mengacu pada Lampiran 02 – Daftar Sandi Pihak Lawan." (penjelasan golongan a-e #page 163-164). FK ke ojk_pihak_lawan. Sumber Form 06.00 kolom XXIV butir 1. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_penjamin_bagian_pct IS
    'Persentase bagian yang dijamin penjamin. Lampiran II Form 06.00-2, PDF #page 154: "Diisi dengan persentase bagian yang dijamin sampai dengan 2 (dua) digit desimal dibelakang koma." Rentang 0-100. Sumber Form 06.00 kolom XXIV butir 2. NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
COMMENT ON COLUMN loans.ojk_tanggal_mulai_macet IS
    'Tanggal kredit mulai dinyatakan berkualitas macet. Lampiran II Form 06.00-2, PDF #page 153: "Diisi dengan tanggal kredit mulai dinyatakan kualitas macet." Sumber Form 06.00 kolom XV. Bukan turunan DPD (DPD menghitung hari sejak jatuh tempo, bukan tanggal peralihan kualitas). NULL = belum diisi, laporan menulis "-". Diisi bank lewat SQL/seed.';
