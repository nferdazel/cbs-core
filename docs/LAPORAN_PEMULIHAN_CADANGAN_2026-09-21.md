# Laporan Latihan Pemulihan Cadangan `cbs`

- **Tanggal latihan**: 2026-09-21 (WIB, VPS produksi; alamat dan nama host sengaja tidak ditulis karena repositori ini publik)
- **Lingkup**: dump `cbs` → pulihkan ke target terpisah → verifikasi → bersihkan.
- **Batasan yang dipatuhi**: tidak ada perubahan isi `cbs`, tidak ada migrasi dijalankan,
  tidak ada container produksi di-restart. Hanya baca `cbs` + target latihan sementara.

---

## 1. Postur cadangan SEBELUM latihan (apa adanya)

**Ada cadangan otomatis, dan berjalan.** Sumber: `systemd` timer
`qouver-postgres-backup.timer` (daily 02:00), eksekutor
`/srv/qouver/scripts/backup-postgres.sh`.

- Skrip mem-backup **semua** database non-template di container `qouver-postgres`
  (enumerasi dinamis, tanpa hardcode), termasuk `cbs`.
- Format `pg_dump -Fc` (custom), **divalidasi** dengan `pg_restore --list` setelah
  ditulis; dump kosong/gagal dihapus dan run di-fail (`fail()`).
- Rotasi: daily 7 hari, weekly 28 hari, monthly 93 hari.
- Notifikasi: Uptime Kuma push + Telegram (`tg-notify.sh`).
- Lokasi: `/srv/qouver/backups/postgres/{daily,weekly,monthly}` (+ `globals` per hari).
- Status run terakhir: **sukses**, `Result=success`, `ExecMainStatus=0`
  (journal 2026-09-21 02:00:31, "Backup completed successfully").

**Cadangan `cbs` yang ada pada saat itu:**

| Lokasi | Berkas `cbs` | Keterangan |
|---|---|---|
| daily | `2026-09-14` s.d. `2026-09-21` (8 berkas) | 45.882 B (14–18 Sep) naik ke 107.405 B (21 Sep) |
| weekly | `2026-09-06`, `2026-09-13`, `2026-09-20` | 45.882 B / 45.882 B / 98.499 B |
| monthly | **tidak ada `cbs`** | dump monthly 2026-09-01 hanya memuat `bm`, `bm_dev`, `postgres`, `qouver`, `sds`, `kuma` |

Artinya `cbs` baru ikut tercadang sekitar awal September; belum ada salinan
monthly untuk `cbs` (monthly berikutnya 1 Oktober). Berkas cadangan `cbs` sangat
kecil karena database memang masih ramping (11 MB; 26 tabel; 72 `accounts`; 0
jurnal/kredit).

**Celah postur yang terlihat:**
1. **Tidak ada salinan offsite.** Seluruh dump ada di disk/host yang **sama**
   dengan database (`/dev/vda1`). Kerusakan disk/host menghapus keduanya.
   Tidak ada `rclone`/`borg`/`restic`/upload di `/srv/qouver/scripts`.
2. **Belum pernah ada bukti restore diuji.** Tidak ditemukan log/laporan latihan
   pemulihan sebelumnya; validasi `pg_restore --list` hanya memeriksa struktur
   dump, bukan pemulihan nyata.
3. Cakupan retensi `cbs` masih pendek (tertua 6 Sep) karena DB baru ditambahkan.
4. `globals` (role) dicadangkan di `daily/`; namun ia **bukan** bagian dump `cbs`,
   sehingga pemulihan ke instance baru tetap butuh `globals_*.sql` agar role
   `qouver`/`cbs_app` ada sebelum restore.

---

## 2. Cara pemulihan yang dipilih dan alasannya

**Dipilih: database baru `cbs_restore_drill` di instance yang sama** (bukan skema
baru di dalam `cbs`).

Alasan memilih database terpisah, bukan skema di dalam `cbs`:

- Isolasi penuh: `pg_restore` tidak pernah menyentuh `cbs`; tidak ada objek `public`
  yang bisa tertimpa dan tidak ada perubahan `search_path`/GRANT yang dapat
  mengubah perilaku kueri `cbs`.
