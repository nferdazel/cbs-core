# PENAMAAN-TABEL.md — kamus penamaan tabel

Dokumen ini berlaku untuk **tabel baru**. Isinya disusun dari nama tabel yang
benar-benar ada di `packages/db-migrations` (`CREATE TABLE`), bukan dari cita-cita
skema. Tujuannya satu: tabel baru konsisten dengan yang lama, sehingga query,
repo, dan laporan tidak perlu memetakan dua konvensi.

## 1. Aturan umum

- `snake_case`, bahasa Inggris, **plural** untuk tabel yang menyimpan banyak baris
  (`accounts`, `loans`, `journal_lines`, `eod_step_runs`).
- Tanpa prefix `tbl_`, tanpa camelCase, tanpa singkatan ganda.
- Kunci asing: `<nama_entitas_tunggal>_id` (mis. `product_id`, `branch_id`,
  `journal_entry_id`).
- Kolom waktu: `<peristiwa>_at` (`created_at`, `posted_at`, `dormant_at`);
  kolom boolean: `is_*` (`is_header`, `is_restructured`, `is_head_office`).
- Singkatan hanya yang sudah mapan di repo: `coa` (chart of accounts), `eod`,
  `eom`, `eoy`, `ojk`, `ckpn`, `lps`, `njop`, `aro`, `cif`.
- Migrasi bersifat aditif dan idempoten: tabel baru memakai
  `CREATE TABLE IF NOT EXISTS`, indeks `CREATE INDEX IF NOT EXISTS`, dan ditulis di
  berkas migrasi baru berurutan (`packages/db-migrations/0000NN_*.up.sql`) beserta
  pasangan `.down.sql`. Jangan mengubah migrasi yang sudah dirilis.
- Nama indeks baru: `idx_<tabel>_<tujuan>` atau `idx_<singkatan>_<kolom>`
  (mis. `idx_deposits_maturity`, `idx_journal_idempotency`).

## 2. Kamus per kelompok

Kelompok di bawah adalah pengelompokan domain yang dipakai repo, bukan skema
fisik. Tabel boleh muncul di satu kelompok saja.

### Aktor (staf, akses, grup)

| Tabel | Dipakai untuk |
|---|---|
| `staff_users` | akun pegawai bank (peran, cabang, buku) |
| `staff_sessions` | sesi login dan token |
| `branches` | cabang dan unit organisasi (jenjang lewat `parent_id`) |
| `user_groups` | grup pengguna untuk menu dan tiering persetujuan |
| `user_group_members` | keanggotaan staf di grup (tabel relasi) |
| `group_permissions` | izin yang dipegang sebuah grup |
| `menu_catalog` | katalog menu yang dapat diberikan |
| `menu_permissions` | izin yang dibutuhkan sebuah menu |
| `customers` | nasabah/CIF. Bukan staf, tetapi mengikuti pola identitas yang sama |

Pola baru: relasi aktor-ke-aktor memakai bentuk `<entitas>_<tujuan>`
(`user_group_members`, `group_permissions`), bukan tabel `*_mapping`.

### Produk (data referensi)

| Tabel | Dipakai untuk |
|---|---|
| `banking_products` | produk simpanan, deposito, kredit, dan pembiayaan (termasuk `book`) |
| `product_journal_mapping` | pemetaan event produk ke kode COA |
| `collateral_lampiran_ii_weights` | bobot agunan per jenis (Lampiran II SEOJK) |

Pola baru: data referensi yang dipakai lintas modul memakai nama domainnya
langsung (`banking_products`) atau sufiks `_mapping` bila isinya pemetaan
(`product_journal_mapping`). Parameter yang hari ini menumpang `banking_products`
tidak dipecah tanpa migrasi terkendali.

### Kredit & pembiayaan

| Tabel | Dipakai untuk |
|---|---|
| `loans` | pokok kredit/pembiayaan dan statusnya |
| `loan_schedules` | jadwal angsuran (pokok, margin/bunga, denda) |
| `loan_collaterals` | agunan kredit |
| `loan_ckpn_individual_assessments` | asesmen CKPN individual (T0 panel) |
| `loan_cashflow_projections` | proyeksi arus kas CKPN individual |

Pola baru: seluruh tabel subdomain kredit memakai prefix `loan_`
(`loan_schedules`, `loan_collaterals`), kecuali tabel induk `loans`. Tabel
pendukung CKPN individual mengikuti prefix `loan_` (mis. `loan_ckpn_*`), bukan
prefix `ckpn_` tingkat modul.

