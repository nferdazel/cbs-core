# Keputusan OJK — Pemilik Mendelegasikan (27 Sep 2026)

Pemilik sistem menyerahkan keputusan produk/regulasi yang tertunda kepada tim build.
Dokumen ini mencatat keputusan, bawaan konservatif, dan cara bank mengubahnya. Prinsip
yang dipegang di seluruh dokumen: **tidak mengarang angka/aturan regulasi**. Bila angkanya
tidak punya rujukan tertulis di repo, keputusannya adalah "sediakan mekanisme + parameter
yang diisi bank, default non-enforcing, laporan menulis `-`".

Rujukan: `docs/CELAH-LAPORAN-OJK.md` (triase), `docs/LAMPIRAN-OJK.md` (sumber sandi),
`docs/KEPUTUSAN.md` (keputusan lama), `docs/CKPN-SIAP-RILIS.md`.

## 1. BMPK

| Aspek | Keputusan | Bawaan | Cara bank mengubah |
|---|---|---|---|
| Basis batas | Batas **nominal per pihak** (`bmpk_limits.max_amount`) otoritatif | — (bank isi) | Isi `bmpk_limits` |
| Batas % modal | Ditunda: basis KPMM & persentase belum ada rujukannya | Tidak dihitung, kolom laporan "belum tersedia" | Dibangun bila Lampiran/POJK tersedia |
| Taksonomi pihak terkait | Teks bebas kebijakan bank | — | Isi `bmpk_related_parties.relationship_type` |
| Istilah resmi | Status internal netral (`DALAM_BATAS`/`MELAMPAUI_BATAS`) | Netral | Dipetakan bila istilah resmi (pelanggaran/pelampauan) ditetapkan |
| Nomor/sandi kolom Laporan BMPK | Tidak dikarang; kolom memakai penamaan internal | Belum tersedia | Dipetakan bila Lampiran diperoleh |

## 2. Jalur tulis data OJK

- **API-first**, bukan SQL/seed jangka panjang. Sudah tersedia:
  - `PUT /api/v1/ojk/loan-codes/{loanId}` (izin `system:config`) — **17 kolom Form 06**
    yang harus diisi bank (sandi inline, tanggal, nominal amortisasi/agunan/kelonggaran).
  - `PUT/GET /api/v1/ojk/placement-codes/{placementId}` + `GET .../reports/ojk/placements`
    (izin `system:config`) — **8 kolom Form 05** penempatan, termasuk baca nilai tersimpan.
  - `PUT /customers/{id}` — Form 06 IX/XVIII/XX.
  - `GET/PUT/DELETE /api/v1/reports/ojk/bmpk/master|related-parties|limits` — pihak
    terkait & batas BMPK (izin `system:config`, teraudit satu transaksi).
  - `GET/PUT/DELETE /api/v1/reports/ojk/kelembagaan/offices|management` — data
    kelembagaan (kantor + direksi/komisaris/pejabat).
  - `GET/PUT/DELETE /api/v1/reports/ojk/off-balance/items` — register rekening
    administratif (komitmen/kontinjensi).
  - `GET/PUT /api/v1/system/ckpn-pabl` (izin `system:config` / `:read`) — enam kunci
    `ckpn.pabl.*` (saklar + PD/LGD per golongan + aturan aset baik), sebelumnya SQL-only.
- Semuanya punya layar di `pengaturan/ojk` (Kelembagaan, Rekening Administratif,
  BMPK, Penempatan, plus OJK Reference Codes yang kini memuat 17 bidang kredit)
  sehingga **28 field** terisi tanpa SQL.
- **SQL/seed hanya untuk backfill sekali**; data berkelanjutan wajib lewat API teraudit.

## 3. CKPN

- `ckpn.enabled` tetap **false** sampai parameter `FINAL` + ratifikasi Direksi/akuntan lengkap
  dan mode bayangan berjalan. **Nilai SEMENTARA dilarang masuk laporan OJK** (aturan panel
  §1.5 — keputusan ini mengikat, jangan dilonggarkan ronde implementasi apa pun).
- `ckpn.pabl.enabled` menyala hanya setelah bank mengisi PD/LGD per golongan **dan**
  kesiapan CKPN utama terpenuhi (parameter `FINAL` + ratifikasi lengkap). Syarat itu
  ditegakkan mesin lewat `PUT /api/v1/system/ckpn-pabl` (gap ditolak `422`) sehingga
  Form 05 kolom XII/XXI tidak pernah membawa nilai dari parameter SEMENTARA.
