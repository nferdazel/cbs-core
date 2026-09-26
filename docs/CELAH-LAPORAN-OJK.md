# Celah Kelengkapan Laporan OJK — Triase

Dokumen ini menriase setiap kolom/laporan yang **belum diisi angka** oleh
`apps/api/internal/ojkreport` (dinyatakan sebagai `Reason`/`UnavailableReason`) menjadi
keranjang kerja. Tujuannya memisahkan **"akan dimodelkan"** dari **"memang di luar cakupan"**
agar sisa pekerjaan tidak terlihat sebagai utang mengambang.

Status: penilaian awal per 26 Sep 2026, berbasis pembacaan kode (`form05.go`, `form06.go`,
`definitions.go`) dan skema migrasi. Nomor kolom Romawi diambil dari konstanta sandi di kode,
bukan dari ingatan. Urutan pengerjaan dan kelayakan adalah penilaian pengembang, bukan
keputusan pemilik sistem.

> **Revisi 26 Sep 2026:** keranjang **RO diperbaiki** setelah riset lampiran
> (`docs/LAMPIRAN-OJK.md`) diverifikasi ke PDF resmi 528 hlm. Ternyata hanya **3** kolom yang
> benar-benar butuh lampiran; sebagian "RO" aslinya sandi **inline** (sudah tersurat di PDF)
> atau **CIF internal**, dan "Sandi Bank" butuh APOLO/SPOJK. Rinciannya di §1, §2a–2c.

## Kategori

| Kode | Arti | Tindak lanjut |
|---|---|---|
| **K1** | Bisa dimodelkan, perubahan kecil (kolom/sandi/pemetaan) | Kerjakan bila bank butuh |
| **K2** | Bisa dimodelkan, perlu register/modul baru | Rencanakan terpisah |
| **KB** | Kondisional bank (hanya bila bank peserta) | Tanyakan ke bank saat onboarding |
| **RO** | Butuh referensi OJK (Lampiran 02/03) | Unduh lampiran dulu |
| **DK** | Butuh keputusan kebijakan/definisi | Pemilik sistem + bank/akuntan |
| **LX** | Di luar cakupan sistem (berkas/dokumen manual) | Tidak dikerjakan |

## 1. Form 05.00 — Daftar Penempatan pada Bank Lain (10 kolom)

| Kolom | Nama | Kategori | Catatan |
|---|---|---|---|
| II | Sandi Bank | K2 | Bukan Lampiran 02: sandi bank 6 digit dari Sistem Pelaporan OJK (APOLO/SPOJK), belum ada sumber publik |
| III | Lokasi Bank | RO | Sandi Kabupaten/Kota per Lampiran 03 (`docs/LAMPIRAN-OJK.md`) |
| V | Hubungan dengan Bank | K1 | Sandi inline: 12 terkait, 20 tidak terkait |
| X | Nominal yang Diblokir/Dijaminkan | K1 | Tambah kolom nominal pada `lps_placements` |
| XI | Alasan Diblokir | K1 | Tambah kolom sandi alasan (butuh daftar sandi) |
| XIII | Pendapatan Bunga yang Akan Diterima | K1 | Akrual bunga penempatan |
| XIV | Pendapatan Bunga Dalam Penyelesaian | K1 | Bunga penempatan menunggak |
| XV | Status BMPK Individu | K2 | Perlu uji BMPK per bank lawan |
| XVI | ID Pihak Lawan | K1 | Sebenarnya CIF internal (SLIK), bukan sandi lampiran |
| XX | Klasifikasi Aset Keuangan | DK | Klasifikasi SAK EP per penempatan |

## 2. Form 06.00 — Daftar Kredit yang Diberikan (36 kolom)

### 2a. Butuh referensi OJK — sandi Lampiran II (2)

| Kolom | Nama | Sumber |
|---|---|---|
| XVIII | Jenis Debitur | Lampiran 02 Daftar Sandi Pihak Lawan (`docs/LAMPIRAN-OJK.md`) |
| XX | Sektor Ekonomi | Lampiran 05 Daftar Sandi Sektor Ekonomi |

### 2b. Bisa dimodelkan, perubahan kecil (15)

