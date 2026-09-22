# Keputusan, Dasar Regulasi, dan Rancangan — CBS Core

Dokumen ini merangkum keputusan yang **tahan lama**: dasar hukum, keputusan
produk, keputusan arsitektur, prasyarat environment, dan utang yang masih terbuka.
Asalnya adalah `BACKLOG.md` lokal (di-ignore git, sudah dihapus) karena seluruh
alasan di balik keputusan hanya hidup di berkas lokal dan bisa hilang.

Yang **tidak** ada di sini: status pekerjaan harian dan catatan per-commit — itu
terbaca dari riwayat git. Bila dokumen ini dan kode berbeda, **kode dan migrasi
adalah kebenaran**; perbarui dokumen ini.

---

## 1. Dasar hukum

### 1.1 Kualitas aset dan PPKA — POJK No. 1 Tahun 2024

- Dasar yang berlaku: **POJK No. 1 Tahun 2024** "Kualitas Aset Bank Perekonomian
  Rakyat" (berlaku 11 Januari 2024). Mencabut **POJK 33/POJK.03/2018**.
- Istilah lama **PPAP** menjadi **PPKA** (Penyisihan Penghapusan Kualitas Aset);
  tarifnya identik: 0,5% / 3% / 10% / 50% / 100%.
- Nomor pasal bergeser: tarif PPAP Pasal 16 → **Pasal 19**; agunan pengurang
  Pasal 17 → **Pasal 20**; restrukturisasi berada di **Pasal 29–35** (batas kualitas **Pasal 31**, akuntansi **Pasal 32**).
- Komentar kode lama yang menyebut "POJK 40/2019" salah: itu kerangka **Bank
  Umum**, bukan BPR — sekaligus menjelaskan kenapa pita lamanya (30/90/180)
  menyerupai pita Bank Umum yang digeser.

**Pita kolektibilitas yang diimplementasikan** — satu sumber kebenaran
`domain.CollectibilityFromPosition(dpd, daysPastMaturity, ambang)`:

- Angsuran **≥ 1 bulan**: Lancar 30 / DPK 90 / Kurang Lancar 180 / Diragukan 360.
- Angsuran **< 1 bulan**: Lancar 15 / DPK 30 / Kurang Lancar 90 / Diragukan 180.
  Sudah ada dan teruji, **belum dikabelkan** karena seluruh jadwal sistem masih
  bulanan.
- Dimensi jatuh tempo: DPK ≤ 15 / Kurang Lancar ≤ 30 / Diragukan ≤ 60 / Macet
  > 60 hari.
- Golongan akhir = **yang terburuk** dari dimensi tunggakan dan jatuh tempo
  (regulasi memakai "dan/atau").

**Prinsip konfigurasi**: POJK adalah standar **minimum**. Bank boleh
menggolongkan lebih buruk, tidak boleh lebih baik. Nilai kosong → default POJK;
nilai lebih longgar → dipulihkan ke default (`tighten`); nilai lebih ketat →
dihormati.

### 1.2 Agunan pengurang PPKA — koreksi Pasal 20, bukan Pasal 17

Diverifikasi ke teks resmi POJK 1/2024. **Catatan lama "Pasal 20" BENAR;
komentar migrasi `000026` yang menyebut "Pasal 17" SALAH.**

- **Pasal 19(3)** — tarif PPKA khusus 3/10/50/100% "setelah dikurangi dengan
  nilai agunan".
- **Pasal 20(1)** — batas atas nilai agunan yang boleh jadi pengurang: emas
  perhiasan 85%; tanah/bangunan bersertifikat ber-hak tanggungan/fidusia 80%;
  resi gudang ≤ 12 bulan 70%; tanah/bangunan bersertifikat **tanpa** hak
  tanggungan 60% (NJOP); tanah adat 50%; tempat usaha 50%; kendaraan/alat berat
  ber-hipotek 50%; resi gudang 12–18 bulan 50%; dijamin BUMN/BUMD 50%; resi
  gudang 18–24 bulan 30%; agunan lain (dinilai penilai independen ≤ 1 tahun) 20%.
- **Pasal 20(2)** — agunan di luar daftar itu **tidak** diperhitungkan sebagai
  pengurang.
- **Pasal 20(3)/(5)** — untuk kualitas macet, pengurangan menurun menurut waktu:
  50% setelah 2–4 tahun (huruf b, d, e, f) dan 1–2 tahun (huruf g), lalu **tidak
  boleh** diperhitungkan setelah 4 tahun (b/d/e/f) atau 2 tahun (g).
- **Pasal 20(4)** — pengecualian dari ayat (3) bila tanah/bangunan bersertifikat
  ber-hak tanggungan, dinilai penilai independen dalam 1 tahun terakhir, dan nilai
  hak tanggungan menutup seluruh kewajiban debitur.
- **Pasal 21(2)** — agunan **tidak** jadi pengurang bila belum dinilai, tidak
  diketahui keberadaannya, tidak dapat dieksekusi, atau milik pihak lain tanpa
  persetujuan pemiliknya.
- **Pasal 17** adalah agunan tunai, dipakai Pasal 19(4)(b) untuk pengecualian
  PPKA umum — bukan pasal pengurang PPKA khusus.

**Konsekuensi implementasi.** `haircut_percent` DEFAULT 100 berarti pengurang
nol; itu benar sebagai bawaan (semangat Pasal 20(2): jangan mengurangi sebelum
jenis dan nilainya memenuhi syarat). Yang BELUM ada: pemetaan tarif per jenis
agunan Pasal 20(1), syarat Pasal 21(2), penurunan menurut waktu Pasal 20(3)/(5),
dan pengecualian Pasal 20(4). Karena itu **`ppap.collateral.enabled` tetap
`false`** sampai keempat hal itu dikerjakan — bahkan bila saklar dinyalakan,
perhitungannya belum patuh Pasal 20.

