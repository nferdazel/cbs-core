# Transkrip Struktur Form OJK — untuk implementasi A3
Sumber: SEOJK No. 16/SEOJK.03/2024 (PDF resmi, 528 hlmn) di `/tmp/ojk/s16b.pdf` (PyMuPDF/fitz, halaman 1-based `PDF #n`; halaman cetak = idx−51 = `PDF #n − 52`).
Status: BACA-SAJA. Tidak ada perubahan kode/ repo.

## Konvensi
- **`PDF #n`** = nomor halaman PDF 1-based (fitz `n-1`).
- **cetak** = angka di kaki halaman PDF (terverifikasi: `PDF #193`→`- 141 -`, `PDF #182`→`- 130 -`, `PDF #191`→`- 139 -`).
- Setiap form daftar/rincian punya halaman berurut: **`FORM xx.00 – 1`** (judul + kepala tabel), **`– 2` (SANDI …)** (tabel sandi/kolom), **`– 3` (PENJELASAN …)**. Pemisahan ini menentukan di mana nama kolom vs aturan pengisian.
- Roman (I, II, …) adalah **sandi kolom** yang tercetak di header.

---

# 1. FORM 08.00 — Daftar Aset Tetap, Inventaris, dan Aset Tidak Berwujud
- Nama persis: **FORM 08.00 – Daftar Aset Tetap, Inventaris, dan Aset Tidak Berwujud** (`PDF #182`, cetak 130).
- **Rentang sebenarnya: `PDF #182–186`** — bukan 182–184. Form di `#182`; sandi kolom di `#183`; penjelasan di `#184–186` (terpotong di akhir 184, lanjut 185–186). `#60` (cetak 8) hanya **daftar isi form** ("11. Form 08.00 …"), bukan isi form.

### Kolom (9 kolom, urut)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Jenis Aset | sandi (pilihan) |
| III | Sumber Perolehan | sandi (pilihan) |
| IV | Status Aset | sandi (pilihan) |
| V | Biaya Perolehan | angka (rupiah penuh) |
| VI | Akumulasi Penyusutan/Amortisasi | angka (rupiah penuh) |
| VII | Akumulasi Kerugian Penurunan Nilai | angka (rupiah penuh) |
| VIII | Nilai Tercatat | angka (rupiah penuh) |
| IX | Metode Pengukuran | sandi (pilihan) |
Sumber: `PDF #182` (header), daftar sandi `PDF #183`.

### Sandi baris/kolom tetap
**II. Jenis Aset** (`PDF #183`, penjelasan `PDF #184`):
1. Aset Tetap dan Inventaris — a. Tanah **101**; b. Bangunan **102**; c. Peralatan dan perlengkapan **103**; d. Kendaraan **104**; e. Lainnya **199**
2. Aset Tidak Berwujud — a. Program aplikasi (software) **201**; b. Goodwill **202**; c. Lainnya **299**

**III. Sumber Perolehan**: 1. Sewa Pembiayaan **01**; 2. Modal Disetor **02**; 3. Modal Sumbangan **03**; 4. Sumber Perolehan Lainnya **99**.

**IV. Status Aset**: 1. Dijaminkan **1**; 2. Tidak Dijaminkan **2** (`PDF #183`).

**IX. Metode Pengukuran**: 1. Model Biaya **1**; 2. Model Revaluasi **2** (`PDF #183`).

### Sifat baris
Per-**kombinasi jenis aset / sumber perolehan / status / metode pengukuran**, bukan per-nasabah dan bukan satu baris tetap per jenis. Ada baris **JUMLAH** (`PDF #182`). Aturan penggabungan: *"BPR dapat menggabungkan pelaporan aset tetap dan inventaris serta aset tidak berwujud yang memiliki kesamaan sandi rincian dan angka pada kolom I sampai dengan kolom VI."* (`PDF #184`, cetak 132).