- `cbs` hanya punya satu skema pengguna (`public`). Menaruh hasil restore ke skema
  lain di `cbs` menuntut remap skema yang tidak dilakukan `pg_restore` secara
  aman, sehingga justru lebih berisiko.
- Target mudah dibersihkan dan diverifikasi (`createdb`/`dropdb`).

**Dump yang dipakai: whole-database (`pg_dump -Fc -d cbs`), bukan `-n public`.**
Temuan penting: dump `-n public` **tidak dapat dipulihkan sendirian** karena:

- dump itu menyertakan `CREATE SCHEMA public;` (owner `public` di `cbs` adalah
  `pg_database_owner`, jadi `pg_dump -n` memaksa DDL skema) → bentrok dengan
  `public` bawaan DB target ("schema public already exists"), dan
- dump itu **tidak** menyertakan `CREATE EXTENSION "uuid-ossp"`, padahal tabel
  memakai `public.uuid_generate_v4()` → "function does not exist".

Karena `public` adalah satu-satunya skema pengguna di `cbs`, dump whole-database
secara isi setara dump `public` ditambah DDL extension yang wajib. Restore
dijalankan dengan `--exit-on-error` agar pemulihan parsial tidak lolos sebagai
"sukses".

Perintah inti (dijalankan lewat SSH; ringkas):

```bash
podman exec qouver-postgres pg_dump -U qouver -d cbs -Fc > cbs_full_<ts>.dump
podman exec qouver-postgres createdb -U qouver -O qouver cbs_restore_drill
podman exec -i qouver-postgres pg_restore -U qouver -d cbs_restore_drill --exit-on-error < cbs_full_<ts>.dump
```

---

## 3. Angka verifikasi: sumber (`cbs`) vs target (`cbs_restore_drill`)

| Metrik | `cbs` (sumber) | `cbs_restore_drill` (target) | Selisih |
|---|---:|---:|---:|
| Tabel `public` (BASE TABLE) | 26 | 26 | 0 |
| `journal_entries` | 0 | 0 | 0 |
| `journal_lines` | 0 | 0 | 0 |
| `accounts` | 72 | 72 | 0 |
| `loans` | 0 | 0 | 0 |
| `customers` | 0 | 0 | 0 |
| `schema_migrations` | 48 | 48 | 0 |
| Migrasi terakhir | `000048_key_versioning.up.sql` | `000048_key_versioning.up.sql` | sama |
| Invarian: total DEBIT | 0 | 0 | 0 |
| Invarian: total KREDIT | 0 | 0 | 0 |
| Ukuran DB | 11 MB | 10.102 kB | ~0 (tanpa bloat) |

**Invarian keuangan**: total debit = total kredit **0 = 0**. Perlu dinyatakan terus
terang: `journal_lines` **kosong** di environment ini, sehingga invarian terbukti
benar tetapi **trivial**. Latihan ini belum membuktikan rekonsiliasi debit/kredit
pada data bervolume; itu keterbatasan latihan, bukan kegagalan.

Catatan jumlah migrasi: dokumentasi tugas menyebut 51 migrasi, kenyataannya
database `cbs` berisi **48** entri `schema_migrations` dan repo saat itu juga
memuat 48 file `*.up.sql`. Angka 48 yang dilaporkan di atas adalah keadaan nyata.

**Hasil restore**: sukses utuh (`--exit-on-error`), 0 galat. Ringkasan `pg_restore --list`:
260 entri, termasuk `CREATE EXTENSION uuid-ossp` dan `ACL - SCHEMA public`.

**Waktu & ukuran:**
- Dump: 131.134 byte (131 KB), **~0,24 detik**.
- Restore: **~0,54 detik**.
- SHA-256 dump latihan: `1d42b9a649829aae73a24e451903ab6eee52feaa4f363fb68c780d8b6e4dec02`.
- Total latihan (dump + create + restore + verifikasi + bersih): di bawah ~2 menit.

---

## 4. Bukti `cbs` tidak berubah

Metrik kunci `cbs` diukur **sebelum** latihan dan **sesudah** pembersihan, identik:

```
tables_public=26  journal_entries=0  journal_lines=0  accounts=72
loans=0  customers=0  schema_migrations=48  migrations_last=000048_key_versioning.up.sql
```

