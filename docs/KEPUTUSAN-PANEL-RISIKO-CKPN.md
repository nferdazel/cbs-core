# Keputusan Panel Risiko — CKPN, Ta'zir, Agunan, CKPN Individual, Identitas Bank

Status: **KEPUTUSAN PANEL — MENGIKAT UNTUK RILIS CBS CORE, KECUALI YANG DISEBUT MILIK BANK DI BAGIAN 5.**
Panel: auditor bank (BPK/KAP), bankir BPR/BPRS, manajemen risiko kredit, akuntan (SAK EP/PSAK 71),
pengawas syariah (DPS).

Dokumen ini menutup hal-hal yang sebelumnya dilempar kembali ke bank. Setiap butir berakhir dengan
**KEPUTUSAN**. Yang benar-benar tidak dapat diputuskan panel hanya ada di **Bagian 5**.

Rujukan hukum memakai pasal/butir dari teks di `/tmp/ojk/` (tidak ikut dirilis) dan berkas
`berkas:baris` pada repositori. Bila dasar tertulis tidak ditemukan, ditandai
**"praktik industri, bukan aturan tertulis"**. Tidak ada kredensial pada dokumen ini.

---

## Koreksi angka lantai PPAP/PPKA (dipakai seluruh dokumen)

Pertanyaan menyebut lantai cadangan minimum **0,5%/1%/5%/15%/50%**. Angka itu **salah untuk BPR/BPRS**.
Angka yang benar (setelah dikurangi nilai agunan) menurut **POJK No. 1 Tahun 2024 Pasal 19 ayat (3)**
(`pojk1_2024.txt:720-738`) dan **POJK No. 24 Tahun 2024 Pasal 24 ayat (3)** (`pojk24_2024.txt:1172-1194`):

| Golongan | Tarif PPKA khusus minimum |
|---|---|
| Lancar | 0,5% (PPKA **umum**, Pasal 19(2)/24(2)) |
| Dalam Perhatian Khusus | **3%** |
| Kurang Lancar | **10%** |
| Diragukan | **50%** |
| Macet | **100%** |

**KEPUTUSAN:** seluruh perhitungan lantai memakai 0,5% / 3% / 10% / 50% / 100%. Angka 1%/5%/15%/50%
tidak dipakai dan tidak boleh muncul di konfigurasi (`ppap.rate_frac.gol_*` sudah benar:
`0.005 / 0.03 / 0.10 / 0.5 / 1.0`).

---

## 1. PD/LGD sementara (provisional) — JANGAN DIKOSONGKAN

### 1.1 Dasar

- **POJK 1/2024 Pasal 26** (`pojk1_2024.txt:973-975`): *"BPR wajib membentuk CKPN sesuai standar
  akuntansi keuangan."* Bunyi sama untuk BPRS di **POJK 24/2024 Pasal 33** (`pojk24_2024.txt:1543-1545`).
- **POJK 1/2024 Pasal 27 ayat (2)** (`pojk1_2024.txt:983-990`): bila CKPN < PPKA, selisihnya menjadi
  **pengurang modal**. Sama untuk BPRS di **POJK 24/2024 Pasal 34 ayat (2)** (`pojk24_2024.txt:1552-1559`).
- **PA BPR (SEOJK 21/SEOJK.03/2024) Bab XII** (`pa_bpr.txt`): PD (12.6, hlm. 99), LGD (12.7, hlm. 103),
  rumus `CKPN = PD × LGD × EAD` (12.8), ilustrasi 12.9.
- PD/LGD **wajib dari data historis bank**; periode observasi minimum **3 tahun** (12.4.g.2.c),
  contoh PD 12.6.1 memakai 12 bulan data untuk penyederhanaan.

### 1.2 Nilai yang DITETAPKAN (fraksi 0..1, bukan persen)

**KEPUTUSAN:** panel menetapkan seperangkat nilai sementara berikut untuk **BPR konvensional dan BPRS
sekaligus** (LGD tunggal *all account*, sesuai PA BPR 12.7.a bila data belum mendukung pengelompokan):

