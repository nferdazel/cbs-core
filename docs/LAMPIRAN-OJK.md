# Inventaris Lampiran Sandi — SEOJK No. 16/SEOJK.03/2024

Tujuan dokumen ini: menutup keranjang **RO (butuh referensi OJK)** di
[`CELAH-LAPORAN-OJK.md`](CELAH-LAPORAN-OJK.md) dengan sumber **resmi OJK**, bukan tebakan.
Isinya hanya peta lampiran → kolom; **daftar sandinya sendiri tidak disalin** ke repo
(ratusan hingga ribuan baris). Status: riset 26 Sep 2026, berbasis PDF resmi yang diunduh
dan dibaca langsung (bukan ingatan).

> **Catatan verifikasi:** nomor halaman pada bagian "Apa yang sebenarnya membuka 11 kolom RO"
> sudah dicek ulang langsung dari PDF (bukan perkiraan). Kolomnya adalah **PDF #page**, bukan
> halaman cetak.

## Sumber resmi

- **Halaman resmi:** <https://ojk.go.id/id/regulasi/Pages/SEOJK-16-SEOJK.03-2024-Pelaporan-Melalui-Sistem-Pelaporan-Otoritas-Jasa-Keuangan-dan-Transparansi-Kondisi-Keuangan-bagi.aspx>
- **PDF resmi (528 hlm., 1 berkas, memuat Lampiran I–V):**
  <https://www.ojk.go.id/id/regulasi/Documents/Pages/SEOJK-16-SEOJK.03-2024-Pelaporan-Melalui-Sistem-Pelaporan-Otoritas-Jasa-Keuangan-dan-Transparansi-Kondisi-Keuangan-bagi/SEOJK%2016-SEOJK.03-2024%20Pelaporan%20Melalui%20Sistem%20Pelaporan%20Otoritas%20Jasa%20Keuangan%20dan%20Transparansi%20Kondisi%20Keuangan%20bagi%20Bank%20Perekonomian%20Rakyat.pdf>

Catatan akses: PDF **dapat diunduh** (±3,6 MB). Situs OJK menolak klien non-browser
(HTTP 403 WAF); unduh lewat peramban atau klien dengan User-Agent peramban. Nomor halaman
di bawah ditulis dua kali: **hlm. cetak** (tercetak di PDF) dan **#page** (urutan PDF,
untuk pranala `#page=`).

## Struktur