### 1.3 Pelaporan OJK — POJK 23/2024 dan SEOJK 16/2024

Bukan "bank harus memberi daftar"; dasar hukumnya dapat dibaca siapa pun:

- **POJK No. 23 Tahun 2024** — Pelaporan melalui Sistem Pelaporan OJK dan
  Transparansi Kondisi Keuangan bagi BPR/BPRS; berlaku 1 Desember 2024.
  Mencabut POJK 13/POJK.03/2019 (pelaporan BPR/BPRS) serta POJK
  48/POJK.03/2017 dan POJK 35/POJK.03/2019 (transparansi).
- **SEOJK No. 16/SEOJK.03/2024** — pelaksanaannya. Lampiran I = daftar laporan
  berkala dan insidental; Lampiran II = format laporan bulanan; Lampiran III =
  format laporan tahunan dan laporan keuangan publikasi. Mencabut SEOJK
  12/SEOJK.03/2022 (Laporan Bulanan BPR).
- Penyampaian daring lewat **APOLO**; Lampiran II memuat sandi form (antara lain
  Form 00.00 Informasi Pokok BPR, 05.00 Penempatan pada Bank Lain, 06.00 Daftar
  Kredit yang Diberikan, 09.00 Rincian Aset Lainnya, 13.00 Simpanan dari Bank
  Lain, 00.08 Rasio Keuangan Triwulanan, 00.13 Dokumen Pendukung, 00.14/00.15
  data nasabah dan risiko TPPU), disampaikan sebagai berkas teks dan dalam
  rupiah penuh.
- **Laporan berkala bulanan** (batas **tanggal 10** bulan berikutnya; koreksi
  sampai tanggal 15): Laporan Bulanan BPR, Laporan Kelembagaan, Laporan BMPK,
  Laporan Keuangan Publikasi, Laporan Perbedaan Kualitas Aset Produktif, Laporan
  Dokumen Penilaian Risiko TPPU/TPPT/PPSPM, Laporan Bukti Pengumuman Laporan
  Tahunan. **Triwulanan**: Laporan Keuangan Publikasi (tanggal 10) dan Laku
  Pandai (tanggal 15). **Tahunan**: rencana bisnis 15 Desember, penunjukan
  akuntan publik, realisasi penggunaan jasa akuntan publik ≤ 6 bulan setelah
  tahun buku.
- **Laporan Keuangan Publikasi wajib diumumkan** di situs web BPR untuk posisi
  Maret, Juni, September, Desember; ditambah surat kabar/media kantor bila aset
  ≥ Rp10 miliar.

**Yang masih perlu diunduh, bukan ditebak:** format rinci tiap form ada di
Lampiran II SEOJK 16/2024 (PDF). Itu bahan konkret untuk modul ekspor, bukan
pertanyaan untuk bank.

**Tambahan yang belum ada di sistem:** sejak 1 Januari 2025 BPR wajib membentuk
**CKPN menurut standar akuntansi** (PSAK), di samping PPKA menurut POJK. Sistem
baru punya PPKA; CKPN/PSAK belum.

### 1.4 Cap restrukturisasi Pasal 31

Aturan Pasal 23 POJK 1/2024 ditegakkan untuk kredit yang direstrukturisasi
(kolektibilitas tidak boleh langsung kembali ke Lancar). Detail teknis dan
status ada di kode `domain/ppap.go`, `service/collectibility_rules.go`, dan
migrasi `000028_restructure_collectibility.up.sql`.

---

## 2. Keputusan produk dan perilaku

### 2.1 Koreksi transaksi kredit (keputusan "5c") — disetujui

Dua jalur, keduanya sudah dibangun:

- **5c(a) pembatalan pencairan** — hanya bila belum ada angsuran. Jurnal
  pencairan dikontra, jurnal asal ditandai `REVERSED`, baris jadwal dihapus,
  status kredit `CANCELLED`, `outstanding` nol, audit dengan alasan. Permission
  baru `loans:cancel` (Superadmin/Admin). Endpoint
  `POST /api/v1/loans/{id}/cancel-disbursement`.
- **5c(b) koreksi nominal** — bila sudah ada angsuran. Hanya angsuran **belum
  dibayar** yang dihitung ulang (riwayat tidak ditulis ulang), jurnal selisih
  mengikuti arah jurnal pencairan. Wajib maker-checker
  (`maker_checker.LOAN_CORRECTION.threshold`); ambang yang belum diisi berarti
  **setiap** koreksi wajib disetujui. Permission `loans:correct`. Endpoint
  `POST /api/v1/loans/{id}/correct-amount`.

**Batasan yang dicatat, belum diputuskan bank:**

- Bunga/margin per angsuran **tidak** dihitung ulang (hanya pokok). Untuk
  murabahah bisa diperdebatkan (margin = kesepakatan); untuk konvensional bunga
  atas pokok yang berubah semestinya ikut berubah.
- Angsuran yang dibayar sebagian diperlakukan sebagai "dibekukan".
- Koreksi tidak boleh surut ke tanggal bisnis sebelumnya (selalu tanggal bisnis
  berjalan).

**Pelajaran teknis (dua bug nyata):**

- Jurnal kontra sempat memakai tanggal kalender UTC, bukan tanggal bisnis.
  Diperbaiki di kedua jalur pembatalan; bila tanggal bisnis tidak terbaca,
  pembatalan **ditolak** — menebak tanggal lebih buruk daripada menolak. Default
  `EntryDate` nol = tanggal kalender UTC sengaja tetap berlaku untuk jalur
  posting lain (setoran, angsuran, akrual).
- Bila jurnal pencairan memuat lebih dari satu baris per arah, kode lama
  mengambil baris pertama dan koreksi bisa mengalir ke akun yang salah; sekarang
  ditolak dan diminta diperiksa manusia.

