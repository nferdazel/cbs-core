-- 000089_staff_employee_id_sequence.up.sql
-- Run after: 000088_widen_org_unit_code.up.sql
--
-- ALASAN (temuan E2E N1): employee_id dibentuk di service dari
-- time.Now().Nanosecond()%100000, sehingga dua pembuatan staf dapat memperoleh nomor
-- yang sama. Unique constraint staff_users.employee_id lalu menolaknya sebagai 500,
-- bukan pesan jelas. Nomor pegawai sekarang diambil dari sequence database (nextval
-- atomik), sehingga tidak mudah bentrok bahkah pada pembuatan bersamaan.
--
-- Sequence di-seed di atas nomor numerik terbesar yang sudah terpakai agar nomor lama
-- tidak dipakai ulang. Baris dengan format employee_id lain (mis. 'EMP-UJI-...')
-- diabaikan; format produksi adalah 'EMP-<tahun>-<nomor>'.
--
-- Sifat: aditif dan IDEMPOTENT (IF NOT EXISTS + setval). Tidak mengubah data lama,
-- angka akuntansi, jurnal, maupun ckpn.enabled.

CREATE SEQUENCE IF NOT EXISTS staff_employee_id_seq;

SELECT setval(
    'staff_employee_id_seq',
    COALESCE((
        SELECT MAX((regexp_replace(employee_id, '^EMP-[0-9]{4}-', ''))::bigint)
        FROM staff_users
        WHERE employee_id ~ '^EMP-[0-9]{4}-[0-9]+$'
    ), 0) + 1,
    false
);