### Simpanan (tabungan & deposito)

| Tabel | Dipakai untuk |
|---|---|
| `time_deposits` | deposito berjangka (penempatan, ARO, jatuh tempo). Sebelumnya bernama `deposits`; di-rename migrasi 000101 karena nama lama ambigu (giro dan tabungan tersimpan di `accounts`). Indeks lama `idx_deposits_*` dipertahankan. |
| `interest_accruals` | penanda & jejak akrual bunga/bagi hasil per rekening per periode |
| `admin_fee_charges` | penanda & jejak potongan biaya administrasi per rekening per periode |
| `account_number_sequences` | penomoran rekening per produk/cabang |

Pola baru: prefix domain tidak dipakai bila entitasnya sudah khas
(`time_deposits`). Tabel bantu penomoran memakai sufiks `_sequences`.

### Akuntansi

| Tabel | Dipakai untuk |
|---|---|
| `accounts` | rekening buku besar yang dapat diposting (rekening nasabah & akun GL internal) |
| `chart_of_accounts` | bagan akun (kode, tipe, `book`) |
| `journal_entries` | kepala jurnal (referensi, tanggal bisnis, status, cabang) |
| `journal_lines` | baris jurnal (akun, arah, nominal, saldo setelah) |
| `idempotency_keys` | kunci idempotensi jurnal |
| `year_end_closings` | penutupan buku akhir tahun per buku |
| `lps_placements` | penempatan LPS (akuntansi dan pelaporan) |

Pola baru: entri dan baris selalu dipisah (`journal_entries` / `journal_lines`);
jangan menyimpan baris jurnal di JSON. Tabel yang menyimpan jejak pemrosesan
memakai sufiks `_closings`/`_placements`, bukan prefix modul.

### Operasional (EOD, laporan, penomoran)

| Tabel | Dipakai untuk |
|---|---|
| `eod_step_definitions` | definisi urutan langkah EOD (konfigurasi) |
| `eod_step_runs` | riwayat eksekusi tiap langkah (hasil) |
| `eod_triggers` | pemicu jadwal EOD |
| `ojk_mapping_reviews` | keputusan pemetaan COA ke laporan OJK |

Pola baru: pasangan **definisi vs eksekusi** memakai `_definitions` /
`_runs` (bukan `_config` / `_history`). Semua tabel subdomain EOD memakai
prefix `eod_`.

### Konfigurasi & audit

| Tabel | Dipakai untuk |
|---|---|
| `system_config` | konfigurasi sistem berkunci (`key`/`value`) |
| `bank_profile` | identitas bank tingkat instalasi |
| `audit_logs` | jejak audit (pembuat, aksi, resource, payload) |
| `customer_name_tokens` | token pencarian nama nasabah (privasi + pencarian) |

Pola baru: konfigurasi global memakai bentuk **tunggal** (`system_config`,
`bank_profile`) karena hanya ada satu baris logis. Tabel jejak memakai plural
(`audit_logs`) atau sufiks `_tokens`/`_reviews`.

### Maker-checker (persetujuan)

| Tabel | Dipakai untuk |
|---|---|
| `maker_checker_requests` | pengajuan yang menunggu persetujuan pejabat kedua |

Pola baru: satu tabel pengajuan generik dengan kolom `action_type` dan `payload`
(JSON), bukan tabel persetujuan per fitur.

## 3. Catatan tentang rename

Rename menyeluruh **tidak layak** dan tidak akan dikerjakan:

- Nama tabel muncul di **164 berkas** (audit putaran ini): repo, service, handler,
  uji, dan SQL mentah. Rename massal berarti mengubah keempat lapisan sekaligus
  dalam satu langkah tanpa nilai bisnis.
- Rename tertarget hanya untuk yang benar-benar **menyesatkan** (mis. nama yang
  menyiratkan entitas lain), dan dilakukan lewat migrasi terkendali:
  `ALTER TABLE ... RENAME TO ...` disertai `RENAME` indeks/kendala, sinkronisasi
  kode di satu PR, dan verifikasi integrasi. Tanpa satu di antaranya, rename
  ditunda.
- Penamaan tabel baru tidak boleh menambah utang: selama tidak ada alasan kuat,
  ikuti kamus di atas apa adanya.