### Aturan penting (kutipan)
- Definisi aset: *"Aset tetap dan inventaris yaitu aset berwujud yang dimiliki BPR dan digunakan dalam kegiatan operasional untuk periode lebih dari 1 (satu) tahun. Aset tidak berwujud yaitu aset nonmoneter … tidak mempunyai wujud fisik …"* (`PDF #184`).
- **Status Aset**: *"Kolom ini dikosongkan untuk aset tidak berwujud."* (`PDF #185`, cetak 133 — penjelasan IV).
- **Biaya Perolehan**: *"…Dalam hal aset telah dilakukan revaluasi, nilai yang dilaporkan pada kolom ini merupakan nilai aset setelah revaluasi."* (`PDF #185`).
- **Nilai Tercatat**: *"…biaya perolehan dikurangi akumulasi penyusutan atau amortisasi dan kerugian penurunan nilai."* (`PDF #185`).
- **Penurunan nilai** (VII): hanya diisi *"Jika terdapat bukti objektif terjadinya penurunan nilai aset…"*; kerugian *"dapat dipulihkan kembali maksimum sampai dengan biaya perolehan atau nilai revaluasi awal bersih dari penyusutan…"* (`PDF #185`).
- Tidak ditemukan aturan "hanya posisi Desember" pada form ini.

### Kaitan posisi keuangan
`PDF #106` (cetak 54): pos aset tetap/inventaris *"Pos ini dirinci pada Form 08.00 – Daftar Aset Tetap, Inventaris dan Aset Tidak Berwujud."*

---

# 2. FORM 10.00 — Rincian Liabilitas Segera
- Nama persis: **FORM 10.00 – Rincian Liabilitas Segera** (`PDF #191`, cetak 139).
- Rentang: form `PDF #191`; penjelasan `PDF #192` (cetak 140). `PDF #106` (cetak 54) adalah penjelasan **pos Liabilitas Segera di laporan posisi keuangan**, bukan form: *"Pos ini dirinci pada Form 10.00 – Rincian Liabilitas Segera."*

### Kolom (4 kolom, urut)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Nama Rekening | teks/nama pos |
| III | Sandi | sandi pos (10 digit) |
| IV | Jumlah | angka (rupiah penuh) |
Sumber: `PDF #191`.

### Struktur baris: TETAP (daftar pos baku, bukan per-nasabah)
Baris + sandi (dari form `PDF #191`, definisi `PDF #192`):
| Nama Rekening | Sandi |
|---|---|
| Liabilitas kepada Pemerintah yang Harus Dibayar | 2101010000 |
| Sanksi Liabilitas Membayar kepada Otoritas yang Belum Dibayarkan | 2101020000 |
| Titipan Nasabah | 2101030000 |
| Kredit yang Diberikan Bersaldo Kredit | 2101040000 |
| Dividen yang Belum Dibayarkan | 2101050000 |
| Selisih Lebih Hasil Penjualan Agunan Milik Nasabah | 2101060000 |
| Imbalan Kerja | 2101070000 |
| Lainnya | 2101990000 |
| JUMLAH | — |

### Aturan penting (kutipan)
- Definisi umum: *"Liabilitas segera yaitu liabilitas BPR yang telah jatuh tempo dan/atau yang dapat segera ditagih oleh pemiliknya dan harus segera dibayar. Seluruh pos liabilitas segera diisi dalam rupiah penuh."* (`PDF #192`).
- Pos 1 (*kewajiban kepada pemerintah*) = PPh badan terutang, pajak final bunga tabungan/deposito, PPh 21, *"untuk periode sebelum bulan laporan yang dibayarkan pada bulan laporan."* (`PDF #192`).
- Pos 2 = sanksi administratif/denda yg sudah disampaikan otoritas via surat pemberitahuan namun belum dibayar.
- Pos 3 *Titipan Nasabah* = antara lain pengurusan asuransi, biaya notaris, kiriman uang, setoran tak teridentifikasi.
- Pos 4 = kredit **bersaldo kredit** akibat kelebihan pembayaran pelunasan.
- Pos 8 *Lainnya* = antara lain iuran air/listrik/telepon, dana penerusan kredit yang belum dikembalikan. **Tidak ada** aturan redirect ke form lain (berbeda dari 09.00/14.00), dan tidak ada aturan "Desember".

---

# 3. FORM 11.00 — Daftar Tabungan
- Nama persis: **FORM 11.00 – Daftar Tabungan** (`PDF #193`, cetak 141).
- Rentang: form `#193–194`; sandi `#195–196`; penjelasan `#197–199` (cetak 145–147).

