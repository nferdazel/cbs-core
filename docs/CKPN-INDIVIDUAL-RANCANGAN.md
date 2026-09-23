# CKPN Individual — Rancangan Teknis & Akuntansi

Status: **rancangan, belum dibangun.** `ckpn.enabled = false`; sistem baru menghitung CKPN
kolektif (`CKPN = EAD × PD × LGD`). Panel menilai **CKPN individual (impairment spesifik)
belum ada** dan menyebutnya pekerjaan besar. Dokumen ini merancangnya agar siap dieksekusi
programmer.

Rujukan teknis memakai `berkas:baris`. Rujukan hukum memakai pasal/butir. Teks OJK dibaca
dari `/tmp/ojk/` (tidak ikut dirilis). Berkas yang benar-benar dipakai:

- `pojk1_2024.txt` — POJK No. 1 Tahun 2024 (Kualitas Aset BPR).
- `pojk24_2024.txt` — POJK No. 24 Tahun 2024 (Kualitas Aset BPRS).
- `pa_bpr.txt` — salinan penuh SEOJK No. 21/SEOJK.03/2024 (Panduan Akuntansi Perbankan
  bagi BPR, "PA BPR"), Bab XII.
- `ckpn_93_106.txt` — petikan Bab XII PA BPR (halaman 93–106). Isinya sama dengan
  `pa_bpr.txt`; kutipan di bawah sudah dicocokkan ke `pa_bpr.txt`.
- `ckpn_60_90.txt`, `seojk2_2025.txt`, `pojk5_2015.txt` — pembanding (restrukturisasi,
  KPMM).

Catatan kejujuran: berkas dengan nama `seojk21_*.txt` **tidak ada** di `/tmp/ojk/`; yang
ada hanya petikan `ckpn_93_106.txt` + salinan penuh `pa_bpr.txt`. Karena itu seluruh
kutipan Bab XII diverifikasi dari `pa_bpr.txt`.

---

## 1. Kapan CKPN individual wajib

### 1.1 Dasar kewajiban

- **POJK No. 1 Tahun 2024 Pasal 26** (`pojk1_2024.txt:973-975`):
  > "BPR wajib membentuk CKPN sesuai standar akuntansi keuangan."
  Untuk BPRS, bunyi yang sama di **POJK No. 24 Tahun 2024 Pasal 33**
  (`pojk24_2024.txt:1543-1545`). POJK hanya menyerahkan **metode** ke SAK EP; rinciannya
  ada di PA BPR.
- **Pengurang modal inti**: POJK 1/2024 Pasal 27 (1)–(2) (`pojk1_2024.txt:977-997`) / POJK
  24/2024 Pasal 34 (`pojk24_2024.txt:1547-1562`): bila CKPN < PPKA, selisihnya mengurangi
  modal. Ini **tidak** berubah oleh CKPN individual — `required_ckpn` yang benar sudah
  mencerminkannya.
- **SAK EP Bab 11** (dasar pengaturan PA BPR Bab XII, `ckpn_93_106.txt:262-263`).

### 1.2 Alur tiga langkah (PA BPR butir 12.3)

Sumber: `ckpn_93_106.txt:369-436`.

1. **Langkah Pertama — kriteria aset baik** (12.3.a.1, `ckpn_93_106.txt:373-390`):
   (a) diterbitkan Pemerintah Pusat RI; (b) dijamin LPS; (c) **tidak menunggak lebih dari
   7 hari dan tidak pernah direstrukturisasi**.
   - Memenuhi kriteria → *"BPR dapat tidak membentuk CKPN"* (12.3.a.2.a,
     `ckpn_93_106.txt:402-406`).
   - Tidak memenuhi → lanjut Langkah Kedua.
2. **Langkah Kedua — penilaian signifikansi** (12.3.b, `ckpn_93_106.txt:411-420`):
   - **aset keuangan signifikan → dinilai INDIVIDUAL** (Langkah Ketiga);
   - **tidak signifikan → CKPN KOLEKTIF**.
3. **Langkah Ketiga — penilaian individual bukti objektif** (12.3.c,
   `ckpn_93_106.txt:421-436`):
   - ada bukti objektif penurunan nilai → **CKPN individual**;
   - tidak ada → **CKPN kolektif**.

