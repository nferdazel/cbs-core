# Panduan Deploy Operasional CBS Core

Dokumen ini untuk operator yang memasang atau memperbarui CBS Core di VPS.
Isinya sengaja konkret: perintah yang benar-benar dipakai repo ini, bukan
gambaran umum. Sumber kebenaran tetap `scripts/migrate.sh` dan `scripts/preflight.sh`.

> Sebelum membaca lebih jauh: **migrasi bersifat up-only secara default.** `migrate.sh`
> hanya menerapkan `*.up.sql`; kesalahan yang sudah masuk produksi sebaiknya
> dikembalikan lewat **restore backup**. Migrasi baru (sejak `000099`) tetap ditulis
> dengan pasangan `*.down.sql` yang simetris. Sejak 1 Okt 2026 tersedia **jalur rollback
> opt-in** (`scripts/migrate.sh --down --yes [N]`) yang membalik N migrasi terakhir
> memakai berkas `.down.sql` — lihat bagian "Rollback migrasi" di bawah. Jalur ini
> destruktif dan **tidak** dijalankan otomatis oleh deploy maupun CI.

---

## 1. Prasyarat environment

| Komponen | Nilai produksi | Keterangan |
|---|---|---|
| Database | PostgreSQL **18**, container `qouver-postgres` | DB `cbs`, owner `qouver` |
| Role aplikasi | `cbs_app` | Dibuat manual oleh operator, **tidak** ada di repo |
| Role owner objek | `qouver` | DDL dijalankan sebagai owner ini |
| API | container `cbs-api` | di belakang Caddy, port host `127.0.0.1:8095` |
| Web | container `cbs-web` | di belakang Caddy, port host `127.0.0.1:3005` |
| Edge | Caddy | `cbs.qouver.com` → `cbs-web`; `api.qouver.com/cbs/*` → `cbs-api` |

### 1.1 Role aplikasi `cbs_app` — WAJIB dibuat sebelum migrasi

Migrasi `000012_app_role_grants` memberi hak tabel/sequence ke `cbs_app` dan
**sengaja gagal** bila role itu belum ada (pesannya jelas; lebih baik gagal di
migrasi daripada aplikasi gagal runtime dengan `permission denied`). Buat role
sekali per environment, dengan password yang diisi operator:

```sql
-- Jalankan sebagai owner/superuser. Ganti <RAHASIA> dengan password kuat.
CREATE ROLE cbs_app LOGIN PASSWORD '<RAHASIA>';
```

Contoh di VPS (password tidak boleh masuk shell history/repo — isi manual):

```bash
podman exec -u postgres qouver-postgres \
  psql -U qouver -d cbs -c "CREATE ROLE cbs_app LOGIN PASSWORD '<RAHASIA>';"
```

Password `cbs_app` kemudian disimpan hanya di berkas env mode 600 (§5), bukan
di repo.

### 1.2 Owner skema harus `qouver`

`migrate.sh` menjalankan DDL sebagai `qouver` (default `CBS_DB_OWNER`). Bila
skema `public` dimiliki role lain, grant dan perubahan objek bisa salah pemilik.
Periksa lewat `scripts/preflight.sh`; perbaiki dengan (sebagai superuser):

```sql
ALTER SCHEMA public OWNER TO qouver;
```

---

## 2. Berkas rahasia (mode 600, gitignored)

| Berkas | Isi | Mode |
|---|---|---|
| `scripts/.ssh-host` | host SSH VPS untuk `migrate.sh --remote` | `600` |
| `/srv/qouver/apps/cbs/env/cbs-prod.env` | env container `cbs-api` (JWT, kunci enkripsi, DSN app) | `600` |
| `/srv/qouver/apps/cbs/env/cbs-web.env` | env container `cbs-web` | `600` |

Buat/ubah izin:

```bash
chmod 600 scripts/.ssh-host
# di VPS:
chmod 600 /srv/qouver/apps/cbs/env/cbs-prod.env /srv/qouver/apps/cbs/env/cbs-web.env
```

Catatan: `CBS_SSH_HOST`, `CBS_SSH_USER`, dan `CBS_SSH_KEY` bisa dipakai sebagai
variabel lingkungan alternatif daripada `scripts/.ssh-host`. Host SSH sengaja
tidak punya default di repo karena repo publik.

---

