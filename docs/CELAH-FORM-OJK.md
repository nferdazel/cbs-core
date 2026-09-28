# Celah Form Laporan Berkala Bulanan BPR — triase seluruh daftar form

**Status:** triase lengkap, 28 Sep 2026. **Bukan** pengganti
[`CELAH-LAPORAN-OJK.md`](CELAH-LAPORAN-OJK.md) — dokumen itu hanya mencakup kolom
Form 05.00/06.00 (46) dan 14 butir laporan. Ini melengkapi: **seluruh daftar form**
Laporan Berkala Bulanan BPR menurut SEOJK No. 16/SEOJK.03/2024.

Sumber: PDF resmi (diunduh ulang ke `/tmp/ojk/s16b.pdf`, 528 halaman). Daftar form
resmi ada di halaman cetak **-7-** (Laporan Gabungan, 24 form) dan **-8-** (Laporan per
Kantor, 24 form); daftar isi di cetak -3- sampai -6-. Setiap klaim di bawah menyertakan
`PDF #page` (nomor halaman PDF, 1-based) atau `file:line`.

Metode: empat pemeriksaan paralel — halaman `-1` (susunan), `SANDI`, dan `PENJELASAN`
tiap form dibaca dari PDF, lalu dicocokkan dengan skema (`packages/db-migrations/*.sql`),
domain, `internal/ojkreport/**`, dan kode `LAPORAN_*`.

## 1. Ringkasan

Form unik di regulasi: **45** (daftar Laporan Gabungan cetak -7- digabung Laporan per
Kantor cetak -8-, dikurangi irisan `01.00`/`01.01`/`02.00`; diverifikasi dengan memindai
seluruh 528 halaman PDF). Terdaftar di sistem (`OJKBulananForms`, `definitions.go:142`):
**12** — `00.00, 00.08, 01.00, 01.01, 02.00, 05.00, 06.00, 09.00, 13.00, 00.13, 00.14,
00.15` (10 buildable, 2 ber-alasan). Sebelas di antaranya termasuk daftar regulasi;
**`00.14` tidak ditemukan di SEOJK 16/2024** — lihat §6. Sisanya **34 form tidak
terdaftar sama sekali** — itulah isi triase ini.

| Klasifikasi | Jumlah | Arti |
|---|---:|---|
| `SUDAH ADA` | 4 | sudah terbit (dengan catatan cakupan) |
| `SEBAGIAN` | 10 | sebagian sumber sudah ada, sebagian kolom belum |
| `BELUM DIMODELKAN` | 11 | tidak ada tabel/kolom/sumber sama sekali |
| `KONDISIONAL` | 6 | hanya dilaporkan bila terjadi X (syaratnya dikutip) |
| `DOKUMEN` | 3 | berkas manual/PDF, bukan angka |

Klasifikasi `PERLU CEK` tidak ada — tidak ada form yang gagal disimpulkan.

## 2. Tabel triase (34 form)