### Kolom (18 kolom, urut)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | ID Pihak Lawan | sandi unik |
| III | No. Rekening | teks angka/huruf |
| IV | Jenis | sandi pilihan (10/20) |
| V | Hubungan dengan Bank | sandi pilihan (12/20) |
| VI | Golongan Nasabah | sandi (Lampiran 02) |
| VII | Lokasi Nasabah | sandi (Lampiran 03) |
| VIII | Jangka Waktu | tanggal (2 sub) |
| IX | Suku Bunga | angka (%) |
| X | Nominal | angka (rupiah penuh) |
| XI | Nominal yang Diblokir/Dijaminkan | angka (rupiah penuh) |
| XII | Alasan Diblokir | sandi pilihan (01/02/99) |
| XIII | Biaya Transaksi Belum Diamortisasi | angka (rupiah penuh) |
| XIV | Jumlah | angka (rupiah penuh) |
| XV | Nomor Identitas | teks (NIK/Paspor/KITAS/NPWP) |
| XVI | Jenis PEP | sandi pilihan (1/2) |
| XVII | Risiko Nasabah | sandi pilihan (1/2/3) |
| XVIII | Status Data | sandi pilihan (1/2) |
Sumber: header `PDF #193` (I–VIII) + `PDF #194` (IX–XVIII); sandi `PDF #195–196`.
Catatan: kolom **VIII Jangka Waktu** terdiri 2 sub-header **Tanggal Mulai** dan **Tanggal Jatuh Tempo** (format `TT-BB-TTTT`), tetapi tetap satu sandi kolom VIII.

### Sandi tetap
- **IV Jenis**: 1. Tabungan yang dapat ditarik sewaktu-waktu **10**; 2. Tabungan berjangka **20** (`PDF #195`).
- **V Hubungan dengan Bank**: 1. Terkait **12**; 2. Tidak Terkait **20** (`PDF #195`).
- **XII Alasan Diblokir**: 1. Escrow Account **01**; 2. Cash Collateral **02**; 3. Lainnya **99** (`PDF #195`, penjelasan `PDF #197–198`).
- **XVI Jenis PEP**: PEP **1**; Bukan PEP **2**. **XVII Risiko Nasabah**: Rendah **1**; Sedang **2**; Tinggi **3**. **XVIII Status Data**: terdapat pengkinian data bulan berjalan **1**; tidak **2** (`PDF #196`).

### Sifat baris
**Per-nasabah/per-rekening tabungan** (1 baris = 1 rekening), bukan pos tetap. Ada baris **JUMLAH** (`PDF #194`).
- *"setiap rekening tabungan diisi dengan 1 (satu) nomor rekening tabungan yang unik (tidak boleh sama) untuk setiap rekening tabungan nasabah."* Rekening diisi angka/huruf; karakter lain dibuang (`PDF #197`).

### Aturan penting (kutipan)
- Definisi: *"Tabungan yaitu simpanan milik pihak ketiga bukan bank pada BPR yang penarikannya hanya dapat dilakukan menurut syarat tertentu yang disepakati, namun tidak dapat ditarik dengan cek, bilyet giro…"* (`PDF #197`).
- **X Nominal**: *"jumlah saldo tabungan pada akhir bulan laporan."* (`PDF #197`).
- **XI**: jumlah tabungan diblokir/diagunkan (cash collateral, diblokir untuk penyidikan) (`PDF #197`).
- **XIII Biaya Transaksi Belum Diamortisasi**: *"bagian dari biaya transaksi yang belum diamortisasi dan belum menjadi penambah beban bunga pada periode berjalan atas tabungan."* (`PDF #198`).
- **XIV Jumlah**: *"nominal dikurangi biaya transaksi yang belum diamortisasi."* (`PDF #198`).
- **XV Nomor Identitas**: NIK/No. Paspor/KITAS/KITAP untuk perorangan, NPWP untuk badan usaha (`PDF #198`).
- Referensi umum: II (ID Pihak Lawan), V (Hubungan dengan Bank), VI (Pihak Ketiga Bukan Bank), VII (Lokasi), VIII (Jangka Waktu), IX (Suku Bunga) → Bab II Penjelasan Umum (`PDF #197`).
- Tidak ada aturan "Desember".