## 3. Urutan langkah (environment yang sudah berjalan)

```bash
# 0. dari akar repo, berkas .ssh-host sudah mode 600
# 1. PREFLIGHT — read-only, harus lulus
./scripts/preflight.sh --remote

# 2. MIGRASI — menerapkan *.up.sql yang belum tercatat
./scripts/migrate.sh --remote

# 3. VERIFIKASI — preflight ulang harus lulus, lalu cek jumlah migrasi
./scripts/preflight.sh --remote
```

### 3.1 Menjalankan migrasi

```bash
./scripts/migrate.sh --remote            # produksi lewat SSH (cara normal)
./scripts/migrate.sh --dry-run           # lihat migrasi yang akan diterapkan
./scripts/migrate.sh                     # lokal: container di mesin ini
./scripts/migrate.sh --container NAMA    # container postgres lain
```

`migrate.sh` menjalankan `psql` di dalam container lewat
`ssh … podman exec -i -u postgres`, satu transaksi per berkas
(`ON_ERROR_STOP=1`, `-1`) sehingga migrasi yang gagal di-rollback utuh. Setiap
berkas yang sukses dicatat di `schema_migrations`.

### 3.1.1 Rollback migrasi (opt-in, destruktif)

```bash
./scripts/migrate.sh --down --dry-run        # lihat migrasi terakhir yang akan dibalik
./scripts/migrate.sh --down --yes            # balik 1 migrasi terakhir
./scripts/migrate.sh --down --yes 3          # balik 3 migrasi terakhir
./scripts/migrate.sh --remote --down --yes 1 # di produksi lewat SSH
```

- Membalik N migrasi **terakhir yang tercatat**, terbaru lebih dulu, dengan menjalankan
  berkas `.down.sql` pasangannya dan menghapus baris `schema_migrations` di
  **transaksi yang sama** (tidak ada state "sudah dibalik tetapi masih tercatat").
- Berhenti bila `.down.sql` tidak ada — hanya migrasi sejak `000099` yang punya pasangan.
- **Wajib `--yes`** (tanpa itu skrip menolak) karena operasi ini menghapus objek.
- Tetap utamakan **restore backup** untuk insiden produksi; rollback hanya untuk koreksi
  terkendali (mis. migrasi terakhir yang jelas keliru dan belum dipakai luas).

### 3.2 Environment benar-benar baru (bootstrap)

Preflight dirancang untuk environment **yang sudah punya skema**. Pada server
baru, urutannya sedikit berbeda karena belum ada tabel apa pun:

1. Pastikan PostgreSQL 18 dan container `qouver-postgres` hidup, DB `cbs` ada,
   owner `qouver`.
2. Buat role `cbs_app` (§1.1).
3. Jalankan `./scripts/migrate.sh --remote` (atau lokal) sampai selesai.
4. Baru kemudian preflight akan lulus penuh.

---

## 4. Verifikasi setelah migrasi

```bash
# Semua migrasi repo tercatat (jumlah harus sama dengan jumlah *.up.sql)
podman exec -u postgres qouver-postgres psql -U qouver -d cbs -tA \
  -c "SELECT count(*) FROM schema_migrations;"

# Kesehatan API. /healthz ada di root API; lewat edge diakses di api.qouver.com/cbs/.
# (cbs.qouver.com hanya me-rewrite /api/* ke API, jadi /healthz tidak lewat sana.)
curl -fsS https://api.qouver.com/cbs/healthz >/dev/null && echo "API OK"

# Readiness (dependensi, termasuk ping DB). 503 bila basis data tidak sehat.
curl -fsS https://api.qouver.com/cbs/ready >/dev/null && echo "API READY"
```

### Metrik & tracing (W18)

- **`GET /metrics`** (Prometheus, teks) dan **`GET /ready`** berada di root API.
- **`/metrics` sengaja DIBLOKIR di edge publik**: Caddy membalas `403` untuk
  `api.qouver.com/cbs/metrics`. Prometheus harus melakukan scrape **langsung** ke
  `http://127.0.0.1:8095/metrics` (di host yang sama) atau lewat jaringan internal /
  SSH tunnel, bukan lewat domain publik. Label metrik memakai POLA rute, bukan path
  berisi id, dan tidak memuat data nasabah.