| Kunci | Golongan | Nilai SEMENTARA |
|---|---|---|
| `ckpn.pd_frac.gol_1` | Lancar | **0.0111** |
| `ckpn.pd_frac.gol_2` | DPK | **0.0667** |
| `ckpn.pd_frac.gol_3` | Kurang Lancar | **0.2222** |
| `ckpn.pd_frac.gol_4` | Diragukan | **1.0000** |
| `ckpn.pd_frac.gol_5` | Macet | **1.0000** |
| `ckpn.lgd_frac` | semua golongan | **0.45** |

Dasar pemilihan angka (dinyatakan jujur):

- Angka ini **disandera dari tarif PPKA dibagi LGD 0,45** (`0.005/0.45=0.0111`,
  `0.03/0.45=0.0667`, `0.10/0.45=0.2222`). Untuk golongan 1–3 hasilnya **setara PPKA**.
- Golongan 4–5 PD wajib ≤ 1 (batas matematis), sehingga `EAD × PD × LGD = 45%`, **di bawah** PPKA
  50%/100%. Kekurangan itu ditutup **lantai wajib** §1.3.
- Angka ini **bukan** hasil perhitungan PD/LGD menurut 12.6/12.7 dan **bukan** aturan tertulis:
  ia adalah **praktik industri / jembatan sementara**, dan **statusnya SEMENTARA** (§1.4).
- LGD tunggal 45% dipilih karena data BPR belum mendukung pengelompokan per jenis agunan
  (PA BPR 12.7.a mengizinkan *LGD all account*). Bank dengan data per kategori **boleh** memecah LGD
  kelak setelah ratifikasi.

### 1.3 Lantai wajib (tidak boleh menghasilkan CKPN di bawah lantai PPKA)

**KEPUTUSAN:** selama parameter berstatus SEMENTARA, sistem **wajib** menegakkan lantai:

```
target_ckpn_i = max( EAD_i × PD_gol × LGD , required_ppap_i )
```

- `required_ppap_i` adalah PPKA per kredit yang sudah dihitung jalur PPAP (Pasal 19(3), setelah
  agunan). Basis EAD CKPN sudah dikurangi saldo kerugian restrukturisasi, sama dengan basis PPKA,
  sehingga perbandingan konsisten (Pasal 32 jo. PA BPR Bab 5.2).
- **Alasan ekonomi:** lantai ini **tidak menaikkan modal secara tidak sah**. Bila CKPN = PPKA, beban
  naik tetapi tidak ada pengurang modal; bila CKPN < PPKA, beban turun tetapi selisihnya menjadi
  pengurang modal. Efek total terhadap modal **sama**. Lantai hanya memindahkan angka antara "beban"
  dan "pengurang modal", bukan menambah modal.
- **Pengaman:** server menolak menyalakan `ckpn.enabled=true` bila `required_ppap` basi (gerbang
  tanggal bisnis PPAP sudah ada), dan melaporkan `ParameterGaps` bila PD/LGD tidak lengkap — bukan
  menjatuhkannya ke nol.
- **Setelah ratifikasi:** lantai menjadi pilihan bank lewat kunci `ckpn.floor.ppka_enabled`
  (bawaan `true`, direkomendasikan tetap `true`). Menonaktifkannya wajib persetujuan Direksi + akuntan
  dan dicatat di audit.

### 1.4 Siklus hidup parameter SEMENTARA

**KEPUTUSAN:**

1. **Pelabelan.** Kunci baru `ckpn.parameters.status` = `SEMENTARA` (enum: `SEMENTARA` | `RATIFIED`).
   Selama `SEMENTARA`, setiap ringkasan CKPN/EOD/KPMM **wajib** memuat label teks
   "PARAMETER SEMENTARA — belum disetujui bank/akuntan" (`Assumptions`/`BasisNote` yang sudah ada
   diperluas, bukan diganti).