---

# 4. FORM 12.00 — Daftar Deposito
- Nama persis: **FORM 12.00 – Daftar Deposito** (`PDF #200`, cetak 148).
- Rentang: form `#200–201`; sandi `#202–203`; penjelasan `#204–206` (cetak 152–154).

### Kolom (17 kolom, urut) — beda dari 11.00: **TIDAK ada kolom "Jenis"**
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | ID Pihak Lawan | sandi unik |
| III | No. Rekening | teks angka/huruf |
| IV | Hubungan dengan Bank | sandi pilihan (12/20) |
| V | Golongan Nasabah | sandi (Lampiran 02) |
| VI | Lokasi Nasabah | sandi (Lampiran 03) |
| VII | Jangka Waktu | tanggal (2 sub) |
| VIII | Suku Bunga | angka (%) |
| IX | Nominal | angka (rupiah penuh) |
| X | Nominal yang Diblokir/Dijaminkan | angka (rupiah penuh) |
| XI | Alasan Diblokir | sandi pilihan (01/02/99) |
| XII | Biaya Transaksi Belum Diamortisasi | angka (rupiah penuh) |
| XIII | Jumlah | angka (rupiah penuh) |
| XIV | Nomor Identitas | teks (NIK/NPWP) |
| XV | Jenis PEP | sandi pilihan (1/2) |
| XVI | Risiko Nasabah | sandi pilihan (1/2/3) |
| XVII | Status Data | sandi pilihan (1/2) |
Sumber: header `PDF #200` (I–VII) + `PDF #201` (VIII–XVII); sandi `PDF #202–203`.
Catatan: **VII Jangka Waktu** juga 2 sub-header Tanggal Mulai / Tanggal Jatuh Tempo (`TT-BB-TTTT`).

### Sandi tetap
- **IV Hubungan dengan Bank**: Terkait **12**; Tidak Terkait **20** (`PDF #202`).
- **XI Alasan Diblokir**: Escrow Account **01**; Cash Collateral **02**; Lainnya **99** (`PDF #202`, penjelasan `PDF #204`).
- **XV Jenis PEP**: **1/2**. **XVI Risiko Nasabah**: **1/2/3**. **XVII Status Data**: **1/2** (`PDF #203`).

### Sifat baris
**Per-nasabah/per-rekening deposito** (1 baris = 1 rekening unik), bukan pos tetap. Ada baris **JUMLAH** (`PDF #201`).
- *"setiap rekening diisi dengan 1 (satu) nomor rekening deposito yang unik (tidak boleh sama)…"* (`PDF #204`).

### Aturan penting (kutipan)
- Definisi: *"Deposito yaitu simpanan milik pihak ketiga bukan bank pada BPR yang penarikannya dapat dilakukan menurut suatu jangka waktu tertentu berdasarkan perjanjian."* (`PDF #204`).
- **IX Nominal**: *"nilai nominal deposito pada tanggal laporan."* (`PDF #204`).
- **XII Biaya Transaksi Belum Diamortisasi**: *"biaya transaksi yang belum diamortisasi dan belum menjadi penambah beban bunga pada periode berjalan atas deposito."* (`PDF #204`).
- **XIII Jumlah**: *"nominal dikurangi dengan biaya transaksi belum diamortisasi."* (`PDF #205`).
- **XIV Nomor Identitas**: NIK (perorangan) atau NPWP (badan usaha) (`PDF #205`). (Teks penjelasan menyebut "berupa tabungan" — lihat anomali.)
- Referensi umum sama dengan 11.00 (`PDF #204`). Tidak ada aturan "Desember".

---

# 5. FORM 14.00 — Rincian Liabilitas Lainnya
- Nama persis: **FORM 14.00 – Rincian Liabilitas Lainnya** (`PDF #213`, cetak 161; judul tercetak di kaki hasil ekstraksi teks-box).
- Rentang: form `#213`; penjelasan `#214–215` (cetak 162–163). `PDF #212` adalah penjelasan **Form 13.00 / Daftar Simpanan dari Bank Lain** (kolom X–XV), bukan bagian 14.00.

### Kolom (4 kolom, urut)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Nama Rekening | teks/nama pos |
| III | Sandi | sandi pos (10 digit) |
| IV | Jumlah | angka (rupiah penuh) |
Sumber: `PDF #213`.