Tambahan penting yang **memaksa penilaian individual tanpa memandang signifikansi**
(PA BPR butir 12.2.d, `ckpn_93_106.txt:309-322`):
> "BPR menilai aset keuangan berikut secara individual untuk penurunannya: 1) seluruh
> instrumen ekuitas tanpa memperhatikan signifikansinya; dan 2) aset keuangan lainnya
> yang secara individual signifikan."

### 1.3 Bukti objektif penurunan nilai (PA BPR 12.2.b, `ckpn_93_106.txt:271-302`)

Data observasian yang menjadi peristiwa kerugian:

1. kesulitan keuangan signifikan penerbit/obligor;
2. pelanggaran kontrak (gagal bayar / keterlambatan bunga atau pokok);
3. **kreditor memberi konsesi kepada debitur karena kesulitan keuangannya**
   (= restrukturisasi);
4. kemungkinan besar debitur bangkrut atau reorganisasi keuangan;
5. data observasian yang menunjukkan penurunan terukur estimasi arus kas kelompok.

Faktor lain (12.3.c, `ckpn_93_106.txt:304-307`): perubahan teknologi, pasar, lingkungan
ekonomi/legal yang merugikan. Faktor individual (12.4.e.1.(2), `ckpn_93_106.txt:584-600`):
kinerja debitur, kapasitas bayar/arus kas, jenis-jenis jumlah agunan & legalitas, garansi,
prospek usaha. **Frekuensi rollover** juga indikator (12.4.e.1.(3), `ckpn_93_106.txt:601-603`).

Periode evaluasi: **setiap akhir bulan atau paling lambat akhir triwulan**, dan bila ada
bukti objektif sebelum tanggal evaluasi berikutnya, arus kas diestimasi ulang saat itu
(12.4.f, `ckpn_93_106.txt:639-649`).

### 1.4 Ambang kuantitatif: TIDAK ADA di ketentuan → kebijakan bank

Teks menyerahkannya **eksplisit** ke bank (12.4.e.1.(1), `ckpn_93_106.txt:563-576`):
> "BPR menentukan tingkat signifikansi kredit yang akan dievaluasi secara individual ...
> Pada umumnya aset keuangan yang dinilai secara individu dihitung untuk eksposur yang
> besar. **BPR menentukan nilai eksposur besar sesuai dengan kompleksitas usahanya.**"

→ **Tidak ditemukan ambang angka apa pun** di POJK 1/2024, POJK 24/2024, maupun PA BPR.
Usulan berikut adalah **praktik industri**, BUKAN aturan:

- **Ambang signifikansi (praktik):** outstanding ≥ **Rp1.000.000.000** per debitur,
  dan/atau 20 debitur dengan eksposur terbesar. Angka ini harus **dapat diubah lewat
  konfigurasi** (`ckpn.individual.significance_amount`), bukan ditanam di kode.
- **Pemicu individual yang WAJIB apa pun besarnya** (bukan praktik: diturunkan dari
  bukti objektif dan kriteria aset baik):
  1. pernah **direstrukturisasi** (konsesi = bukti objektif; sekaligus gugur kriteria
     aset baik), atau
  2. kolektibilitas **Macet (gol. 5)** / DPD > 90 hari, atau
  3. ada penurunan nilai agunan signifikan, atau
  4. ada bukti objektif lain yang ditandai pengelola kredit.

Pilihan bank harus **terdokumentasi** (12.4.c.1 & 12.4.c.3, `ckpn_93_106.txt:455-532`).

---

## 2. Metode yang layak untuk BPR/BPRS

PA BPR butir 12.4.g.1 menyediakan dua teknik individual (`ckpn_93_106.txt:664-759`).

### 2.1 Nilai kini arus kas (Discounted Cash Flow / DCF)

12.4.g.1.a (`ckpn_93_106.txt:676-719`): kredit yang menurun nilainya dicatat pada
*discounted value*, bukan nilai buku. Nilai kini diperoleh dari estimasi arus kas masa
datang (pokok + bunga) yang didiskonto dengan **suku bunga efektif awal** kredit. Untuk
kredit suku bunga **tetap**, EIR tidak berubah. Untuk suku bunga **mengambang**, bank
"dapat menggunakan suku bunga efektif terkini pada saat terdapat bukti objektif" dan
memakainya untuk evaluasi berikutnya.

