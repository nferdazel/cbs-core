-- CBS Migration 000109: nilai agunan diperhitungkan untuk PPKA & kelonggaran tarik.
-- Run after: 000108_ojk_kb_participation_and_asset_classification.up.sql (urutan; tidak
-- bergantung isinya). Nomor 000109 dipesan khusus pekerjaan ini.
--
-- Menutup dua kolom K2 Form 06.00 (docs/CELAH-LAPORAN-OJK.md §2c):
--   * loans.ojk_agunan_ppka_amount       -> XXV Nilai Agunan Diperhitungkan untuk PPKA.
--   * loans.ojk_kelonggaran_tarik_amount -> XXVI Kelonggaran Tarik.
--
-- XXV: nilainya SUDAH dihitung modul PPAP (`CollateralValue`/pengurang Pasal 20 dan
-- pengecualian agunan tunai Pasal 19(4)(b); lihat service/ppap_service.go), tetapi dulu
-- hanya hidup di hasil run. Kolom ini menyimpannya per kredit agar laporan dapat
-- menelusuri angka tanpa menghitung ulang dengan rumus di perakit laporan. Yang mengisi:
--   - modul PPAP, saat ppap.collateral.enabled aktif. Bila saklar mati (bawaan), sistem
--     TIDAK menghitung pengurang, sehingga kolom tidak diubah (isian bank tetap); selama
--     belum pernah diisi nilainya NULL -> laporan menulis "-", bukan 0.
--   - bank lewat SQL/seed/API bila modul agunan belum diaktifkan.
--
-- XXVI: kelonggaran tarik adalah bagian plafon komitmen yang BELUM ditarik. Fasilitas
-- komitmen belum dimodelkan, jadi bank yang mengisi (SQL/seed) sampai jalur API ada;
-- baris kosong ditulis "-", bukan diturunkan dari plafon dikurangi baki (itu
-- mengasumsikan seluruh plafon adalah komitmen, asumsi yang tidak boleh dikarang).
--
-- NULL dibedakan dari 0: NULL = belum diisi/belum dihitung, 0 = nilai yang benar-benar
-- nol. CHECK menjaga nilai tidak negatif bila diisi.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT. ADD COLUMN IF NOT EXISTS membuat pengulangan
-- tidak menambah kolom/CHECK/COMMENT kedua. Tidak ada data yang diubah.

ALTER TABLE loans
    ADD COLUMN IF NOT EXISTS ojk_agunan_ppka_amount NUMERIC(28,4)
        CHECK (ojk_agunan_ppka_amount IS NULL OR ojk_agunan_ppka_amount >= 0),
    ADD COLUMN IF NOT EXISTS ojk_kelonggaran_tarik_amount NUMERIC(28,4)
        CHECK (ojk_kelonggaran_tarik_amount IS NULL OR ojk_kelonggaran_tarik_amount >= 0);

COMMENT ON COLUMN loans.ojk_agunan_ppka_amount IS
    'Nilai agunan yang diperhitungkan untuk PPKA, sumber Form 06.00 kolom XXV. Diisi modul PPAP saat ppap.collateral.enabled aktif (hasil run PPAP terakhir yang mengubah state kredit); bila modul agunan belum diaktifkan atau belum dijalankan, kolom tidak diubah dan bila belum pernah diisi nilainya NULL sehingga laporan menulis "-", BUKAN nol. Bank boleh mengisinya lewat SQL/seed/API bila modul belum diaktifkan. Tidak dihitung ulang oleh perakit laporan.';

COMMENT ON COLUMN loans.ojk_kelonggaran_tarik_amount IS
    'Kelonggaran tarik (komitmen): bagian plafon yang belum ditarik, sumber Form 06.00 kolom XXVI. Fasilitas komitmen belum dimodelkan, sehingga bank mengisinya lewat SQL/seed sampai jalur API tersedia. NULL = belum diisi, laporan menulis "-", bukan diturunkan dari plafon dikurangi baki.';
