-- 000071_ckpn_coa_syariah.up.sql
-- Run after: 000070_single_head_office.up.sql
--
-- Menambahkan kunci COA penjurnalan CKPN khusus unit syariah:
--   - ckpn.coa.expense.syariah
--   - ckpn.coa.reserve.syariah
--
-- Latar: kunci ckpn.coa.expense/ckpn.coa.reserve (migrasi 000069) bersifat GLOBAL
-- dan berisi akun buku KONVENSIONAL (50301/10950). Selama kunci itu terisi, jurnal
-- CKPN pembiayaan syariah ikut memakai akun konvensional meskipun produknya
-- berbook SYARIAH, sehingga dana UUS tercampur buku konvensional. Kunci baru ini
-- memberi pemetaan per unit usaha tanpa mengubah kunci global.
--
-- Nilai KOSONG berarti: ikut kunci global ckpn.coa.expense/ckpn.coa.reserve. Bank
-- yang belum mengisi kunci syariah karena itu tidak mengalami perubahan perilaku
-- sama sekali — kredit syariah tetap memakai kunci global (lalu fallback per buku
-- bila kunci global juga kosong). Hanya setelah bank mengisi kode akun syariah,
-- kredit berbook SYARIAH memakai akun itu.
--
-- Idempotent: aman dijalankan ulang. Nilai hanya diisi bila masih KOSONG, sehingga
-- pemetaan yang sudah disesuaikan operator TIDAK ditimpa.

INSERT INTO system_config (key, value, description) VALUES
    ('ckpn.coa.expense.syariah', '',
     'Kode COA beban kerugian penurunan nilai CKPN untuk unit syariah (sisi debit pembentukan; sisi kredit pemulihan). KOSONG = mengikuti kunci global ckpn.coa.expense (lalu fallback 15901 bila kunci global juga kosong). Diisi hanya bila bank ingin akun beban CKPN pembiayaan berbeda dari akun konvensional.'),
    ('ckpn.coa.reserve.syariah', '',
     'Kode COA cadangan CKPN untuk unit syariah (sisi kredit pembentukan; sisi debit pemulihan). KOSONG = mengikuti kunci global ckpn.coa.reserve (lalu fallback 11950 bila kunci global juga kosong). Diisi hanya bila bank ingin akun cadangan CKPN pembiayaan berbeda dari akun konvensional.')
ON CONFLICT (key) DO UPDATE
    SET value = EXCLUDED.value,
        description = EXCLUDED.description
    WHERE system_config.value = '';