### 2.2 Nilai realisasi agunan bersih (net proceed)

12.4.g.1.b (`ckpn_93_106.txt:720-746`): dipakai bila salah satu kondisi berikut:
(a) kredit **collateral dependent** (pelunasan hanya dari agunan); (b) sulit menentukan
jumlah & saat arus kas pokok/bunga dengan andal; dan/atau (c) pengambilalihan agunan
**kemungkinan besar** dan didukung aspek legal. Nilai yang dipakai merujuk pada **harga
pelepasan agunan (net proceed) setelah dikurangi biaya pelepasan**
(`ckpn_93_106.txt:744-746`).

12.4.g.1.c (`ckpn_93_106.txt:748-759`) — **aturan pemilihan**:
> "Dalam hal BPR telah menghitung CKPN individu dengan pendekatan discounted cash flow,
> dan kemudian diperoleh fakta bahwa debitur tidak memiliki kemampuan membayar, maka BPR
> menghitung CKPN individu dengan pendekatan agunan. **CKPN yang dibentuk dengan
> pendekatan agunan minimal sama dengan CKPN yang telah dibentuk sebelumnya.**"

### 2.3 Keputusan metodologi yang diambil

1. **DCF adalah metode utama** bila estimasi arus kas per debitur tersedia dan andal.
2. **Agunan dipakai** bila kondisi 2.2 (a)/(b)/(c) terpenuhi.
3. Bila **keduanya dapat dihitung**, target individual = **yang lebih konservatif**
   (CKPN terbesar). Ini sekaligus memenuhi syarat lantai 12.4.g.1.c tanpa logika
   khusus: `target_individu = max(CKPN_DCF, CKPN_agunan)`.
4. **Lantai mutlak**: bila beralih dari DCF ke agunan, `target_individu` **tidak boleh
   turun** di bawah CKPN individual yang sudah dibentuk sebelumnya (12.4.g.1.c).

### 2.4 Rumus eksplisit

Basis nilai tercatat (sama dengan PPKA & CKPN kolektif, agar tidak ada dua basis):

```
carrying = PPAPCarryingAmount(outstanding_principal, restructure_loss_balance)
         = outstanding_principal − restructure_loss_balance      # domain/ppap.go:145
```

**DCF** (arus kas proyeksi periode 1..n, EIR bulanan):

```
PV_DCF        = Σ_{t=1..n} CF_t / (1 + eir_monthly)^t             # domain PresentValue, restructure_loss.go:221
CKPN_DCF      = max(0, RoundToRupiah(carrying − PV_DCF))
```

**Agunan** (hanya agunan status ACTIVE dan ikatannya sah):

```
NRV_i         = (appraisal_value_i × (1 − haircut_i/100)) − biaya_pelepasan_i
                # appraisal_value & haircut ada di loan_collaterals (000036:45,54);
                # bound_amount sudah = appraisal × (1−haircut) (000036:60)
NRV           = Σ_i NRV_i
CKPN_agunan   = max(0, RoundToRupiah(carrying − NRV))
```

**Target individual dan lantai:**

```
target_individu = max(CKPN_DCF, CKPN_agunan)          # metode tersedia
target_individu = max(target_individu, CKPN_individual_sebelumnya)   # 12.4.g.1.c
```

`target_individu` inilah yang disimpan ke **`loans.required_ckpn`** (satu sumber
kebenaran; lihat §5.4). Pembulatan `RoundToRupiah` mengikuti jalur kolektif
(`domain/ckpn.go:234-235`).

---

## 3. Masukan data: tersedia vs harus disiapkan bank

### 3.1 SUDAH tersedia di sistem

