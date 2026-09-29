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
seluruh 528 halaman PDF). **Seluruh 45 kini terdaftar di `OJKBulananForms`**, **28 di antaranya `Buildable:true`**
(angka resmi dijaga `definitions_test.go`); yang belum terbit `Buildable:false` + alasan dan ikut tercetak di
berkas ekspor sebagai `# FORM <kode> TIDAK DIBANGUN`. **`00.14` tidak ditemukan di SEOJK
16/2024** dan sudah ditarik dari manifest — lihat §6. Tabel di bawah merinci 34 form yang
waktu triase belum terdaftar; kolom klasifikasinya sudah diperbarui sejak itu.

| Klasifikasi | Jumlah | Arti |
|---|---:|---|
| `SUDAH ADA` | 14 | sudah terbit (dengan catatan cakupan) |
| `SEBAGIAN` | 8 | sebagian sumber sudah ada, sebagian kolom belum |
| `BELUM DIMODELKAN` | 5 | tidak ada tabel/kolom/sumber sama sekali |
| `KONDISIONAL` | 4 | hanya dilaporkan bila terjadi X (syaratnya dikutip) |
| `DOKUMEN` | 3 | berkas manual/PDF, bukan angka |

Klasifikasi `PERLU CEK` tidak ada — tidak ada form yang gagal disimpulkan.

## 2. Tabel triase (34 form)