Sandi-sandi BPR tidak berdiri sebagai berkas lampiran terpisah. Semuanya berada **di
dalam Lampiran II** (Pedoman Penyusunan Laporan Berkala Bulanan), pada "DAFTAR LAMPIRAN"
mulai hlm. cetak 249 (PDF #page 301). Halaman daftar isi lampiran ada di hlm. cetak 5
(PDF #page 57).

## Tabel lampiran sandi

| Nama lampiran | Isi | URL resmi | Status akses | Kolom yang terbuka (kode CELAH) |
|---|---|---|---|---|
| **Lampiran 01 — Daftar Sandi Jenis Agunan** | Jenis agunan: likuid/non likuid, sandi **3 digit** (mis. 101 likuid, 202 tanah+bangunan, 299 lainnya) | PDF resmi, hlm. cetak 249 (PDF #page=301) | Terbuka | Jenis agunan pada form agunan/PPKA (bukan salah satu dari 11 RO) |
| **Lampiran 02 — Daftar Sandi Pihak Lawan** | **Kategori** pihak lawan (golongan), sandi **3 digit** (mis. 001 Bank Indonesia, 600 BPR, 700 bank umum, 800 pemerintah pusat, 860 perusahaan swasta, 880 penjamin asuransi jiwa) | PDF resmi, hlm. cetak 251 (PDF #page=303) | Terbuka | **Form 06.00 XVIII Jenis Debitur (RO)**; Form 06.00 XXIV Penjamin (golongan penjamin). Juga dipakai form lain: 00.16 XI Golongan Pihak Lawan, 11.00/12.00 Golongan Nasabah, 00.07 II Gol. Kreditur, 13.00 IV Jenis Bank |
| **Lampiran 03 — Daftar Sandi Kabupaten atau Kota** | Kabupaten/kota per provinsi, sandi **4 digit** (mis. 0197 Kota Depok, 1291 Kota Surabaya) | PDF resmi, hlm. cetak 252 (PDF #page=304) | Terbuka | **Form 05.00 III Lokasi Bank (RO)**; Form 06.00 XXII Lokasi Penggunaan. Juga: Form 00.00 butir 3 (migrasi `000046`), 00.16 XVI Lokasi, 11.00/12.00 Lokasi Nasabah |
| **Lampiran 04 — Daftar Sandi Valuta Asing** | Sandi valuta asing | PDF resmi, hlm. cetak 264 (PDF #page=316) | Terbuka | Kolom valuta pada form penempatan/simpanan/kredit (bukan RO di CELAH) |
| **Lampiran 05 — Daftar Sandi Sektor Ekonomi** | Sektor/lapangan usaha (label + sandi **6 digit** + definisi; mirip KBLI), mis. A00000 pertanian, 011200 pertanian padi | PDF resmi, hlm. cetak 270 (PDF #page=322) | Terbuka | **Form 06.00 XX Sektor Ekonomi (RO)** |
| **Lampiran 06 — Daftar Sandi Wilayah Kerja OJK** | Wilayah kerja/kantor OJK, sandi **3 digit** (mis. 011 Jabodebek+Banten, 021 Jabar) | PDF resmi, hlm. cetak 368 (PDF #page=420) | Terbuka | Form 00.00 butir 4 (wilayah kerja kantor pusat; migrasi `000046`) |
| **Lampiran 07 — Daftar Sandi LPBBTI** | Penyelenggara fintech P2P lending + sandi (mis. 810011 Akseleran) | PDF resmi, hlm. cetak 369 (PDF #page=421) | Terbuka | **Form 06.00 XLIII Sandi LPBBTI (CELAH: KB)** |
| **Lampiran 08 — Daftar Lembaga Pemeringkat** | Lembaga pemeringkat + sandi | PDF resmi, hlm. cetak 372 (PDF #page=424) | Terbuka | Form 00.16 XII Lembaga Pemeringkat |
| **Lampiran 09 — Daftar Peringkat Surat Berharga** | Peringkat + sandi | PDF resmi, hlm. cetak 373 (PDF #page=425) | Terbuka | Form 00.16 XIII Peringkat |
| **Lampiran 10 — Daftar Sandi Negara** | Negara + sandi | PDF resmi, hlm. cetak 374 (PDF #page=426) | Terbuka | Form 00.16 VII/VIII Kewarganegaraan & Negara |

Pranala langsung per lampiran: tambahkan `#page=N` ke URL PDF di atas, dengan N = kolom
"PDF #page". Contoh: `...Rakyat.pdf#page=303` → Lampiran 02.

## Apa yang sebenarnya membuka 11 kolom RO

Pembacaan form (Lampiran II: FORM 05.00–2/–3, FORM 06.00–2/–3, dan BAB II "Penjelasan Umum
Kolom") menunjukkan **tidak semua 11 kolom RO butuh lampiran**. Rinciannya:

| Kolom CELAH | Sumber sandi sebenarnya | Berkas |
|---|---|---|
| Form 05.00 **II Sandi Bank** | **Bukan Lampiran 02.** Sandi bank 6 digit "sebagaimana terdapat pada Sistem Pelaporan OJK". Lampiran 02 hanya kategori (600/700/…) | APOLO/SPOJK — **tidak ada di PDF**; definisi di PDF #page=140 |
| Form 05.00 **III Lokasi Bank** | Lampiran 03 | PDF #page=304 |
| Form 05.00 **V Hubungan dengan Bank** | *Inline*: 12 = terkait, 20 = tidak terkait | PDF #page=140 (daftar), #page=63 (penjelasan) |
| Form 05.00 **XVI ID Pihak Lawan** | **Bukan sandi OJK.** ID Pihak Lawan = nomor CIF internal BPR (harus sama dengan CIF di SLIK); diatur di BAB II | PDF #page=66 |
| Form 06.00 **II ID Pihak Lawan** | idem — CIF internal | PDF #page=66 |
| Form 06.00 **VIII Jenis Penggunaan** | *Inline*: 10/20/31/32/35/39 | PDF #page=152 |
| Form 06.00 **IX Hubungan dengan Bank** | *Inline*: 11/12/20 | PDF #page=152 (daftar), #page=160 (penjelasan) |
| Form 06.00 **XI Periode Pembayaran Pokok dan Bunga** | *Inline*: 1–8 (harian…setiap saat) | PDF #page=153 |
| Form 06.00 **XVIII Jenis Debitur** | Lampiran 02 | PDF #page=303 |
| Form 06.00 **XIX Sandi Bank** | **Bukan Lampiran 02** — sandi APOLO/SPOJK 6 digit | APOLO/SPOJK — **tidak ada di PDF** |
| Form 06.00 **XX Sektor Ekonomi** | Lampiran 05 | PDF #page=322 |

**Kesimpulan triase:** dari 11 kolom RO, hanya **3** yang benar-benar menunggu lampiran —
dan ketiganya **sudah ditemukan** (Lampiran 02, 03, 05). Enam kolom terisi sandi *inline*
yang sudah ada di PDF; dua kolom "Sandi Bank" butuh daftar sandi APOLO (bukan lampiran
SEOJK), dan dua kolom "ID Pihak Lawan" sebenarnya **internal** (CIF), bukan sandi OJK.
Label "RO — butuh Lampiran 02/03" di CELAH terlalu longgar untuk kolom-kolom itu.

## Belum ditemukan

- **Daftar sandi bank 6 digit** (BPR/BPRS/bank umum) untuk kolom "Sandi Bank". SEOJK
  16/2024 hanya merujuk "sebagaimana terdapat pada Sistem Pelaporan Otoritas Jasa
  Keuangan" — tidak diterbitkan dalam PDF ini. Perlu akses APOLO/SPOJK. **Tidak ditemukan
  sebagai dokumen publik.**
- Lampiran sandi lain yang dirujuk kode **semuanya ditemukan** (Lampiran 01–10).
- Tidak ada berkas lampiran terpisah; seluruh sandi menyatu dalam satu PDF 528 halaman.
- Di `/tmp/ojk/` **tidak ada** `seojk16.pdf` maupun `pa_bpr.pdf`. Yang ada adalah
  `ckpn_93_106.txt` = kutipan **SEOJK 21/SEOJK.03/2024** (Panduan Akuntansi/PA BPR)
  hlm. 93–106 — bukan daftar sandi. PDF SEOJK 16 diunduh ke `/tmp/ojk/s16.pdf` (di luar
  repo) untuk riset ini.

## Cara memakai

Setelah lampiran didapat, langkah berikutnya (belum dikerjakan, perlu keputusan pemilik):

1. **Ekstrak per lampiran** dari PDF ke berkas antara di `/tmp` (bukan repo), mis. per
   rentang halaman: Lampiran 02 (#page 303), 03 (#page 304–315), 05 (#page 322–419).
   Lampiran 02 kecil (satu halaman) bisa diketik manual; 03 belasan halaman; 05 puluhan halaman.
2. **Bangun tabel referensi** (migrasi, bukan berkas besar):
   `ojk_pihak_lawan` (kode 3 digit), `ojk_kabupaten` (4 digit), `ojk_sektor_ekonomi`
   (6 digit + label + definisi). Sertakan versi/asal (SEOJK 16/2024) untuk pelacakan.
3. **Sandi *inline*** (Hubungan dengan Bank, Jenis Penggunaan, Periode Pembayaran, alasan
   diblokir, status BMPK, kualitas, jenis, kategori usaha) tidak perlu tabel DB —
   cukup enum/konstanta di `apps/api/internal/ojkreport`, karena sudah tersurat di PDF.
4. **ID Pihak Lawan** dipetakan ke **CIF internal** (sama dengan SLIK), bukan ke Lampiran 02.
5. **Sandi Bank 6 digit** menunggu akses APOLO/SPOJK; jangan dikarang dari Lampiran 02.
6. Perbarui [`CELAH-LAPORAN-OJK.md`](CELAH-LAPORAN-OJK.md) §2a/§1 setelah register dibuat,
   dan turunkan status kolom yang ternyata *inline*/internal dari RO.

Prinsip lama tetap: **jangan commit PDF** (3,6 MB) ke repo; commit tabel turunannya saja
beserta rujukan halaman cetak.