| Masukan | Sumber `berkas:baris` | Catatan |
|---|---|---|
| Sisa pokok, kolektibilitas, DPD, status, pernah restrukturisasi | `repository/postgres/ckpn_repo.go:30-44`; `domain/loan.go:174-274` | Sudah dibaca mesin CKPN |
| Nilai tercatat (pokok − saldo kerugian restrukturisasi) | `domain/ppap.go:145`; kolom `restructure_loss_balance` migrasi `000043:59-60` | Pakai ini, jangan hitung sendiri |
| **Suku bunga efektif orisinal bulanan** | kolom `loans.original_eir_monthly` migrasi `000043:39-43`; dibaca `loan_repo.go:62`; dipakai `restructure_loss_service.go:107-112` | **Ada**, dihitung & disimpan saat pencairan (`storeOriginalEIR`, `restructure_loss_service.go:159-173`) |
| Jadwal angsuran (pokok+bunga, tanggal) | `domain/loan.go:276-306`; `CashFlowsFromSchedules` (`restructure_loss.go:286-292`) | Bisa jadi dasar proyeksi bila asumsi = jadwal berjalan |
| Agunan: taksasi, tanggal, haircut, **bound_amount**, status, ikatan, tunai | `loan_collaterals` migrasi `000036:31-71`, `000040:45-54`, `000087:53-72`; `domain/collateral.go` | `bound_amount` = database generated |
| Umur taksasi kebijakan | `collateral.appraisal.validity_months` (`000090:40-41`; `collateral.go:117`) | Menandai taksasi kedaluwarsa |
| Pemetaan akun CKPN per buku | `ckpn.coa.*` migrasi `000042:78-81`, `000069`, `000071`, `000090` | Syariah `15901`/`11950` |
| Target PPKA & CKPN per kredit | `loans.required_ppap`, `loans.required_ckpn` (`ckpn_repo.go:123-131`) | Pembanding & idempotensi |
| Primitif nilai kini & diskonto | `domain.PresentValue` (`restructure_loss.go:221-236`), `CalculateRestructureLoss` (`:257-281`) | **Dapat dipakai ulang** untuk DCF individual |

### 3.2 HARUS disiapkan bank (belum ada di sistem)

| Kebutuhan | Keadaan jujur |
|---|---|
| **Proyeksi arus kas per debitur** (nominal, waktu, asumsi) | **Tidak ada tabel/kolom.** `loan_schedules` hanya jadwal kontraktual, bukan estimasi pemulihan. Wajib dibangun (tabel baru) + diisi bank dengan *professional judgment* (12.4.e.1.(2), `ckpn_93_106.txt:584-600`) |
| **Biaya pelepasan agunan** (biaya penjualan) | **Tidak ada kolom.** `loan_collaterals` hanya punya `appraisal_value` dan `haircut_percent`; `haircut` adalah kebijakan pengurang PPAP, **bukan** biaya pelepasan. Perlu kolom baru |
| **EIR kredit lama** | Kredit yang cair **sebelum migrasi `000043`** tidak menyimpan EIR (`restructure_loss_service.go:118-131`). Perlu override kebijakan atau backfill |
| **EIR kini untuk suku bunga mengambang** | Hanya EIR orisinal yang tersimpan; PA BPR 12.4.g.1.a mengizinkan EIR terkini. Belum ada |
| **Penanda signifikansi & bukti objektif per kredit** | Tidak ada. `form05.go:78` & `form06.go:156` bahkan mencatat "jenis CKPN individual/kolektif belum disimpan" |
| **Kebijakan ambang signifikansi & metode** | Tidak ada kunci konfigurasi. Perlu ditambah |
| **Akun pemulihan individual (bila pakai 12.9)** | Lihat §5.3 |

---

## 4. Tingkat diskonto

### 4.1 Dasar yang benar

- **Kredit suku bunga tetap:** **suku bunga efektif ORISINAL** (PA BPR 12.4.g.1.a,
  `ckpn_93_106.txt:699-706`), dan tidak berubah selama umur kredit.
- **Kredit suku bunga mengambang:** **suku bunga efektif terkini** pada saat bukti
  objektif (12.4.g.1.a, `ckpn_93_106.txt:707-719`).
- **Jangan** memakai suku bunga kontraktual nominal sebagai pengganti EIR. Preseden yang
  sama sudah ditegakkan jalur kerugian restrukturisasi: `ErrEIRMissing` dan menolak
  berjalan (`restructure_loss_service.go:107-112`).

### 4.2 Cara mengambil dari data yang ada

- Baca `loans.original_eir_monthly` (fraksi/bulan). Sudah tersedia untuk kredit yang
  dicairkan setelah migrasi `000043`; nilainya presisi `numeric(18,10)`
  (`restructure_loss.go:96-97`).