### 2.2 Koreksi pencairan deposito (keputusan "6a") — disetujui

**Ditolak; bank menagih dulu.** Membuat saldo negatif lewat jurnal koreksi
menyembunyikan piutang yang sebenarnya harus tercatat dan ditagih. Jurnal
deposito tidak diberi penanda alur `source`, sehingga penjaga cakupan pembatalan
menolaknya secara alami. Sisa: catat di SOP bahwa koreksinya adalah penagihan,
bukan pembatalan.

### 2.3 PPh bagi hasil mudharabah (keputusan "7a") — disetujui

**Tidak dipotong**, mengikuti perlakuan bagi hasil sebagai bukan bunga, dengan
kewajiban pelaporan tersendiri. `savingsAccrualTax` mengembalikan nol untuk buku
selain konvensional; bagi hasil syariah tidak dipotong dan tidak dicatat sebagai
utang pajak. Remunerasi/amil DPS menjadi **modul terpisah** agar perubahan aturan
pajak bunga tidak menyentuh syariah.

### 2.4 Pelaporan OJK (keputusan "8") — menunggu bahan

Bukan keputusan, melainkan **bahan**: daftar laporan, periodisitas, dan format
berkas wajib. Tanpa itu sistem hanya bisa menyediakan ekspor data mentah, bukan
laporan resmi. Kerangka hukumnya sudah ditemukan (§1.3); format rinci diunduh
dari lampiran SEOJK, bukan dikarang.

### 2.5 Akrual bunga kredit (dikonfirmasi pemilik produk)

- Berbasis **jadwal angsuran**, bukan harian: nilainya persis sama dengan jadwal
  sehingga tidak ada selisih pembulatan yang perlu direkonsiliasi.
- Dihentikan untuk kolektibilitas 3–5; akruan yang sudah terbentuk tidak
  dibalik, hanya dihentikan.
- Hanya kredit konvensional; syariah belum punya pemetaan `INTEREST_ACCRUAL`.
- Pembayaran memakai `settle = min(porsi bunga, profit_accrued_amount)` dan
  melunasi piutang bunga (CREDIT 10400) alih-alih mengakui pendapatan lagi.

Catatan: akrual **denda** pada kredit NPL sudah dihentikan (gerbang kolektibilitas
yang sama dengan bunga; §6.1). Denda konvensional tetap mengakui pendapatan (40500);
denda syariah/ta'zir masuk akun 12500 Dana Kebajikan (§7.2).

### 2.6 Rekening dormant

- Ambang dari `system_config` `account.dormant.after_months`, fallback kode 12
  bulan.
- Dasar waktu `COALESCE(last_activity_at, opened_at, created_at)`; dormant tidak
  bergantung saldo.
- Kredit (setoran/transfer masuk) tetap diterima; debit ditolak 422. Setoran
  **tidak** mengaktifkan kembali rekening — reaktivasi harus eksplisit.
- Reaktivasi memakai permission `PermAccountsFreeze`
  (SUPERADMIN/ADMIN/SUPERVISOR), terikat cabang rekening (403 bila beda cabang).
- Penandaan dormant tidak menghasilkan jurnal karena bukan peristiwa ekonomi.

### 2.7 Pembatalan transaksi (reversal) — disetujui

- **A. Tanggal posting selalu tanggal bisnis berjalan**, tidak pernah mundur ke
  tanggal asal. Entri tanggal asal sudah membentuk laporan periode/PPAP/dasar
  pajak yang mungkin sudah dilaporkan; koreksi periode tertutup **bukan** reversal
  dan tidak dibangun. Deskripsi jurnal: `Pembatalan <REF-ASAL> — <alasan>`.
- **B. Cakupan fase 1** hanya transaksi tanpa state domain: setoran, penarikan,
  transfer, biaya admin. Angsuran/pencairan kredit/pencairan deposito tidak lewat
  reversal generik karena jurnal dan data domain tidak akan sinkron; koreksinya
  lewat fitur domain masing-masing.
- **C. Wewenang**: permission `transactions:reverse` mulai SUPERVISOR (TELLER
  tidak). Same-day boleh supervisor langsung; lintas tanggal wajib maker-checker.
  Alasan wajib diisi dan masuk audit log + deskripsi jurnal.
- **Penanda alur `source`** diperlukan untuk menegakkan cakupan B dengan benar;
  label `transaction_type` saja tidak cukup (dipakai bersama teller dan
  deposito/kredit). Yang sudah ditegakkan: jurnal `INTEREST_ACCRUAL`,
  `ADJUSTMENT`, dan jurnal ber-`created_by` SYSTEM/`''` (batch/EOD) ditolak.
  Sampai penanda `source` lengkap, endpoint pembatalan hanya untuk kesalahan
  input teller pada rekening.
- Jurnal `REVERSAL` tidak boleh dibalik lagi; hanya jurnal `POSTED` yang bisa
  dibatalkan. Atribusi siapa/kapan/kenapa terekam di `audit_logs`
  (event `REVERSE_TRANSACTION`).
- Bila tanggal bisnis tidak terbaca atau layanan persetujuan tidak ada,
  pembatalan lintas hari **ditolak** — tidak pernah diloloskan tanpa pejabat
  kedua.

---

## 3. Keputusan arsitektur

### 3.1 Persistensi

- **Tanpa ORM.** Query eksplisit `database/sql` + pgx: kontrol `SELECT FOR
  UPDATE` dan urutan kunci harus presisi; `NUMERIC`/decimal di-scan eksplisit;
  query eksplisit lebih mudah diaudit untuk sistem uang.
- Repository sebagai batas akses data; tidak ada SQL di service.
- Transaksi dibuat di service (use case) dan diteruskan ke repo. Interface domain
  memakai `tx any` (opaque) agar domain tidak impor `database/sql`.

### 3.2 Lapisan dan pola