- Akun pemulihan mengikuti keputusan 12.5.b; akun pendapatan tersendiri tidak dibangun.

## 4. Butir DK (definisi) — diputuskan

| Butir | Keputusan |
|---|---|
| XLVII / Form 05 XX Klasifikasi Aset Keuangan | **Sediakan kolom** (migrasi `000108`), **tanpa auto-klasifikasi**; bank mengisi. |
| XLIV–XLVI CKPN per stage | Tetap **belum tersedia**: stage 1/2/3 ditujukan BPR berpasar modal (SAK Indonesia); instalasi ini SAK EP tanpa model SICR/default. |
| XXXVI Pendapatan Bunga Dalam Penyelesaian | Tetap **belum tersedia**: sistem memakai basis kas untuk NPL; tidak diturunkan dari kolektibilitas (akan menggandakan kolom XXXV). |
| III No. Identitas (NIK/NPWP) | **Tidak dipaparkan** sebagai keluaran laporan (keputusan privasi). |

## 5. Butir KB (kondisional bank) — kebijakan

- Bank **bukan peserta** KUR/LPBBTI/Laku Pandai secara bawaan; partisipasi dikonfirmasi saat
  onboarding. Selama bukan peserta, kolom terkait (VI, XXXIX, XL, XLIII; Laporan Laku Pandai,
  Form 00.15) ditulis `-` dan itu **sah, bukan cacat data**.
- Saklar `bank.participates.*` **sengaja tidak di-seed** sampai modul KUR/LPBBTI/Laku Pandai
  benar-benar dibangun dan ada kode yang membacanya. Kunci config tanpa pembaca hanya menjadi
  sampah; menaruh namanya di string dokumentasi akan meloloskan invarian config secara semu.

## 6. Di luar cakupan build

- **Tarif ta'zir**: milik DPS bank (sistem hanya menyediakan bawaan `0`, plafon penjaga, akun,
  gerbang klausul akad). Jangan diisi mesin.
- **Hapus buku**: sudah ada (POJK 1/2024 Pasal 42–43); gerbang pelepasan cadangan jangan diubah.
- **Dekom**: tidak dimodelkan, bukan objek otomasi.
- **Laporan LX** (publikasi keuangan, bukti pengumuman, TPPU/TPPT sebagai dokumen, Form 00.13):
  berkas manual dari luar sistem.

## 7. Yang TIDAK boleh diotomasi/ditebak mesin

1. Angka PD/LGD sah & status `FINAL` CKPN.
2. Sandi Bank 6 digit APOLO/SPOJK dan nama/sandi kolom resmi Laporan BMPK.
3. Taksonomi pihak terkait & batas BMPK.
4. Istilah hukum "pelanggaran/pelampauan", tarif ta'zir, kebijakan hapus buku/dekom.
5. Klasifikasi SAK EP, stage CKPN, pendapatan bunga dalam penyelesaian.
6. Membuka NIK/NPWP ke keluaran laporan.

## 8. Form 09.00 — Rincian Aset Lainnya

- **Sumber nilai = saldo COA saja**, lewat `COAMapping09Draft` terpisah dari
  `COAMappingDraft` (satu kode COA = satu entri, jadi pemetaan rincian tidak boleh
  menimpa pemetaan neraca). Konsekuensinya total Form 09.00 selalu **sama persis** dengan
  pos Aset Lainnya (`1299000000`) Form 01.00 — dijaga test invarian.
- Pos 1.a (Penempatan pada Bank Lain), 1.c (Surat Berharga), dan 1.d (Lainnya) ditulis
  `-` + alasan: tidak ada akun COA untuk akruan pos itu. **Data Form 05.00/06.00 tidak
  dipakai** meski Form 05.00 punya kolom akruan — memakainya membuat total Form 09.00
  meleset dari Form 01.00, dan kekonsistenan dua form resmi lebih penting daripada
  mengisi satu baris.
- Enam akun baru `10310`–`10360` (migrasi `000114`) menyediakan tempat mencatat premi
  LPS dibayar di muka, uang muka pajak, aset pajak tangguhan, biaya dibayar di muka,
  tagihan asuransi, dan uang muka kegiatan operasional; keduanya juga dipetakan ke pos
  Aset Lainnya Form 01.00.
