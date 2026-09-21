-- CBS Migration 000050: keputusan bank atas pemetaan COA ke pos laporan OJK.
-- Run after: 000048 (nomor 000049 sengaja tidak dipakai: perbaikan pencatatan EIR tidak menuntut perubahan skema) (nomor 000050 dipesan untuk gelombang ini).
-- Aditif & idempotent: hanya membuat tabel/indeks baru; aman dijalankan berulang dan
-- tidak mengubah objek yang sudah ada.
--
-- ALASAN tabel ini ada:
-- Pemetaan COA internal ke pos OJK pada kode (apps/api/internal/ojkreport/coa_mapping.go)
-- berstatus DRAF dan TIDAK BOLEH ditentukan pengembang. SEOJK No. 16/SEOJK.03/2024
-- mewajibkan setiap BPR memiliki pedoman konversi sendiri, sehingga bank harus dapat
-- meninjau, menyetujui, atau mencatat keberatan atas tiap baris pemetaan.
-- Keputusan itu tidak disimpan di berkas kode (tidak dapat diaudit dan hilang saat
-- deploy), melainkan di basis data: satu baris per (form, kode COA) beserta siapa dan
-- kapan memutuskan. Status pemetaan tetap DRAF sampai bank menyatakan sebaliknya;
-- tabel ini mencatat pernyataan itu, bukan menyalakan laporan secara otomatis.
--
-- Idempotensi keputusan ditegakkan indeks unik (form, coa_code) + INSERT ... ON CONFLICT
-- DO UPDATE di repositori: pengiriman ulang tidak menggandakan baris, hanya memperbarui
-- keputusan terakhir.

CREATE TABLE IF NOT EXISTS ojk_mapping_reviews (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- form: "01.00" atau "02.00" (lihat OJKBulananForms).
    form          VARCHAR(16) NOT NULL,
    -- coa_code: kode COA internal; "-" adalah baris penyeimbang laba/rugi berjalan.
    coa_code      VARCHAR(32) NOT NULL,
    -- decision: DISETUJUI = bank menyetujui pemetaan; DICATAT = bank mencatat keberatan.
    decision      VARCHAR(16) NOT NULL,
    note          TEXT NOT NULL DEFAULT '',
    -- decided_by: nama pengguna pelaku; decided_by_id: uuid staf bila tersedia.
    -- ON DELETE SET NULL agar riwayat keputusan tetap ada walau akun staf dihapus.
    decided_by    VARCHAR(64) NOT NULL DEFAULT '',
    decided_by_id UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    decided_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ojk_mapping_reviews_decision_check
        CHECK (decision IN ('DISETUJUI', 'DICATAT'))
);

-- Satu keputusan per baris pemetaan; kunci idempotensi upsert.
CREATE UNIQUE INDEX IF NOT EXISTS uq_ojk_mapping_reviews_line
    ON ojk_mapping_reviews (form, coa_code);

-- Daftar baris yang belum disetujui adalah tampilan utama, jadi keputusan diindeks.
CREATE INDEX IF NOT EXISTS idx_ojk_mapping_reviews_decision
    ON ojk_mapping_reviews (decision);

COMMENT ON TABLE ojk_mapping_reviews IS
    'Keputusan bank (disetujui/dicatat keberatannya) atas tiap baris pemetaan COA ke pos laporan OJK; bukti audit siapa dan kapan. Status pemetaan tetap draf sampai bank menyatakan sebaliknya.';
