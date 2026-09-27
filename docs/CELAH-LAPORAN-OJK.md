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
>
> **Revisi 27 Sep 2026:** ketiga kolom RO **tersambung** lewat migrasi `000104` ke tabel
> referensi `000102` (`customers.ojk_pihak_lawan_code`, `customers.ojk_sektor_ekonomi_code`,
> `lps_placements.ojk_kabupaten_code`, semuanya FK + nullable). Nilainya belum diisi
> aplikasi: bank mengisinya lewat SQL/seed sampai ada rute API. Baris tanpa sandi ditulis
> `-`. Rinciannya di §1, §2a, §2f.

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
| II | Sandi Bank | NI | Bukan Lampiran 02: sandi bank 6 digit dari APOLO/SPOJK, **tidak dapat diimplementasikan** (tidak ada sumber publik di repo/PDF) |
| III | Lokasi Bank | ✓ | Tersambung `lps_placements.ojk_kabupaten_code` (migrasi 000104, FK ke `ojk_kabupaten` Lampiran 03); `-` bila bank belum mengisi sandi |
| V | Hubungan dengan Bank | ✓ | `lps_placements.ojk_hubungan_bank_code` (migrasi 000106, inline 12/20); `-` bila kosong |
| X | Nominal yang Diblokir/Dijaminkan | ✓ | `lps_placements.blocked_amount` (migrasi 000106); `-` bila kosong |
| XI | Alasan Diblokir | ✓ | `lps_placements.ojk_alasan_diblokir_code` (migrasi 000106) — disimpan, belum dipetakan (daftar sandi alasan belum ada di repo) |
| XIII | Pendapatan Bunga yang Akan Diterima | ✓ | `lps_placements.accrued_interest_receivable` (migrasi 000106) |
| XIV | Pendapatan Bunga Dalam Penyelesaian | ✓ | `lps_placements.accrued_interest_pending` (migrasi 000106) |
| XV | Status BMPK Individu | ✓ | Terisi dari modul BMPK (`lps_placements.customer_id`); `-` bila penempatan belum ditautkan |
| XVI | ID Pihak Lawan | ✓ | `lps_placements.counterparty_cif` (migrasi 000106, CIF internal) |
| XX | Klasifikasi Aset Keuangan | ✓ | `lps_placements.ojk_klasifikasi_aset_code` (migrasi 000108, diisi bank, tanpa auto-klasifikasi); `-` bila kosong |

## 2. Form 06.00 — Daftar Kredit yang Diberikan (36 kolom)

### 2a. Sandi referensi OJK — tersambung (2, migrasi 000104)

Kolom XVIII dan XX kini tersambung: `customers.ojk_pihak_lawan_code` /
`customers.ojk_sektor_ekonomi_code` (FK ke tabel `ojk_*` migrasi 000102). Nilai diisi
bank (kini juga lewat API: `PUT /api/v1/ojk/loan-codes/{loanId}` untuk kredit dan
`PUT /customers/{id}` untuk nasabah); baris tanpa sandi ditulis `-`. Tidak lagi masuk
keranjang RO.

| Kolom | Nama | Sumber |
|---|---|---|
| XVIII | Jenis Debitur | Lampiran 02 Daftar Sandi Pihak Lawan (`docs/LAMPIRAN-OJK.md`) |
| XX | Sektor Ekonomi | Lampiran 05 Daftar Sandi Sektor Ekonomi |

### 2b. Bisa dimodelkan, perubahan kecil (0)

Semua kolom §2b sudah dikerjakan (migrasi `000107`) — bukti di §2f.

### 2c. Perlu register/modul baru (0)

Tidak ada sisa. XIX (Sandi Bank) tidak dapat diimplementasikan (APOLO/SPOJK);
XXIX–XXXIII sudah dikerjakan (migrasi 000111, bukti di §2f).

### 2d. Kondisional bank (4) — kebijakan: bank bukan peserta bawaan, bukan celah

| Kolom | Nama | Syarat |
|---|---|---|
| VI | Jenis | Kanal sindikasi/kerja sama/LPBBTI |
| XXXIX | Kredit Program Pemerintah | Hanya bila bank menyalurkan KUR/program |
| XL | Sektor Kredit Usaha Rakyat | Hanya bila bank menyalurkan KUR |
| XLIII | Sandi LPBBTI | Hanya bila bank bekerja sama dengan LPBBTI |

### 2e. Diputuskan (5) — lihat `docs/KEPUTUSAN-OJK.md`