- Pemetaan tetap `DRAF-BELUM-TERVERIFIKASI` — status yang sama dengan Form 01.00/02.00
  yang sejak lama buildable; verifikasi bank/akuntan tetap diperlukan, tetapi bukan lagi
  penghalang pembangunan form.

## 9. Audit as-of — diputuskan (29 Sep 2026, bertindak sebagai SME)

Empat pertanyaan §8 `CELAH-FORM-OJK.md` sebelumnya ditahan "menunggu praktik bank".
**Keempatnya kini diputuskan tegas** dengan dasar tertulis; tidak ada yang dikembalikan ke
bank sebagai "terserah". 9.1 menemukan invarian tabel yang sudah tertulis dan
menegakkannya di kode.

### 9.1 `lps_placements.as_of` = snapshot posisi per tanggal — **keputusan + satu pertanyaan produk**

- **Fakta kode**: `ListPlacements` memakai `WHERE p.as_of <= $asOf` tanpa dedup
  (`lps_placement_repo.go`). Tabel `lps_placements` **tidak punya kunci unik**, **tidak
  punya jalur tulis INSERT di produksi** (hanya UPDATE per `id`), dan `as_of` bertipe
  DATE. **Tidak ada identitas penempatan** (tidak ada nomor bilyet/nomor rekening
  penempatan; `counterparty_cif` adalah CIF bank lawan, bukan identitas penempatan).
- **Risiko**: bila bank menyimpan baris untuk penempatan yang sama pada dua tanggal,
  `as_of <= asOf` membaca keduanya dan Form 05.00/ringkasan PPKA **dapat menghitung dua
  kali**.
- **Keputusan semantik (tegas)**: `as_of` adalah **posisi per tanggal laporan**; laporan
  OJK memuat posisi ("Baki Debet"), bukan akumulasi riwayat. Ini penetapan cara baca,
  bukan pengarang angka.
- **Mengapa TIDAK langsung `DISTINCT ON`**: tanpa identitas penempatan, dedup hanya bisa
  memakai `(coa_code, counterparty_bank, placement_type)`. Kunci itu **salah untuk
  menggabungkan baris** — dua deposito berjangka berbeda di bank yang sama dengan COA dan
  tipe sama adalah **dua penempatan sah**, akan tersembunyi satu. Menutup bug dengan bug
  baru bukan perbaikan.
- **Koreksi (sumber ditemukan)**: invarian tabel **sudah tertulis** di
  `docs/CKPN-SIAP-RILIS.md`: "satu baris per penempatan, `as_of` adalah keadaan terkini
  (diperbarui bank saat barisnya diubah); histori asesmen CKPN tersimpan di
  `pabl_ckpn_assessments`, bukan dengan menduplikasi baris". Artinya pertanyaan "snapshot
  berulang vs satu baris ditimpa" **tidak perlu diajukan**: jawabannya satu baris ditimpa.
  Dua baris untuk penempatan logis yang sama adalah **pelanggaran invarian**, bukan variasi
  sah.
- **Keputusan (final)**: invarian di atas **ditegakkan di kode**. `ListPlacements` menolak
  pembacaan dengan galat jelas bila mendapati penempatan logis yang sama muncul lebih dari
  sekali, alih-alih menghitung dobel atau menyembunyikan salah satunya. `DISTINCT ON` tetap
  **tidak** dipakai (guard mendeteksi, bukan menggabungkan). Dua penempatan berbeda di bank
  sama tetap dua baris sah dan tidak ditandai.
- **Sisa milik bank**: kapan bank memperbarui `as_of` — bebas; invarian hanya menuntut satu
  baris per penempatan. Kolom identitas penempatan **tidak** diperlukan dan **tidak**
  ditambahkan.

### 9.2 `off_balance_items.status` = keadaan-kini, bukan snapshot per tanggal

- **Keputusan**: baris rekening administratif dilaporkan menurut `status` **saat ini**
  (`AKTIF` pada tanggal laporan); `as_of` menandai tanggal pembukuan baris, bukan
  pemotretan ulang. Tidak ada pemotretan ganda.
- **Dasar**: komitmen/kontinjensi Form 01.01 adalah saldo berjalan yang berakhir saat
  direalisasi/dibatalkan; melaporkan versi historisnya menampilkan komitmen yang sudah
  selesai. Perubahan status adalah peristiwa, bukan mutasi harian.