| Form | Nama resmi | Klasifikasi | Sumber & kekurangan utama | PDF #page |
|---|---|---|---|---|
| 00.01 | Data Kepemilikan BPR | `BELUM DIMODELKAN` | Tidak ada tabel pemegang saham sama sekali; yang ada hanya 3 kunci teks untuk Form 00.00 (`000096:25,38,40`) | 72–74 |
| 00.02 | Data Anggota Direksi & DK | `SEBAGIAN` | **Terbit sebagai bagian `LAPORAN_KELEMBAGAAN`** (`kelembagaan.go:73`), sumber `bank_management` (`000112:89`); 5 blok kolom ditandai Unavailable (NIK, Alamat, sertifikat+pendidikan, komite, XIII–XVII) | 75–82 |
| 00.03 | Data Pejabat Eksekutif | `SEBAGIAN` | Terbit di `LAPORAN_KELEMBAGAAN` (`kelembagaan.go:80`); kurang Alamat, NIK, fungsi, keanggotaan komite | 83–87 |
| 00.04 | Data Kantor BPR | `SEBAGIAN` | Terbit di `LAPORAN_KELEMBAGAAN` (`kelembagaan.go:91`), sumber `bank_offices` (`000112:41`); **10 dari 15 blok** Unavailable | 88–95 |
| 00.05 | Data Pihak Terkait Lainnya | `SEBAGIAN` | `bmpk_related_parties` + `customers` (`000103:27`, `000001:24`); kurang sandi Jenis & Hubungan (kini teks bebas), NPWP, pihak terkait bukan nasabah | 96–98 |
| 00.06 | Modal Disetor/Sum bangan/DSM | `SEBAGIAN` | Hanya saldo `30100`/`13100` → `coa_mapping.go:166,174`; tidak ada akun Modal Sumbangan & DSM, tidak ada kolom Jenis & Tanggal Persetujuan OJK | 245–247 |
| 00.07 | Daftar Pinjaman yang Diterima | `BELUM DIMODELKAN` | Tidak ada register pinjaman kreditur; akun liabilitas `20100–20800` tanpa pinjaman-kreditur (`000005:198`); `off_balance_items` hanya komitmen belum ditarik | 248–254 |
| 00.09 | Direksi & DK yang Berhenti | `KONDISIONAL` | Sumber `bank_management` (ada `ended_at`/`note`), **tak ada perakit 00.09**; kurang NIK (keputusan privasi), komite, alasan vs penyebab | 258–263 |
| 00.10 | Pejabat Eksekutif yang Berhenti | `KONDISIONAL` | Sumber `bank_management` + `license_number/date`; **tak ada perakit 00.10**; kurang Surat Pemberhentian (beda dari pengangkatan) | 264–269 |
| 00.11 | Kantor Selain Pusat/Cabang + TPE | `SEBAGIAN` | `bank_offices` (`office_type` teks bebas) + `branches.parent_id`; kurang sandi induk/pendahulu, koordinat, pimpinan, telepon; **tak ada perakit 00.11** | 270–275 |
| 00.12 | Penutupan Kantor & TPE | `KONDISIONAL` | `bank_offices.closed_at` + `status='TUTUP'`; kurang sandi Jenis OJK, sandi induk, koordinat; **tak ada perakit 00.12** | 276–280 |
| 00.16 | Daftar Pihak Lawan | `SEBAGIAN` | `customers` + `customers.ojk_pihak_lawan_code` (`000104:26`) + `counterparty_cif` (`000106:37`); kurang jenis identitas, jenis kelamin, NPWP, kewarganegaraan, tanggal lahir, grup, pemeringkat | 286–292 |
| 00.17 | Laporan Perubahan Ekuitas | `BELUM DIMODELKAN` | 15 baris sandi × 9 komponen ekuitas × 3 tahun; COA ekuitas hanya 5 (`000005:211`) tanpa Cadangan Tujuan/Surplus Revaluasi/DSM/Dividen | 293–294 |
| 00.18 | Laporan Arus Kas | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form18.go` + `COAMapping18Draft`): 41 sandi ditranskrip dari PDF, kolom T dan T-1 dihitung dari jurnal, neto per aktivitas dijaga test invarian; hanya posisi Desember, baris tanpa sumber ditulis `-`. Sebelumnya cuma `GetCashFlow` yang belum dipetakan ke sandi OJK | 295–297 |
| 00.19 | Struktur Organisasi | `DOKUMEN` | Berkas PDF; **bahan sudah ada** (`bank_offices`, `bank_management`, `branches`) sehingga PDF-nya bisa dirakit otomatis | 298 |
| 00.20 | Struktur Kelompok Usaha | `DOKUMEN` | Berkas PDF; bahan hanya satu string `ojk.report.ultimate_shareholders` (`000096:40`) | 299 |
| 00.21 | Dokumen Penilaian Risiko TPPU/TPPT/PPSPM | `DOKUMEN` | `LAPORAN_TPPU_TPPT_PPSPM` sudah terdaftar `Buildable=false` "disusun manual di luar sistem" (`definitions.go:105`) | 300, 60 |
| 03.00 | Daftar Kas dalam Valuta Asing | `KONDISIONAL` | Hanya valas sebagai **pedagang valuta asing**; tidak ada tabel kas valas/tabel kurs; pos `1101020000` sengaja tanpa sumber COA (`mapping_review_test.go:54`); status PVA sudah dicatat di `ojk.report.pva_status` (`000096:28`) | 126–128, 102 |
| 04.00 | Daftar Surat Berharga | `BELUM DIMODELKAN` | Tidak ada register surat berharga (25 kolom) dan tidak ada akun COA "Surat Berharga"; pos `1102000000` hanya sandi (`forms.go:122`) | 129–134 |
| 06.01 | Daftar Agunan | `SEBAGIAN` | `loan_collaterals` (`000036:31`) + `ojk_agunan_ppka_amount` (`000109:31`); kurang alamat agunan, **nilai yang diagunkan**, sandi jenis agunan Lampiran 01 (kini enum 5 nilai), PPKA per agunan | 169–172, 301 |
| 06.02 | Daftar Kredit Sindikasi | `BELUM DIMODELKAN` | Tidak ada tabel/kolom sindikasi (grep nihil); builder sendiri menyatakan kanal penyaluran belum dimodelkan (`form06.go:99`) | 173–177 |
| 07.00 | Daftar Agunan yang Diambil Alih | `BELUM DIMODELKAN` | Hanya saldo agregat COA `10500` → `coa_mapping.go:110`; tidak ada register AYDA (tanggal, nilai pengakuan awal, akum. kerugian, NRV) | 179–181, 104 |
| 08.00 | Daftar Aset Tetap, Inventaris, AT Berwujud | `BELUM DIMODELKAN` | Hanya agregat `10600`/`10700` → `forms.go:136-137`; tidak ada register aset, COA per jenis, sumber perolehan, metode pengukuran. Struktur 9 kolom kini ditranskrip (`docs/transkrip-form-08-10-11-12-14.md`); penjelasannya di **#184–186**, bukan 182–184 | 182–186, 60 |
| 09.01 | Rincian Aset Lainnya – Lain-lain | `KONDISIONAL` | Syarat **25%** dari jumlah aset lainnya; Form 09.00 sudah ada, pos `1299990000` dari COA 10305/10999/11700; kurang register baris "Uraian" | **190**, 188–189 |
| 10.00 | Rincian Liabilitas Segera | `BELUM DIMODELKAN` | Hanya agregat `2101000000` yang diisi 4 COA (`coa_mapping.go:142`); 8 pos rincian `2101010000`–`2101990000` tidak punya akun sama sekali. Transkrip: 4 kolom (Sandi Kantor/Nama Rekening/Sandi/Jumlah), baris **tetap** 8 pos + JUMLAH, rupiah penuh | 191–192, 106 |
| 11.00 | Daftar Tabungan | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form11.go`): baris per-rekening dari `accounts`×`banking_products`(SAVINGS)×`customers` pada `periodEnd`, saldo dari jurnal. 11 dari 18 kolom bersumber; sisanya `-` + alasan: IV Jenis, VIII Jangka Waktu, XI–XII blokir, XIII biaya, XV NIK (privasi), XVI–XVIII PEP/Risiko/Status Data. **Keputusan:** IV dan VIII tetap `-` (jangan diturunkan dari tenor — berisiko salah kelas); **Jumlah = Nominal** karena biaya belum disimpan, keterbatasan ini ditulis di `Notes` | 193–199 |
| 12.00 | Daftar Deposito | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form12.go`): baris per-kontrak dari `time_deposits`×`customers`, `start_date <= periodEnd` dan belum ditutup. **17 kolom** — sama dengan 11.00 **tanpa kolom "Jenis"** (penomoran bergeser satu). 10 kolom bersumber; `-` + alasan untuk X–XI blokir, XII biaya, XIV NIK, XV–XVII PEP/Risiko/Status Data. Kolom "diblokir" tetap tidak dikarang (`bank_deposit_repo.go:30`) | 200–206 |
| 14.00 | Rincian Liabilitas Lainnya | `SEBAGIAN` | 16 pos resmi `2299010100`–`2299990000`; hanya 4 COA (`20400`–`20700`) dan tidak ada satu pun entri `coa_mapping.go` bersandi pos itu. Transkrip: 4 kolom, baris **tetap**, dan aturan **pos "Lainnya" >25% dari total → wajib dirinci ke Form 14.01** | 213–215 |
| 14.01 | Rincian Liabilitas Lainnya – Lain-lain | `KONDISIONAL` | Syarat **25%** dari jumlah liabilitas lainnya; bergantung Form 14.00 lebih dulu | **217**, 216 |
| 15.00 | Daftar Aset Produktif yang Dihapus Buku | `SEBAGIAN` | Status `WRITTEN_OFF` + `written_off_amount` (`000085:29`); **tidak ada tanggal hapus buku**, nominal gabungan (pokok+bunga+denda) tak bisa dipisah per kolom; penempatan tak punya status hapus buku | 218–221 |
| 16.00 | Daftar Penyertaan Modal | `BELUM DIMODELKAN` | Tidak ada tabel register penyertaan dan tidak ada akun COA "Penyertaan Modal" | 223–227 |
| 17.00 | Daftar Properti Terbengkalai | `BELUM DIMODELKAN` | Tidak ada tabel/flag properti terbengkalai (grep nihil; hanya penyebutan dokumentatif) | 229–231 |
| 18.00 | Daftar Aset Keuangan Lainnya | `BELUM DIMODELKAN` | Tidak ada register per rekening, tidak ada akun COA, kata "fraud" tidak muncul di kode | 233–237 |
| 19.00 | Daftar Perbedaan Kualitas Aset Produktif | `SUDAH ADA` | `LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF` + `GET /reports/ojk/perbedaan-kualitas`; **cakupan kredit saja, kolom I–X saja** — XI–XIX "Pada BPR Lain" Unavailable (`perbedaan_kualitas.go:95`) | 239–243 |

## 3. Temuan lintas potong

1. **Bundel mengungkap form yang tidak ikut terbit — SELESAI (28 Sep 2026).** Kini
   `OJKBulananForms` memuat 45 form (persis daftar SEOJK 16/2024) beserta `Buildable:false`
   + alasan, `skippedForms()` (`builder.go:567`) mengumpulkannya, dan `format.go:124`
   menulis baris `# FORM <kode> TIDAK DIBANGUN: <alasan>` ke berkas ekspor — bank tidak lagi
   mengira bundelnya lengkap. Sebelumnya: 12 entri, 34 form hilang tanpa keterangan.