Tidak ada DDL/DML yang dijalankan terhadap `cbs`; hanya `pg_dump` (baca-saja) dan
kueri `SELECT`. Tidak ada container produksi (`qouver-postgres`, `cbs-api`,
`cbs-web`) yang di-restart.

---

## 5. Pembersihan (bukti)

- Target `cbs_restore_drill` di-drop (`podman exec ... dropdb --if-exists`).
- Berkas dump latihan (`cbs_public_*.dump`, `cbs_full_*.dump`) dihapus; direktori
  `/srv/qouver/backups/restore_drill` dihapus.
- Kueri sisa database restore: **kosong**.
- Container `cbs-e2e-batch` **tidak pernah dibuat** (uji Task 1 dijalankan tanpa DB);
  container yang ada (`cbs-e2e-rc`, `cbs-e2e-eir`, `cbs-e2e-coa`) milik agen lain dan
  tidak disentuh. Tidak ada tunnel SSH lokal yang tertinggal.

---

## 6. Celah yang ditemukan

1. **[Kritis] Tidak ada salinan offsite.** Cadangan dan database berada di host/disk
   yang sama. Ini satu-satunya jaring pengaman untuk 48 migrasi up-only, tetapi ia
   bisa hilang bersama datanya.
2. **[Tinggi] Belum pernah diuji pulih.** Pengujian ini adalah yang pertama. Validasi
   otomatis hanya `pg_restore --list` (struktur), bukan pemulihan.
3. **[Sedang] Dump per-skema tidak memadai.** `pg_dump -n public` tidak restorable
   karena extension/`CREATE SCHEMA`; prosedur pemulihan harus memakai dump
   whole-database + `globals`.
4. **[Sedang] Belum ada salinan monthly `cbs`.** Baru daily/weekly; jendela retensi
   efektif `cbs` ±2 minggu.
5. **[Rendah] `globals` tersimpan terpisah per hari.** Pemulihan ke instance baru
   butuh berkas globals yang cocok; pastikan dipakai bersama dump DB.
6. **[Rendah] Invariant uji trivial.** Tidak ada jurnal di environment ini, sehingga
   rekonsiliasi debit/kredit belum teruji dengan data nyata.

---

## 7. Rekomendasi (berprioritas)

1. **P1 — Salinan offsite harian.** Kirim dump (atau minimal weekly/monthly) ke
   penyimpanan lain (S3-compatible/object storage/VPS kedua) dengan enkripsi dan
   retensi. Tanpa ini cadangan bukan jaring pengaman.
2. **P1 — Jadwalkan uji restore otomatis.** Bulanan: pulihkan dump terbaru ke DB
   sementara (pola `cbs_restore_drill`), bandingkan jumlah tabel/baris kunci &
   invarian debit=kredit, lalu drop. Kirim hasil ke Uptime Kuma/Telegram.
3. **P2 — Validasi dump lebih kuat.** Setelah `pg_restore --list`, tambahkan
   `pg_restore -l` untuk memastikan `CREATE EXTENSION` ada, dan dokumentasikan
   prosedur restore yang benar (whole-DB + globals, `--exit-on-error`).
4. **P2 — Tambah salinan monthly `cbs`** (otomatis sudah berjalan tiap tanggal 1;
   pastikan retensi 93 hari mencakupnya).
5. **P3 — Uji invarian keuangan pada data bervolume** (setelah ada jurnal nyata),
   bukan hanya 0=0.
6. **P3 — Catat jadwal & hasil uji restore di `docs/`** agar bukti tidak hilang
   seperti catatan backlog lain.

---

## 8. Hal yang tidak saya yakini

- Apakah `cbs` yang terlihat (0 jurnal, 0 kredit, 72 akun) benar-benar database
  produksi atau environment demonstrasi. Bila ada instance/DB lain yang dimaksud,
  angka di laporan ini tidak mewakilinya.
- Apakah `dropdb` pada target latihan memakai DB maintenance `postgres` yang sama
  aman sepenuhnya; secara praktik berhasil dan tidak menyentuh `cbs`.
- Efek jangka panjang penambahan tabel/objek baru ke `cbs` setelah dump; karena
  migrasi up-only, salinan offsite tetap wajib.
