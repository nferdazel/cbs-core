-- 000051_limits_and_config.up.sql
-- Menyatukan batas transaksi pada kunci yang benar-benar dibaca kode dan melengkapi
-- kunci konfigurasi yang dibaca kode tetapi belum pernah di-seed.
--
-- ALASAN (masalah 1): internal/service/limit_service.go membaca
--   limit.<peran>.<jenis_transaksi>.per_transaction|daily|approval_above
-- sedangkan migrasi 000002 hanya meng-seed role_limit.<PERAN>.per_transaction. Kunci
-- limit.* tidak pernah ada, sehingga penjaga batas selalu jatuh ke nilai bawaan
-- (50 juta per transaksi, 500 juta harian, 10 juta ambang persetujuan) untuk SEMUA
-- peran. Niat yang tertulis di seed lama (TELLER 50 juta, AO 250 juta, SUPERVISOR
-- 500 juta, ADMIN tanpa batas) tidak pernah berlaku, dan ambang persetujuan 10 juta
-- menimpa ambang maker_checker.* yang di-seed.
--
-- Keputusan pemilik sistem: pindahkan niat role_limit.* ke kunci limit.* yang benar
-- per jenis transaksi, lalu hapus role_limit.*. Untuk peran/jenis transaksi yang
-- niatnya TIDAK tertulis di seed lama, nilainya disamakan dengan bawaan sekarang
-- (supaya perilaku tidak berubah di luar niat tertulis) dan deskripsinya menandai
-- bahwa bank harus meninjau. Ambang persetujuan disamakan dengan
-- maker_checker.<jenis>.threshold agar kedua mekanisme tidak saling menimpa.
--
-- ALASAN (masalah 2): 18 kunci dibaca kode (GetDecimal/GetInt/GetString) tetapi tidak
-- ada di seed. Dua di antaranya jatuh ke nol secara senyap:
--   deposit.mudharabah.yield_annual (bagi hasil nol) dan
--   loan.penalty.rate.daily.per_mille (denda nol).
-- Kuncinya di-seed eksplisit; yang nilainya masih perlu ditinjau bank disebutkan pada
-- deskripsi. Kunci yang kosong ('') berarti "pakai nilai produk/fallback per buku",
-- bukan nol.
--
-- Aditif dan idempotent: aman dijalankan ulang, tidak menimpa nilai yang sudah
-- disesuaikan operator.

-- ── 1. Buang kunci lama yang tidak dibaca kode ────────────────────────────────
-- role_limit.* tidak pernah dibaca service mana pun (hanya limit.* yang dibaca
-- limit_service.go). Nilainya sudah dipindahkan ke limit.* di bawah.
DELETE FROM system_config WHERE key LIKE 'role_limit.%';