- Arah dependensi selalu ke dalam:
  `handler -> service -> domain <- repository`. Domain tidak mengenal infra.
- `internal/app` sebagai composition root; `main.go` hanya memanggilnya.
- Pola: repository, service layer, posting engine (satu titik untuk jurnal),
  dependency inversion, functional options.
- **Tidak** ada CQRS, event sourcing, mediator, atau domain events —
  over-engineering untuk skala BPR.
- Double-entry divalidasi di posting engine, bukan di tiap service.
- Sentinel error di domain untuk kasus khusus, wrapping `%w`; error internal
  tidak bocor ke client.

### 3.3 Testing

- Unit test untuk logika murni; stub hand-written untuk repo (bukan mock
  framework).
- Test integrasi DB untuk posting engine.
- Pelajaran: untuk penegakan aturan yang membaca kolom database, test harus
  membaca database — bukan hanya memeriksa permintaan.

### 3.4 Arsitektur logging

Dua jenis log yang **tidak boleh dicampur**:

- **Application log (teknis)** — JSON terstruktur ke stdout, dikelola
  systemd/journald. Level `debug`/`info`/`warn`/`error`. Memuat `ts`, `level`,
  `msg`, `request_id`, `user_id`, `role`, `branch`, `method`, `path`, `status`,
  `latency_ms`, `error`. Satu `request_id` dibuat di middleware dan diteruskan;
  header `X-Request-ID` divalidasi lalu dipakai.
- **Audit log (bisnis)** — tabel `audit_logs`, append-only, tanpa endpoint
  ubah/hapus. Ditulis untuk semua aksi finansial, perubahan master data,
  login/logout/gagal login, ekspor laporan, perubahan konfigurasi, akses data
  nasabah. Transaksional dengan aksi bisnisnya.

**Larangan keras:** password/hash/secret/kunci, NIK/nomor rekening/nomor
HP/email/alamat, isi body transaksi finansial, cookie, dan header Authorization
tidak boleh masuk log. Redaksi otomatis di helper terpusat. Bila
`APP_ENV=production`, paksa level minimum `info` dan format JSON.

### 3.5 Konfigurasi terpusat

Prinsip: **parameter bisnis adalah data, bukan kode.** Nilai yang bisa berubah
di-load dari DB dengan cache dan validasi saat start; default hanya *fallback
instalasi baru*, bukan pengganti konfigurasi. Yang dipindah ke DB antara lain:
tarif PPKA dan ambang DPD, limit per role dan threshold maker-checker, tarif
pajak, bunga/margin/nisbah di `banking_products`, identitas bank di
`bank_profile`, konfigurasi dokumen, jumlah hari kerja/jam operasional/tanggal
mulai EOD, dan kebijakan password/TTL token.

### 3.6 Baseline security dan enkripsi

- Identitas pelaku selalu dari JWT claim; `created_by` dari body diabaikan.
- Access/refresh token di cookie httpOnly + Secure + SameSite; rotasi refresh
  token dan deteksi reuse; CSRF double-submit untuk endpoint tulis.
- Permission dicek di router dan service (defense in depth). Branch scoping
  tahap 1–5; laporan agregat sengaja bank-wide.
- Password bcrypt cost 12. Data pribadi AES-256-GCM **field-level, data key per
  record**, master key dari env di luar repo. Blind index HMAC-SHA256(NIK) untuk
  pencarian tanpa membuka enkripsi. Master key punya id/versi agar bisa dirotasi.
- TLS di edge (Caddy) + HSTS; tidak ada port HTTP terbuka untuk API.

### 3.7 Rancangan modul agunan

Titik sambung: `ppapService.run` mengambil `[]domain.PPAPLoanSnapshot` dan
`processLoan` menghitung target dari tarif × eksposur, jadi pengurangan agunan
masuk lewat field baru pada snapshot.

- Tabel `loan_collaterals` (migrasi `000036`) dengan identitas, penilaian,
  `haircut_percent NUMERIC(5,2)` **bawaan 100**, `bound_amount` sebagai kolom
  generated `appraisal_value × (1 − haircut_percent/100)`, status
  `ACTIVE/RELEASED/EXECUTED`, dan audit kolom.
- Kunci konfigurasi `collateral.haircut.{tanah_bangunan,kendaraan,deposit,
  mesin_peralatan,lainnya}`, semuanya **bawaan 100** (tanpa pengurangan).
- **Semantik haircut**: `haircut_percent` adalah bagian nilai taksasi yang TIDAK
  dihitung sebagai pengurang; `100` berarti agunan tidak mengurangi eksposur.
  Bawaan 100 dipilih agar rilis tidak menggeser angka PPAP diam-diam dan bank
  tidak dipaksa memakai kebijakan yang dikarang.
- Saklar `ppap.collateral.enabled` **bawaan `false`**: selama `false`,
  pengurangan tidak diterapkan walau agunan sudah tercatat.
- **Pagar**: formula pengurangan dan pasal yang disitir wajib diverifikasi ke
  teks POJK 1/2024 lebih dulu (lihat §1.2). Sampai verifikasi lengkap, kunci
  tetap `false` dan PPAP tetap atas pokok penuh.

---

## 4. Keputusan operasional dan environment

### 4.1 Topologi

PostgreSQL **18** di container `qouver-postgres`, DB `cbs`, owner `qouver`.
Aplikasi berjalan di container `cbs-api` dan `cbs-web` di belakang Caddy
(`cbs.qouver.com` → web, `api.qouver.com/cbs/*` → API).

### 4.2 Prasyarat yang hanya tercatat di berkas migrasi

- **Role `cbs_app` harus dibuat operator sebelum migrasi.** Migrasi
  `000012_app_role_grants` sengaja gagal tanpa role itu (password tidak boleh
  berada di repo publik). Perintah:
  `CREATE ROLE cbs_app LOGIN PASSWORD '<rahasia>';`