| Form | Nama resmi | Klasifikasi | Sumber & kekurangan utama | PDF #page |
|---|---|---|---|---|
| 00.01 | Data Kepemilikan BPR | `SUDAH ADA` | **Register kedua — dibangun 28 Sep 2026** (`kepemilikan_bpr_register`, migrasi `000116`, pola AYDA) dengan API `reports/ojk/kepemilikan` + `form00_01.go`; baris per pemegang tanpa JUMLAH, Σ persentase tidak divalidasi, alamat kosong ikut aturan <2% tanpa menghapus data. **Kolom IV No. Identitas selalu `-` + alasan** — keputusan privasi (NIK tidak disimpan) tetap berlaku; kolom itu butuh keputusan pemilik bila OJK menolak `-`. UI web menyusul (AYDA sudah punya di `pengaturan/ojk`). Tidak ada tabel pemegang saham sama sekali; yang ada hanya 3 kunci teks untuk Form 00.00 (`000096:25,38,40`) | 72–74 |
| 00.02 | Data Anggota Direksi & DK | `SEBAGIAN` | **Terbit sebagai bagian `LAPORAN_KELEMBAGAAN`** (`kelembagaan.go:73`), sumber `bank_management` (`000112:89`); 5 blok kolom ditandai Unavailable (NIK, Alamat, sertifikat+pendidikan, komite, XIII–XVII) | 75–82 |
| 00.03 | Data Pejabat Eksekutif | `SEBAGIAN` | Terbit di `LAPORAN_KELEMBAGAAN` (`kelembagaan.go:80`); kurang Alamat, NIK, fungsi, keanggotaan komite | 83–87 |
| 00.04 | Data Kantor BPR | `SEBAGIAN` | Terbit di `LAPORAN_KELEMBAGAAN` (`kelembagaan.go:91`), sumber `bank_offices` (`000112:41`); **10 dari 15 blok** Unavailable | 88–95 |
| 00.05 | Data Pihak Terkait Lainnya | `SEBAGIAN` | `bmpk_related_parties` + `customers` (`000103:27`, `000001:24`); kurang sandi Jenis & Hubungan (kini teks bebas), NPWP, pihak terkait bukan nasabah | 96–98 |
| 00.06 | Modal Disetor/Sum bangan/DSM | `SEBAGIAN` | Hanya saldo `30100`/`13100` → `coa_mapping.go:166,174`; tidak ada akun Modal Sumbangan & DSM, tidak ada kolom Jenis & Tanggal Persetujuan OJK | 245–247 |
| 00.07 | Daftar Pinjaman yang Diterima | `SUDAH ADA` | **Register ketiga — dibangun 29 Sep 2026** (`pinjaman_diterima_register`, migrasi `000117`) dengan API `reports/ojk/pinjaman` + `form00_07.go`. Gol. Kreditur → FK `ojk_pihak_lawan` (Lampiran 02), Lokasi → FK `ojk_kabupaten` (Lampiran 03), aturan tanpa agunan ditegakkan dua arah (CHECK DB + domain), **XV Baki Debet Neto dihitung** `XII−XIII−XIV` (negatif → `-`), JUMLAH hanya bila lengkap. Kolom I = ID Pihak Lawan, jadi **tanpa Sandi Kantor** (sesuai transkrip). Tidak ada register pinjaman kreditur; akun liabilitas `20100–20800` tanpa pinjaman-kreditur (`000005:198`); `off_balance_items` hanya komitmen belum ditarik | 248–254 |
| 00.09 | Direksi & DK yang Berhenti | `KONDISIONAL` | Sumber `bank_management` (ada `ended_at`/`note`), **tak ada perakit 00.09**; kurang NIK (keputusan privasi), komite, alasan vs penyebab | 258–263 |
| 00.10 | Pejabat Eksekutif yang Berhenti | `KONDISIONAL` | Sumber `bank_management` + `license_number/date`; **tak ada perakit 00.10**; kurang Surat Pemberhentian (beda dari pengangkatan) | 264–269 |
| 00.11 | Kantor Selain Pusat/Cabang + TPE | `SEBAGIAN` | `bank_offices` (`office_type` teks bebas) + `branches.parent_id`; kurang sandi induk/pendahulu, koordinat, pimpinan, telepon; **tak ada perakit 00.11** | 270–275 |
| 00.12 | Penutupan Kantor & TPE | `KONDISIONAL` | `bank_offices.closed_at` + `status='TUTUP'`; kurang sandi Jenis OJK, sandi induk, koordinat; **tak ada perakit 00.12** | 276–280 |
| 00.16 | Daftar Pihak Lawan | `SEBAGIAN` | `customers` + `customers.ojk_pihak_lawan_code` (`000104:26`) + `counterparty_cif` (`000106:37`); kurang jenis identitas, jenis kelamin, NPWP, kewarganegaraan, tanggal lahir, grup, pemeringkat | 286–292 |
| 00.17 | Laporan Perubahan Ekuitas | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form17.go`, versi jujur-parsial): 17 baris × 12 kolom, **hanya Desember** (gerbang sama dengan 00.18), tiga saldo akhir tahun dari COA. Hanya III Modal Disetor (`30100`/`13100`), X Cadangan Umum (`30400`), XI Saldo Laba (`30200`+`30300`/`13200`) yang bersumber; kolom IV–IX tanpa akun → `-`, baris mutasi TIDAK diisi delta, "Pos Penambah/Pengurang Lainnya" tidak diisi residu. 15 baris sandi × 9 komponen ekuitas × 3 tahun; COA ekuitas hanya 5 (`000005:211`) tanpa Cadangan Tujuan/Surplus Revaluasi/DSM/Dividen | 293–294 |
| 00.18 | Laporan Arus Kas | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form18.go` + `COAMapping18Draft`): 41 sandi ditranskrip dari PDF, kolom T dan T-1 dihitung dari jurnal, neto per aktivitas dijaga test invarian; hanya posisi Desember, baris tanpa sumber ditulis `-`. Sebelumnya cuma `GetCashFlow` yang belum dipetakan ke sandi OJK | 295–297 |
| 00.19 | Struktur Organisasi | `DOKUMEN` | Berkas PDF; **bahan sudah ada** (`bank_offices`, `bank_management`, `branches`) sehingga PDF-nya bisa dirakit otomatis | 298 |
| 00.20 | Struktur Kelompok Usaha | `DOKUMEN` | Berkas PDF; bahan hanya satu string `ojk.report.ultimate_shareholders` (`000096:40`) | 299 |
| 00.21 | Dokumen Penilaian Risiko TPPU/TPPT/PPSPM | `DOKUMEN` | `LAPORAN_TPPU_TPPT_PPSPM` sudah terdaftar `Buildable=false` "disusun manual di luar sistem" (`definitions.go:105`) | 300, 60 |
| 03.00 | Daftar Kas dalam Valuta Asing | `KONDISIONAL` | Hanya valas sebagai **pedagang valuta asing**; tidak ada tabel kas valas/tabel kurs; pos `1101020000` sengaja tanpa sumber COA (`mapping_review_test.go:54`); status PVA sudah dicatat di `ojk.report.pva_status` (`000096:28`) | 126–128, 102 |
| 04.00 | Daftar Surat Berharga | `BELUM DIMODELKAN` | Tidak ada register surat berharga (25 kolom) dan tidak ada akun COA "Surat Berharga"; pos `1102000000` hanya sandi (`forms.go:122`) | 129–134 |
| 06.01 | Daftar Agunan | `SEBAGIAN` | `loan_collaterals` (`000036:31`) + `ojk_agunan_ppka_amount` (`000109:31`); kurang alamat agunan, **nilai yang diagunkan**, sandi jenis agunan Lampiran 01 (kini enum 5 nilai), PPKA per agunan | 169–172, 301 |
| 06.02 | Daftar Kredit Sindikasi | `BELUM DIMODELKAN` | Tidak ada tabel/kolom sindikasi (grep nihil); builder sendiri menyatakan kanal penyaluran belum dimodelkan (`form06.go:99`) | 173–177 |
| 07.00 | Daftar Agunan yang Diambil Alih | `SUDAH ADA` | **Register pertama — dibangun 28 Sep 2026** (`ayda_register`, migrasi `000115`, pola `off_balance_items`) dengan API `reports/ojk/ayda` + `form07.go`. Kolom II–VI dan VIII dari register bank; **VII Jumlah dihitung** `min(NRV, Nilai Pengakuan Awal − Akumulasi KPN)`; JUMLAH hanya bila seluruh baris lengkap. Tie ke COA `10500`/pos `1201000000` **tidak otomatis** — dinyatakan di `Notes`. Hanya saldo agregat COA `10500` → `coa_mapping.go:110`; tidak ada register AYDA (tanggal, nilai pengakuan awal, akum. kerugian, NRV) | 179–181, 104 |
| 08.00 | Daftar Aset Tetap, Inventaris, AT Berwujud | `SUDAH ADA` | **Register kelima — dibangun 29 Sep 2026** (`aset_tetap_register`, migrasi `000119`) dengan API `reports/ojk/aset-tetap` + `form08_00.go`. Bank mengisi **per aset**, builder **mengelompokkan** per kombinasi Jenis×Sumber×Status×Metode (aturan gabung PDF); kolom VIII Nilai Tercatat **turunan** `V−VI−VII` (negatif → `-`); Status Aset nullable — 2xx (tidak berwujud) selalu kosong, 1xx kosong → `-`+alasan; JUMLAH hanya bila lengkap; tie ke `1202010000`/`1202020000` tidak dipaksa (di-Notes) — penjelasan sebelumnya soal "hanya agregat `10600`/`10700`" kini tergantikan register ini. Hanya agregat `10600`/`10700` → `forms.go:136-137`; tidak ada register aset, COA per jenis, sumber perolehan, metode pengukuran. Struktur 9 kolom kini ditranskrip (`docs/transkrip-form-08-10-11-12-14.md`); penjelasannya di **#184–186**, bukan 182–184 | 182–186, 60 |
| 09.01 | Rincian Aset Lainnya – Lain-lain | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form09_01.go`): kondisional — terbit hanya bila pos Lainnya >25% dari total aset lainnya (penyebut = pos `1299000000` Form 01.00, ambang persis 25% tidak memicu); baris per akun COA sehingga total dijamin sama dengan pos Lainnya Form 09.00; Syarat **25%** dari jumlah aset lainnya; Form 09.00 sudah ada, pos `1299990000` dari COA 10305/10999/11700; kurang register baris "Uraian" | **190**, 188–189 |
| 10.00 | Rincian Liabilitas Segera | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form10.go`): baris tetap 8 pos + JUMLAH, `COAMapping10Draft` hanya memetakan `20400`/`20600`/`12400` ke pos "Lainnya" (`2101990000`); 7 pos lain belum punya akun → alasan. **Pos 1 (pajak) sengaja `-`**: definisinya "pajak periode sebelum bulan laporan **yang dibayarkan pada bulan laporan**" (arus), sedangkan yang tersedia hanya saldo akhir periode — tidak dipaksa; `20500` juga tidak dipindah ke "Lainnya" agar tidak menggandakan pos Utang Pajak Form 14.00. JUMLAH `-` selama sebagian pos ber-alasan | 191–192, 106 |
| 11.00 | Daftar Tabungan | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form11.go`): baris per-rekening dari `accounts`×`banking_products`(SAVINGS)×`customers` pada `periodEnd`, saldo dari jurnal. 11 dari 18 kolom bersumber; sisanya `-` + alasan: IV Jenis, VIII Jangka Waktu, XI–XII blokir, XIII biaya, XV NIK (privasi), XVI–XVIII PEP/Risiko/Status Data. **Keputusan:** IV dan VIII tetap `-` (jangan diturunkan dari tenor — berisiko salah kelas); **Jumlah = Nominal** karena biaya belum disimpan, keterbatasan ini ditulis di `Notes` | 193–199 |
| 12.00 | Daftar Deposito | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form12.go`): baris per-kontrak dari `time_deposits`×`customers`, `start_date <= periodEnd` dan belum ditutup. **17 kolom** — sama dengan 11.00 **tanpa kolom "Jenis"** (penomoran bergeser satu). 10 kolom bersumber; `-` + alasan untuk X–XI blokir, XII biaya, XIV NIK, XV–XVII PEP/Risiko/Status Data. Kolom "diblokir" tetap tidak dikarang (`bank_deposit_repo.go:30`) | 200–206 |
| 14.00 | Rincian Liabilitas Lainnya | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form14.go`): 17 pos tetap (koreksi dari catatan lama "16 pos"). `COAMapping14Draft` hanya memetakan `20500`→`2299020000` Utang Pajak dan `20700`→`2299990000` Lainnya; 15 pos lain belum punya akun COA → alasan, bukan nol, dan `20300`/`12500`/`12900` sengaja tidak ditebak ke pos mana pun. Akibatnya **tidak ada tie otomatis ke `2299000000` Form 01.00** — hal ini ditulis eksplisit di `Notes` bersama aturan >25% → Form 14.01 | 213–215 |
| 14.01 | Rincian Liabilitas Lainnya – Lain-lain | `SUDAH ADA` | **Dibangun 28 Sep 2026** (`form14_01.go`): kondisional, penyebut = pos `2299000000` Form 01.00; PDF tidak menegaskan ikatan total seperti 09.01 — dicatat di `Notes`; Syarat **25%** dari jumlah liabilitas lainnya; bergantung Form 14.00 lebih dulu | **217**, 216 |
| 15.00 | Daftar Aset Produktif yang Dihapus Buku | `SEBAGIAN` | Status `WRITTEN_OFF` + `written_off_amount` (`000085:29`); **tidak ada tanggal hapus buku**, nominal gabungan (pokok+bunga+denda) tak bisa dipisah per kolom; penempatan tak punya status hapus buku | 218–221 |
| 16.00 | Daftar Penyertaan Modal | `BELUM DIMODELKAN` | Tidak ada tabel register penyertaan dan tidak ada akun COA "Penyertaan Modal" | 223–227 |
| 17.00 | Daftar Properti Terbengkalai | `SUDAH ADA` | **Register keempat — dibangun 29 Sep 2026** (`properti_terbengkalai_register`, migrasi `000118`) dengan API `reports/ojk/properti` + `form17_00.go`. **No. Register unik & no-reuse**: `UNIQUE` penuh + hapus = soft-delete (`NONAKTIF`), baris NONAKTIF tetap memegang nomor (diuji guard + integrasi). Kolom I Sandi Kantor dan IX Jumlah (`VII−VIII`) turunan; tanpa baris JUMLAH (sesuai PDF). Tidak ada tabel/flag properti terbengkalai (grep nihil; hanya penyebutan dokumentatif) | 229–231 |
| 18.00 | Daftar Aset Keuangan Lainnya | `BELUM DIMODELKAN` | Tidak ada register per rekening, tidak ada akun COA, kata "fraud" tidak muncul di kode | 233–237 |
| 19.00 | Daftar Perbedaan Kualitas Aset Produktif | `SUDAH ADA` | `LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF` + `GET /reports/ojk/perbedaan-kualitas`; **cakupan kredit saja, kolom I–X saja** — XI–XIX "Pada BPR Lain" Unavailable (`perbedaan_kualitas.go:95`) | 239–243 |

## 3. Temuan lintas potong

1. **Bundel mengungkap form yang tidak ikut terbit — SELESAI (28 Sep 2026).** Kini
   `OJKBulananForms` memuat 45 form (persis daftar SEOJK 16/2024) beserta `Buildable:false`
   + alasan, `skippedForms()` (`builder.go:567`) mengumpulkannya, dan `format.go:124`
   menulis baris `# FORM <kode> TIDAK DIBANGUN: <alasan>` ke berkas ekspor — bank tidak lagi
   mengira bundelnya lengkap. Sebelumnya: 12 entri, 34 form hilang tanpa keterangan.
