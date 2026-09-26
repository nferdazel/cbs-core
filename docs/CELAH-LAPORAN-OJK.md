# Celah Kelengkapan Laporan OJK — Triase

Dokumen ini menriase setiap kolom/laporan yang **belum diisi angka** oleh
`apps/api/internal/ojkreport` (dinyatakan sebagai `Reason`/`UnavailableReason`) menjadi
keranjang kerja. Tujuannya memisahkan **"akan dimodelkan"** dari **"memang di luar cakupan"**
agar sisa pekerjaan tidak terlihat sebagai utang mengambang.

Status: penilaian awal per 26 Sep 2026, berbasis pembacaan kode (`form05.go`, `form06.go`,
`definitions.go`) dan skema migrasi. Nomor kolom Romawi diambil dari konstanta sandi di kode,
bukan dari ingatan. Urutan pengerjaan dan kelayakan adalah penilaian pengembang, bukan
keputusan pemilik sistem.

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
| II | Sandi Bank | RO | Sandi bank lawan per Lampiran 02; kini hanya nama bank |
| III | Lokasi Bank | RO | Sandi Kabupaten/Kota per Lampiran 03 |
| V | Hubungan dengan Bank | RO | Sandi relasi pihak terkait; butuh daftar sandi OJK |
| X | Nominal yang Diblokir/Dijaminkan | K1 | Tambah kolom nominal pada `lps_placements` |
| XI | Alasan Diblokir | K1 | Tambah kolom sandi alasan (butuh daftar sandi) |
| XIII | Pendapatan Bunga yang Akan Diterima | K1 | Akrual bunga penempatan |
| XIV | Pendapatan Bunga Dalam Penyelesaian | K1 | Bunga penempatan menunggak |
| XV | Status BMPK Individu | K2 | Perlu uji BMPK per bank lawan |
| XVI | ID Pihak Lawan | RO | Kunci internal belum = sandi Lampiran 02 |
| XX | Klasifikasi Aset Keuangan | DK | Klasifikasi SAK EP per penempatan |

## 2. Form 06.00 — Daftar Kredit yang Diberikan (36 kolom)

### 2a. Butuh referensi OJK — sandi Lampiran II (7)

| Kolom | Nama | Catatan |
|---|---|---|
| II | ID Pihak Lawan | Sistem menyimpan UUID internal, bukan sandi Lampiran 02 |
| VIII | Jenis Penggunaan | Tujuan kredit tersimpan sebagai teks bebas (`loans.purpose`), belum dipetakan |
| IX | Hubungan dengan Bank | Perlu sandi relasi pihak terkait |
| XI | Periode Pembayaran Pokok dan Bunga | Perlu sandi periode OJK |
| XVIII | Jenis Debitur | Perlu sandi perorangan/badan usaha (datanya pun belum ada — lihat 2b) |
| XIX | Sandi Bank | Hanya relevan bila kredit diteruskan ke bank lain |
| XX | Sektor Ekonomi | Perlu sandi sektor Lapangan Usaha |

### 2b. Bisa dimodelkan, perubahan kecil (13)

| Kolom | Nama | Catatan |
|---|---|---|
| IV | Kode Kelompok Kredit | Kelompok peminjam pihak tidak terkait |
| X | Sumber Dana Pelunasan | Kolom sandi sumber dana |
| XIII | Angsuran Pokok Pertama | Jadwal angsuran sudah ada (migrasi `000006`), tinggal dimuat ke baris laporan |
| XV | Tanggal Mulai Macet | Bisa diturunkan dari tunggakan/jadwal, bukan hanya DPD |
| XVII | Nominal Tunggakan Pokok dan Bunga | Diambil dari jadwal/tunggakan per kredit |
| XXI | Kategori Usaha | Mikro/kecil/menengah |
| XXII | Lokasi Penggunaan | Kolom lokasi pemakaian kredit |
| XXIV | Penjamin | Perlu data penjamin + bagian yang dijamin |
| XXV | Nilai Agunan Diperhitungkan untuk PPKA | Sudah dihitung modul PPAP; perlu disingkapkan ke baris kredit |
| XXXV | Pendapatan Bunga yang Akan Diterima | Piutang bunga sudah per angsuran; dimuat ke daftar kredit |
| XXXVI | Pendapatan Bunga Dalam Penyelesaian | Bunga kredit menunggak |
| XXXVIII | Sifat Kredit | Pengalihan piutang/lainnya |
| XLII | Tanggal Akad Akhir | Tanggal addendum terakhir |

### 2c. Perlu register/modul baru (7)

| Kolom | Nama | Catatan |
|---|---|---|
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

Kolom form: **46** (Form 05.00 = 10, Form 06.00 = 36). Laporan/berkas: **14**. Total **60**.

| Kategori | Kolom | Laporan | Total |
|---|---|---|---|
| K1 — bisa, kecil | 17 | 2 | 19 |
| K2 — butuh modul | 8 | 5 | 13 |
| KB — kondisional bank | 4 | 2 | 6 |
| RO — butuh referensi OJK | 11 | 0 | 11 |
| DK — butuh keputusan | 6 | 0 | 6 |
| LX — di luar cakupan | 0 | 5 | 5 |
| **Total** | **46** | **14** | **60** |

## 5. Urutan yang disarankan

1. **Unduh Lampiran 02/03 SEOJK 16/2024** — memblokir 11 kolom (RO) sekaligus menutup banyak
   pemetaan sandi. Prasyarat termurah dengan dampak terbesar.
2. **K1 yang datanya sudah ada** — Angsuran Pokok Pertama, Nominal Tunggakan, Agunan PPKA,
   piutang bunga (5–6 kolom): tinggal menyambungkan modul yang sudah ada.
3. **Tanya partisipasi bank** (KB, 6 kolom) — kalau bank bukan peserta KUR/LPBBTI/Laku Pandai,
   kolom itu sah dikosongkan, bukan utang.
4. **K2** (BMPK, amortisasi, off-balance, kelembagaan) — pekerjaan modul, rencanakan terpisah.
5. **DK** — bawa ke panel/pemilik bersama bank/akuntan.

## 6. Catatan

- `builder.go` juga punya 6 `UnavailableReason`, tetapi itu **kondisi runtime**
  (identitas bank belum diisi, sumber data belum dikonfigurasi, belum ada penempatan),
  bukan celah permanen — tidak masuk triase ini.
- Angka ini bisa berubah bila OJK memperbarui format; verifikasi ulang saat menaikkan
  versi SEOJK.