- **Baris `system.business_date` tidak ada** di database yang baru dimigrasi;
  barisnya dibuat saat pertama kali dipakai. Tanpa baris itu, seluruh pembatalan
  transaksi dianggap lintas hari dan wajib persetujuan pejabat (arah yang aman,
  tetapi same-day tidak pernah aktif).
- Prosedur lengkap ada di `docs/DEPLOY.md`.

### 4.3 Migrasi dan rahasia

- `scripts/migrate.sh` adalah **satu-satunya** cara menerapkan migrasi; ada
  tabel pelacak `schema_migrations` dan mode `--remote` untuk DB di VPS.
- **Migrasi up-only**: tidak ada `*.down.sql`; kesalahan hanya bisa dikembalikan
  lewat restore backup.
- Rahasia: `JWT_SECRET` dan `ENCRYPTION_MASTER_KEY` dibuat di VPS dan disimpan
  di `/srv/qouver/apps/cbs/env/cbs-prod.env` (mode 600, di luar repo). Host SSH
  disimpan di `scripts/.ssh-host` (mode 600, gitignored) atau `CBS_SSH_HOST`.
- Verifikasi end-to-end dijalankan dengan container sekali pakai
  (`cbs-e2e-db`, image `postgres:18`, `POSTGRES_USER=qouver`) yang dihapus
  setelah selesai; data uji tidak menyentuh produksi.

### 4.4 Keamanan repo publik

Repo publik. Beberapa kebocoran pernah ada dan sudah dibersihkan: hash bcrypt
superadmin produksi, password dalam teks polos, IP VPS produksi (dipindah ke
`CBS_VPS`), fallback JWT secret di kode, dan CORS wildcard. Aturan: tidak ada
kredensial di repo, gagal cepat bila secret wajib kosong di production, dan
seed user memakai placeholder.

### 1.6 Koreksi: Pasal 23 mengatur hal lain

Kutipan resmi Pasal 23 POJK No. 1 Tahun 2024: "Bagian Penempatan pada Bank Lain yang memenuhi
persyaratan kriteria penjaminan Lembaga Penjamin Simpanan dapat dijadikan sebagai faktor
pengurang dalam pembentukan perhitungan PPKA umum dan khusus."

Jadi Pasal 23 **bukan** tentang restrukturisasi. Rujukan lama "Pasal 23 = restrukturisasi"
berasal dari POJK 33/POJK.03/2018 yang sudah dicabut. Semua sitasi di kode dan dokumen sudah
dipindahkan ke Pasal 31 (batas kualitas) dan Pasal 32 (perlakuan akuntansi).

**Aturan yang belum diimplementasikan dari Pasal 23:** bagian Penempatan pada Bank Lain yang
dijamin LPS dapat mengurangi PPKA umum **dan** khusus. Sistem belum menghitung ini. Perlu
dikerjakan bila bank punya penempatan pada bank lain yang dijamin LPS.

**Kerugian restrukturisasi:** landasannya Pasal 32 beserta penjelasannya, dengan metode
didelegasikan ke standar akuntansi (ditambah panduan akuntansi perbankan untuk BPR). Belum
diakui di sistem karena tiga hal belum ada: penyimpanan suku bunga efektif orisinal, PPKA yang
dihitung atas saldo setelah kerugian, dan pemetaan jurnal untuk beban kerugian penurunan nilai.
Tidak dibangun setengah-setengah karena memakai suku bunga kontraktual sebagai ganti suku bunga
efektif orisinal akan menghasilkan angka yang salah.

---

# KEPUTUSAN YANG MENUNGGU BANK

Daftar ini disimpan di sini — bukan di catatan kerja yang bisa hilang — supaya keputusan yang
menumpuk tidak terlupakan dan tidak menghalangi pekerjaan yang tidak bergantung padanya.
Setiap butir mencatat apa yang terhalang, bukan sekadar daftar keinginan.

## 1. Panduan konversi COA ke pos laporan OJK
SEOJK mewajibkan setiap bank punya pedoman konversinya sendiri. Selama belum ada, pemetaan
`internal/ojkreport/coa_mapping.go` berstatus draf dan laporan belum boleh dikirim ke APOLO.
**Menghalangi:** seluruh pelaporan OJK.

## 2. Saklar kerugian restrukturisasi (`loan.restructure.loss.enabled`)
Secara teknis sudah siap — cacat saldo piutang negatif sudah tertutup amortisasi bunga efektif.
Yang menunggu keputusan bank: membuka saklarnya di produksi, COA yang dipakai, dan kebijakan
tingkat diskonto untuk kredit lama yang tidak punya suku bunga efektif tersimpan.
**Menghalangi:** pengakuan kerugian restrukturisasi di produksi.

## 3. PD dan LGD untuk CKPN (`ckpn.pd_frac.gol_1`..`ckpn.pd_frac.gol_5`, `ckpn.lgd_frac`)
Sejak 22 Sep 2026 parameternya **diisi sementara** dengan asumsi PD disandera dari bobot PPKA
dibagi LGD 45% (fraksi 0..1; §7.3), supaya CKPN dapat dihitung dalam mode bayangan. Pengisian
permanen menuntut data migrasi sendiri (target 12 bulan data untuk estimasi PD) dan wajib
disetujui bank; kapan `ckpn.enabled` dinyalakan masih menunggu bank.
**Menghalangi:** perhitungan CKPN di produksi dengan parameter yang disetujui bank.

## 4. Cara mencatat penempatan pada bank lain
Sistem belum memodelkan penempatan pada bank lain; yang ada hanya akun GL agregat. Penandanya
sudah tersedia (`lps_placements`) tetapi nilainya diisi bank dan belum direkonsiliasi otomatis
dengan saldo COA. Keputusan yang ditunggu: bagaimana penempatan dicatat, apakah PPKA-nya
diposting ke GL, dan kebijakan plafon LPS (serta apakah kelebihan jaminan ditolak atau dipotong).
**Menghalangi:** Pasal 23 dan Form 05.00/13.00 laporan OJK.