2. **`00.02`/`00.03`/`00.04` ikut bundel bulanan — SELESAI (28 Sep 2026).** Daftar resmi
   Laporan Gabungan (`PDF #59` butir 3–5) menempatkan ketiganya di laporan bulanan, jadi
   ketidakhadiran mereka dulu memang cacat kelengkapan. Kini `buildTables` memanggil sumber
   kelembagaan (`RepoSource.KelembagaanReport`) dan meng-append bagian yang berisi data;
   bagian kosong dicatat di `SkippedForms` dengan alasan, bukan muncul sebagai tabel kosong.
   `LAPORAN_KELEMBAGAAN` tidak berubah; komentar "tidak ikut bundel" dan angka `00.17`
   (17 baris × 12 kolom, diverifikasi di `PDF #293`) di `definitions.go` sudah diperbaiki.
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
5. **`00.17`/`00.18` hanya untuk posisi Desember** (`PDF #293`, `#296-297`) — kini **sudah
   ditegakkan** untuk `00.18` lewat gerbang Desember + `runtimeSkipped` di `builder.go`
   (bukan Desember → `# FORM … TIDAK DIBANGUN`, pola `form18.go`); `00.17` bisa memakai
   pola yang sama saat dibangun. Riset 28 Sep: aturan "hanya Desember" lain di seluruh
   Bab II cuma satu — kolom Jenis CKPN (Desember 2024 = sandi Kolektif 2) — dan tidak
   berlaku untuk `08.00`/`10.00`/`11.00`/`12.00`/`14.00`.