-- ── 2. Batas transaksi per peran dan jenis transaksi (kunci limit.*) ──────────
-- Jenis transaksi mengikuti tiga panggilan guardLimit di ledger_service.go:
-- deposit, withdrawal, transfer. Peran mengikuti seluruh staff_role.
-- per_transaction
INSERT INTO system_config (key, value, description) VALUES
    ('limit.superadmin.deposit.per_transaction', '50000000', 'Batas per transaksi SUPERADMIN untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.superadmin.withdrawal.per_transaction', '50000000', 'Batas per transaksi SUPERADMIN untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.superadmin.transfer.per_transaction', '50000000', 'Batas per transaksi SUPERADMIN untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.admin.deposit.per_transaction', '0', 'Batas per transaksi ADMIN untuk DEPOSIT (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.ADMIN.per_transaction.'),
    ('limit.admin.withdrawal.per_transaction', '0', 'Batas per transaksi ADMIN untuk WITHDRAWAL (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.ADMIN.per_transaction.'),
    ('limit.admin.transfer.per_transaction', '0', 'Batas per transaksi ADMIN untuk TRANSFER (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.ADMIN.per_transaction.'),
    ('limit.supervisor.deposit.per_transaction', '500000000', 'Batas per transaksi SUPERVISOR untuk DEPOSIT (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.SUPERVISOR.per_transaction.'),
    ('limit.supervisor.withdrawal.per_transaction', '500000000', 'Batas per transaksi SUPERVISOR untuk WITHDRAWAL (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.SUPERVISOR.per_transaction.'),
    ('limit.supervisor.transfer.per_transaction', '500000000', 'Batas per transaksi SUPERVISOR untuk TRANSFER (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.SUPERVISOR.per_transaction.'),
    ('limit.teller.deposit.per_transaction', '50000000', 'Batas per transaksi TELLER untuk DEPOSIT (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.TELLER.per_transaction.'),
    ('limit.teller.withdrawal.per_transaction', '50000000', 'Batas per transaksi TELLER untuk WITHDRAWAL (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.TELLER.per_transaction.'),
    ('limit.teller.transfer.per_transaction', '50000000', 'Batas per transaksi TELLER untuk TRANSFER (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.TELLER.per_transaction.'),
    ('limit.cs.deposit.per_transaction', '50000000', 'Batas per transaksi CS untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.cs.withdrawal.per_transaction', '50000000', 'Batas per transaksi CS untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.cs.transfer.per_transaction', '50000000', 'Batas per transaksi CS untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.ao.deposit.per_transaction', '250000000', 'Batas per transaksi AO untuk DEPOSIT (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.AO.per_transaction.'),
    ('limit.ao.withdrawal.per_transaction', '250000000', 'Batas per transaksi AO untuk WITHDRAWAL (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.AO.per_transaction.'),
    ('limit.ao.transfer.per_transaction', '250000000', 'Batas per transaksi AO untuk TRANSFER (IDR; 0 = tanpa batas). Dipindahkan dari role_limit.AO.per_transaction.'),
    ('limit.auditor.deposit.per_transaction', '50000000', 'Batas per transaksi AUDITOR untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.auditor.withdrawal.per_transaction', '50000000', 'Batas per transaksi AUDITOR untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.'),
    ('limit.auditor.transfer.per_transaction', '50000000', 'Batas per transaksi AUDITOR untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (50 juta). Bank harus meninjau.')
ON CONFLICT (key) DO NOTHING;

-- daily
INSERT INTO system_config (key, value, description) VALUES
    ('limit.superadmin.deposit.daily', '500000000', 'Batas akumulasi harian SUPERADMIN untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.superadmin.withdrawal.daily', '500000000', 'Batas akumulasi harian SUPERADMIN untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.superadmin.transfer.daily', '500000000', 'Batas akumulasi harian SUPERADMIN untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.admin.deposit.daily', '500000000', 'Batas akumulasi harian ADMIN untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.admin.withdrawal.daily', '500000000', 'Batas akumulasi harian ADMIN untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.admin.transfer.daily', '500000000', 'Batas akumulasi harian ADMIN untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.supervisor.deposit.daily', '500000000', 'Batas akumulasi harian SUPERVISOR untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.supervisor.withdrawal.daily', '500000000', 'Batas akumulasi harian SUPERVISOR untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.supervisor.transfer.daily', '500000000', 'Batas akumulasi harian SUPERVISOR untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.teller.deposit.daily', '500000000', 'Batas akumulasi harian TELLER untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.teller.withdrawal.daily', '500000000', 'Batas akumulasi harian TELLER untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.teller.transfer.daily', '500000000', 'Batas akumulasi harian TELLER untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.cs.deposit.daily', '500000000', 'Batas akumulasi harian CS untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.cs.withdrawal.daily', '500000000', 'Batas akumulasi harian CS untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.cs.transfer.daily', '500000000', 'Batas akumulasi harian CS untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.ao.deposit.daily', '500000000', 'Batas akumulasi harian AO untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.ao.withdrawal.daily', '500000000', 'Batas akumulasi harian AO untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.ao.transfer.daily', '500000000', 'Batas akumulasi harian AO untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.auditor.deposit.daily', '500000000', 'Batas akumulasi harian AUDITOR untuk DEPOSIT (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.auditor.withdrawal.daily', '500000000', 'Batas akumulasi harian AUDITOR untuk WITHDRAWAL (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.'),
    ('limit.auditor.transfer.daily', '500000000', 'Batas akumulasi harian AUDITOR untuk TRANSFER (IDR; 0 = tanpa batas). Tidak ada niat tertulis di seed lama; nilai = bawaan lama (500 juta). Bank harus meninjau.')