2. **Batas waktu ratifikasi.** **Maksimal 12 bulan** sejak `ckpn.enabled=true` pertama atau sejak
   pemakaian pertama parameter. Perpanjangan hanya lewat **berita acara Direksi + akuntan bertanggal**,
   maksimal 12 bulan lagi, dan wajib tercatat.
3. **Pemicu peninjauan wajib:** (i) jatuh tempo 12 bulan; (ii) perubahan `ppap.rate_frac.gol_*`;
   (iii) temuan auditor/OJK; (iv) pergerakan NPL/migrasi kolektibilitas melewati ambang kebijakan;
   (v) peninjauan tahunan.
4. **Peringatan otomatis yang WAJIB muncul di sistem selama SEMENTARA:**
   - saat **start server** bila `ckpn.enabled=true` dan `status=SEMENTARA`;
   - pada **ringkasan EOD** tiap hari;
   - pada **laporan KPMM** dan **endpoint CKPN**;
   - peringatan **meningkat (escalating)** bila umur parameter > 12 bulan.
   Peringatan **tidak memblokir** perhitungan bayangan; **memblokir ekspor/laporan OJK** (§1.5).

### 1.5 Boleh dipakai untuk laporan OJK? — TIDAK

**KEPUTUSAN: nilai SEMENTARA ini HANYA untuk internal (mode bayangan, laporan manajemen, uji
sensitivitas). DILARANG dipakai sebagai dasar kolom CKPN pada laporan OJK/APOLO.**

- **Alasan:** Pasal 26 mewajibkan CKPN *sesuai SAK*; SAK EP/PA BPR mewajibkan PD/LGD dari **data
  historis bank** (12.4.g.2.c, 12.6, 12.7). Angka turunan bobot PPKA **bukan** metode itu, sehingga
  laporan berbasis nilai ini adalah salah saji menurut standar. Tidak ditemukan pasal yang
  mengizinkan substitusi tarif PPKA menjadi PD/LGD.
- **Penggantinya:** bank menghitung PD menurut **12.6** (net flow / migration analysis) dan LGD menurut
  **12.7** (expected recoveries / collateral shortfall) dari data historisnya sendiri, atau memakai
  **excel parameter PD/LGD resmi OJK** (PA BPR 12.6 menyebut excel tersedia di situs OJK), lalu
  diratifikasi Direksi + akuntan dan `status` diubah `RATIFIED`. Untuk BPRS, ratifikasi juga oleh DPS.
- **Selama belum ada pengganti:** kolom CKPN pada laporan OJK memakai **PPKA** dengan pengungkapan
  selisih kepada OJK; sistem menolak mengekspor angka CKPN SEMENTARA (`status=SEMENTARA` +
  `ckpn.enabled=true` ⇒ ekspor CKPN diblokir, pesan menyebut penggantinya). `ckpn.enabled=true`
  secara teknis boleh untuk pembukuan internal, tetapi laporan OJK tetap memakai PPKA.

### 1.6 Boleh disesuaikan bank? — Ya, terbatas

- Bank **boleh** mengubah angka PD/LGD kapan saja **asal** tetap SEMENTARA, bersatuan fraksi 0..1,
  dan melalui izin `system:config` + maker-checker + audit (lihat Bagian 5 untuk pola).
- Bank **tidak boleh** menonaktifkan lantai §1.3 atau label SEMENTARA selama periode sementara.
- Setelah `RATIFIED`, nilai milik bank sepenuhnya; panel hanya menuntut dasar perhitungan terdokumentasi.

---

## 2. Ta'zir / dana kebajikan — yang dapat ditetapkan

Dasar tertulis yang ditemukan:

- **POJK 24/2024** mewajibkan ta'zir dan ta'widh **diungkapkan dalam informasi produk sebelum
  penandatanganan akad** (`pojk24_2024.txt:10915` dan `:10935`).
- **Fatwa DSN-MUI No. 17/DSN-MUI/IX/2000** (di luar `/tmp/ojk/`, terverifikasi terpisah): sanksi hanya
  untuk **nasabah mampu yang sengaja menunda**; yang tidak mampu karena *force majeure* **tidak boleh**
  dikenai sanksi; besarnya **disepakati dan dibuat saat akad**; dananya **dana sosial**, bukan
  pendapatan LKS.