6. **Inkonsistensi internal PDF pada Form 06.01**: sel "Likuid | Non Likuid" berada di
   kolom VIII pada halaman -1 (`PDF #169`) tetapi penjelasan menaruhnya di bawah kolom IV
   Jenis Agunan (`PDF #171`). Perlu konfirmasi sebelum dibangun.
7. **NIK sengaja tidak disimpan** (keputusan privasi, `kelembagaan.go:175`) → kolom NIK
   pada `00.02`/`00.03`/`00.09`/`00.10` tidak akan terisi tanpa perubahan keputusan.
8. **Form 06.00 tidak bisa dibaca penuh pada posisi akhir periode — keputusan 28 Sep 2026.**
   Hasil riset: riwayat kolektibilitas/DPD/CKPN **tidak ada sama sekali** (run PPAP harian
   menimpa baris pinjaman, `ppap_repo.go:204-216`; `loan_ckpn_individual_assessments`
   belum pernah ditulis kode); `journal_lines` tidak punya kolom pinjaman — pengait
   per-kredit hanya lewat format `idempotency_key` yang menurut repo sendiri bukan kontrak
   skema (`loan_repo.go:1138-1139`); dan jadwal dihapus saat restrukturisasi
   (`loan_repo.go:614-620`). **Keputusan: pakai pola jujur** — kolom XIV Kualitas dan XVI
   DPD selalu `-` + alasan; kolom materiil lain (XXVIII baki debet, XXXIII neto, XXXIV
   CKPN, XXXV bunga akruan, XVII tunggakan) hanya diisi angka **bila `periodEnd` jatuh pada
   bulan berjalan** (posisi kini ≈ posisi akhir periode), periode lampau `-` + alasan.
   Alternatif ditahan: rekonstruksi dari jurnal (tanpa migrasi, tetapi rapuh terhadap
   format kunci) dan snapshot + riwayat kolektibilitas (butuh migrasi ganda; catat langkah
   EOD terjadwal masih nonaktif, `000077:127-131`) — hanya bila OJK menuntut periode lampau.