2. **`00.02`/`00.03`/`00.04` sudah dibangun tetapi tidak ikut bundel bulanan.** Ketiganya
   terbit di dalam `LAPORAN_KELEMBAGAAN` (`kelembagaan.go:70-88`) tetapi tidak masuk
   `ExportMonthly` (`ojk_report_handler.go:317`).
3. **Kolom "Sandi Kantor" — SELESAI (28 Sep 2026).** Sumbernya `bank_offices.code`
   (migrasi `000112`: "Sandi kantor Form 00.04 kolom I, 3 angka; kosong = laporan menulis
   `-`"), dibaca lewat `RepoSource.ReportingOffice` (`repo_source.go:348`). Aturan pemilihan
   kantor pelapor: hanya kantor `AKTIF` ber-`code`; **tepat satu** → dipakai, **nol** atau
   **lebih dari satu** → kolom tetap `-` + alasan (sistem tidak memilih kantor pelapor
   sendiri, `office_type` adalah teks bebas bank jadi tidak ditebak). Sudah dipakai Form
   `09.00` dan `01.01`; form berikutnya (`08.00`/`09.01`/`10.00`/`14.00`) tinggal meneruskan
   `kantor` yang sama. Pemeriksaan sebelumnya tetap relevan: `journal_lines` tanpa atribut
   cabang, `journal_entries.branch_id` ada (`000021:17`), `branches.code` tidak
   didokumentasikan sebagai sandi OJK (`000081:67`), `BalanceSheet` menerima `asOf` tetapi
   selalu bank-wide (`reporting_repo.go:168`).
4. **Snapshot akhir periode — DISELESAIKAN dengan query as-of, tanpa tabel snapshot
   (28 Sep 2026).** `ExportMonthly` mengirim `periodEnd = MonthEnd(period)` ke sumber;
   `bank_deposit_repo` dan `savings_customer_repo` kini membaca posisi `asOf` (saldo
   rekening dari jurnal `entry_date <= asOf`; kontrak deposito difilter `start_date <= asOf`
   dan belum ditutup saat itu), bukan keadaan saat ekspor. Dua hal sengaja TIDAK dikarang:
   kolom XII Form 13.00 "Diblokir/Dijaminkan" kini `-` + alasan (`accounts.hold_balance`
   hanya menyimpan keadaan kini), dan baris kredit Form 06.00 (`outstanding_principal`,
   kolektibilitas, DPD) masih keadaan kini karena riwayatnya tidak tersimpan — butuh
   keputusan arsitektur, bukan asumsi.
5. **`00.17`/`00.18` hanya untuk posisi Desember** (`PDF #293`, `#296-297`) — builder punya
   penanda `Deadline` (`builder.go:235`) tetapi tidak ada aturan "hanya Desember". Riset
   28 Sep: aturan "hanya Desember" lain di seluruh Bab II cuma satu — kolom Jenis CKPN
   (Desember 2024 = sandi Kolektif 2) — dan tidak berlaku untuk `08.00`/`10.00`/`11.00`/
   `12.00`/`14.00`.
6. **Inkonsistensi internal PDF pada Form 06.01**: sel "Likuid | Non Likuid" berada di
   kolom VIII pada halaman -1 (`PDF #169`) tetapi penjelasan menaruhnya di bawah kolom IV
   Jenis Agunan (`PDF #171`). Perlu konfirmasi sebelum dibangun.
7. **NIK sengaja tidak disimpan** (keputusan privasi, `kelembagaan.go:175`) → kolom NIK
   pada `00.02`/`00.03`/`00.09`/`00.10` tidak akan terisi tanpa perubahan keputusan.
   Data identitas pribadi `00.16` juga perlu keputusan serupa (tersimpan terenkripsi,
   `000006:10`).

## 4. Aksi berikutnya (diurutkan)

| # | Aksi | Sifat |
|---|---|---|
| A1 | Daftarkan ke-34 form di `definitions.go` dengan `Buildable:false` + alasan konkret dari triase ini, supaya bundel menyebut form yang tidak ikut terbit | kode murni, tanpa keputusan regulasi |
| A2 | ~~Sumberkan kolom Sandi Kantor + snapshot saldo akhir bulan~~ **SELESAI 28 Sep 2026** — `bank_offices.code` + query as-of (rincian di §3 poin 3 dan 4) | prasyarat banyak form |
| A3 | ~~Bentuk form `SEBAGIAN`~~ **`11.00` dan `12.00` SELESAI 28 Sep 2026**; sisa `14.00` (butuh COA per posisi `2299xxxxxx`, lihat barisnya) dan `06.01` (ditahan — §3 poin 6). Transkrip: `docs/transkrip-form-08-10-11-12-14.md` | perlu kolom tambahan + keputusan kolom tanpa sumber |
| A4 | Modul baru untuk `BELUM DIMODELKAN`: `04.00`, `08.00`, `07.00`, `10.00`, `16.00`, `17.00`, `18.00`, `00.07`, `00.01`, `00.17`. Transkrip struktur sudah ada di `docs/transkrip-form-a4-a5-a6.md` | skema baru; urutkan menurut kebutuhan bank |
| A5 | Bentuk `KONDISIONAL` setelah induknya ada (`09.01`, `14.01`, `00.09`, `00.10`, `00.12`, `03.00`). Transkrip + pemicu tiap form di `docs/transkrip-form-a4-a5-a6.md` | turunan |
| A6 | `DOKUMEN` (`00.19`, `00.20`, `00.21`) — `00.19` bisa dirakit otomatis dari data kelembagaan, dua lainnya tetap manual. Rujukan: `docs/transkrip-form-a4-a5-a6.md` | keputusan bank |

## 7. Anomali hasil transkrip A4/A5/A6 (perlu konfirmasi sebelum dibangun)

Hasil riset PDF 28 Sep 2026 (`docs/transkrip-form-a4-a5-a6.md`, 17 form, semua ketemu):

1. **`00.10` kolom III Jabatan** dipecah jadi 5 sub-kolom fungsi di header (`PDF #264`)
   tetapi tabel sandinya (`#266`) hanya `00/01/02` tanpa pemetaan per fungsi — paling
   berisiko untuk implementasi.
2. **`00.09` kolom V** dipecah jadi 4 sub-kolom komite, masing-masing bersandi `00/01/02`
   — encoding per komite perlu dikonfirmasi.
3. **`16.00` typo romawi `XII` dua kali** di header (`#224`); ikuti tabel sandi (XII–XV).
4. **`09.01` vs `14.01` tidak simetris**: penjelasan `09.01` mewajibkan Jumlah = pos
   "Lainnya" Form 09.00, `14.01` tidak menyebut Form 14.00.
5. **`04.00` kolom XV Kualitas** hanya sandi `1/3/5` (melompati 2 dan 4 yang ada di Bab II).
6. **`16.00`/`17.00`/`18.00` tanpa baris JUMLAH**, sedangkan `00.07`/`04.00`/`07.00` punya.
7. **`18.00` format tanggal `TT-MM-TTTT`**, beda dari `TT-BB-TTTT` di form lain.
8. Label halaman `– 1` diulang untuk form multi-halaman (`00.07`/`00.09`/`00.10`) — jangan
   dipakai sebagai penanda; rentang triase `09.01` juga menyesatkan (form di `#189`,
   `#188` milik 09.00).

## 6. Selisih: Form 00.14 — TERVERIFIKASI, bukan form SEOJK 16/2024

**Kesimpulan: `00.14` ada di SEOJK No. 12/SEOJK.03/2022 (halaman PDF 231), bukan di
SEOJK No. 16/SEOJK.03/2024** — dan SEOJK 12/2022 justru dicabut oleh SEOJK 16/2024
(`KEPUTUSAN.md:88-91`). Pemindaian 528 halaman PDF 16/2024: `00.14` nol kemunculan
(bandingkan `00.13` 5 kali, `00.15` 6 kali); daftar formnya melompat dari `00.13` ke `00.15`.

Bukti tambahan: **isi formnya juga tidak cocok.** Form 00.14-1 resmi (SEOJK 12/2022
halaman PDF 231) memakai sandi tetap TPPU/APU-PPT — `1000` PEP, `2000` jenis nasabah
penyimpan (`2100`/`2200`), `3000` komposisi risiko (`3100`/`3200`/`3300`), `4000` jenis
simpanan (`4100`/`4200`) — kolom *Jenis Nasabah/Simpanan · Sandi · Nominal · Jumlah
Nasabah/Rekening*. Sementara `form00_14.go` membangun rincian **per produk** dengan
kolom *Jenis/Kode/Nama Produk + Golongan Nasabah (Lampiran 02) + Jumlah Rekening + Total
Nominal* — format yang berbeda.

**Keputusan:** (1) klaim sitasi di `KEPUTUSAN.md:95` sudah dikoreksi; (2) label "Form
00.14" untuk agregasi jenis nasabah per produk **ditarik dari pelaporan OJK** — nomor itu
tidak dipakai regulasi yang berlaku; agregasi tetap berguna sebagai data internal, tetapi
tidak boleh disebut nomor form OJK; (3) `CELAH-LAPORAN-OJK.md` tetap mencatatnya sebagai
kemampuan sistem, bukan kewajiban.

## 5. Yang menjadi milik bank (bukan celah kode)

- Verifikasi pemetaan COA (`DRAF-BELUM-TERVERIFIKASI`), ratifikasi parameter CKPN,
  partisipasi KUR/LPBBTI/Laku Pandai, tarif ta'zir, Sandi Bank APOLO.
- Keputusan privasi: NIK/NPWP/identitas pihak lawan pada keluaran laporan.
- Konfirmasi OJK atas inkonsistensi Form 06.01 (butir 6 di §3).
