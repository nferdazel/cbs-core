-- CBS Migration 000038: kepatuhan Pasal 20 & Pasal 21 POJK No. 1 Tahun 2024 pada
-- perhitungan PPKA.
-- Run after: 000037_loan_cancelled.up.sql
-- Idempotent: aman dijalankan ulang (ADD COLUMN IF NOT EXISTS).
--
-- Mengapa migrasi ini ada: 000036 sudah menyediakan data agunan, tetapi penurunan PPKA
-- masih MATI karena tiga hal inti Pasal 20 belum punya datanya — tarif batas atas menurut
-- jenis (ayat 1), penurunan menurut waktu saat kredit Macet (ayat 3/5), pengecualian untuk
-- tanah ber-hak tanggungan (ayat 4), dan syarat pengurang Pasal 21 ayat (2). Migrasi ini
-- menambahkan penanda yang dibutuhkan domain untuk menegakkan pasal-pasal itu.
--
-- Saklar ppap.collateral.enabled TIDAK disentuh: bawaannya tetap false. Migrasi ini hanya
-- melengkapi kolom, bukan mengaktifkan pengurangan.

-- 1. Penanda kapan kredit ditetapkan Macet. Diperiksa lebih dulu: belum ada kolom serupa
--    di loans (tidak ada macet_at/classified_macet). Dasar hukum: Pasal 20 ayat (3) dan (5)
--    menghitung umur sejak kredit macet, sehingga butuh titik awal yang tetap.
ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS macet_at TIMESTAMPTZ;

COMMENT ON COLUMN loans.macet_at IS
    'Saat kredit pertama kali digolongkan Macet; dasar umur penurunan pengurang agunan Pasal 20 ayat (3)/(5) POJK 1/2024. Diisi sekali, tidak pernah ditimpa.';

-- 2. Penanda agunan Pasal 20(1), 20(4), dan Pasal 21(2).
--    Nilai bawaan dipilih konservatif: apa pun yang belum diisi operator dianggap TIDAK
--    memenuhi syarat pengurang (false), kecuali keberadaan dan dapat-dieksekusi yang
--    memang keadaan normalnya ya (true). Bawaan ini melindungi dari pengurangan yang
--    terlalu besar, bukan dari pengurangan yang terlalu kecil.
ALTER TABLE loan_collaterals
    -- Pasal 20(1) huruf b/d/e/f dan Pasal 20(4): tanah/bangunan baru dapat diperhitungkan
    -- bila bersertifikat, dan kadarnya berbeda bila tidak terikat hak tanggungan.
    ADD COLUMN IF NOT EXISTS certified BOOLEAN NOT NULL DEFAULT FALSE,
    -- Pasal 20(1) huruf b/g dan Pasal 20(4): ikatan hak tanggungan/fidusia/hipotek.
    ADD COLUMN IF NOT EXISTS mortgaged BOOLEAN NOT NULL DEFAULT FALSE,
    -- Pasal 20(4): nilai hak tanggungan pembanding "seluruh kewajiban debitur".
    ADD COLUMN IF NOT EXISTS mortgage_value NUMERIC(18,2) NOT NULL DEFAULT 0
        CHECK (mortgage_value >= 0),
    -- Pasal 20(1) huruf l dan Pasal 20(4): penilaian oleh penilai independen.
    ADD COLUMN IF NOT EXISTS appraiser_independent BOOLEAN NOT NULL DEFAULT FALSE,
    -- Pasal 21(2): keberadaan agunan diketahui. Bawaan TRUE karena agunan dicatat justru
    -- karena sudah diperiksa; baru berubah bila operator menemukan sebaliknya.
    ADD COLUMN IF NOT EXISTS exists_known BOOLEAN NOT NULL DEFAULT TRUE,
    -- Pasal 21(2): agunan dapat dieksekusi. Bawaan TRUE dengan alasan yang sama.
    ADD COLUMN IF NOT EXISTS executable BOOLEAN NOT NULL DEFAULT TRUE,
    -- Pasal 21(2): agunan milik pihak lain hanya jadi pengurang bila pemiliknya setuju.
    ADD COLUMN IF NOT EXISTS third_party_owner BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS owner_consent BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN loan_collaterals.certified IS
    'Agunan bersertifikat; syarat kadar Pasal 20 ayat (1) huruf b/d dan pengecualian ayat (4).';
COMMENT ON COLUMN loan_collaterals.mortgaged IS
    'Agunan terikat hak tanggungan/fidusia/hipotek; syarat Pasal 20 ayat (1) huruf b/g dan ayat (4).';
COMMENT ON COLUMN loan_collaterals.mortgage_value IS
    'Nilai hak tanggungan; pembanding seluruh kewajiban debitur pada Pasal 20 ayat (4).';
COMMENT ON COLUMN loan_collaterals.appraiser_independent IS
    'Agunan dinilai penilai independen dalam 1 tahun terakhir; syarat Pasal 20 ayat (1) huruf l dan ayat (4).';
COMMENT ON COLUMN loan_collaterals.exists_known IS
    'Keberadaan agunan diketahui; bila false, agunan bukan pengurang menurut Pasal 21 ayat (2).';
COMMENT ON COLUMN loan_collaterals.executable IS
    'Agunan dapat dieksekusi; bila false, agunan bukan pengurang menurut Pasal 21 ayat (2).';
COMMENT ON COLUMN loan_collaterals.third_party_owner IS
    'Agunan milik pihak lain; hanya mengurangi bila owner_consent true (Pasal 21 ayat (2)).';
COMMENT ON COLUMN loan_collaterals.owner_consent IS
    'Persetujuan pemilik pihak lain atas penggunaan agunan sebagai jaminan (Pasal 21 ayat (2)).';

-- Tidak ada backfill macet_at untuk kredit yang sudah Macet: tanggal awalnya tidak pernah
-- tercatat, dan menebaknya justru membuat umur Pasal 20(3)/(5) tampak pasti padahal bukan.
-- Kredit tanpa macet_at diperlakukan sebagai baru digolongkan Macet (pengurang penuh) dan
-- titik awalnya tersimpan pada perhitungan PPAP berikutnya.