- Bila `original_eir_monthly = 0`: gunakan **override konfigurasi** pola yang sudah ada
  `loan.restructure.loss.discount_rate_annual_pct` (`000043:87-88`), dibaca
  `restructureLossPolicy` (`restructure_loss_service.go:80-92`) dan dipecah
  `persen/tahun ÷ 12` (`:103-106`). Untuk CKPN individual tambahkan kunci sendiri,
  mis. `ckpn.individual.discount_rate_annual_pct`.
- **Bila EIR tidak tersedia dan override kosong** → kredit **gagal dihitung** dengan
  pesan yang menyebut kunci yang harus diisi, **bukan** dihitung dengan nol atau suku
  bunga kontraktual.

### 4.3 Yang perlu ditambah

1. Kolom/key untuk **EIR terkini** kredit mengambang (opsional; bila bank memakai).
2. Override diskonto individual (di atas).
3. Kesadaran bahwa `DisburseLoan` **tidak memotong** provisi/biaya transaksi
   (dicatat pada `EIRBasis.Note`, `restructure_loss.go:325-327`), sedangkan PA BPR
   hlm. 42-43 meminta biaya transaksi diperhitungkan. EIR tersimpan karena itu **hanya
   berasal dari bentuk jadwal** — batas yang harus diungkap, bukan disembunyikan.

---

## 5. Perlakuan akuntansi

### 5.1 Jurnal pembentukan & pemulihan

PA BPR 12.5 (`ckpn_93_106.txt:859-889`) dan ilustrasi 12.9 (`pa_bpr.txt:15845-15855`):

| Peristiwa | Jurnal | Akun |
|---|---|---|
| Pembentukan | Db. Beban kerugian penurunan nilai – kredit | `ckpn.coa.expense` / `.syariah`, fallback `50301` / `15901` |
| | Kr. CKPN – kredit | `ckpn.coa.reserve` / `.syariah`, fallback `10950` / `11950` |
| Pemulihan (12.5.b) | Db. CKPN | akun cadangan |
| | Kr. Beban kerugian penurunan nilai | akun beban (jurnal balik) |
| Pemulihan (alternatif 12.9) | Db. CKPN | akun cadangan |
| | Kr. Pendapatan operasional – Pemulihan CKPN | **akun baru**, belum dibangun |

Urutan pemilihan akun per buku sudah ada dan **tidak perlu diubah**: pemetaan produk
`CKPN_PROVISION`/`CKPN_REVERSAL` lebih dulu, lalu kunci konfigurasi per buku, lalu
fallback (`ckpn_service.go:514-552`, `:679-708`). Untuk instalasi `SYARIAH`/`DUAL`,
`ckpn.coa.expense.syariah`/`ckpn.coa.reserve.syariah` wajib terisi (`000090` mengisi
bawaan `15901`/`11950` bila kosong).

### 5.2 Perbedaan dengan CKPN kolektif

| Aspek | Kolektif | Individual |
|---|---|---|
| Metode | `EAD × PD × LGD` (`domain/ckpn.go:164`) | DCF dan/atau realisasi agunan (12.4.g.1) |
| Parameter | PD per golongan, LGD (`ckpn.pd_frac.gol_*`, `ckpn.lgd_frac`) | EIR, proyeksi arus kas, taksasi & biaya pelepasan |
| Dasar angka | data historis kelompok | penilaian per debitur |
| Jurnal | sama | sama |

Akun jurnalnya **sama** — aturan tidak mewajibkan akun terpisah untuk individual. Yang
wajib dibedakan hanyalah **metode** dan **basis per kredit**.

### 5.3 Larangan penghitungan ganda

- Kredit yang sudah dinilai **individual tidak boleh** dihitung kolektif. Ini mengikuti
  alur 12.3: sekali masuk jalur individual (Langkah Ketiga), ia berhenti di situ; hanya
  yang "tidak signifikan" atau "tanpa bukti objektif" yang masuk kolektif.
- Cara menegakkan: simpan penanda metode per kredit dan **keluarkan** kredit
  individual dari query/loop kolektif. Bila tidak, cadangan akan dihitung dua kali dan
  `required_ckpn` menjadi salah saji.
- **Satu angka per kredit**: `required_ckpn` tetap target tunggal. Mesin kolektif
  **tidak boleh** menimpa target kredit individual. Penulis tunggal `required_ckpn`
  adalah satu fungsi (jembatan yang membedakan metode), bukan dua jalur terpisah.

