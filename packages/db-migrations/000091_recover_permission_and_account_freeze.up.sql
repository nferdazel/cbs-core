-- 000091_recover_permission_and_account_freeze.up.sql
-- Run after: 000090 (lihat catatan repositori: alur lain memakai 000090).
--
-- Dua keputusan panel pada matriks kewenangan & kelengkapan operasi:
--
-- 1. IZIN PEMULIHAN HAPUS BUKU (loans:recover) DIPINDAH KE PEJABAT.
--    Migrasi 000079 men-seed loans:recover untuk TELLER dan AO karena alasan
--    "tidak ada yang terkunci" pada saat itu. Panel menilai pemulihan hapus buku
--    adalah tindakan atas aset yang sudah dilepas dari neraca, sehingga bukan
--    wewenang pelaksana layanan. Izin DICABUT dari TELLER dan AO, dan diberikan
--    kepada SUPERVISOR. SUPERADMIN/ADMIN tetap memegangnya agar tidak ada yang
--    terkunci. Izin efektif runtime dibaca dari group_permissions, jadi migrasi
--    inilah yang mengubah perilaku; domain.RolePermissions hanya seed awal.
--
-- 2. RUTE PEMBEKUAN REKENING + PEMISAHAN TUGAS.
--    Sebelum ini API tidak punya rute pembekuan (hanya lewat SQL), sehingga
--    pelaksana pembekuan tidak tercatat dan tidak dapat dicegah membatalkan
--    pembekuannya sendiri. Kolom frozen_by/frozen_at menyimpan pelaksana dan waktu
--    pembekuan terakhir; service menolak unfreeze oleh frozen_by yang sama.
--    Kewenangan pembekuan memakai izin accounts:freeze yang sudah ada dan hanya
--    dipegang SUPERADMIN/ADMIN/SUPERVISOR — tidak ada izin baru yang dibuat.
--
-- Sifat migrasi: ADITIF dan IDEMPOTENT (ADD COLUMN IF NOT EXISTS; INSERT ... ON
-- CONFLICT DO NOTHING; DELETE hanya baris yang ada). Tidak mengubah akun jurnal,
-- angka akuntansi, maupun ckpn.enabled. Tidak menyentuh ambang maker-checker apa pun.

-- 1a. Kolom pelaksana pembekuan rekening. Tanpa FK ketat: baris staf boleh diarsipkan,
-- dan pembekuan lama tetap sah walau pelaksananya sudah tidak ada. Nilai NULL berarti
-- pembekuan pra-migrasi (boleh dibatalkan siapa pun yang berwenang).
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS frozen_by UUID REFERENCES staff_users(id);
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS frozen_at TIMESTAMPTZ;

-- 2a. Cabut pemulihan dari pelaksana layanan (TELLER, AO).
DELETE FROM group_permissions
WHERE (group_id, permission) IN (
    SELECT g.id, v.permission
    FROM (VALUES
        ('ROLE_TELLER', 'loans:recover'),
        ('ROLE_AO',     'loans:recover')
    ) AS v(code, permission)
    JOIN user_groups g ON g.code = v.code
);

-- 2b. Berikan pemulihan kepada pejabat (SUPERVISOR). ADMIN/SUPERADMIN sudah punya.
INSERT INTO group_permissions (group_id, permission)
SELECT g.id, v.permission
FROM (VALUES
    ('ROLE_SUPERVISOR', 'loans:recover')
) AS v(code, permission)
JOIN user_groups g ON g.code = v.code
ON CONFLICT (group_id, permission) DO NOTHING;