- **Akun 12500 "Dana Kebajikan"** ada di bagan akun (`apps/api/internal/service/loan_penalty.go:22-31`)
  dan berjenis **LIABILITY** (kewajiban) — benar, bukan pendapatan.
- Perlakuan saldo mengendap > 3 bulan sudah ada peringatan EOD
  (`batch_process_service.go:38,341-369`).

### 2.1 Yang DITETAPKAN panel (bukan DPS)

1. **Tarif bawaan sistem tetap `0`** (`loan.penalty.rate.daily.per_mille = 0`). Sistem **tidak pernah**
   mengakru ta'zir sebelum DPS bank menetapkan tarifnya. Ketika diisi, nilai rekomendasi kerja
   **1‰/hari** dengan plafon `loan.penalty.cap_pct = 10` — tetapi angka ini **keadaan sistem, bukan
   ketetapan panel**; yang mengikat adalah angka di akad.
2. **Batas atas kewajaran (guard sistem, praktik industri — bukan aturan tertulis):**
   - `loan.penalty.cap_pct` **maksimum 100** (total ta'zir ≤ 100% pokok angsuran tertunggak). Nilai
     di atas 100 ditolak sistem.
   - Tarif harian **tidak boleh melebihi** tarif denda konvensional produk setara (anti-arbitrase).
   - Total ta'zir per pembiayaan **tidak boleh melebihi sisa pokok**.
3. **Kewajiban pencantuman di akad.** **KEPUTUSAN:** sistem **menolak mengakru ta'zir** bila kunci
   `loan.penalty.syariah.akad_disclosed=false` (bawaan `false`). Penetapan `true` oleh pejabat berizin
   dengan bukti nomor/versi akad; tanpa itu, ta'zir = 0 dan alasan dicatat. Ini menegakkan
   POJK 24/2024 B.1 + Fatwa 17/2000 poin 5.
4. **Belum mampu ≠ objek ta'zir.** **KEPUTUSAN:** ta'zir hanya dikenakan bila penilaian kemampuan
   (kriteria DPS) menyatakan "mampu"; force majeure → 0. Penilaian disimpan per nasabah/pembiayaan
   agar dapat diaudit.
5. **Mekanisme distribusi (bawaan sistem, dapat diubah DPS):** frekuensi **sekurang-kurangnya sekali
   per kuartal**; penerima = **dana sosial** (fakir/miskin/yatim/dhuafa/kepentingan umum), **dilarang**
   menjadi pendapatan atau biaya operasional bank; bukti = **berita acara + kuitansi**; laporan ke DPS
   tiap kuartal.
6. **Saldo mengendap.** **KEPUTUSAN:** saldo 12500 yang tidak bergerak **> 2 kuartal (6 bulan)**
   memicu peringatan EOD **tingkat tinggi** (bukan hanya > 3 bulan) dan wajib dilaporkan ke DPS pada
   rapat terdekat. Sistem **tidak pernah** mengakui saldo itu sebagai pendapatan. Bila bank
   memutuskan memindahkan sisa dana ke dana sosial lain, wajib berita acara DPS.
7. **Hanya boleh ditetapkan DPS bank (bukan sistem, bukan vendor):** (a) apakah ta'zir dikenakan;
   (b) besar tarif yang benar-benar ditagih sesuai akad; (c) kriteria "mampu" dan mekanisme
   penilaiannya; (d) penerima dan bentuk penyaluran dana kebajikan; (e) rumusan klausul akad;
   (f) perlakuan atas saldo tak tersalur. Sistem/vendor hanya menetapkan: bawaan 0, plafon penjaga,
   akun tujuan, peringatan, dan pelaporan.

### 2.2 Perbaikan `docs/draft-surat-keputusan-dps-tazir.md` (dan alasannya)

| Bagian | Masalah | Perbaikan |
|---|---|---|
| **Pasal 1.3** | Menyebut "1‰/hari setara ±3%/bulan" seolah tetap; 1‰/hari = 3% hanya untuk bulan 30 hari, dan angka wajib mengikuti akad | Tambahkan: nilai **wajib sama dengan klausul akad**; bila akad "1%/bulan" maka **0,33‰/hari**; 3% hanya ilustrasi 30 hari |
| **Pasal 1.5** | "Akrual dihentikan saat NPL" diletakkan sebagai aturan syariah, padahal syariah menghentikan karena **tidak mampu**, bukan karena NPL | Ganti: penghentian ta'zir bila **tidak mampu/force majeure** (kriteria DPS); gerbang NPL hanyalah **pengaman sistem**, bukan pemenuhan syarat syariah |
| **Pasal 1.6–1.7** | Dibiarkan `[PILIHAN]` | **Diputuskan**: ta'zir hanya untuk mampu-yang-menunda; pembiayaan direstrukturisasi **dikecualikan selama masa restrukturisasi** (konsesi = nasabah tertekan) |
| **Pasal 2.2** | Menyebut akun ditentukan "pemetaan produk … dibaca dari konfigurasi `loan.penalty.syariah.social_fund.coa`", kurang tepat | Perjelas: akun mengikuti **pemetaan jurnal EVENT `loan_penalty`**; kunci config adalah **bawaan** tujuan (12500); kode **menolak** pemetaan ke akun pendapatan (40500) untuk syariah (`loan_penalty.go:250`) |
| **Pasal 3.1–3.5** | Seluruhnya `[PILIHAN]` | Diisi **bawaan** §2.1(5)–(6); nilai dapat diubah **hanya oleh DPS** bank |
| **(baru) Pasal 1a** | Belum ada kewajiban pencantuman di akad | Tambahkan pasal: ta'zir **tidak diakru** tanpa klausul akad terkonfirmasi (POJK 24/2024 B.1; Fatwa 17/2000 poin 5) |

**KEPUTUSAN:** draf DPS direvisi sesuai tabel di atas sebelum ditandatangani; selama belum ditandatangani,
tarif tetap **0** dan ta'zir tidak diakru.

---

## 3. Batas kesenjangan agunan & aktivasi bobot agunan

### 3.1 Daftar periksa objektif & dapat diperiksa mesin

Dasar bobot: **SEOJK No. 2/SEOJK.03/2025 Lampiran II** — emas 15% (butir 5.a/hlm. 5), tanah/bangunan
ber-hak tanggungan 30% (`seojk2_2025.txt:446-450`), tanah tanpa beban 50% (`:600-602`), UMK 70%
(`:603-633`), kendaraan ber-hipotek 70% (`:634-653`), lainnya/jatuh tempo/macet 100% (`:654-677`),
sengketa 100% (`:712-716`). Umur taksasi **tidak diatur** teks ini → **praktik industri, bukan aturan
tertulis** (lihat `collateral.go:110-122`).

**KEPUTUSAN — satu agunan boleh memakai bobot < 100% hanya bila SEMUA benar:**

| # | Syarat | Diperiksa mesin dari |
|---|---|---|
| C1 | `binding_type IS NOT NULL` | `loan_collaterals.binding_type` |
| C2 | `insurance_expiry_date IS NOT NULL AND insurance_expiry_date >= business_date` | `MissingLampiranIIFields` + cek tanggal |
| C3 | `appraisal_valid_until IS NOT NULL AND NOT AppraisalExpired(asOf, validity_months)` | `collateral.go:453-492` |
| C4 | `disputed = false` | `loan_collaterals.disputed`; bila `true` → **wajib 100%** (SEOJK 2/2025 angka 8) |
| C5 | Kategori `(collateral_type, binding_type)` ada di `collateral_lampiran_ii_weights` dengan `official_weight_frac` terisi | tabel `000087` |
| C6 | Tidak ada bagian kredit yang **tidak dijamin agunan** yang ikut diturunkan (SEOJK 2/2025 angka 7) | eksposur − Σ nilai agunan; sisa wajib 100% |
| C7 | `applied_weight_frac = official_weight_frac` untuk SEMUA baris | tabel `000087`; nilai lain **ditolak** |
| C8 | Ada **kebijakan Direksi bertanggal** yang menyetujui bobot | prasyarat maker-checker + catatan audit (nomor/tanggal surat) |
| C9 | Aktivasi lewat izin **`system:config`** dengan **maker-checker** dua orang | `router.go:379`, service konfigurasi + audit |

`MissingLampiranIIFields` saat ini menandai `binding_type`, `insurance_expiry_date`,
`appraisal_valid_until`, `appraisal_expired` (`collateral.go:477-492`). **KEPUTUSAN:** C2 ditambah cek
kedaluwarsa polis (bukan hanya NULL), dan C4 wajib mengesampingkan bobot apa pun.

### 3.2 Lama mode bayangan sebelum aktivasi

**KEPUTUSAN: minimal 2 bulan bisnis penuh (dua siklus EOD + dua laporan bulanan).** Selama itu ATMR
dihitung **dua kali**: (a) semua bobot kredit 100% (baseline sekarang) dan (b) kandidat bobot resmi.
Selisih ATMR/rasio KPMM **wajib dilaporkan**, bukan hanya disimpan.

### 3.3 Bila sebagian data tidak lengkap — per agunan, tetap 100%

**KEPUTUSAN: aktivasi bersifat PER AGUNAN (per collateral), dan data tidak lengkap → agunan itu tetap
100%.** Mekanismenya sudah ada: `CollateralRiskWeightFrac` mengembalikan 100% bila
`MissingLampiranIIFields` tidak kosong (`collateral.go:502-509`). Karena itu:

- Tidak ada satu pun jalur "per bank serentak" yang menurunkan semua bobot sekaligus. Satu agunan gagal
  syarat ⇒ hanya agunan itu tidak mengurangi eksposur.
- **Pagar anti-cherry-picking:** sebelum sebuah **kategori** (`enabled=true`) dinyalakan, maker-checker
  wajib memverifikasi **≥ 90% dari nilai agunan aktif** sudah lolos C1–C4. Bila belum, kategori
  ditolak aktivasi (mencegah bobot rendah hanya dipakai pada kredit yang menguntungkan).
- Bagian kredit yang **tidak dijamin** agunan tetap 100% (C6).

### 3.4 Tanggung jawab bila bobot menurunkan ATMR secara tidak sah

**KEPUTUSAN:** tanggung jawab hukum ada pada **Direksi/Bank** (kebenaran data agunan dan kebijakan
bobot; laporan KPMM adalah tanggung jawab bank). Tanggung jawab internal ada pada **pembuat (maker)**
dan **penyetuju (checker)** aktivasi, terekam di `audit_logs`. **Vendor tidak bertanggung jawab** atas
angka, tetapi sistem **wajib**: menolak `applied_weight_frac ≠ official_weight_frac`, menolak aktivasi
tanpa C8–C9, mencatat siapa/kapan, dan menampilkan 100% untuk agunan tidak lengkap. Sistem **tidak
boleh** menurunkan ATMR tanpa kebijakan Direksi bertanggal.

---

## 4. Cakupan & urutan CKPN individual

Dasar: PA BPR **12.3** (tiga langkah), **12.2.d** (individual wajib untuk ekuitas dan eksposur
signifikan), **12.4.e.1.(1)** (ambang signifikansi ditentukan bank), **12.4.g.1.a/b/c** (DCF EIR
orisinal; agunan *net proceed*; lantai CKPN agunan ≥ CKPN sebelumnya).

### 4.1 Ambang kuantitatif rilis pertama — DISETUJUI, dengan syarat

**KEPUTUSAN: setujui usulan Rp1.000.000.000 per debitur dan/atau 20 debitur eksposur terbesar**, sebagai
kunci konfigurasi (bukan ditanam di kode):

- `ckpn.individual.significance_amount = 1000000000`
- `ckpn.individual.significance_top_n = 20`
- `ckpn.individual.enabled = false` pada rilis pertama.

**Alasan:** PA BPR 12.4.e.1.(1) **menyerahkan eksplisit** ke bank dan mencontohkan "eksposur yang besar";
tidak ditemukan angka di POJK/PA BPR → **praktik industri, bukan aturan tertulis**. Ambang wajib
ditinjau tahunan dan bukan alasan menunda T1.

### 4.2 Pemicu non-nominal yang WAJIB (tanpa memandang nominal)

**KEPUTUSAN:** kredit wajib masuk jalur individual bila salah satu benar (bukan praktik — diturunkan
dari bukti objektif PA BPR 12.2.b/12.3.c dan kriteria aset baik 12.3.a.1.c):

1. **Macet (golongan 5)** atau DPD > 90 hari;
2. **pernah direstrukturisasi** (konsesi = bukti objektif, sekaligus gugur aset baik);
3. **penurunan nilai agunan signifikan**;
4. **bukti objektif lain** yang ditandai pengelola kredit.

Untuk pemicu 1 dan 2, bila input DCF belum ada, hitung dengan **pendekatan agunan (12.4.g.1.b)** yang
otomatis dari data agunan — **jangan** jatuh ke nol. **KEPUTUSAN:** `target_individu =
max(CKPN_DCF, CKPN_agunan, CKPN_individual_sebelumnya)` memenuhi lantai 12.4.g.1.c.

### 4.3 Urutan rilis dan alasan

**KEPUTUSAN:**

| Urutan | Tahap | Alasan (nilai risiko vs biaya) |
|---|---|---|
| 1 | **T0** (migrasi kolom/tabel/kunci config; semua saklar `false`) | Nol perubahan perilaku; prasyarat mutlak, biaya kecil |
| 2 | **T1** (DCF individual + baca EIR; **mode bayangan baca-saja**) | Memakai data yang **sudah ada** (`original_eir_monthly`); nilai diagnostik tinggi, biaya sedang |
| 3 | **T2** (agunan `NRV` + biaya pelepasan + aturan `max`/lantai) | Otomatis dari data agunan; menutup kredit macet yang DCF-nya sulit |
| 4 | **T3** (alur signifikansi + bukti objektif + input proyeksi arus kas) | Biaya terbesar; baru diperlukan agar bank dapat memutuskan |
| 5 | **T4** (integrasi EOD + anti-double-count + jurnal) | Hanya setelah T3; jangan nyalakan `ckpn.enabled` individual sebelum T3 |
| 6 | **T5** (hapus buku pakai `required_ckpn`; Form 05/06 & KPMM) | Konsistensi laporan |

### 4.4 Data wajib sebelum T1 boleh dirilis

**KEPUTUSAN:** T1 boleh dirilis (mode bayangan) hanya bila:

1. Migrasi T0 sudah terpasang: kolom `ckpn_method`, `ckpn_significant`, `ckpn_objective_evidence`,
   `ckpn_individual_target`; tabel `loan_ckpn_individual_assessments`,
   `loan_cashflow_projections`; kolom `loan_collaterals.selling_cost_amount`.
2. Kunci config ada: `ckpn.individual.enabled`, `.significance_amount`, `.method` (`MAX`),
   `.discount_rate_annual_pct`, `.mandatory_on_macet`, `.mandatory_on_restructured`.
3. `loans.original_eir_monthly` terbaca untuk kredit pasca migrasi `000043`; bila kosong dan override
   `ckpn.individual.discount_rate_annual_pct` kosong → **`ErrEIRMissing`**, bukan nol dan bukan suku
   bunga kontraktual (pola `restructure_loss_service.go:107-112`).
4. Mekanisme input **proyeksi arus kas manual** tersedia (API/UI) — inilah data operasional bank.

### 4.5 Proyeksi arus kas = data operasional, bukan keputusan

**KEPUTUSAN tegas:** proyeksi arus kas debitur adalah **data operasional bank**, bukan objek keputusan
panel. **T1 wajib jalan dengan input manual** dan **tidak boleh** menggantung menunggu "keputusan".
Panel sudah memutuskan metode (DCF EIR orisinal + agunan `net proceed`), ambang, dan pemicu; sisanya
diisi bank lewat API/UI. Kredit individual yang datanya belum siap **dilaporkan gagal dengan alasan**,
bukan dihitung nol.

---

## 5. Nama badan hukum & data hanya-bank (HANYA bagian ini milik bank)

Daftar ini **hanya** berisi hal yang benar-benar tidak dapat diputuskan panel. Sisanya sudah diputuskan
di Bagian 1–4. Tidak ada nilai identitas yang boleh dikarang sistem.

| # | Data | Keadaan sistem | Bila kosong: apa yang terjadi | Aman? | Boleh rilis? | Nilai tempat sementara | Peringatan wajib |
|---|---|---|---|---|---|---|---|
| 1 | `bank_profile.bank_name` (nama PT/badan hukum) | Ada kolom, **produksi kosong** | `app-info` memakai `branding.display_name`; dokumen mencetak penanda "profil bank belum dikonfigurasi" | **Ya**, selama penanda eksplisit & laporan identitas OJK belum dikirim | **Ya** | UI: `branding.display_name`; **dokumen/laporan: TIDAK boleh** pakai branding | Log start + penanda dokumen + halaman Identitas Bank |
| 2 | `bank_profile.npwp`, `address`, `city`, `phone` | Ada kolom, kosong | Dokumen/laporan identitas bolong | **Ya**, sama seperti #1 | **Ya** | Kosong + penanda "belum diisi" | Penanda dokumen |
| 3 | Akta pendirian + tanggal, SK Kemenkumham | **Tidak dimodelkan** | Form 00.00 "Informasi Pokok BPR" tidak dapat diisi | **Tidak** untuk laporan identitas OJK | Core: **Ya**; laporan identitas: **Tidak** | Tidak ada; wajib diisi/ekstensi `bank_profile` | Modul laporan identitas menandai **INCOMPLETE** dan menolak kirim |
| 4 | Nomor & tanggal izin usaha OJK | **Tidak dimodelkan** | idem #3 | **Tidak** untuk laporan identitas OJK | Core: **Ya**; laporan identitas: **Tidak** | Tidak ada | idem #3 |
| 5 | Nama & jabatan anggota DPS | **Tidak dimodelkan** | Draf surat keputusan DPS tetap kerangka; dokumen syariah tanpa nama penanda tangan | **Ya** (dana kebajikan tarif 0, tidak ada akibat finansial) | **Ya** | Placeholder `[nama anggota DPS]` — **tidak boleh** dicetak sebagai keputusan berlaku | Draf berlabel "RANCANGAN — BELUM DISETUJUI" |
| 6 | Besaran ta'zir final & kebijakan penyaluran dana kebajikan | Config ada, tarif bawaan **0** | Ta'zir tidak diakru; saldo 12500 mengendap | **Ya** (konservatif, tanpa pendapatan) | **Ya** | `0` (bukan tebakan) | Peringatan EOD saldo mengendap |

**KEPUTUSAN:** butir 1, 2, 5, 6 **boleh rilis** dalam keadaan kosong karena sistem gagal-aman (tidak
mengarang identitas, tidak mengakui pendapatan). Butir 3 dan 4 **boleh rilis untuk core banking**,
tetapi **modul laporan identitas OJK wajib menolak kirim** sampai diisi. Tidak ada nilai tempat yang
aman untuk nama badan hukum, akta, nomor izin, atau nama DPS: sistem **tidak boleh** menebaknya.

---

## Penutup

Seluruh butir di Bagian 1–4 adalah **keputusan** dan siap dieksekusi/diverifikasi mesin. Milik bank
hanya Bagian 5 (identitas badan hukum/DPS) dan, di dalam Bagian 2, penetapan angka syariah oleh DPS
bank — bukan sistem, bukan vendor. Dokumen ini **tidak** mengubah kode, tidak menyentuh git, dan tidak
menyentuh host/database produksi.