ON CONFLICT (key) DO NOTHING;

-- approval_above
INSERT INTO system_config (key, value, description) VALUES
    ('limit.superadmin.deposit.approval_above', '100000000', 'Ambang persetujuan SUPERADMIN untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.superadmin.withdrawal.approval_above', '50000000', 'Ambang persetujuan SUPERADMIN untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.superadmin.transfer.approval_above', '50000000', 'Ambang persetujuan SUPERADMIN untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.admin.deposit.approval_above', '100000000', 'Ambang persetujuan ADMIN untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.admin.withdrawal.approval_above', '50000000', 'Ambang persetujuan ADMIN untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.admin.transfer.approval_above', '50000000', 'Ambang persetujuan ADMIN untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.supervisor.deposit.approval_above', '100000000', 'Ambang persetujuan SUPERVISOR untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.supervisor.withdrawal.approval_above', '50000000', 'Ambang persetujuan SUPERVISOR untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.supervisor.transfer.approval_above', '50000000', 'Ambang persetujuan SUPERVISOR untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.teller.deposit.approval_above', '100000000', 'Ambang persetujuan TELLER untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.teller.withdrawal.approval_above', '50000000', 'Ambang persetujuan TELLER untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.teller.transfer.approval_above', '50000000', 'Ambang persetujuan TELLER untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.cs.deposit.approval_above', '100000000', 'Ambang persetujuan CS untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.cs.withdrawal.approval_above', '50000000', 'Ambang persetujuan CS untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.cs.transfer.approval_above', '50000000', 'Ambang persetujuan CS untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.ao.deposit.approval_above', '100000000', 'Ambang persetujuan AO untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.ao.withdrawal.approval_above', '50000000', 'Ambang persetujuan AO untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.ao.transfer.approval_above', '50000000', 'Ambang persetujuan AO untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.auditor.deposit.approval_above', '100000000', 'Ambang persetujuan AUDITOR untuk DEPOSIT (IDR). Disamakan dengan maker_checker.deposit.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.auditor.withdrawal.approval_above', '50000000', 'Ambang persetujuan AUDITOR untuk WITHDRAWAL (IDR). Disamakan dengan maker_checker.withdrawal.threshold agar tidak menimpa ambang maker-checker.'),
    ('limit.auditor.transfer.approval_above', '50000000', 'Ambang persetujuan AUDITOR untuk TRANSFER (IDR). Disamakan dengan maker_checker.transfer.threshold agar tidak menimpa ambang maker-checker.')
ON CONFLICT (key) DO NOTHING;