9. **Data referensi Lampiran belum punya endpoint** (temuan 29 Sep). Tabel `ojk_pihak_lawan`
   (Lampiran 02) dan `ojk_kabupaten` (Lampiran 03) ada di DB, tetapi tidak ada endpoint yang
   menyajikannya ke web — semua UI (termasuk kartu Pinjaman yang baru) mengisi sandi sebagai
   **teks bebas + hint**, tanpa mengarang daftar. Bila ingin dropdown, perlu endpoint read-only
   terpisah (mis. `GET /reports/ojk/reference/…`, izin `system:config:read`); daftar Lampiran 01
   (jenis agunan) dan Lampiran 04 (valuta) bahkan belum ada sebagai tabel.

## 4. Aksi berikutnya (diurutkan)

| # | Aksi | Sifat |
|---|---|---|
| A1 | Daftarkan ke-34 form di `definitions.go` dengan `Buildable:false` + alasan konkret dari triase ini, supaya bundel menyebut form yang tidak ikut terbit | kode murni, tanpa keputusan regulasi |
| A2 | ~~Sumberkan kolom Sandi Kantor + snapshot saldo akhir bulan~~ **SELESAI 28 Sep 2026** — `bank_offices.code` + query as-of (rincian di §3 poin 3 dan 4) | prasyarat banyak form |
| A3 | ~~Bentuk form `SEBAGIAN`~~ **SELESAI 28 Sep 2026 untuk `11.00`, `12.00`, `14.00`**; sisa `06.01` ditahan menunggu konfirmasi OJK (§3 poin 6). Transkrip: `docs/transkrip-form-08-10-11-12-14.md` | perlu kolom tambahan + keputusan kolom tanpa sumber |
| A4 | Modul baru untuk `BELUM DIMODELKAN` — **pola register kini terbukti lima kali** (`ayda_register`, `kepemilikan_bpr_register`, `pinjaman_diterima_register`, `properti_terbengkalai_register`, `aset_tetap_register`; AYDA + Kepemilikan + Pinjaman + Properti sudah punya UI di `pengaturan/ojk`), jadi sisa form tinggal mengulang pola yang sama: `04.00`, `16.00`, `18.00`. Peringkat nilai÷biaya ada di catatan riset; transkrip di `docs/transkrip-form-a4-a5-a6.md` | skema baru; urutkan menurut kebutuhan bank |
| A5 | **SELESAI 28 Sep 2026 kecuali `03.00`**: `09.01` + `14.01` (ambang 25%, baris per akun COA) dan `00.09` + `00.10` + `00.12` (jendela peristiwa dalam bulan periode dari `bank_management`/`bank_offices`; NIK, komite, penyebab, sandi jenis/induk/koordinat tetap `-` + alasan). Sisa `03.00` butuh register valas + kurs + Lampiran 04 → jalur A4. Transkrip: `docs/transkrip-form-a4-a5-a6.md` | turunan |
| A6 | `DOKUMEN` (`00.19`, `00.20`, `00.21`) — `00.19` bisa dirakit otomatis dari data kelembagaan, dua lainnya tetap manual. Rujukan: `docs/transkrip-form-a4-a5-a6.md` | keputusan bank |