### Struktur baris: TETAP (daftar pos baku)
| Nama Rekening | Sandi |
|---|---|
| Utang Bunga *(header grup, tanpa baris sandi sendiri)* | — |
| — Tabungan Berjangka | 2299010100 |
| — Deposito a. Sudah Jatuh Tempo | 2299010201 |
| — Deposito b. Belum Jatuh Tempo | 2299010202 |
| — Simpanan dari Bank lain a. Sudah Jatuh Tempo | 2299010301 |
| — Simpanan dari Bank lain b. Belum Jatuh Tempo | 2299010302 |
| — Pinjaman yang Diterima dari Bank a. Sudah Jatuh Tempo | 2299010401 |
| — Pinjaman yang Diterima dari Bank b. Belum Jatuh Tempo | 2299010402 |
| — Pinjaman yang Diterima dari Pihak Ketiga Bukan Bank a. Sudah Jatuh Tempo | 2299010501 |
| — Pinjaman yang Diterima dari Pihak Ketiga Bukan Bank b. Belum Jatuh Tempo | 2299010502 |
| — Utang Bunga Lainnya | 2299019900 |
| Utang Pajak | 2299020000 |
| Liabilitas Imbalan Kerja | 2299030000 |
| Liabilitas Sewa Pembiayaan | 2299040000 |
| Taksiran Pajak Penghasilan | 2299050000 |
| Pendapatan yang Ditangguhkan | 2299060000 |
| Liabilitas Pajak Tangguhan | 2299070000 |
| Lainnya | 2299990000 |
| JUMLAH | — |
Sumber: `PDF #213`; definisi tiap pos `PDF #214–215`.

### Aturan penting (kutipan)
- **Utang Bunga** = *"seluruh liabilitas BPR berupa liabilitas bunga kepada nasabah yang belum dibayarkan …"*; dirinci a–f (Tabungan Berjangka, Deposito, Simpanan dari Bank Lain, Pinjaman dari Bank, Pinjaman dari Pihak Ketiga Bukan Bank, Utang Bunga Lainnya) (`PDF #214`).
  - Tabungan Berjangka = akrual bunga belum jatuh tempo; Deposito "Sudah Jatuh Tempo" = bunga belum ditarik, "Belum Jatuh Tempo" = akrual bunga saat jatuh tempo (`PDF #214`).
- **Utang Pajak** = PPh Pasal 29 (PPh Badan) selisih kurang setelah PPh 25, dan/atau pajak yang ditetapkan kantor pajak (`PDF #215`).
- **Liabilitas Imbalan Kerja** = *"…jumlah yang didiskontokan, kecuali untuk imbalan kerja jangka pendek dilaporkan sebesar jumlah yang terutang dan tidak didiskontokan."* (`PDF #215`).
- **Pendapatan yang Ditangguhkan**: *"Tidak termasuk pada pos ini pendapatan bunga ditangguhkan dalam rangka restrukturisasi."* (`PDF #215`).
- **Lainnya** = antara lain dana penerusan kredit yang belum disalurkan. **Aturan kunci (redirect):** *"Jika jumlah pos ini melebihi 25% (dua puluh lima persen) dari jumlah pos liabilitas lainnya maka pos tersebut dilaporkan pada Form 14.01 - Rincian Liabilitas Lainnya – Lain-Lain."* (`PDF #215`).
- Tidak ada aturan "Desember".

---

