-- 000079_user_groups_permissions.up.sql
-- Run after: 000078_rename_config_keys.up.sql
--
-- ALASAN (keputusan pemilik sistem, docs/BACKLOG.md item 14/17/18/20):
-- izin TIDAK BOLEH hanya hidup di kode. Sebelum ini 8 peran & 30 izin dipetakan di
-- apps/api/internal/domain/staff.go (RolePermissions) dan disalin lagi di web
-- (apps/web/src/components/layout/nav.ts). Akibatnya:
--   - mengubah pemetaan izin menuntut rilis kode (bertentangan dengan keputusan
--     pemilik: "izin harus bisa diubah tanpa rilis kode");
--   - web menyimpan salinan peran sebagai daftar, sehingga bisa berbeda dari server;
--   - tidak ada jejak audit saat pemetaan berubah.
-- Migrasi ini memindahkan sumber kebenaran pemetaan izin ke database:
--   user_groups          -> grup (satu grup bawaan per peran + grup tambahan bank)
--   group_permissions    -> izin yang diberikan ke sebuah grup
--   user_group_members   -> keanggotaan pengguna (boleh lebih dari satu grup)
--   menu_catalog         -> katalog menu aplikasi (kunci stabil untuk web)
--   menu_permissions     -> izin yang membuka sebuah menu (0 baris = semua pengguna)
--
-- Kode tetap menjadi SEED AWAL, bukan sumber kedua: blok seed di bawah WAJIB
-- mereproduksi RolePermissions yang berlaku sekarang, sehingga tidak ada pengguna
-- yang kehilangan akses setelah migrasi. Bila kode diubah tanpa memperbarui migrasi
-- ini, uji invarian (permission_seed_invariant_test.go) GAGAL dan ketidaksinkronan
-- itu terlihat; runtime memakai DATABASE sebagai pemenang (kode tidak dibaca lagi
-- saat memeriksa izin).
--
-- Grup dipakai untuk DUA hal berbeda dari cakupan buku (`book` = lini usaha pada
-- data): (1) tiering kewenangan persetujuan lewat kolom approval_limit_role yang
-- menunjuk matriks limit per peran yang SUDAH ADA (000051) — tidak ada matriks kedua;
-- (2) akses menu. Model kerja tetap antrean bersama berdasarkan peran + unit, bukan
-- penugasan ke orang: grup hanya menambah izin, tidak mengikat pekerjaan ke individu.
--
-- Idempotent & aditif: IF NOT EXISTS / ON CONFLICT DO NOTHING. Nilai yang sudah
-- disesuaikan operator TIDAK ditimpa. Dijalankan scripts/migrate.sh dalam SATU
-- transaksi per berkas, jadi tidak ada BEGIN/COMMIT eksplisit di sini.

-- 1. Grup pengguna.
CREATE TABLE IF NOT EXISTS user_groups (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code         TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    -- is_system = grup bawaan hasil seed peran. Tidak boleh dihapus karena
    -- keanggotaan pengguna dan penurunan izin bertumpu padanya. Boleh diubah
    -- izinnya lewat maker-checker.
    is_system    BOOLEAN NOT NULL DEFAULT FALSE,
    -- Kaitan dengan matriks limit yang SUDAH ADA (limit.<peran>.<jenis>.* di 000051):
    -- peran yang batasnya dipakai sebagai jenjang kewenangan persetujuan grup ini.
    -- NULL berarti memakai peran pengguna sendiri. Kolom ini hanya KAIT, bukan
    -- matriks kedua; nilainya dibaca penjaga batas yang sama, bukan penjaga baru.
    approval_limit_role staff_role,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_groups IS
    'Grup pengguna untuk akses menu dan tiering kewenangan persetujuan. Berbeda dari cakupan buku (book = lini usaha pada data); grup tidak menentukan nasabah siapa yang ditangani. Diubah lewat maker-checker dan teraudit.';
COMMENT ON COLUMN user_groups.approval_limit_role IS
    'Peran pada matriks limit 000051 yang menjadi jenjang kewenangan persetujuan grup ini. NULL = peran pengguna. Kait tanpa matriks limit kedua.';

-- 2. Pemetaan grup -> izin. Nama izin adalah antarmuka dengan kode (kode yang
-- menegakkan), jadi nilai seed wajib sama dengan konstanta Perm* di domain.
CREATE TABLE IF NOT EXISTS group_permissions (
    group_id     UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    permission   TEXT NOT NULL,
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, permission)
);