## 7. Anomali transkrip A4/A5/A6 — SEMUA TERPECAHKAN DARI PDF (28 Sep 2026)

Pemeriksaan ulang terhadap teks resmi menutup kedelapan butir; **tidak ada yang
benar-benar butuh konfirmasi OJK** untuk bisa diimplementasikan:

1. **`00.10` kolom III Jabatan** — penjelasan `PDF #268-269` mengurutkan 5 fungsi **sama
   persis** dengan sub-kolom header `#264`; pemetaan posisional → tiap sub-kolom diisi
   sandi `00/01/02` per fungsi. (Konfirmasi OJK hanya bila ingin pasti soal semantik `00`.)
2. **`00.09` kolom V komite** — pola sama: penjelasan `#262` mengurutkan 4 komite sejajar
   header `#258` → `00` tidak / `01` ketua / `02` anggota per sub-kolom.
3. **`09.01` vs `14.01`** — perbedaan memang nyata di teks (hanya `09.01` yang mengikat
   "harus sama dengan pos Lainnya Form 09.00") → ikuti apa adanya, bukan salah baca.
4. **`04.00` kolom XV Kualitas `1/3/5`** — tabel sandi form memang `1/3/5` (`#133`),
   identik Form 05.00 (`#140`); penjelasan `#135` merujuk Bab II (5 sandi), tetapi form
   ini memakai subset → bukan typo.