## 5. Kepemilikan database dan kredensial
Kepemilikan database `cbs` **sudah dipindahkan** dari role aplikasi `cbs_app` ke `qouver`
(22 Sep 2026; §7.7), sehingga role aplikasi tidak lagi dapat mengubah struktur di luar migrasi.
Higiene produksi: kepemilikan database sudah berpindah ke `qouver`. Rotasi kata sandi
superadmin serta salinan cadangan offsite **dicabut dari daftar kerja** pada 22 Sep 2026 atas
keputusan pemilik sistem — risiko diterima secara sadar, bukan terlewat. `reset-superadmin.txt`
dan dua berkas `cbs-*.env.bak.*` juga dibiarkan apa adanya dengan alasan yang sama.
**Menghalangi:** tata kelola akses produksi (sisa higiene).

## 6. Kredit yang tertahan di luar push
Commit sengaja ditahan sampai kode dinilai bebas cacat. Push menyentuh produksi (migrasi
up-only, tidak dapat dibatalkan), jadi ini keputusan pemilik sistem, bukan keputusan teknis.

## Butir teknis kecil yang menunggu keputusan
- Kriteria aset baik CKPN huruf (a) dan (b) tidak dapat dinilai karena datanya tidak ada;
  saat ini diperlakukan konservatif.
- CKPN individual (arus kas terdiskonto) belum dibangun karena menuntut estimasi arus kas per
  debitur.
- Kebijakan `as_of` penempatan: satu baris terkini per penempatan, tanpa deduplikasi histori.
- Perlakuan cadangan PPAP/CKPN saat kredit lunas sudah diperbaiki; pola `slog.Warn` lalu jatuh
  ke buku konvensional saat produk tidak terbaca masih ada dan belum diputuskan.

---

# TEMUAN VALIDASI RILIS (belum diputuskan)

## Penghalang: batas transaksi dan ambang persetujuan per peran tidak berlaku
Kode membangun kunci konfigurasi batas sebagai `limit.<peran>.<jenis_transaksi>.<sufiks>`
(`internal/service/limit_service.go`), sedangkan migrasi seed menaruh
`role_limit.<PERAN>.per_transaction`. Karena kunci `limit.*` **tidak pernah di-seed**, penjaga
batas selalu memakai nilai bawaan: **50 juta per transaksi, 500 juta harian, ambang persetujuan
10 juta — untuk semua peran.** Niat yang tertulis di seed (AO 250 juta, Supervisor 500 juta,
Admin tanpa batas) tidak pernah berlaku, dan ambang persetujuan efektif 10 juta menimpa ambang
`maker_checker.*` yang di-seed (setoran 100 juta, transfer/penarikan 50 juta).

Ini kontrol operasional, bukan sekadar salah tampilan: batas peran dan ambang persetujuan adalah
bagian dari pengendalian internal bank. **Menghalangi push sampai diperbaiki atau diputuskan.**
Pilihannya: (a) samakan kunci di kode dengan seed dan pindahkan niat batasnya, atau (b) seed kunci
`limit.*` secara eksplisit lalu hapus kunci `role_limit.*` yang tidak dibaca.

## Perlu diperhatikan
- **18 kunci konfigurasi dibaca kode tetapi tidak di-seed.** Dua di antaranya jatuh ke nol secara
  senyap: `deposit.mudharabah.yield_annual_pct` (bagi hasil nol) dan
  `loan.penalty.rate.daily.per_mille` (denda nol; kini diisi `1` di produksi, §7.1). Lima kunci lain di-seed tetapi tidak dibaca
  kode (kode mati): `auth.password_expiry_days`, `role_limit.{TELLER,AO,SUPERVISOR,ADMIN}`.
- **Satu uji integrasi gagal bila seluruh uji dijalankan pada satu database**
  (`TestIntegrasiOJKFormDaftarDanNPL` meng-assert rasio NPL bank-wide secara eksak, sedangkan uji
  lain mengisi kredit ke database yang sama). Lolos bila dijalankan sendiri. Ini kelemahan
  isolasi uji, bukan tabrakan fitur — perbaiki dengan mengurung asersi pada data uji atau
  memisahkan database per paket.
- **PPAP berjalan sebelum amortisasi** sehingga memakai saldo kerugian pra-amortisasi (selisih
  satu hari). Tidak diubah karena memindahkannya mengubah angka PPAP, bukan sekadar urutan.
- **Perbandingan CKPN masih dapat dipanggil manual di luar tutup hari** dengan `required_ppap`
  basi. Menutupnya butuh penanda run PPAP yang persisten.
- Salinan cadangan **tidak ada di luar VPS**; latihan pemulihan pertama berhasil tetapi tidak
  ada pengujian rutin.

## Sudah aman (terbukti)
- Rantai migrasi dari nol bersih dan idempoten; satu transaksi per berkas; tanpa kegagalan.
- Biner API menyala terhadap database hasil migrasi dan melayani `/healthz` serta login (HTTP 200).
- Cadangan otomatis sudah berjalan (harian 02:00, rotasi 7/28/93 hari, notifikasi), dan latihan
  pemulihan memulihkan database utuh ke target terpisah dengan jumlah tabel, akun, dan migrasi
  yang identik, lalu membersihkannya tanpa mengubah `cbs`.

## 5. Utang terbuka (dipindahkan dari BACKLOG.md, 2026-09-21)