- **Tracing opt-in**: set `OTEL_EXPORTER_OTLP_ENDPOINT` (mis. `localhost:4318`) untuk
  menyalakan. Kosong = tracing mati (tanpa overhead). `OTEL_TRACE_SAMPLE_RATIO`
  (0..1) menimpa; bawaan 0.1 di produksi, 1.0 di non-produksi. Span terakhir di-flush
  saat shutdown rapi.
- **Graceful shutdown**: SIGINT/SIGTERM menghentikan server dengan rapi (permintaan
  berjalan diselesaikan, batas 20 detik) sebelum `db.Close()`.

Selain itu `preflight.sh` memeriksa baris `system.business_date`:

> Baris `system.business_date` **tidak ada** di database yang baru dimigrasi —
> baru dibuat saat pertama kali tanggal bisnis disetel/dipakai. Selama baris itu
> belum ada, **seluruh pembatalan transaksi dianggap lintas hari dan wajib
> persetujuan pejabat** (arah yang aman, tetapi pembatalan same-day tidak pernah
> aktif). Setelah migrasi, jalankan tutup hari sekali lewat aplikasi atau isi
> barisnya agar perilaku same-day bekerja.

---

### 4.1 Bila migrasi sudah "ter-apply" tetapi tidak tercatat

Gejala: `preflight` melaporkan migrasi tertinggal padahal skema sudah sesuai; atau
`migrate.sh` gagal di tengah pada migrasi yang menyentuh kolom lama, mis.
`UPDATE ... kolom_lama` padahal kolom itu sudah tidak ada.

Sebab: migrasi pernah dijalankan (atau dampaknya sudah ada karena perbaikan manual),
tetapi barisnya tidak masuk `schema_migrations`. Sebelum 26 Sep 2026 ini bisa terjadi karena
apply dan rekam di `migrate.sh` memakai dua koneksi terpisah — proses yang mati di antaranya
meninggalkan state "ter-apply tetapi tidak tercatat". Sejak 26 Sep 2026 keduanya satu
transaksi, jadi penyebab itu tidak lagi ada; bagian ini dipertahankan untuk DB lama. Karena
`migrate.sh` memakai `ON_ERROR_STOP=1`, migrasi yang gagal memblokir migrasi sesudahnya.

Tindakan, SETELAH memastikan skema memang sudah sesuai:
1. Bandingkan berkas repo dengan catatan DB:
   `comm -3 <(ls packages/db-migrations/*.up.sql | xargs -n1 basename | sort) <(psql ... "SELECT filename FROM schema_migrations ORDER BY filename")`
2. Verifikasi objek yang ditargetkan migrasi itu (kolom/constraint/index) memang sudah
   dalam keadaan akhir yang diinginkan.
3. Rekam barisnya tanpa mengubah skema:
   `INSERT INTO schema_migrations (filename) VALUES ('<nama>.up.sql') ON CONFLICT DO NOTHING;`
4. Jalankan `preflight` ulang; jumlah baris harus sama dengan jumlah `*.up.sql`.

Catatan: pola ini dipakai 26 Sep 2026 untuk
`000098_consolidate_collateral_cost_columns` — skema produksi sudah sesuai
(`disposal_cost_amount` + CHECK ada, `selling_cost_amount` tidak ada), hanya barisnya
yang hilang. Sejak 26 Sep 2026 `000098` juga diberi **penjaga keberadaan kolom**, sehingga
menjalankannya ulang aman (tidak lagi gagal `column does not exist`). Pola di atas tetap
berlaku untuk migrasi lain yang menyentuh kolom lama.

## 5. Rotasi password superadmin

Akun `superadmin` di-seed dengan hash placeholder yang **tidak bisa dipakai
login**. Komentar migrasi `000002` menyebut provisioning lewat
`AUTH_BOOTSTRAP_PASSWORD`, tetapi (diverifikasi) tidak ada kode yang membaca
variabel itu saat ini.

**Rotasi rutin (setelah bisa login):**

1. Login sebagai `superadmin`.
2. Panggil `POST /api/v1/staff/me/change-password` (atau menu penggantian
   password di UI) dengan password lama dan baru.
3. Sesi lama tetap berlaku sampai kedaluwarsa; untuk memutus paksa, nonaktifkan
   lalu aktifkan kembali akun, atau ganti `JWT_SECRET` di env (membatalkan semua
   sesi).