5. **Label halaman `– 1` diulang** pada form multi-halaman (`#264-265`, `#258-259`) →
   nomor halaman cetak tidak boleh dipakai penanda.
6. **`16.00` typo `XII` dua kali** (`#224`) → ikuti tabel sandi `#225`; label ke-4 `XIII`.
7. **`16.00`/`17.00`/`18.00` memang tanpa baris JUMLAH** (`#223-224`, `#229-230`,
   `#233-234`) → jangan dirender, jangan divalidasi.
8. **`18.00` format tanggal `TT-MM-TTTT`** (`#235`) → beda dari form lain; ikuti.

Tambahan: `PDF #486` menyebut "Kolom V (Kualitas) pada Form 04.00 – Daftar Surat
Berharga" padahal konteksnya Penyertaan Modal (Form **16.00**) — salah cetak regulasi,
jangan diikuti.

Satu-satunya butir yang masih menunggu pihak luar adalah **§3 poin 6 (Form 06.01)**,
yaitu inkonsistensi internal PDF soal sel "Likuid | Non Likuid", bukan salah satu dari
delapan anomali ini.

## 8. Audit query as-of (28 Sep 2026) — mana yang aman, mana yang butuh keputusan

Audit baca-saja terhadap seluruh sumber laporan yang memakai `asOf`/`periodEnd`,
berfokus pada multiplicitas baris (ganda vs hilang). Ringkasnya:

**Aman (teragregasi / unik per entitas):** `BalanceSheet`, `IncomeStatement`,
`CashFlow`, `TrialBalance` (SUM jurnal `GROUP BY` COA); Form 13.00 (saldo per
`account_number` unik + kontrak satu baris); Form 11.00 (CTE per `account_id`, arah
jurnal identik `reporting_repo.go:170-171`); Form 12.00 (unik per kontrak); agregat
jenis nasabah; BMPK per pihak (`GROUP BY customer_id` + upsert); pinjaman (satu baris
per `loan_number`).