### 5.4 `required_ckpn` tetap satu sumber kebenaran

- Semua selisih (pembentukan/pemulihan) **tetap dihitung** `target − existing` lalu
  dijurnal dengan kunci idempotensi `CKPN-<no kredit>-<tanggal>-<selisih>`
  (`ckpn_service.go:507-508`). Individual memakai mekanisme yang sama; yang berbeda
  hanya **cara memperoleh `target`**.
- Rekonsiliasi wajib lulus: **saldo GL COA CKPN = Σ `required_ckpn`** (semua kredit,
  kolektif + individual). Uji ini sudah menjadi bagian daftar periksa rilis
  (`docs/CKPN-SIAP-RILIS.md` §e.6).

### 5.5 Hapus buku

Pelepasan cadangan saat hapus buku sekarang mengikuti saklar dan memakai
`required_ppap` sebagai ukuran (`loan_service.go:1335-1341`, `:1376-1419`; catatan di
`docs/CKPN-SIAP-RILIS.md` §c.6). Setelah individual ada, ukuran pelepasan **harus**
memakai `required_ckpn` (termasuk bagian individual), bukan `required_ppap`. Ini
pekerjaan lanjutan yang sudah tercatat sebagai temuan auditor.

---

## 6. Rancangan teknis ringkas

### 6.1 Migrasi (usulan nomor `000092`, aditif & idempotent)

1. **Kolom pada `loans`**:
   - `ckpn_method TEXT NOT NULL DEFAULT 'COLLECTIVE'` — nilai:
     `COLLECTIVE`, `INDIVIDUAL_DCF`, `INDIVIDUAL_COLLATERAL`, `INDIVIDUAL_MAX`,
     `EXCLUDED_ASET_BAIK`.
   - `ckpn_significant BOOLEAN NOT NULL DEFAULT FALSE` (hasil Langkah Kedua).
   - `ckpn_objective_evidence BOOLEAN NOT NULL DEFAULT FALSE` (hasil Langkah Ketiga).
   - `ckpn_individual_target NUMERIC(28,4)` — target individual terakhir (jejak; angka
     resmi tetap `required_ckpn`).
2. **Tabel `loan_ckpn_individual_assessments`** (audit trail): `loan_id`, `as_of`,
   `method`, `carrying_amount`, `present_value`, `collateral_nrv`, `target`, `basis`
   (JSONB: asumsi, arus kas, EIR yang dipakai), `decided_by`, `created_at`.
3. **Tabel `loan_cashflow_projections`**: `loan_id`, `as_of`, `period`, `amount`,
   `source` (mis. `MANUAL`, `SCHEDULE`), `created_by`, `created_at`. Tanpa ini estimasi
   arus kas tidak dapat diaudit.
4. **Kolom pada `loan_collaterals`**: `selling_cost_amount NUMERIC(18,2) NOT NULL DEFAULT 0`
   (biaya pelepasan) — dasar `NRV`.
5. **Kunci konfigurasi** (`system_config`, `ON CONFLICT DO NOTHING`):
   - `ckpn.individual.enabled` = `false` (saklar utama; mati = perilaku sekarang).
   - `ckpn.individual.significance_amount` = `''` (ambang praktik, bank mengisi).
   - `ckpn.individual.method` = `MAX` (`DCF`/`COLLATERAL`/`MAX`).
   - `ckpn.individual.discount_rate_annual_pct` = `''` (override bila EIR kosong).
   - `ckpn.individual.coa.recovery` = `''` (opsional, bila bank pilih pemulihan 12.9
     sebagai pendapatan).
   - `ckpn.individual.mandatory_on_macet` = `true`; `ckpn.individual.mandatory_on_restructured`
     = `true` (pemicu wajib §1.4).

### 6.2 Alur di EOD — rekomendasi

**Rekomendasi: jadikan bagian dari langkah CKPN yang ada (`ckpn_comparison`,
`core`, sequence 50, prasyarat `ppap`), bukan langkah baru.**

Alasan:
- Sumber kebenaran tunggal (`required_ckpn`) hanya dapat dijamin bila individual dan
  kolektif dihitung dalam **satu proses** yang mengetahui daftar kredit mana yang
  individual, sehingga tidak ada kredit dihitung dua kali.