| Kolom | Nama | Keputusan yang ditunggu |
|---|---|---|
| III | No. Identitas (NIK/NPWP) | Tersimpan terenkripsi; membukanya sebagai keluaran laporan adalah keputusan privasi, bukan celah teknis |
| XXXVI | Pendapatan Bunga Dalam Penyelesaian | Sistem memakai basis kas untuk NPL (`accrual_status = CASH_BASIS_NPL`), bukan reklasifikasi bunga terakru ke akun "dalam penyelesaian". Butuh keputusan kebijakan + pemodelan, jangan diturunkan dari kolektibilitas (akan menggandakan XXXV) |
| XLIV | CKPN Aset Baik | Stage 1/2/3 hanya untuk bank pasar modal (SAK Indonesia); instalasi ini SAK EP. Lihat `CKPN-SIAP-RILIS.md` §(f) |
| XLV | CKPN Aset Kurang Baik | idem |
| XLVI | CKPN Aset Tidak Baik | idem |

### 2f. Sudah dikerjakan (25)

| Kolom | Nama | Bukti |
|---|---|---|
| XIII | Angsuran Pokok Pertama | `f6f6a60`: MIN(due_date) jadwal angsuran; `-` bila tanpa jadwal |
| XVII | Nominal Tunggakan Pokok dan Bunga | `f6f6a60`: sisa pokok+bunga untuk angsuran jatuh tempo sebelum `as_of` |
| XVIII | Jenis Debitur (Pihak Lawan) | migrasi 000104: `customers.ojk_pihak_lawan_code`; `-` bila kosong |
| XX | Sektor Ekonomi | migrasi 000104: `customers.ojk_sektor_ekonomi_code`; `-` bila kosong |
| XXXV | Pendapatan Bunga yang Akan Diterima | SUM(`profit_accrued_amount`) = sisa akruan/piutang bunga 10400 per kredit |
| II | ID Pihak Lawan | migrasi 000107 (tanpa kolom): CIF internal dari `customers.cif_number` (PDF #page 66); `-` bila kosong |
| IV | Kode Kelompok Kredit | migrasi 000107: `loans.ojk_kelompok_kredit_code` (kode unik bank, bukan sandi OJK; PDF #page 158) |
| X | Sumber Dana Pelunasan | migrasi 000107: `loans.ojk_sumber_dana_code` (inline 10/21/22/31/32; PDF #page 153) |
| XV | Tanggal Mulai Macet | migrasi 000107: `loans.ojk_tanggal_mulai_macet` (diisi bank, bukan turunan DPD; PDF #page 153) |
| XXI | Kategori Usaha | migrasi 000107: `loans.ojk_kategori_usaha_code` (1/2/3/4; PDF #page 154) |
| XXIV | Penjamin | migrasi 000107: `loans.ojk_penjamin_code` (FK `ojk_pihak_lawan`) + `ojk_penjamin_bagian_pct`; dua subkolom |
| XXXVIII | Sifat Kredit | migrasi 000107: `loans.ojk_sifat_kredit_code` (2/9; PDF #page 155) |
| XLII | Tanggal Akad Akhir | turunan `restructured_at`/`akad_date` (PDF #page 167); addendum non-restrukturisasi belum punya tanggal tersendiri |
| XLVII | Klasifikasi Aset Keuangan | migrasi 000108: `loans.ojk_klasifikasi_aset_code` (diisi bank, tanpa auto-klasifikasi); `-` bila kosong |
| XXV | Nilai Agunan Diperhitungkan untuk PPKA | migrasi 000109: `loans.ojk_agunan_ppka_amount`; diisi modul PPAP saat run (COALESCE, isian bank tidak terhapus), bank mengisi bila modul agunan mati |
| XXVI | Kelonggaran Tarik | migrasi 000109: `loans.ojk_kelonggaran_tarik_amount` (bank-fill); TIDAK diturunkan dari plafon−baki |
| XXIX | Provisi Belum Diamortisasi | migrasi 000111: `loans.ojk_provisi_belum_diamortisasi_amount` (bank-fill) |
| XXX | Biaya Transaksi Belum Diamortisasi | migrasi 000111: `loans.ojk_biaya_transaksi_belum_diamortisasi_amount` (bank-fill) |
| XXXI | Pendapatan Bunga Ditangguhkan | migrasi 000111: `loans.ojk_pendapatan_bunga_ditangguhkan_amount` (bank-fill) |
| XXXII | Cadangan Kerugian Restrukturisasi | migrasi 000111: `loans.ojk_cadangan_kerugian_restrukturisasi_amount` (bank-fill) |
| XXXIII | Baki Debet Neto | dihitung `baki debet − XXIX + XXX − XXXI − XXXII` (PDF #page 165); `-` bila komponen belum tersimpan |
| VIII | Jenis Penggunaan | migrasi 000105: `loans.ojk_jenis_penggunaan_code` (inline 10/20/31/32/35/39); `-` bila kosong |
| IX | Hubungan dengan Bank | migrasi 000105: `customers.ojk_hubungan_bank_code` (inline 11/12/20) |
| XI | Periode Pembayaran Pokok dan Bunga | migrasi 000105: `loans.ojk_periode_pembayaran_code` (inline 1–8) |
| XXII | Lokasi Penggunaan | migrasi 000105: `loans.ojk_kabupaten_code` (FK `ojk_kabupaten` Lampiran 03) |

## 3. Laporan/berkas di luar form bulanan (14)

| Kode | Nama | Kategori | Catatan |
|---|---|---|---|
| LAPORAN_LAKU_PANDAI | Perkembangan Laku Pandai | KB | Hanya bila bank jadi agen Laku Pandai |
| LAPORAN_KEUANGAN_PUBLIKASI_TRIWULANAN | Publikasi keuangan triwulanan | LX | Rasionya sudah; berkas publikasinya manual |
| LAPORAN_KEUANGAN_PUBLIKASI | Bukti pengumuman keuangan | LX | Dokumen dari luar sistem |
| LAPORAN_BUKTI_PENGUMUMAN_TAHUNAN | Bukti pengumuman tahunan | LX | Dokumen dari luar sistem |
| LAPORAN_TPPU_TPPT_PPSPM | Penilaian risiko TPPU | LX | Dokumen manual |
| Form 01.01 | Rekening Administratif | K2 | Pos komitmen/kontinjensi belum ada di bagan akun |
| Form 09.00 | Rincian Aset Lainnya | **Terblokir** | Bergantung pemetaan COA→pos 09.00 yang masih `DRAF-BELUM-TERVERIFIKASI` (`coa_mapping.go`); verifikasi bank/akuntan, bukan celah kode |
| Form 00.13 | Dokumen Pendukung | LX | Berkas PDF, bukan angka |
| Form 00.15 | Rincian Transaksi TPPU/TPPT | KB | Hanya bila bank punya kewajiban pelaporan ini |

## 4. Ringkasan

Kolom form: **46** (Form 05.00 = 10, Form 06.00 = 36) — **35 selesai**, **9 diputuskan/
kondisional** (KB 4, DK 5), **2 tidak dapat diimplementasikan** (Sandi Bank APOLO: Form 05 II,
Form 06 XIX). **Tidak ada kolom tersisa.** Laporan/berkas: **14** — **5 selesai** (BMPK;
Perbedaan Kualitas; Form 00.14; Form 13.00; Laporan Kelembagaan), **7 diputuskan/di luar
cakupan** (KB 2, LX 5), **2 di ujung** (Form 01.01 butuh register; Form 09.00 terblokir
pemetaan COA DRAF). **Total pekerjaan tersisa: 2.**

| Kategori | Kolom | Laporan | Total sisa |
|---|---|---|---|
| K1 — bisa, kecil | 0 | 0 | 0 |
| K2 — butuh modul | 0 | 2 | 2 |
| KB — kondisional (kebijakan: bukan peserta bawaan) | 0 | 0 | 0 |
| NI — tidak dapat diimplementasikan (APOLO/SPOJK) | 0 | 0 | 0 |
| RO — referensi OJK | 0 | 0 | 0 |
| DK — diputuskan (lihat `KEPUTUSAN-OJK.md`) | 0 | 0 | 0 |
| LX — di luar cakupan sistem | 0 | 0 | 0 |
| **Total sisa** | **0** | **2** | **2** |

## 5. Urutan yang disarankan

1. **Sandi inline (K1, +4 kolom)** — V/IX Hubungan dengan Bank, VIII Jenis Penggunaan,
   XI Periode Pembayaran: sandinya sudah tersurat di PDF, cukup enum/konstanta di
   `apps/api/internal/ojkreport`. Tidak perlu tabel referensi.
2. **Lampiran 02/03/05** — SUDAH tersambung (migrasi 000104) ke Form 05 III, Form 06
   XVIII, dan Form 06 XX lewat tabel referensi `ojk_*` (migrasi 000102). Kolom terisi
   begitu bank mengisi sandinya lewat SQL/seed; baris tanpa sandi ditulis `-`.
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