`BACKLOG.md` (catatan kerja awal, 2033 baris) dan `docs/RENCANA_KERJA.md` dihapus
karena statusnya sudah tidak benar — rencana kerja item 1–8 selesai, dan BACKLOG
masih menyatakan batas peran "sudah ditegakkan di jalur transaksi" padahal kunci
konfigurasinya tidak pernah di-seed. Hanya item yang masih terbuka yang dipindahkan
ke sini, setelah diverifikasi ulang ke kode. Item 9 (housekeeping) ada di bagian
"KEPUTUSAN YANG MENUNGGU BANK".

### Diverifikasi masih terbuka

- `docker-compose.yml:16` hanya me-mount migrasi `000001`, sehingga setup lokal tidak
  pernah lengkap. Perlu mount seluruh `packages/db-migrations` atau arahkan ke
  `scripts/migrate.sh`.
- `docker-compose.yml:18-25` masih menjalankan Redis (`cbs-redis`) padahal tidak ada
  kode yang memakainya.
- `apps/api/go.mod:1` module path `cbs-core/apps/core-api` tidak cocok dengan folder
  `apps/api`. Kosmetik, tapi menyesatkan pembaca baru.

### Dinyatakan selesai setelah verifikasi (dulu tercatat sebagai utang)

- Referensi jurnal jatuh ke `time.Now().UnixNano()` bila sequence gagal dibaca:
  tidak ada lagi di kode non-uji.
- Port Caddyfile: sudah `8095`, cocok dengan Quadlet.

### Belum diverifikasi ulang (jangan dianggap selesai)

- `GetByIDs` (`= ANY($1)` dengan `[]uuid.UUID`) diduga belum diuji terhadap database
  nyata; uji `document_service_test.go` hanya memakai stub.
- Akrual **denda** pada kredit NPL (kolektibilitas 3–5): kini dihentikan sama seperti bunga
  (gerbang `IsNPL()`; §6.1). Perilaku historis tidak diubah dan belum diuji ulang.
- Akrual margin syariah (murabahah) belum punya pemetaan; bagi hasil mudharabah diakui
  saat realisasi sehingga tidak diakru.
- Akrual bunga kredit tidak di-backfill: migrasi `000023` sengaja tidak menandai
  angsuran lama.
- Tampilan frontend lanjutan: rincian per-angsuran akruan bunga dan daftar rekening
  dormant lintas halaman.

### Saklar yang sengaja tetap mati

`ppap.collateral.enabled`, `ckpn.enabled`, `loan.restructure.loss.enabled`, modul
Pasal 23 (LPS), dan kebijakan kedaluwarsa kata sandi (`auth.password_expiry_days = 0`,
0 berarti nonaktif).

## 6. Keputusan bank yang menyusul (saran pengembang)

### 6.1 Tarif denda keterlambatan
Satuan: `loan.penalty.rate.daily.per_mille` (per-mille **per hari**). Sejak 22 Sep 2026 diisi `1`
di produksi dengan plafon `loan.penalty.cap_pct = 10`; rincian di §7.1.
- **Fakta yang ditemukan:** akrual denda untuk kredit kolektibilitas 3–5 **belum** dihentikan — kodenya hanya memeriksa status kredit, berbeda dari akrual bunga yang punya gerbang `IsNPL()`. Kalau tarif denda dinyalakan sebelum ini diperbaiki, bank akan mengakui pendapatan denda atas kredit macet. **Sudah diperbaiki**: denda kini memakai gerbang kolektibilitas yang sama dengan bunga (berlaku ke depan; data historis tidak diubah).
- **Saran:** isi 0,5–1 per-mille per hari dengan batas atas (mis. tidak melebihi 10% pokok tertunggak). Angkanya wajib berasal dari **perjanjian kredit bank** — bukan angka regulasi, dan bukan angka yang ditentukan pengembang.
- **Konsekuensi bila kredit sembuh dari NPL:** hari-hari selama masa NPL ikut tertagih saat akrual berjalan lagi (konsisten dengan perilaku akrual bunga). Bank perlu memutuskan apakah itu memang dikehendaki; keputusan 22 Sep 2026: akrual denda **dihentikan saat kredit NPL** (§7.1).
- **Status:** berlaku dengan tarif produksi `1` per-mille/hari dan plafon 10%; tarif final tetap wajib mengikuti **perjanjian kredit bank** (§7.1).

### 6.2 CKPN — mode bayangan
- `ckpn.enabled` tetap `false` (jalur penjurnalan/pengurangan modal inti tidak aktif).
- `ckpn.shadow_mode.enabled` menjalankan perhitungan CKPN dan membandingkannya dengan PPKA **tanpa menjurnal apa pun**.
- **Asumsi yang dipakai dan dilabeli sebagai sementara:** PPKA sebagai lantai; PD per bucket kolektibilitas **disandera dari bobot PPKA dibagi LGD 45%** (fraksi 0..1, §7.3); LGD default dengan mempertimbangkan agunan. Baris yang dihitung per kredit, sehingga potensi pengurang modal inti = Σ max(PPKA − CKPN, 0) per kredit — **bukan** selisih agregat.
- **Akun jurnal:** `ckpn.coa.expense = 50301`, `ckpn.coa.reserve = 10950`; kunci **global** sehingga unit syariah tanpa pemetaan produk memakai akun konvensional (§7.5).
- **Saran:** jalankan sebagai bayangan 2–3 bulan, bandingkan dengan PPKA, baru nyalakan `ckpn.enabled` bila modal inti tahan dan angkanya masuk akal.
- **Keputusan bank:** asumsi sementara di atas **belum disetujui**; kapan `ckpn.enabled` dinyalakan masih menunggu bank (§7.3).