Hierarki peran: `ADMIN` **tidak boleh** mereset password `SUPERADMIN`. Hanya
`SUPERADMIN` yang boleh mereset `SUPERADMIN` lain, dan setiap reset tercatat di
`audit_logs`.

**Bila terkunci total (last resort):** set hash langsung di DB, lalu login dan
segera rotasi lewat langkah di atas. Hash harus bcrypt dengan cost 12 dan prefix
`$2a$` agar cocok dengan verifikasi aplikasi:

```sql
-- Ganti <HASH_BCRYPT_COST_12> dengan hash yang dihasilkan tool bcrypt tepercaya.
-- Jangan pernah menulis password asli ke migrasi/commit.
UPDATE staff_users
SET password_hash = '<HASH_BCRYPT_COST_12>', is_active = TRUE
WHERE username = 'superadmin';
```

Cara menghasilkan hash bergantung tooling di mesin operator; aplikasi memakai
bcrypt cost 12. Jangan menyimpan password asli di repo.

---

## 6. Arti kegagalan preflight dan tindakannya

`preflight.sh` bersifat **read-only** (hanya `SELECT`) dan keluar dengan kode
bukan-nol bila ada masalah. Setiap pesan menyertakan cara memperbaiki.

| Pemeriksaan | Pesan kunci | Arti | Tindakan |
|---|---|---|---|
| Koneksi DB | `koneksi database tidak bisa dibuka` | Container/SSH/DB tidak dapat dijangkau | `podman ps -a` di host; cek `CBS_DB_CONTAINER`; untuk `--remote` cek `scripts/.ssh-host` + `CBS_SSH_KEY` |
| Role `cbs_app` | `role aplikasi 'cbs_app' belum ada` | Migrasi `000012_app_role_grants` pasti gagal | Buat role (§1.1) sebelum migrasi |
| `schema_migrations` | `tertinggal N migrasi` | Ada berkas migrasi repo yang belum diterapkan | Jalankan `migrate.sh` (`--remote` untuk produksi); pesan mencantumkan nama berkasnya |
| `schema_migrations` | `tabel schema_migrations belum ada` | Environment belum pernah dimigrasi | Bootstrap (§3.2) |
| `system.business_date` | `baris 'system.business_date' belum ada` | Semua pembatalan dianggap lintas hari | Jalankan tutup hari sekali, atau isi baris `system.business_date` |
| Owner skema | `owner skema public adalah 'X', bukan 'qouver'` | DDL dijalankan bukan sebagai owner objek | `ALTER SCHEMA public OWNER TO qouver;` sebagai superuser |

Semua kegagalan dilaporkan sekaligus (dikumpulkan), lalu skrip keluar dengan
kode `1`. Jangan lanjutkan migrasi/deploy sebelum preflight lulus.

## 7. Batas laju login dan jumlah instance API

Endpoint `POST /api/v1/auth/login` dijaga pembatas percobaan gagal (per akun 5/15
menit, per IP 20/15 menit) di `apps/api/internal/middleware/ratelimit.go`. Penghitungnya
**di memori proses**, jadi ia berlaku per instance API.

KEPUTUSAN untuk topologi sekarang: **satu instance API** di belakang Caddy
(`cbs-api`), sehingga penghitung per proses sama dengan penghitung bank-wide dan
pembatas ini sudah memadai. Tidak ada penyimpanan bersama yang ditambahkan, karena
itu menambah ketergantungan (dan satu putaran DB pada setiap percobaan login) untuk
masalah yang belum ada.

WAJIB ditinjau ulang bila topologi berubah menjadi lebih dari satu instance API
(mis. dua replika `cbs-api`, atau API di lebih dari satu host di belakang edge):
penghitung per proses akan melipatgandakan ambang efektif (N instance = N kali
percobaan yang diizinkan), sehingga perlindungan brute force melemah tanpa terlihat.
Saat itu terjadi, pindahkan pembatas ke salah satu dari:
- Caddy `limit_req` di edge (paling sederhana bila edge tetap satu titik), atau
- penyimpanan bersama (mis. tabel `system_config`/tabel khusus atau Redis) yang
  diakses pembatas, dengan kunci per akun dan per IP.

Sampai salah satu dipilih, jangan menjalankan lebih dari satu instance API.