- Urutan yang dibutuhkan sudah terpenuhi: `ppap` (40) sebelum `ckpn_comparison` (50)
  (`000077:115-118`). Individual memakai `required_ppap` dan jadwal/agunan yang sama.
- Menambah langkah baru berisiko salah urutan/prasyarat (kendala yang sama yang
  memaksa W6 menjaga kesetaraan jurnal).

Di dalam `run` (`ckpn_service.go:132`):

1. Baca kebijakan kolektif **dan** individual.
2. Tentukan per kredit: aset baik? signifikan? ada bukti objektif? → `ckpn_method`.
3. Kredit individual: hitung `target_individu` (§2.4), **jangan** lewat rumus kolektif.
4. Kredit kolektif: jalur `CalculateCKPN` yang ada.
5. `apply`/posting: satu mekanisme, `required_ckpn` tunggal.
6. Ringkasan: tambahkan hitungan/total individual supaya EOD melaporkan keduanya.

Bila bank menginginkan visibilitas terpisah tanpa mengubah akuntansi, tambahkan langkah
**non-core** `ckpn_individual_comparison` (kategori `PELAPORAN`, `enabled=false`) yang
hanya membaca; langkah inti tetap satu.

### 6.3 Interaksi dengan mode bayangan

- Mode bayangan (`ckpn.shadow_mode.enabled`) **harus ikut menghitung individual**, tanpa
  jurnal dan tanpa menulis `required_ckpn` — sama seperti kolektif
  (`ckpn_service.go:137-149`; `batch_process_service.go:604-619`).
- Ringkasan bayangan wajib menampilkan: jumlah kredit individual, total `target_individu`,
  serta `ParameterGaps` untuk data yang belum siap (proyeksi arus kas, EIR, biaya
  pelepasan) — **tanpa menebak** nilainya.
- Saat `ckpn.enabled = true`, jalur resmi (`Run`) membentuk & menjurnal, termasuk bagian
  individual.

### 6.4 Daftar periksa uji yang harus lulus

**Unit (tanpa DSN):**
- Ambang signifikansi: tepat di ambang dan di bawahnya → metode benar.
- Pemicu wajib: macet & pernah restrukturisasi → individual tanpa memandang nominal.
- DCF: arus kas diketahui → PV cocok hitung manual (pakai `PresentValue`).
- Agunan: `NRV = taksasi × (1−haircut) − biaya pelepasan`; agunan non-ACTIVE diabaikan.
- `max(CKPN_DCF, CKPN_agunan)` dan lantai 12.4.g.1.c (tidak turun saat beralih metode).
- Pembulatan rupiah; hasil negatif dijepit ke 0.
- EIR kosong & override kosong → `ErrEIRMissing` (bukan nol, bukan suku bunga kontraktual).
- Pemulihan tidak melebihi CKPN yang pernah dibentuk (12.5.b).
- Kredit individual **tidak** masuk loop kolektif (anti-double-count).
- `required_ckpn` satu penulis; `target − existing` idempoten.

**Integrasi (satu database):**
- Setelah `Run`, Σ `required_ckpn` = saldo GL COA CKPN per buku (`10950`/`11950`).
- Jalankan ulang tanggal bisnis sama → jumlah jurnal tidak bertambah.
- Mode bayangan: tidak ada jurnal baru, `required_ckpn` tidak berubah, ringkasan memuat
  angka individual.
- Buku syariah memakai `15901`/`11950`; konvensional `50301`/`10950`; pemetaan produk
  menang lebih dulu.
- EOD: `ppap` gagal → langkah CKPN menolak (perilaku sekarang tetap).
- Regresi seluruh uji CKPN lama lulus; `gofmt` drift tetap 0 (baseline
  `docs/BACKLOG.md`).

---

## 7. Batasan dan urutan pengerjaan

### 7.1 Perkiraan besar pekerjaan: **BESAR**

Bukan karena perhitungannya (primitif `PresentValue`/EIR sudah ada), tetapi karena:
(a) data masukan per debitur belum ada dan harus dibangun + diisi manusia;
(b) alur signifikansi/bukti objektif butuh UI/API & dokumentasi;
(c) integrasi ke EOD, anti-double-count, jurnal, hapus buku, dan pelaporan.

### 7.2 Tahapan yang dapat dirilis bertahap