### 6.3 Proyeksi bagi hasil mudharabah
- Nilai contoh pengembang (12,5% per tahun) untuk produk `PMB-MUDHARABAH` **ditarik kembali menjadi 0**; jadwal pembiayaan mudharabah ditolak dengan pesan yang menyebut parameter yang wajib diisi.
- **Alasan:** bagi hasil mudharabah bergantung pada pendapatan/laba **aktual** usaha yang dibiayai dan dibagi menurut nisbah. Angka tetap di awal akad berisiko menyimpang dari akad, sehingga tidak boleh ditetapkan pengembang.
- **Saran:** asumsi proyeksi per jenis usaha ditetapkan bersama **Dewan Pengawas Syariah**, dan bank memutuskan dasar pengakuan pendapatan (realisasi atau proyeksi). Parameter produk kini dapat diubah lewat `PUT /api/v1/products/{code}` tanpa menyentuh database.
- **Keputusan bank/DPS:** masih menunggu arahan proyeksi dan dasar pengakuan; sementara ini
  proyeksi tetap `0` dan produk tanpa proyeksi disetujui DPS ditolak saat penjadwalan bagi hasil
  (§7.6).

---

## 7. Keputusan 22 Sep 2026

Keputusan yang diambil 22 September 2026. Tiap butir menyebut statusnya: **sudah berlaku**
di produksi atau **masih menunggu** persetujuan bank/DPS.

### 7.1 Denda keterlambatan — berlaku

- **Basis:** pokok angsuran yang lewat jatuh tempo (bukan pokok kredit penuh), dihitung per
  angsuran lalu dijumlahkan per kredit, berjalan harian sampai dibayar, dan **dihentikan saat
  kredit NPL**.
- **Parameter produksi:** `loan.penalty.rate.daily.per_mille = 1` (0,1%/hari ≈ 3%/bulan dari
  pokok angsuran tertunggak) dan `loan.penalty.cap_pct = 10` (plafon total denda per kredit
  10% dari pokok angsuran tertunggak; plafon tercapai sekitar hari ke-100; `0` = nonaktif).
- **Tarif final wajib mengikuti perjanjian kredit bank.** Bila perjanjian berbunyi "1% per
  bulan", nilai yang benar adalah **0,33‰/hari**.
- Melengkapi §6.1.

### 7.2 Denda syariah / ta'zir — berlaku sementara, menunggu DPS

- Denda pembiayaan syariah dibukukan ke akun **12500 Dana Kebajikan**, **bukan** akun pendapatan
  bank `40500`; kode menolak pemetaan yang mengkredit akun pendapatan untuk produk syariah.
- Ringkasan tutup hari menampilkan `social_fund_balance` (saldo dana kebajikan) beserta catatan.
- **Menunggu DPS:** apakah ta'zir dikenakan, besarannya, dan kebijakan penyaluran dana kebajikan
  (siapa menyalurkan, seberapa sering, bukti penyaluran).

### 7.3 Asumsi CKPN — berlaku sebagai mode bayangan, menunggu bank

- `ckpn.shadow_mode.enabled = true`, `ckpn.enabled = false` (nol jurnal).
- **Satuan parameter adalah FRAKSI 0..1** (0.01 = 1%), bukan persen.
- PD tiap golongan **disandera dari bobot PPKA** dibagi LGD 45%: `ckpn.pd_frac.gol_1 = 0.0111`,
  `ckpn.pd_frac.gol_2 = 0.0667`, `ckpn.pd_frac.gol_3 = 0.2222`, `ckpn.pd_frac.gol_4 = 1.0`, `ckpn.pd_frac.gol_5 = 1.0`,
  `ckpn.lgd_frac = 0.45`. Dasar bobot PPKA `ppap.rate_frac.gol_1..5 = 0.005 / 0.03 / 0.10 / 0.5 / 1.0`.
- **Akibat:** untuk golongan 1–3 CKPN model **setara PPKA**; selisih hanya muncul di golongan
  4–5 (PPKA 50%/100% melebihi LGD 45%).
- **Asumsi sementara** sampai ada data migrasi sendiri (target 12 bulan data untuk estimasi PD),
  wajib disetujui bank. Kapan `ckpn.enabled` dinyalakan **masih menunggu bank**.
- Memperbarui §6.2.

### 7.4 Perbandingan CKPN kini dua basis — belum menjadi kebijakan bank

- Basis **"sesuai kebijakan"** (aset baik dikecualikan, SEOJK 21/2024 12.3.a.2.a) dan basis
  **"setara PPKA"** (aset baik tetap dinilai dengan model yang sama) dilaporkan berdampingan.
- Selisihnya menjelaskan mengapa potensi pengurang modal inti berbeda. Keduanya **belum** menjadi
  kebijakan bank.

### 7.5 Akun jurnal CKPN — perlu ditinjau sebelum diaktifkan

- `ckpn.coa.expense = 50301`, `ckpn.coa.reserve = 10950`, supaya penjurnalan tidak gagal saat
  `ckpn.enabled` dinyalakan.
- **Risiko yang dilaporkan pengembang:** kunci ini **global**, sehingga unit syariah yang tidak
  punya pemetaan produk akan memakai akun konvensional — perlu ditinjau sebelum CKPN dinyalakan.

### 7.6 Bagi hasil mudharabah — masih menunggu DPS

- Proyeksi pendapatan tetap **0** sehingga tidak ada pendapatan yang diakui; pengakuan idelanya
  saat **realisasi**, bukan proyeksi.
- Produk tanpa proyeksi yang disetujui DPS akan ditolak saat penjadwalan bagi hasil.
- Melengkapi §6.3.

### 7.7 Kepemilikan database produksi — berlaku

- Kepemilikan database `cbs` dipindahkan dari role aplikasi `cbs_app` ke `qouver` (22 Sep 2026).
  - **Higiene produksi:** kepemilikan database sudah berpindah ke `qouver`. Rotasi kata sandi
  superadmin dan salinan cadangan offsite dicabut dari daftar kerja pada 22 Sep 2026 atas
  keputusan pemilik sistem (risiko diterima, bukan terlewat).
- Memperbarui butir 5 "KEPUTUSAN YANG MENUNGGU BANK".