-- ── 3. Kunci konfigurasi yang dibaca kode tetapi belum di-seed ────────────────
-- Nilai = bawaan aplikasi saat ini agar perilaku tidak berubah; deskripsi menandai
-- kunci yang wajib ditinjau bank. Kunci ber-nilai '' berarti fallback (produk/per buku).
INSERT INTO system_config (key, value, description) VALUES
    ('account.dormant.after_months', '12', 'Ambang bulan tanpa aktivitas sebelum rekening ditandai DORMANT. Nilai = bawaan aplikasi (12); bank harus meninjau kebijakan dormantnya.'),
    ('cash.coa.conventional', '10101', 'Kode COA kas teller buku konvensional sebagai lawan jurnal transaksi rekening. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('cash.coa.syariah', '11100', 'Kode COA kas buku syariah sebagai lawan jurnal transaksi rekening. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('deposit.mudharabah.yield_annual', '0', 'Proyeksi imbal hasil tahunan deposito bagi hasil (desimal, mis. 0.06 untuk 6% per tahun). Nilai 0 = bagi hasil nol; bank WAJIB meninjau dan mengisi tarif proyeksinya.'),
    ('fee.admin.income.coa.conventional', '40400', 'Kode COA pendapatan administrasi buku konvensional. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('fee.admin.income.coa.syariah', '14500', 'Kode COA pendapatan administrasi buku syariah. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('fee.admin.monthly', '', 'Biaya administrasi bulanan global (IDR). Kosong = pakai admin_fee produk tiap rekening; isi untuk menimpa. Bank wajib meninjau kebijakannya.'),
    ('loan.penalty.rate.daily.per_mille', '0', 'Tarif denda harian per mil (‰) dari pokok tunggakan. 1 = 0,1% per hari; 0 = tidak ada denda diakru. Bank WAJIB meninjau dan mengisi tarifnya.'),
    ('ppap.coa.expense', '', 'Override COA beban PPAP. Kosong = fallback per buku: 50200 konvensional, 15200 syariah. Bank harus meninjau pemetaannya.'),
    ('ppap.coa.reserve', '', 'Override COA cadangan PPAP. Kosong = fallback per buku: 10900 konvensional, 11900 syariah. Bank harus meninjau pemetaannya.'),
    ('retained_earnings.coa.conventional', '30200', 'Kode COA laba ditahan buku konvensional untuk penutupan tahun. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('retained_earnings.coa.syariah', '13200', 'Kode COA laba ditahan buku syariah untuk penutupan UUS. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('savings.interest.expense.coa.conventional', '50100', 'Kode COA beban bunga tabungan buku konvensional. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('savings.interest.expense.coa.syariah', '15100', 'Kode COA bagi hasil tabungan buku syariah. Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('savings.interest.payable.coa.conventional', '20400', 'Kode COA bunga tabungan yang masih harus dibayar (konvensional). Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('savings.interest.payable.coa.syariah', '12400', 'Kode COA bagi hasil tabungan yang masih harus dibayar (syariah). Nilai = bawaan aplikasi; bank harus meninjau bagan akunnya.'),
    ('savings.interest.rate_annual', '', 'Suku bunga tabungan tahunan global dalam persen. Kosong = pakai rate_annual produk tiap rekening; isi untuk menimpa. Bank harus meninjau kebijakannya.'),
    ('savings.mudharabah.distributable_profit', '0', 'Laba yang didistribusikan untuk bagi hasil tabungan mudharabah per periode (IDR). 0 = bagi hasil nihil. Bank WAJIB meninjau dan mengisi porsi labanya.')
ON CONFLICT (key) DO NOTHING;

-- ── 4. Ambang maker-checker untuk aksi yang dibaca kode tetapi belum di-seed ──
-- maker_checker_service.Threshold() membangun kunci maker_checker.<aksi>.threshold
-- untuk SETIAP aksi yang mengajukan persetujuan, termasuk REVERSE_TRANSACTION dan
-- LOAN_CORRECTION. Keduanya belum di-seed sehingga jatuh ke bawaan 50 juta.
-- Nilainya = bawaan sekarang; bank harus meninjau.
INSERT INTO system_config (key, value, description) VALUES
    ('maker_checker.reverse_transaction.threshold', '50000000', 'Ambang nominal (IDR) yang mewajibkan persetujuan pejabat kedua untuk pembatalan transaksi lintas hari. Nilai = bawaan aplikasi (50 juta); bank harus meninjau.'),
    ('maker_checker.loan_correction.threshold', '50000000', 'Ambang nominal (IDR) yang mewajibkan persetujuan pejabat kedua untuk koreksi nominal kredit. Nilai = bawaan aplikasi (50 juta); bank harus meninjau.')
ON CONFLICT (key) DO NOTHING;

-- Catatan: auth.password_expiry_days TIDAK dihapus. Kunci itu kontrol keamanan
-- (kedaluwarsa password) yang seharusnya ada. Penegakannya belum diimplementasikan
-- di jalur login karena memaksa penggantian tanpa jalur ganti-password mandiri akan
-- mengunci pengguna; bukan bagian dari migrasi ini.