- **Konsekuensi**: cukup satu baris per komitmen dengan `status` yang diperbarui; riwayat
  sudah dijaga `audit_log`. Tidak ada perubahan kode.

### 9.3 `time_deposits.start_date` tidak boleh ditimpa rollover

- **Keputusan**: `start_date` adalah **tanggal mulai kontrak yang dilaporkan** dan
  **tidak boleh ditimpa** perpanjangan otomatis (rollover). Perpanjangan adalah peristiwa
  baru (baris/kolom tanggal jatuh tempo), bukan penggeseran `start_date`.
- **Dasar**: form DPK memisahkan "tanggal mulai" dan "tanggal jatuh tempo"; menimpa
  `start_date` menghapus fakta kontrak asal dan membuat jangka waktu laporan salah.
- **Sisa milik bank**: kebijakan kapan kontrak dianggap kontrak baru secara hukum; kode
  tidak menebak, hanya tidak menimpa. Tidak ada perubahan kode.

### 9.4 BMPK **bukan** laporan as-of

- **Keputusan**: BMPK dihitung dari **posisi terkini** batas per pihak
  (`bmpk_limits.max_amount`) terhadap baki berjalan; **bukan** form posisi-per-tanggal dan
  **tidak** memakai `as_of`. Penamaan internal netral `DALAM_BATAS`/`MELAMPAUI_BATAS` tetap.
- **Dasar**: BMPK adalah **kepatuhan berjalan** (POJK 1/2024), diperiksa saat pemberian
  kredit, bukan direkonstruksi per tanggal tutup buku. Memaksakan `as_of` akan
  menyiratkan pelanggaran/pelampauan retroaktif yang tidak diminta regulator.
- **Konsekuensi**: ketiadaan `as_of` pada BMPK adalah **benar**, bukan celah. Tidak ada
  perubahan kode.

### 9.5 Form 06.01 — "Likuid | Non Likuid" bukan kolom kesembilan (SELESAI)

- **Inkonsistensi yang dulu ditahan**: sel "Likuid | Non Likuid" tampak di kolom VIII pada
  halaman susunan (`PDF #169`), tetapi penjelasan (`PDF #171`) meletakkannya di bawah kolom
  IV Jenis Agunan. Triase lama menahan `06.01` menunggu konfirmasi OJK.
- **Keputusan (membaca ulang PDF langsung)**: ini **bukan** inkonsistensi yang butuh OJK.
  **Daftar sandi resmi (`PDF #170`) berhenti di kolom VIII**, dan kolom VIII adalah "Nilai
  yang Diperhitungkan untuk PPKA". Penjelasan (`PDF #171`) menulis "IV. Jenis Agunan …
  1. Likuid … 2. Non Likuid". Jadi Likuid/Non Likuid adalah **kategori induk** sandi 3 digit
  Lampiran 01 (`PDF #page 301`: 101–103 Likuid, 201+ Non Likuid), **bukan kolom tersendiri**.
  Sel di halaman susunan adalah artefak tata letak header kolom IV yang membentang.
- **Aksi**: `06.01` dibangun (migrasi `000125` + kolom Form 06.01 pada `loan_collaterals`,
  perakit `form06_01.go`, rute `reports/ojk/agunan`). Kolom IV menyimpan sandi Lampiran 01
  apa adanya; kategori Likuid/Non Likuid **tidak** disimpan terpisah karena turunan digit
  pertama. **Konfirmasi OJK tidak lagi diperlukan.**
- **Sisa milik bank**: mengisi kode register (wajib unik, no-reuse), jenis, alamat, nilai,
  penilai, dan PPKA per agunan.

**Ringkas**: 9.1 = invarian tabel ditegakkan di kode (guard menolak duplikat penempatan
logis; tanpa pertanyaan produk tersisa); 9.2/9.3/9.4 = penetapan semantik tanpa perubahan
kode; 9.5 = `06.01` selesai, inkonsistensi PDF diputuskan sendiri. Verifikasi pemetaan COA &
ratifikasi parameter CKPN tetap milik bank/akuntan dan tidak diklaim selesai di sini.

---

Semua butir implementable kini dikerjakan; tidak ada lagi celah kode dan tidak ada
pertanyaan produk tersisa dari audit as-of — sisa hanya milik bank (verifikasi pemetaan,
ratifikasi parameter CKPN, partisipasi program).