COMMENT ON TABLE group_permissions IS
    'Izin efektif sebuah grup. Sumber kebenaran pemetaan izin (kode hanya seed awal). Perubahan lewat maker-checker action_type permission_change dan teraudit sebelum->sesudah.';

-- 3. Keanggotaan pengguna -> grup (banyak-ke-banyak; pengguna boleh >1 grup).
CREATE TABLE IF NOT EXISTS user_group_members (
    user_id      UUID NOT NULL REFERENCES staff_users(id) ON DELETE CASCADE,
    group_id     UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id)
);

COMMENT ON TABLE user_group_members IS
    'Keanggotaan pengguna pada grup. Izin efektif = izin grup ROLE_<peran> UNION izin grup lain yang diikuti, sehingga keanggotaan bersifat aditif dan tidak mencabut akses mendadak.';

-- 4. Katalog menu. kunci menu stabil dan dipakai web untuk memetakan tampilan
-- (ikon, label i18n, href); pemetaannya ke izin ada di DB, bukan di kode web.
CREATE TABLE IF NOT EXISTS menu_catalog (
    menu_key     TEXT PRIMARY KEY,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE menu_catalog IS
    'Katalog menu aplikasi. Web hanya menyimpan tampilan (ikon/label/href) menurut menu_key; pemetaan menu->izin dapat dikelola di sini, bukan sebagai daftar peran di kode web.';

-- 5. Pemetaan menu -> izin. Sebuah menu tampil bila pengguna punya SALAH SATU izin
-- yang dipetakan. Menu tanpa baris = tampil untuk semua pengguna terautentikasi.
CREATE TABLE IF NOT EXISTS menu_permissions (
    menu_key     TEXT NOT NULL REFERENCES menu_catalog(menu_key) ON DELETE CASCADE,
    permission   TEXT NOT NULL,
    PRIMARY KEY (menu_key, permission)
);

COMMENT ON TABLE menu_permissions IS
    'Izin yang membuka sebuah menu (semantik salah satu). Nol baris berarti menu tampil untuk semua pengguna terautentikasi (mis. beranda).';

-- 6. Seed grup bawaan per peran. Kode grup = 'ROLE_' || nama peran. Peran SYSTEM
-- tidak disimpan di tabel staf dan tidak punya grup.
INSERT INTO user_groups (code, name, description, is_system, approval_limit_role) VALUES
    ('ROLE_SUPERADMIN', 'Super Administrator',
     'Grup bawaan peran SUPERADMIN. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'SUPERADMIN'),
    ('ROLE_ADMIN', 'Administrator',
     'Grup bawaan peran ADMIN. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'ADMIN'),
    ('ROLE_SUPERVISOR', 'Supervisor',
     'Grup bawaan peran SUPERVISOR. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'SUPERVISOR'),
    ('ROLE_TELLER', 'Teller',
     'Grup bawaan peran TELLER. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'TELLER'),
    ('ROLE_CS', 'Customer Service',
     'Grup bawaan peran CS. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'CS'),
    ('ROLE_AO', 'Account Officer',
     'Grup bawaan peran AO. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'AO'),
    ('ROLE_AUDITOR', 'Auditor',
     'Grup bawaan peran AUDITOR. Direproduksi dari RolePermissions kode agar tidak ada pengguna yang kehilangan akses.', TRUE, 'AUDITOR')
ON CONFLICT (code) DO NOTHING;

-- 7. Seed izin grup bawaan: WAJIB sama dengan domain.RolePermissions.
-- Menambah/mengubah izin di kode WAJIB diikuti perubahan di sini (uji invarian
-- menegakkannya). Urutan tidak penting; himpunan yang dibandingkan.
INSERT INTO group_permissions (group_id, permission)
SELECT g.id, v.permission
FROM (VALUES
    -- SUPERADMIN
    ('ROLE_SUPERADMIN', 'products:read'),
    ('ROLE_SUPERADMIN', 'users:create'),
    ('ROLE_SUPERADMIN', 'users:read'),
    ('ROLE_SUPERADMIN', 'users:update'),
    ('ROLE_SUPERADMIN', 'users:delete'),
    ('ROLE_SUPERADMIN', 'branches:create'),
    ('ROLE_SUPERADMIN', 'customers:create'),
    ('ROLE_SUPERADMIN', 'customers:read'),
    ('ROLE_SUPERADMIN', 'customers:update'),
    ('ROLE_SUPERADMIN', 'accounts:open'),
    ('ROLE_SUPERADMIN', 'accounts:read'),
    ('ROLE_SUPERADMIN', 'accounts:freeze'),
    ('ROLE_SUPERADMIN', 'accounts:close'),
    ('ROLE_SUPERADMIN', 'transactions:deposit'),
    ('ROLE_SUPERADMIN', 'transactions:withdraw'),
    ('ROLE_SUPERADMIN', 'transactions:transfer'),
    ('ROLE_SUPERADMIN', 'transactions:reverse'),
    ('ROLE_SUPERADMIN', 'loans:apply'),
    ('ROLE_SUPERADMIN', 'loans:read'),
    ('ROLE_SUPERADMIN', 'loans:approve'),
    ('ROLE_SUPERADMIN', 'collections:input'),
    ('ROLE_SUPERADMIN', 'loans:cancel'),
    ('ROLE_SUPERADMIN', 'loans:correct'),
    ('ROLE_SUPERADMIN', 'loans:write_off'),
    ('ROLE_SUPERADMIN', 'loans:recover'),
    ('ROLE_SUPERADMIN', 'collateral:read'),
    ('ROLE_SUPERADMIN', 'collateral:manage'),
    ('ROLE_SUPERADMIN', 'maker_checker:approve'),
    ('ROLE_SUPERADMIN', 'maker_checker:reject'),
    ('ROLE_SUPERADMIN', 'ledger:read'),
    ('ROLE_SUPERADMIN', 'coa:manage'),
    ('ROLE_SUPERADMIN', 'audit_logs:read'),
    ('ROLE_SUPERADMIN', 'reports:export'),
    ('ROLE_SUPERADMIN', 'reports:financial:read'),
    ('ROLE_SUPERADMIN', 'permissions:manage'),
    ('ROLE_SUPERADMIN', 'system:config'),
    ('ROLE_SUPERADMIN', 'system:config:read'),
    -- ADMIN
    ('ROLE_ADMIN', 'products:read'),
    ('ROLE_ADMIN', 'users:create'),
    ('ROLE_ADMIN', 'users:read'),
    ('ROLE_ADMIN', 'users:update'),
    ('ROLE_ADMIN', 'customers:create'),
    ('ROLE_ADMIN', 'customers:read'),
    ('ROLE_ADMIN', 'customers:update'),
    ('ROLE_ADMIN', 'accounts:open'),
    ('ROLE_ADMIN', 'accounts:read'),
    ('ROLE_ADMIN', 'accounts:freeze'),
    ('ROLE_ADMIN', 'accounts:close'),
    ('ROLE_ADMIN', 'transactions:deposit'),
    ('ROLE_ADMIN', 'transactions:withdraw'),
    ('ROLE_ADMIN', 'transactions:transfer'),
    ('ROLE_ADMIN', 'transactions:reverse'),
    ('ROLE_ADMIN', 'loans:apply'),
    ('ROLE_ADMIN', 'loans:read'),
    ('ROLE_ADMIN', 'loans:approve'),
    ('ROLE_ADMIN', 'collections:input'),
    ('ROLE_ADMIN', 'loans:cancel'),
    ('ROLE_ADMIN', 'loans:correct'),
    ('ROLE_ADMIN', 'loans:write_off'),
    ('ROLE_ADMIN', 'loans:recover'),
    ('ROLE_ADMIN', 'collateral:read'),
    ('ROLE_ADMIN', 'collateral:manage'),
    ('ROLE_ADMIN', 'maker_checker:approve'),
    ('ROLE_ADMIN', 'maker_checker:reject'),
    ('ROLE_ADMIN', 'ledger:read'),
    ('ROLE_ADMIN', 'coa:manage'),
    ('ROLE_ADMIN', 'audit_logs:read'),
    ('ROLE_ADMIN', 'reports:export'),
    ('ROLE_ADMIN', 'reports:financial:read'),
    ('ROLE_ADMIN', 'permissions:manage'),
    ('ROLE_ADMIN', 'system:config:read'),
    -- SUPERVISOR
    ('ROLE_SUPERVISOR', 'products:read'),
    ('ROLE_SUPERVISOR', 'users:read'),
    ('ROLE_SUPERVISOR', 'customers:read'),
    ('ROLE_SUPERVISOR', 'customers:update'),
    ('ROLE_SUPERVISOR', 'accounts:read'),
    ('ROLE_SUPERVISOR', 'accounts:freeze'),
    ('ROLE_SUPERVISOR', 'transactions:reverse'),
    ('ROLE_SUPERVISOR', 'loans:read'),
    ('ROLE_SUPERVISOR', 'loans:approve'),
    ('ROLE_SUPERVISOR', 'loans:write_off'),
    ('ROLE_SUPERVISOR', 'collateral:read'),
    ('ROLE_SUPERVISOR', 'collateral:manage'),
    ('ROLE_SUPERVISOR', 'maker_checker:approve'),
    ('ROLE_SUPERVISOR', 'maker_checker:reject'),
    ('ROLE_SUPERVISOR', 'ledger:read'),
    ('ROLE_SUPERVISOR', 'audit_logs:read'),
    ('ROLE_SUPERVISOR', 'reports:export'),
    ('ROLE_SUPERVISOR', 'reports:financial:read'),
    ('ROLE_SUPERVISOR', 'system:config:read'),
    -- TELLER
    ('ROLE_TELLER', 'products:read'),
    ('ROLE_TELLER', 'customers:create'),
    ('ROLE_TELLER', 'customers:read'),
    ('ROLE_TELLER', 'accounts:open'),
    ('ROLE_TELLER', 'accounts:read'),
    ('ROLE_TELLER', 'transactions:deposit'),
    ('ROLE_TELLER', 'transactions:withdraw'),
    ('ROLE_TELLER', 'transactions:transfer'),
    ('ROLE_TELLER', 'collections:input'),
    ('ROLE_TELLER', 'loans:recover'),
    ('ROLE_TELLER', 'ledger:read'),
    -- CS
    ('ROLE_CS', 'products:read'),
    ('ROLE_CS', 'customers:create'),
    ('ROLE_CS', 'customers:read'),
    ('ROLE_CS', 'customers:update'),
    ('ROLE_CS', 'accounts:read'),
    ('ROLE_CS', 'transactions:deposit'),
    ('ROLE_CS', 'transactions:withdraw'),
    ('ROLE_CS', 'transactions:transfer'),
    ('ROLE_CS', 'ledger:read'),
    -- AO
    ('ROLE_AO', 'products:read'),
    ('ROLE_AO', 'customers:create'),
    ('ROLE_AO', 'customers:read'),
    ('ROLE_AO', 'customers:update'),
    ('ROLE_AO', 'accounts:read'),
    ('ROLE_AO', 'transactions:deposit'),
    ('ROLE_AO', 'transactions:withdraw'),
    ('ROLE_AO', 'transactions:transfer'),
    ('ROLE_AO', 'loans:apply'),
    ('ROLE_AO', 'loans:read'),
    ('ROLE_AO', 'collections:input'),
    ('ROLE_AO', 'loans:recover'),
    ('ROLE_AO', 'ledger:read'),
    -- AUDITOR
    ('ROLE_AUDITOR', 'products:read'),
    ('ROLE_AUDITOR', 'users:read'),
    ('ROLE_AUDITOR', 'customers:read'),
    ('ROLE_AUDITOR', 'accounts:read'),
    ('ROLE_AUDITOR', 'loans:read'),
    ('ROLE_AUDITOR', 'ledger:read'),
    ('ROLE_AUDITOR', 'audit_logs:read'),
    ('ROLE_AUDITOR', 'reports:export'),
    ('ROLE_AUDITOR', 'reports:financial:read'),
    ('ROLE_AUDITOR', 'system:config:read')
) AS v(code, permission)
JOIN user_groups g ON g.code = v.code
ON CONFLICT (group_id, permission) DO NOTHING;

-- 8. Keanggotaan awal: setiap pengguna lama dimasukkan ke grup perannya. Ini yang
-- membuat izin efektif pengguna lama TIDAK berubah. Pengguna baru ditambahkan ke
-- grup perannya oleh layanan staf; pengguna tanpa keanggotaan tetap memperoleh izin
-- grup ROLE_<perannya> lewat resolusi izin efektif, sehingga tidak ada yang terkunci.
INSERT INTO user_group_members (user_id, group_id)
SELECT u.id, g.id
FROM staff_users u
JOIN user_groups g ON g.code = 'ROLE_' || u.role::text
ON CONFLICT (user_id, group_id) DO NOTHING;

-- 9. Seed katalog menu. Kunci HARUS sama dengan menuKey yang dipakai web
-- (apps/web/src/components/layout/nav.ts). Menu tanpa izin di bawah = semua pengguna.
-- Hanya kunci dan keterangan yang disimpan: urutan/kelompok/ikon adalah tampilan
-- (dimiliki web), sedangkan penyaringan buku memakai cakupan instalasi yang sudah
-- dibaca server lewat /auth/me -> active_books. Menyimpan kolom yang tak dibaca
-- hanya akan menjadi data basi.
INSERT INTO menu_catalog (menu_key, description) VALUES
    ('beranda',      'Dasbor utama.'),
    ('teller',       'Layar teller.'),
    ('transaksi',    'Riwayat transaksi.'),
    ('deposito',     'Deposito berjangka.'),
    ('jatuh_tempo',  'Daftar jatuh tempo operasional.'),
    ('ppap',         'PPAP & kolektibilitas.'),
    ('tutup_hari',   'Tutup hari (EOD).'),
    ('kredit',       'Kredit konvensional.'),
    ('pembiayaan',   'Pembiayaan syariah.'),
    ('persetujuan',  'Antrean persetujuan maker-checker.'),
    ('nasabah',      'Data nasabah (CIF).'),
    ('rekening',     'Rekening nasabah.'),
    ('produk',       'Produk bank.'),
    ('cabang',       'Data cabang.'),
    ('buku_besar',   'Buku besar & jurnal.'),
    ('laporan',      'Laporan.'),
    ('pemetaan_ojk', 'Pemetaan laporan OJK.'),
    ('pengaturan',   'Pengaturan sistem.')
ON CONFLICT (menu_key) DO NOTHING;

-- 10. Seed pemetaan menu -> izin, mereproduksi tampilan menu yang berlaku sekarang
-- (konstanta peran di nav.ts) memakai izin nyata, bukan salinan peran. Menu tanpa
-- baris di bawah sengaja tampil untuk semua pengguna terautentikasi.
INSERT INTO menu_permissions (menu_key, permission) VALUES
    -- teller: layar kasir yang memposting setoran/penarikan/transfer.
    ('teller', 'transactions:deposit'),
    ('teller', 'transactions:withdraw'),
    ('teller', 'transactions:transfer'),
    -- transaksi & jatuh tempo: memakai ledger:read yang memang menjaga endpointnya.
    ('transaksi', 'ledger:read'),
    ('jatuh_tempo', 'ledger:read'),
    -- ppap, kredit, pembiayaan: daftar kredit/pembiayaan dijaga loans:read.
    ('ppap', 'loans:read'),
    ('kredit', 'loans:read'),
    ('pembiayaan', 'loans:read'),
    -- tutup hari: aksi EOD dijaga system:config (hanya SUPERADMIN).
    ('tutup_hari', 'system:config'),
    -- persetujuan: antrean maker-checker.
    ('persetujuan', 'maker_checker:approve'),
    -- cabang: GET /branches dijaga users:read.
    ('cabang', 'users:read'),
    -- buku besar & laporan: dijaga ledger:read.
    ('buku_besar', 'ledger:read'),
    ('laporan', 'ledger:read'),
    -- pemetaan OJK: definisi/ekspor laporan OJK dijaga reports:export.
    ('pemetaan_ojk', 'reports:export')
ON CONFLICT (menu_key, permission) DO NOTHING;