**Perlu keputusan, bukan sekadar kode:**
1. **`lps_placements` → Form 05.00.** `lps_placement_repo.go:146` hanya `as_of <= asOf`
   tanpa dedup; tabel tanpa UNIQUE selain PK (`000045:28-52`) dan **tabel ini tidak punya
   jalur INSERT di aplikasi** — diisi bank lewat SQL. Dua skenario: bank **menimpa** baris
   (menaikkan `as_of`) → penempatan **hilang** dari bulan-bulan sebelumnya; bank **menyimpan
   snapshot per bulan** → semua baris terambil dan nominal **terlipat** (`form05.go:228-237`
   tanpa dedup). Invarian tertulis di `docs/CKPN-SIAP-RILIS.md:350-353` menyebut pola
   "satu baris ditimpa", tetapi tidak dicegah constraint apa pun. **Keputusan bank:** semantik
   `as_of` (tanggal posisi vs tanggal input) — tanpa itu query tidak boleh diubah.
2. **`off_balance_items` → Form 01.01.** `off_balance_repo.go:69-70` memakai
   `status='AKTIF'` (keadaan kini) untuk semua bulan, dan `:123` menimpa `as_of` saat edit
   tanpa unique `(position_code, as_of)` → pos bisa hilang dari bulan lama atau terhitung
   ganda dalam satu bulan.
3. **Rollover deposito menulis ulang `start_date`** (`deposit_repo.go:233-246`) → kontrak
   ARO hilang dari bulan sebelumnya dan/atau nominal terbaca pasca-rollover (Form 12.00 dan
   bagian Form 13.00). Sudah diakui di komentar repo; query tidak bisa merekonstruksi yang
   sudah ditimpa — perubahan jalur tulis.
4. **BMPK bukan as-of** (`bmpk_repo.go:30-59`, `bmpk_service.go:47-61`): `asOf` hanya
   disalin, sedangkan eksposur/batas dibaca keadaan kini → kolom XV Form 05.00 dan XXXVII
   Form 06.00 pada periode lampau memakai posisi kini.

**Sudah diperbaiki 28 Sep 2026:**
- **Paginasi kredit tanpa pemecah seri** — `loan_repo.go` kini `ORDER BY created_at DESC,
  id DESC`; tanpa itu kredit dengan `created_at` kembar bisa terlewat/terulang antar halaman
  saat loop `repo_source.go` menjelajahi OFFSET.
- **Komentar `savings_account_repo.go` yang melebih-lebihkan jaminan** — filter `closed_at`
  ternyata mati karena `accounts.closed_at` tidak pernah ditulis aplikasi; pengaman
  sebenarnya adalah syarat saldo nol. Komentar kini menyatakan itu apa adanya.

**Catatan produksi (dicek 28 Sep 2026):** `lps_placements` dan `off_balance_items`
**kosong (0 baris)**, jadi risiko nomor 1–2 di atas masih latent — belum ada yang bisa
terlipat atau hilang. `bank_management` dan `bank_offices` juga masih 0 baris, sehingga
form peristiwa `00.09`/`00.10`/`00.12` akan berstatus "tidak ada peristiwa" dan kolom
Sandi Kantor seluruh form akan `-` sampai bank mengisi data kelembagaan/kantor lewat API
yang sudah ada. (Data lain: 10 kredit, 6 deposito, 93 rekening.)

**Sudah terdokumentasi sebagai keterbatasan (bukan cacat baru):** baris Form 06.00 untuk
kredit yang lunas setelah `periodEnd` hilang dari periode lampau (kolom posisi memang sudah
ditahan `-`), dan agregat pengurang PABL (`lps_placement.go:272`, `pabl_ckpn.go:197`) ikut
memakai `outstanding` keadaan kini.

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
- **Pengurang modal inti KPMM (AYDA >1 tahun, dll.) tidak punya kelas/kode sama sekali**
  di `coa_mapping.go` (kelas `PENGURANG` sengaja kosong dan dijaga test) — tidak ada kode
  pengurang yang bisa disambungkan ke register AYDA; menambahkannya = keputusan kebijakan
  modal, bukan celah laporan. `000115` menyediakan `acquisition_date` sebagai bahan bila
  keputusan itu diambil.
- Keputusan privasi: NIK/NPWP/identitas pihak lawan pada keluaran laporan.
- Konfirmasi OJK atas inkonsistensi Form 06.01 (butir 6 di §3).