| Tahap | Isi | Hasil | Risiko | Perkiraan |
|---|---|---|---|---|
| **T0** | Migrasi `000092` (kolom, tabel, kunci config); semua saklar `false` | Struktur siap, **nol perubahan perilaku** | Rendah | kecil |
| **T1** | Kalkulasi DCF individual + baca EIR; mode bayangan hanya membaca | Angka bayangan individual terlihat, tanpa jurnal | Sedang | sedang |
| **T2** | Kalkulasi agunan (`NRV`, biaya pelepasan) + aturan `max`/lantai | Metode agunan lengkap di bayangan | Sedang | sedang |
| **T3** | Alur signifikansi & bukti objektif: input proyeksi arus kas + tandai kredit (API/UI) | Bank dapat mengisi data & memutuskan | **Tinggi** (bagian terbesar) | besar |
| **T4** | Integrasi EOD: individual di dalam langkah CKPN, anti-double-count, jurnal, pemulihan | `ckpn.enabled=true` aman termasuk individual | Tinggi | sedang–besar |
| **T5** | Hapus buku pakai `required_ckpn`; pelaporan Form 05/06 (jenis CKPN) & KPMM | Konsistensi laporan & pelepasan | Sedang | sedang |

**Saran rilis:** T0–T2 dahulu (menambah informasi tanpa mengubah akuntansi), lalu T3 di
produksi sebagai alat kerja, baru T4 menyalakan. Jangan menyalakan `ckpn.enabled` untuk
individual sebelum T3 selesai — tanpa data proyeksi, kredit individual akan gagal
dihitung atau (lebih buruk, bila salah desain) jatuh ke nol.

---

## 8. Keputusan bank/DPS dan hal yang tidak dapat dipastikan

### 8.1 Butuh keputusan bank (dan DPS untuk syariah)

1. **Ambang signifikansi** — angka praktik §1.4 (usulan Rp1 miliar / 20 terbesar).
2. **Pemicu wajib** individual (macet, restrukturisasi, penurunan agunan) — setuju/tidak.
3. **Urutan prioritas metode** dan apakah memakai aturan `max` §2.3.
4. **Akun pemulihan**: jurnal balik beban (12.5.b, dipakai kode sekarang) atau pendapatan
   pemulihan (12.9) — bila 12.9, butuh COA baru dan persetujuan DPS.
5. **Proyeksi arus kas & biaya pelepasan agunan**: siapa yang menetapkan asumsi, seberapa
   sering diperbarui, dan bagaimana didokumentasikan (12.4.c.3).
6. **Kebijakan EIR kredit lama**: override global atau backfill per kredit.
7. **Suku bunga mengambang**: memakai EIR terkini (perlu kolom baru) atau EIR orisinal.
8. **Perlakuan syariah**: kesesuaian dengan PSAK syariah/DPS, terutama standar akuntansi
   keuangan syariah yang dirujuk POJK 24/2024 Pasal 33.

### 8.2 Yang tidak dapat saya pastikan

1. **Angka ambang signifikansi** — tidak ada di ketentuan; saya tandai praktik industri,
   bukan aturan. Bila bank mengisi angka, tanggung jawab dokumentasinya milik bank.
2. **Kelengkapan EIR** — saya memastikan kolom & pengisiannya ada untuk kredit pasca
   `000043`, **tetapi tidak dapat memastikan** berapa banyak kredit produksi yang sudah
   terisi. Perlu diukur dari database bank (bukan host produksi dari sini).
3. **Biaya pelepasan agunan** — tidak ada di skema; usulan kolom baru adalah desain, belum
   ada nilai.
4. **Apakah OJK/auditor menuntut format individual tertentu** (mis. template kertas kerja)
   — teks hanya memberi teknik dan prinsip; format tidak diatur.
5. **Nomor migrasi `000092`** — berdasarkan maksimum saat ini (`000091`); bisa bergeser
   bila ada migrasi lain lebih dulu.
6. **Kutipan `seojk21_*.txt`** tidak tersedia; seluruh kutipan saya ambil dari
   `pa_bpr.txt` (salinan penuh PA BPR). Bila ada perbedaan teks antara petikan dan salinan
   penuh, salinan penuh yang saya pakai.
7. **Tidak menyentuh git, kode, host, atau database** — dokumen ini murni rancangan.