# 6. Penjelasan Umum Kolom (Bab II) — dipakai lintas form
`PDF #61–66` (cetak 9–14). Bagian relevan:
- **C. Pihak Ketiga Bukan Bank** (`PDF #61–63`): klasifikasi sektor pemerintah/pemerintah campuran/swasta (perusahaan, koperasi, kelompok, perorangan, dll).
- **D. Golongan Nasabah** (`PDF #63`): pihak ketiga bukan bank pemilik tabungan/deposito sesuai huruf C. Sandi mengacu **Lampiran 02 – Daftar Sandi Pihak Lawan** (form 11.00 `PDF #195`; form 12.00 `PDF #202`).
- **H. Lokasi** (`PDF #63`): wilayah Kabupaten/Kota, mengacu **Lampiran 03**.
- **I. Hubungan dengan Bank** (`PDF #63–64`): definisi **Terkait** (kriteria POJK BMPK BPR/BMPD BPR Syariah) dan **Tidak Terkait**. Form 11.00/12.00 memberi sandi Terkait **12**, Tidak Terkait **20**.
- **K. Jangka Waktu** (`PDF #64`): *"Untuk aset atau liabilitas keuangan yang tidak memiliki jatuh tempo, tanggal jatuh tempo dikosongkan. Untuk … yang diperpanjang, tanggal mulai dan tanggal jatuh tempo dilaporkan berdasarkan perpanjangan perjanjian terakhir."*
- **L. Suku Bunga** (`PDF #64`): *"Jika suku bunga berbeda-beda untuk satu rekening pada bulan laporan maka yang dilaporkan yaitu suku bunga tertinggi."*
- **R. ID Pihak Lawan** (`PDF #65–66`): harus = CIF SLIK bila ada; unik per pihak; **tidak dapat diubah** selama tercatat; **no reuse/no recycle**; karakter non-huruf/angka dibuang; *"Kolom ID Pihak lawan harus diisi (mandatory)."*
- **Q. Jenis CKPN** (`PDF #65`): hanya aturan "posisi Desember" yang ditemukan di seluruh dokumen untuk form ini: *"Untuk laporan berkala bulanan posisi bulan Desember 2024, kolom Jenis CKPN diisi dengan CKPN Kolektif (sandi 2)."* — **tidak berlaku** untuk form 08/10/11/12/14.
- Tidak ditemukan aturan penulisan tanda `"-"` eksplisit; yang ada adalah aturan **"dikosongkan"** (Jangka Waktu tanpa jatuh tempo; Status Aset untuk aset tidak berwujud).

---

# 7. Anomali / perlu konfirmasi
1. **Rentang 08.00 di brief kurang**: penjelasan berlanjut ke `PDF #185–186` (Sumber Perolehan 3–4, Status Aset, V–IX). Jangan berhenti di 184.
2. **11.00 vs 12.00**: 11.00 punya kolom **IV Jenis** (sandi 10/20) dan 18 kolom; 12.00 tidak punya kolom Jenis dan hanya 17 kolom. Kolom lainnya bergeser satu.
3. **11.00 kolom II penamaan ganda**: header tabel = "ID Pihak Lawan" (`PDF #193`), tabel sandi = "ID Pihak Lawan" (`PDF #195`), penjelasan = "**No. ID Pihak Lawan**" (`PDF #197`). Perlu samakan nama field.
4. **11.00 XVII Risiko Nasabah butir 2 (Sedang)** berbunyi *"…memiliki risiko tinggi…"* (`PDF #198`) — copy-paste error PDF; definisi resmi = risiko sedang.
5. **12.00 XIV Nomor Identitas** berbunyi *"…menempatkan dana berupa tabungan kepada BPR"* (`PDF #205`) padahal form deposito — copy-paste dari 11.00. Isi aktual = NIK/NPWP.
6. **14.00 "Utang Bunga"** hanya header grup tanpa sandi/baris tersendiri; yang dilaporkan adalah komponen 22990101xx–22990199xx. Konfirmasi apakah parent 2299010000 perlu ada di file kirim.
7. **14.00 judul form** di ekstraksi teks muncul di akhir halaman (`PDF #213`) karena text-box header; secara visual judul ada di atas. Bukan masalah data.
8. **10.00 vs 14.00**: keduanya berkolom identik (I Sandi Kantor, II Nama Rekening, III Sandi, IV Jumlah) dan baris tetap — pola rendering/implementasi harus sama.
9. **`PDF #106`**: bukan form 10.00/08.00, melainkan penjelasan pos laporan posisi keuangan yang menyebut rinciannya di form tersebut. Hanya dipakai sebagai referensi silang.

## Referensi halaman mentah hasil ekstraksi
`/tmp/ojk/raw/11.00.txt`, `12.00.txt`, `14.00.txt`, `14.00-p212.txt`, `08.00.txt`, `08.00-p60.txt`, `08.00-p185-190.txt`, `10.00.txt`, `10.00-p106.txt`.