| Kolom | Nama | Catatan |
|---|---|---|
| II | ID Pihak Lawan | Bukan sandi lampiran: CIF internal (harus sama dengan SLIK), tinggal disingkapkan |
| VIII | Jenis Penggunaan | Sandi inline 10/20/31/32/35/39; `loans.purpose` masih teks bebas, perlu pemetaan |
| IX | Hubungan dengan Bank | Sandi inline 11/12/20 |
| XI | Periode Pembayaran Pokok dan Bunga | Sandi inline 1–8 |
| IV | Kode Kelompok Kredit | Kelompok peminjam pihak tidak terkait |
| X | Sumber Dana Pelunasan | Kolom sandi sumber dana |
| XV | Tanggal Mulai Macet | Bisa diturunkan dari tunggakan/jadwal, bukan hanya DPD |
| XXI | Kategori Usaha | Mikro/kecil/menengah |
| XXII | Lokasi Penggunaan | Kolom lokasi pemakaian kredit |
| XXIV | Penjamin | Perlu data penjamin + bagian yang dijamin |
| XXV | Nilai Agunan Diperhitungkan untuk PPKA | Sudah dihitung modul PPAP; perlu disingkapkan ke baris kredit |
| XXXV | Pendapatan Bunga yang Akan Diterima | Piutang bunga sudah per angsuran; dimuat ke daftar kredit |
| XXXVI | Pendapatan Bunga Dalam Penyelesaian | Bunga kredit menunggak |
| XXXVIII | Sifat Kredit | Pengalihan piutang/lainnya |
| XLII | Tanggal Akad Akhir | Tanggal addendum terakhir |

### 2c. Perlu register/modul baru (8)

| Kolom | Nama | Catatan |
|---|---|---|
| XIX | Sandi Bank | Butuh daftar sandi bank 6 digit dari APOLO/SPOJK (tidak terbit di SEOJK) |
| XXVI | Kelonggaran Tarik | Perlu pemodelan fasilitas komitmen |
| XXIX | Provisi Belum Diamortisasi | Perlu amortisasi provisi per kredit |
| XXX | Biaya Transaksi Belum Diamortisasi | Perlu amortisasi biaya transaksi per kredit |
| XXXI | Pendapatan Bunga Ditangguhkan (restrukturisasi) | Perlu pencatatan per kredit |
| XXXII | Cadangan Kerugian Restrukturisasi | Perlu pencatatan per kredit |
| XXXIII | Baki Debet Neto | Bergantung pada XXIX/XXX/XXXI/XXXII |
| XXXVII | Status BMPK | Perlu modul BMPK per pihak terkait |

### 2d. Kondisional bank (4)

| Kolom | Nama | Syarat |
|---|---|---|
| VI | Jenis | Kanal sindikasi/kerja sama/LPBBTI |
| XXXIX | Kredit Program Pemerintah | Hanya bila bank menyalurkan KUR/program |
| XL | Sektor Kredit Usaha Rakyat | Hanya bila bank menyalurkan KUR |
| XLIII | Sandi LPBBTI | Hanya bila bank bekerja sama dengan LPBBTI |

### 2e. Butuh keputusan (5)

| Kolom | Nama | Keputusan yang ditunggu |
|---|---|---|
| III | No. Identitas (NIK/NPWP) | Tersimpan terenkripsi; membukanya sebagai keluaran laporan adalah keputusan privasi, bukan celah teknis |
| XLIV | CKPN Aset Baik | Stage 1/2/3 hanya untuk bank pasar modal (SAK Indonesia); instalasi ini SAK EP. Lihat `CKPN-SIAP-RILIS.md` §(f) |
| XLV | CKPN Aset Kurang Baik | idem |
| XLVI | CKPN Aset Tidak Baik | idem |
| XLVII | Klasifikasi Aset Keuangan | Kebijakan klasifikasi SAK EP per kredit |

### 2f. Sudah dikerjakan (2)

| Kolom | Nama | Bukti |
|---|---|---|
| XIII | Angsuran Pokok Pertama | `f6f6a60`: MIN(due_date) jadwal angsuran; `-` bila tanpa jadwal |
| XVII | Nominal Tunggakan Pokok dan Bunga | `f6f6a60`: sisa pokok+bunga untuk angsuran jatuh tempo sebelum `as_of` |

## 3. Laporan/berkas di luar form bulanan (14)

