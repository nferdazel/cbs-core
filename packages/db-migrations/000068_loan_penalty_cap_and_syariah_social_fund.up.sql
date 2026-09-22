-- 000068_loan_penalty_cap_and_syariah_social_fund.up.sql
-- Run after: 000067_ckpn_parameter_units.up.sql
-- Idempotent: aman dijalankan ulang, tidak menimpa nilai yang sudah disesuaikan
-- operator.
--
-- Dua kunci konfigurasi baru untuk akrual denda kredit:
--
-- 1. loan.penalty.cap.percent
--    Plafon denda PER KREDIT, dalam persen dari pokok angsuran yang tertunggak saat
--    itu (basis sama dengan denda: loans.penalty_accrued dibandingkan dengan
--    SUM(GREATEST(principal_amount - paid_principal, 0)) atas angsuran lewat jatuh
--    tempo). Tanpa plafon, tunggakan berbulan-bulan (DPD ratusan) membuat denda
--    melewati kewajiban pokoknya. Bawaan aplikasi 10%; 0 = plafon dimatikan. Bank
--    harus meninjau angka ini terhadap kebijakan internalnya.
--
-- 2. loan.penalty.syariah.social_fund.coa
--    Akun kewajiban tempat denda pembiayaan syariah (ta'zir) diakui, BUKAN akun
--    pendapatan. Fatwa DSN-MUI No. 17/DSN-MUI/IX/2000 menetapkan dana denda sebagai
--    dana sosial; migrasi 000024 sudah menyediakan 12500 "Dana Kebajikan"
--    (LIABILITY, CREDIT, buku SYARIAH) dan memetakan LOAN_PENALTY syariah ke sana
--    (debit 11700 Piutang Denda Syariah / kredit 12500). Kode menolak akrual denda
--    syariah bila pemetaan mengkredit akun selain nilai kunci ini. Ini sikap
--    sementara sampai Dewan Pengawas Syariah menetapkan perlakuan resminya.

INSERT INTO system_config (key, value, description) VALUES
    ('loan.penalty.cap.percent', '10', 'Plafon denda per kredit dalam persen dari pokok tunggakan (0 = tanpa plafon). Bawaan aplikasi 10%; bank harus meninjau terhadap kebijakan internal.'),
    ('loan.penalty.syariah.social_fund.coa', '12500', 'Kode COA kewajiban Dana Kebajikan tempat denda pembiayaan syariah (ta''zir) diakui, bukan pendapatan. Bawaan 12500 Dana Kebajikan (migrasi 000024). Sikap sementara menunggu keputusan DPS.')
ON CONFLICT (key) DO NOTHING;