| Kode | Nama | Kategori | Catatan |
|---|---|---|---|
| LAPORAN_KELEMBAGAAN | Laporan Kelembagaan | K2 | Perlu data jaringan kantor, direksi, komisaris, pejabat |
| LAPORAN_BMPK | Laporan BMPK | K2 | Agregasi eksposur per pihak terkait belum ada |
| LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF | Perbedaan Kualitas | K1 | Perbedaan kualitas komersial vs PPKA per debitur |
| LAPORAN_LAKU_PANDAI | Perkembangan Laku Pandai | KB | Hanya bila bank jadi agen Laku Pandai |
| LAPORAN_KEUANGAN_PUBLIKASI_TRIWULANAN | Publikasi keuangan triwulanan | LX | Rasionya sudah; berkas publikasinya manual |
| LAPORAN_KEUANGAN_PUBLIKASI | Bukti pengumuman keuangan | LX | Dokumen dari luar sistem |
| LAPORAN_BUKTI_PENGUMUMAN_TAHUNAN | Bukti pengumuman tahunan | LX | Dokumen dari luar sistem |
| LAPORAN_TPPU_TPPT_PPSPM | Penilaian risiko TPPU | LX | Dokumen manual |
| Form 01.01 | Rekening Administratif | K2 | Pos komitmen/kontinjensi belum ada di bagan akun |
| Form 09.00 | Rincian Aset Lainnya | K2 | Kini hanya saldo agregat COA |
| Form 13.00 | Simpanan dari Bank Lain | K2 | Kini hanya saldo agregat COA |
| Form 00.13 | Dokumen Pendukung | LX | Berkas PDF, bukan angka |
| Form 00.14 | Jenis Nasabah & Produk Simpanan | K1 | Agregasi jenis nasabah per produk belum ada |
| Form 00.15 | Rincian Transaksi TPPU/TPPT | KB | Hanya bila bank punya kewajiban pelaporan ini |

## 4. Ringkasan

Kolom form: **46** (Form 05.00 = 10, Form 06.00 = 36); **2 sudah dikerjakan** (§2f), sisa 44.
Laporan/berkas: **14**. Total sisa **58**.

| Kategori | Kolom | Laporan | Total |
|---|---|---|---|
| K1 — bisa, kecil | 21 | 2 | 23 |
| K2 — butuh modul | 10 | 5 | 15 |
| KB — kondisional bank | 4 | 2 | 6 |
| RO — butuh referensi OJK | 3 | 0 | 3 |
| DK — butuh keputusan | 6 | 0 | 6 |
| LX — di luar cakupan | 0 | 5 | 5 |
| **Total sisa** | **44** | **14** | **58** |

## 5. Urutan yang disarankan

1. **Sandi inline (K1, +4 kolom)** — V/IX Hubungan dengan Bank, VIII Jenis Penggunaan,
   XI Periode Pembayaran: sandinya sudah tersurat di PDF, cukup enum/konstanta di
   `apps/api/internal/ojkreport`. Tidak perlu tabel referensi.
2. **Lampiran 02/03/05** — kini hanya membuka **3 kolom** (Form 05 III Lokasi Bank, Form 06
   XVIII Jenis Debitur, Form 06 XX Sektor Ekonomi). Bangun tabel referensi `ojk_*` (migrasi).
3. **K1 yang datanya sudah ada** — Angsuran Pokok Pertama & Nominal Tunggakan Form 06.00
   SUDAH dikerjakan; sisanya (Kode Kelompok Kredit, Kategori Usaha, dst.) menyusul.
4. **Tanya partisipasi bank** (KB, 6 kolom) — kalau bank bukan peserta KUR/LPBBTI/Laku Pandai,
   kolom itu sah dikosongkan, bukan utang.
5. **K2** (BMPK, amortisasi, off-balance, kelembagaan, Sandi Bank APOLO) — pekerjaan modul.
6. **DK** — bawa ke panel/pemilik bersama bank/akuntan.

## 6. Catatan

- `builder.go` juga punya 6 `UnavailableReason`, tetapi itu **kondisi runtime**
  (identitas bank belum diisi, sumber data belum dikonfigurasi, belum ada penempatan),
  bukan celah permanen — tidak masuk triase ini.
- Angka ini bisa berubah bila OJK memperbarui format; verifikasi ulang saat menaikkan
  versi SEOJK.
