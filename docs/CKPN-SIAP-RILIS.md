# CKPN Siap Rilis — Daftar Periksa Sebelum Menyalakan

Dokumen ini adalah syarat yang harus dipenuhi **sebelum** `ckpn.enabled` diubah menjadi
`true` di produksi. Status sekarang: `ckpn.enabled = false`, `ckpn.shadow_mode.enabled =
true` (mode bayangan hanya menghitung dan melaporkan, tidak menjurnal). Jangan menyalakan
saklar sebelum seluruh butir di bawah selesai.

Rujukan teknis memakai `berkas:baris` pada repositori. Rujukan hukum memakai pasal/butir
dokumen OJK (teks di `/tmp/ojk/`, tidak ikut dirilis).

---

## (a) Parameter dan satuannya

| Kunci | Arti | Satuan | Bawaan | Bukti |
|---|---|---|---|---|
| `ckpn.enabled` | saklar utama; `false` = tidak ada perhitungan/jurnal | boolean | `false` | `packages/db-migrations/000042_ckpn.up.sql:62`; dibaca `apps/api/internal/service/ckpn_service.go:586` |
| `ckpn.pd_frac.gol_1..5` | PD per golongan kolektibilitas (1 Lancar … 5 Macet) | **FRAKSI 0..1**, bukan persen | kosong | `000042_ckpn.up.sql:64-72`; divalidasi `ckpn_service.go:609` |
| `ckpn.lgd_frac` | LGD (satu nilai *all account* bila data tidak mendukung pengelompokan) | **FRAKSI 0..1** | kosong | `000042_ckpn.up.sql:74`; divalidasi `ckpn_service.go:624` |
| `ckpn.aset_baik.max_dpd` | batas tunggakan hari kriteria aset baik | hari | `7` | `000042_ckpn.up.sql` (seed); dibaca `ckpn_service.go:595` |
| `ckpn.aset_baik.bentuk_ckpn` | saklar aset baik: `false` = dikecualikan (nilai awal); `true` = CKPN tahap-1 tetap dibentuk | boolean | `false` | migrasi `000086`; dibaca `ckpn_service.go` (kebijakan) |
| `ckpn.coa.expense` | COA beban CKPN (global) | kode COA | `50301` | `packages/db-migrations/000069_ckpn_coa_config.up.sql:33` |
| `ckpn.coa.reserve` | COA cadangan CKPN (global) | kode COA | `10950` | `000069_ckpn_coa_config.up.sql:35` |
| `ckpn.coa.expense.syariah` | COA beban CKPN unit syariah | kode COA | `15901` (diisi `000090` bila kosong) | `packages/db-migrations/000071_ckpn_coa_syariah.up.sql:24`; `000090_..._policy.up.sql` |
| `ckpn.coa.reserve.syariah` | COA cadangan CKPN unit syariah | kode COA | `11950` (diisi `000090` bila kosong) | `000071_ckpn_coa_syariah.up.sql:26`; `000090_..._policy.up.sql` |

Catatan satuan (kritis):

- Nilai di luar 0..1 **ditolak** dan dilaporkan, bukan dijatuhkan menjadi nol
  (`ckpn_service.go:609` dan `:624`; pesan di `domain/ckpn.go:208` dan `:215`).
- Migrasi `000067_ckpn_parameter_units.up.sql` **hanya memperjelas deskripsi kunci**;
  ia **tidak mengubah nilai** yang sudah ada. Instalasi yang masih menyimpan nilai
  persen (mis. `1, 5, 15, 35, 60` dan `45`) wajib **mengisi ulang** parameter sebagai
  fraksi sebelum menyalakan CKPN. Nilai tepat `1` ambigu (dibaca 100%), sehingga tidak
  dapat dideteksi otomatis: periksa manual `ckpn.pd_frac.gol_1`.
- PD/LGD adalah **kebijakan bank** dari data historis (periode observasi minimal 3
  tahun). Sistem tidak menebak nilainya; kredit yang parameter golongannya belum diisi
  gagal dengan `ErrCKPNParameterMissing` (`domain/ckpn.go:17`, `:206-216`).

## (b) Akun jurnal per buku

Jurnal pembentukan: **Db. Beban kerugian penurunan nilai; Kr. CKPN** — dijurnal lewat
`ckpn_service.go:548-553`. Jurnal pemulihan (CKPN turun): arahnya dibalik
(`ckpn_service.go:552`).

Urutan pemilihan akun per buku (`ckpn_service.go:664-672`):

1. Produk kredit punya pemetaan jurnal `CKPN_PROVISION`/`CKPN_REVERSAL` → pemetaan
   produk dipakai lebih dulu (`ckpn_service.go:506-523`).
2. Tanpa pemetaan produk: kredit buku **SYARIAH** memakai `ckpn.coa.*.syariah` bila
   **terisi**; bila kosong memakai kunci global; bila keduanya kosong fallback
   `15901`/`11950`. Kredit **KONVENSIONAL** memakai kunci global; fallback
   `50301`/`10950`.

| Buku | Beban (debit saat membentuk) | Cadangan (kredit saat membentuk) | Seed |
|---|---|---|---|
| KONVENSIONAL | `50301` | `10950` | `000042_ckpn.up.sql` (COA), `000069_ckpn_coa_config.up.sql` (konfigurasi) |
| SYARIAH (kunci syariah kosong) | `50301` (**konvensional**) | `10950` (**konvensional**) | `000071_ckpn_coa_syariah.up.sql` |
| SYARIAH (kunci syariah terisi) | `ckpn.coa.expense.syariah` | `ckpn.coa.reserve.syariah` | idem |

**Kewajiban instalasi SYARIAH:** isi `ckpn.coa.expense.syariah` dan
`ckpn.coa.reserve.syariah`. Selama kosong, jurnal CKPN pembiayaan syariah jatuh ke akun
konvensional dan dana UUS tercampur buku konvensional. Peringatan ini muncul untuk
cakupan **SYARIAH dan DUAL** (`installation_validation.go:36`, diperbaiki putaran ini).
Sejak migrasi `000090` kedua kunci **diisi bawaan `15901`/`11950` hanya bila masih
kosong**; peringatan hilang setelah kedua akun terisi, dan pemetaan yang sudah
disesuaikan bank tidak ditimpa.

## (c) Kebijakan yang harus diputuskan bank/DPS (jangan ditebak sistem)

1. **Perlakuan aset baik.** Aset yang memenuhi kriteria aset baik (tidak menunggak
   >7 hari dan tidak pernah direstrukturisasi — PA BPR 12.3.a.1.c) **boleh tidak
   dibentuk CKPN** (PA BPR 12.3.a.2.a). Pilihan kini menjadi saklar
   `ckpn.aset_baik.bentuk_ckpn`: nilai awal `false` = aset baik dikecualikan
   (perilaku sekarang; `domain/ckpn.go` `CalculateCKPN`), `true` = CKPN tahap-1 tetap
   dibentuk memakai PD golongan lancar dan `ckpn.lgd_frac`. Bank/auditor yang menuntut
   ECL tahap-1 dapat menyalakannya; pastikan `ckpn.pd_frac.gol_1` dan `ckpn.lgd_frac`
   terisi agar aset baik tidak gagal parameter. Basis pembanding `setara PPKA` (mode
   bayangan) selalu menilai aset baik tanpa memandang saklar ini.
2. **PD dan LGD** per golongan/kategori dari data historis bank (3 tahun); sistem tidak
   menghitungnya (PA BPR 12.6-12.7).
3. **Pengurang modal inti PPKA–CKPN.** PA BPR 1.1.6: bila PPKA > CKPN, selisihnya
   menjadi pengurang modal inti. Kode menjumlahkan selisih **per kredit**
   `Σ max(PPKA_i − CKPN_i, 0)` (`ckpn_service.go:227-229`). Alternatif tafsir adalah
   selisih **agregat** total PPKA − total CKPN. Sejak
   modul KPMM, pilihan ini menjadi setelan `kpmm.deduction_basis` (`per_kredit` nilai
   awal yang konservatif, atau `agregat`), dan basis yang dipakai tercatat pada bidang
   `DeductionBasis` laporan. Nilai awal `per_kredit` dipertahankan sampai laporan KPMM
   pertama bank disahkan OJK/akuntan; setelah itu bank dapat pindah ke `agregat` (yang
   dibaca regulasi) lewat konfigurasi. Lihat bagian (f).
4. **Akun jurnal pemulihan.** PA BPR 12.9 mencontohkan kredit ke "Pendapatan operasional
   – Pemulihan CKPN", sedangkan 12.5.b menyebut menjurnal balik beban. Kode mengikuti
   12.5.b (mengkredit akun beban, `ckpn_service.go:552`). Bila bank/DPS memilih akun
   pendapatan tersendiri, perlu kunci COA baru — belum dibangun.
5. **Perlakuan agunan.** Mesin CKPN **tidak** mengurangi nilai realisasi agunan dari
   EAD; agunan harus sudah tercermin di LGD (PA BPR 12.7). Pastikan LGD bank memang
   memakai *expected recoveries*/*collateral shortfall* (PA BPR 12.7.1/12.7.2).
6. **Hapus buku.** Akun cadangan yang dilepas sudah mengikuti saklar: saat `ckpn.enabled`
   menyala, kaki debit cadangan diarahkan ke akun CKPN per buku (`10950`/`11950`,
   `loan_service.go:1376-1419`); saat mati, pemetaan produk (PPAP `10900`/`11900`) dipakai
   apa adanya. Namun **ukurannya** masih memakai `required_ppap` (`loan_service.go:1335-1341`)
   dan dilepas sebesar kaki pokok. Bila saldo CKPN yang dibukukan lebih kecil dari pokok,
   pelepasan penuh berpotensi membuat saldo CKPN negatif; putuskan bersama akuntan apakah
   selisihnya diakui sebagai beban dan apakah syarat "cadangan 100%" diukur dari CKPN.
7. **Perlakuan CKPN individual** (discounted cash flow / nilai realisasi agunan)
   memerlukan estimasi arus kas dan suku bunga efektif awal per debitur — belum
   dibangun (`domain/ckpn.go:146-152`).

## (d) Kewajiban CKPN sejak 1 Januari 2025

- BPR: **POJK No. 1 Tahun 2024** (Kualitas Aset BPR). BPRS: **POJK No. 24 Tahun 2024**
  (Kualitas Aset BPRS). Sejak **1 Januari 2025** BPRS memakai **CKPN** sesuai standar
  akuntansi keuangan menggantikan PPKA (POJK 24/2024; FAQ POJK 1/2024).
- Panduan akuntansinya: **SEOJK No. 21/SEOJK.03/2024** (Panduan Akuntansi Perbankan bagi
  BPR, "PA BPR"), berlaku 1 Januari 2025. Bab XII: alur pembentukan (12.3), PD (12.6),
  LGD (12.7), rumus `CKPN = PD × LGD × EAD` (12.8), ilustrasi jurnal (12.9).
- PPKA tetap dihitung menurut POJK kualitas aset; selisih PPKA > CKPN menjadi pengurang
  modal inti (PA BPR 1.1.6). Pada laporan berkala, kolom CKPN diisi PPKA sampai
  31 Desember 2024 dan CKPN sejak 1 Januari 2025 (SEOJK No. 16/SEOJK.03/2024, butir P).
- Hapus buku: hanya atas aset **macet dengan cadangan 100%**, tidak boleh sebagian,
  wajib upaya penagihan terdokumentasi (POJK 1/2024 Pasal 42-43; POJK 24/2024 Pasal
  50-51).

## (e) Langkah menyalakan dan cara memverifikasi

1. Pastikan **instalasi** sudah terpasang migrasi `000042`, `000069`, `000071`
   (COA + konfigurasi CKPN). COA `50301/10950/15901/11950` harus ada.
2. Isi parameter sebagai **fraksi**: `ckpn.pd_frac.gol_1..5`, `ckpn.lgd_frac`. Untuk
   instalasi SYARIAH/DUAL, isi `ckpn.coa.expense.syariah` dan
   `ckpn.coa.reserve.syariah`.
3. Jalankan mode bayangan terlebih dahulu dan periksa: `ParameterGaps` kosong, `Failed`
   sesuai (kredit aset baik tidak butuh PD), dan `ModalIntiDeduction` wajar. Mode
   bayangan tidak menjurnal dan tidak menulis `required_ckpn`
   (`ckpn_service.go:135-144`; uji `ckpn_shadow_integration_test.go`).
4. Langkah tutup hari (`ckpn_comparison`) kini **membentuk dan menjurnal** CKPN saat
   `ckpn.enabled` menyala: memanggil `Run` (`batch_process_service.go:492-496`), lalu
   melaporkan hasilnya. Saat saklar mati, ia tetap memanggil `Compare` baca-saja persis
   seperti sebelumnya (mode bayangan menghitung tanpa menjurnal). Karena pembentukan
   kini otomatis di EOD, jalur manual `POST /api/v1/ckpn/run` hanya diperlukan untuk
   menjalankan ulang di luar tutup hari.
5. Ubah `ckpn.enabled = true`. Matikan mode bayangan (`ckpn.shadow_mode.enabled =
   false`) agar tidak ada dua angka yang membingungkan.
6. Verifikasi setelah menyala:
   - `required_ckpn` tiap kredit terisi (`ckpn_repo.go:124`).
   - Jurnal pembentukan ada di `journal_entries` dengan kunci
     `CKPN-<no kredit>-<tanggal>-<selisih>` (`ckpn_service.go:486`), akun sesuai buku.
   - Jalankan ulang pada tanggal bisnis yang sama: jumlah jurnal **tidak bertambah**
     (idempotensi; kunci unik `packages/db-migrations/000001_init_cbs_schema.up.sql:81`
     dan `posting_service.go:71`).
   - Rekonsiliasi saldo GL COA CKPN dengan jumlah `required_ckpn`.

## (e.1) Onboarding: instalasi BARU vs instalasi yang SUDAH BERJALAN

Keputusan panel: instalasi **baru** diharapkan CKPN **menyala**; instalasi yang **sudah
berjalan tetap mati** sampai daftar periksa ini tuntas. Karena semua instalasi
menjalankan migrasi yang sama, `ckpn.enabled` **tidak pernah dibalik otomatis** oleh
migrasi: menyalakannya pada bank yang sedang berjalan akan membentuk/menjurnal CKPN
secara mundur. Sebagai gantinya sistem memberi mekanisme yang jujur dan aman:

1. **Peringatan saat start.** Bila `ckpn.enabled = false` padahal instalasi sudah
   memiliki jurnal dan/atau tanggal bisnis, server menulis peringatan tegas
   (`CKPNReadinessWarnings`, dipasang di `apps/api/cmd/server/main.go`): bank sudah
   beroperasi tanpa membentuk CKPN, padahal CKPN SAK EP wajib sejak 1 Januari 2025.
   Peringatan ini **tidak memblokir** dan **tidak mengubah** setelan apa pun.
2. **Langkah onboarding instalasi baru** (lakukan sebelum transaksi pertama, atau
   setelah butir (a)–(d) tuntas bagi bank yang sudah berjalan):
   1. Pastikan COA `50301/10950/15901/11950` ada (migrasi `000042`).
   2. Isi parameter sebagai **fraksi**: `ckpn.pd_frac.gol_1..5`, `ckpn.lgd_frac`.
      Instalasi SYARIAH/DUAL: kunci `ckpn.coa.expense.syariah`/`ckpn.coa.reserve.syariah`
      sudah diisi bawaan `15901`/`11950` oleh migrasi `000090` bila masih kosong;
      periksa dan sesuaikan bila bank memakai akun lain.
   3. Jalankan mode bayangan dan periksa hasilnya (ParameterGaps kosong, Failed wajar).
   4. Setel `ckpn.enabled = true`, lalu matikan mode bayangan
      (`ckpn.shadow_mode.enabled = false`) agar tidak ada dua angka.
   5. Verifikasi ulang lewat butir (e) di atas.
3. **Mode bayangan default TRUE untuk instalasi baru** (migrasi `000066`): bayangan
   menyala karena ia alat verifikasi yang **tidak menjurnal**. Instalasi yang sudah
   menjalankan `000066` **tidak berubah** — nilai yang sudah disesuaikan bank (produksi
   saat ini `true`) tetap utuh, karena ledger `schema_migrations` melewati berkas yang
   sudah tercatat.
4. **Akun CKPN syariah** (migrasi `000090`) diisi bawaan hanya bila masih kosong, dan
   **umur taksasi agunan** kini setelan eksplisit `collateral.appraisal.validity_months
   = 12` (keputusan panel sebagai praktik industri, bukan aturan tertulis). Taksasi yang
   lebih tua dari umur itu dianggap **data agunan tidak lengkap** sehingga agunan memakai
   bobot risiko 100% (tidak mengurangi eksposur) sampai dinilai ulang.

## (g) Ratifikasi parameter — prosedur langkah demi langkah (KEPUTUSAN panel)

Panel **tidak** meratifikasi atas nama bank: ratifikasi adalah tindakan manusia yang
bertanda tangan (Direksi + akuntan; DPS untuk BPRS). Yang dilakukan sistem adalah
**menolak** perubahan `ckpn.parameters.status` ke `FINAL` selama bukti belum lengkap,
dengan pesan yang menyebut apa yang kurang. Penegakannya ada di database (trigger
`ckpn_ratification_guard_row`, migrasi `000095`), sehingga jalur SQL langsung pun
ditolak, bukan hanya API. Status bukti dapat diperiksa tanpa EOD lewat
`GET /api/v1/ckpn/status` (`ratification_ready`, `ratification_missing`).

**Format bukti (paling sederhana yang tetap sah — kunci konfigurasi, tanpa tabel baru):**

| Kunci | Isi | Divalidasi mesin |
|---|---|---|
| `ckpn.ratification.ba_number` | nomor berita acara ratifikasi | wajib tidak kosong |
| `ckpn.ratification.ba_date` | tanggal berita acara `YYYY-MM-DD` | format benar & tidak di masa depan |
| `ckpn.ratification.approved_by` | nama dan jabatan pengesah | wajib tidak kosong |
| `ckpn.ratification.pd_lgd_basis` | dasar PD (PA BPR 12.6) & LGD (12.7) dari data historis bank | wajib tidak kosong |
| `ckpn.ratification.pd_lgd_from_bank` | penanda PD/LGD dari data bank, bukan turunan tarif PPKA | wajib `true` |

Kewenangan: mengubah bukti/status memakai izin **`system:config`** (sama dengan setelan
instalasi lain); setiap perubahan tercatat pada `system_config.updated_by`/`updated_at`.
Yang **hanya boleh diisi bank** dan tidak pernah dikarang sistem: nomor/tanggal berita
acara, nama/jabatan pengesah, dan dasar perhitungan PD/LGD.

**Langkah bank:**

1. Hitung PD menurut PA BPR **12.6** (mis. net flow/migration analysis) dan LGD menurut
   **12.7** (expected recoveries/collateral shortfall) dari data historis bank sendiri
   (observasi minimal 3 tahun) atau pakai excel parameter resmi OJK. Angka turunan tarif
   PPKA (yang sekarang berstatus SEMENTARA) **bukan** pengganti.
2. Susun berita acara ratifikasi dan mintakan tanda tangan Direksi + akuntan (DPS untuk
   BPRS). Lampirkan perhitungan PD/LGD.
3. Isi kelima kunci bukti (contoh lewat SQL; menu konfigurasi memakai izin yang sama):

   ```sql
   UPDATE system_config SET value='BA/001/DIR-2026' WHERE key='ckpn.ratification.ba_number';
   UPDATE system_config SET value='2026-09-30'      WHERE key='ckpn.ratification.ba_date';
   UPDATE system_config SET value='Direktur A; Akuntan B' WHERE key='ckpn.ratification.approved_by';
   UPDATE system_config SET value='12.6 net flow 3 tahun; 12.7 LGD agunan; lampiran BA/001'
     WHERE key='ckpn.ratification.pd_lgd_basis';
   UPDATE system_config SET value='true' WHERE key='ckpn.ratification.pd_lgd_from_bank';
   ```

4. Periksa `GET /api/v1/ckpn/status`: `ratification_missing` kosong dan
   `ratification_ready=true`.
5. Setel `ckpn.parameters.status=FINAL`. Bila ada bukti yang kurang, trigger menolak
   dan menyebut kuncinya; lengkapi dulu, jangan mengakalinya.
6. Setelah FINAL: lantai PPKA menjadi pilihan bank lewat `ckpn.floor.ppka_enabled`
   (bawaan `true`; direkomendasikan tetap `true`), dan blokir ekspor OJK parameter
   sementara hilang. Penonaktifan lantai wajib persetujuan Direksi + akuntan dan tercatat.

Siklus hidup: maksimum **12 bulan** sejak `ckpn.parameters.temporary_since`; lewat batas,
sistem memberi peringatan tingkat tinggi. Perpanjangan hanya lewat berita acara
bertanggal (isi bukti baru), maksimal 12 bulan lagi.

## (h) KEPUTUSAN: kapan `ckpn.enabled` dinyalakan pada instalasi ini (butir 2)

**Keputusan panel: instalasi ini tetap `ckpn.enabled=false`** sampai butir (g) tuntas
(status `FINAL` + bukti ratifikasi lengkap) **dan** PD/LGD sah sebagai fraksi 0..1.
Selama itu mode bayangan (`ckpn.shadow_mode.enabled=true`, bawaan `000066`) dijalankan
untuk verifikasi; ia **tidak** menjurnal dan **tidak** menulis `required_ckpn`. Alasan:
menyalakan CKPN sebelum ratifikasi mengalirkan angka PD/LGD sementara ke pembukuan dan
otomatis menutup ekspor OJK — memperbesar masalah, bukan menyelesaikannya. Untuk
**instalasi BARU** (belum punya jurnal/tanggal bisnis), keputusan panel sebelumnya tetap
berlaku: CKPN **diharapkan menyala** saat onboarding sebelum transaksi pertama, karena
tidak ada risiko pembentukan/jurnal mundur. Syarat PD/LGD (fraksi 0..1) dan ratifikasi
tetap wajib dipenuhi lebih dulu pada instalasi baru; perbedaannya hanya waktu (sebelum
transaksi, bukan menunggu). Untuk **instalasi ini** (sudah beroperasi) risiko jurnal
mundur nyata, sehingga keputusannya **tetap mati** sampai butir (g) tuntas.

Sistem memberi tahu apa yang menahan lewat **satu daftar yang dapat diperiksa mesin**
pada `GET /api/v1/ckpn/status` (`enablement_gaps`) dan peringatan start/EOD:

| Penahan | Sumber pemeriksaan |
|---|---|
| Parameter belum `FINAL` (bukti ratifikasi kurang) | `ckpn.parameters.status`, kunci `ckpn.ratification.*` |
| PD/LGD belum diisi atau bukan fraksi 0..1 | `ckpn.pd_frac.gol_1..5`, `ckpn.lgd_frac` |
| Akun CKPN syariah belum dipetakan (instalasi SYARIAH/DUAL) | `ckpn.coa.expense.syariah`, `ckpn.coa.reserve.syariah` |
| Instalasi sudah beroperasi tapi CKPN mati | peringatan start `CKPNReadinessWarnings` |

Setelah seluruh penahan kosong: setel `ckpn.enabled=true` dan matikan mode bayangan
(`ckpn.shadow_mode.enabled=false`) agar tidak ada dua angka. Urutan verifikasi lanjutan
ada di (e). Syarat pengawasan: bank melaporkan tanggal penyalakan, dasar PD/LGD, dan
berita acara ratifikasi kepada pengawas; sebelum ratifikasi, kolom CKPN laporan OJK
tetap memakai PPKA dengan pengungkapan selisih.

## (f) KPMM/ATMR — verifikasi bobot risiko & penyambungan ke Form 00.08

Modul KPMM (`apps/api/internal/ojkreport/kpmm.go`,
`internal/service/kpmm_service.go`, `internal/domain/kpmm.go`) menghitung modal
inti (ekuitas + laba/rugi tahun berjalan dikurangi selisih PPKA–CKPN), ATMR aset
neraca berbobot risiko, dan rasio KPMM/modal inti; disajikan baca-saja lewat
`GET /api/v1/reports/kpmm?period=YYYY-MM` (izin `reports:financial:read`, hanya
peran lintas cabang). Parameter ada di konfigurasi (migrasi `000082`), bukan
ditanam di kode.

### Verifikasi bobot risiko (putaran ini)

Dibandingkan terhadap **SEOJK No. 2/SEOJK.03/2025 Lampiran II** ("Perhitungan
ATMR"), disimpan di `/tmp/ojk/seojk2_2025.txt` (tidak ikut dirilis). Ambang modal
dibandingkan terhadap **POJK No. 5/POJK.03/2015**, yang disempurnakan **POJK
No. 7 Tahun 2026** (berlaku 30 Juni 2026).

| Kunci / komponen | Nilai | Hasil | Rujukan |
|---|---|---|---|
| `kpmm.min_frac` | 0,12 | benar | POJK 5/2015 Pasal 2 (KPMM ≥12% ATMR) |
| `kpmm.modal_inti_min_frac` | 0,08 | benar | POJK 5/2015 Pasal 4 (modal inti ≥8% ATMR) |
| `kpmm.modal_inti_min_amount` | 6.000.000.000 | benar nilainya; **sitasi diperbaiki** | POJK 5/2015 **Pasal 13**, bukan Pasal 14 (dikoreksi `000084`) |
| `kpmm.modal_pelengkap_max_frac` | 1,00 | benar | POJK 5/2015 Pasal 3 ayat (2) (modal pelengkap ≤100% modal inti) |
| `kpmm.modal_pelengkap_instrumen_max_frac` | 0,50 | benar | POJK 5/2015 Pasal 10 ayat (2) (komponen pelengkap ber-instrumen ≤50% modal inti). Baru di migrasi `000086`; diterapkan `ojkreport.BatasModalPelengkap` |
| `kpmm.ppka_umum_rwa_max_frac` | 0,0125 | benar | POJK 5/2015 Pasal 10 ayat (1) huruf c; SEOJK 2/2025 II.1.c.3 |
| `kpmm.deduction_basis` | `per_kredit` | **tidak dapat diverifikasi** (tafsir) | PA BPR 1.1.6 tidak menegaskan tingkat perhitungan; tetap setelan bank/OJK |
| `kpmm.rwa_frac.kas` | 0,00 | benar | Lampiran II no. 1 (Kas 0%) |
| `kpmm.rwa_frac.antar_bank` | 0,20 | benar (posisi performing) | Lampiran II no. 9 (penempatan bank lain 20%); penempatan macet seharusnya 100% (no. 22) dan belum dipisah |
| `kpmm.rwa_frac.kredit` | 1,00 | **konservatif** (granular tidak dapat diverifikasi) | Lampiran II no. 21/22 (100%); bobot lebih rendah per agunan (0/15/20/30/50/70%, no. 5/8/10/12/18/19) belum diterapkan |
| `kpmm.rwa_frac.ayda` | 1,00 | **konservatif** | Lampiran II no. 24 (AYDA <1 tahun 100%); AYDA >1 tahun seharusnya 0% (no. 6) + pengurang modal, belum dipisah |
| `kpmm.rwa_frac.aset_tetap` | 1,00 | benar | Lampiran II no. 23 (aset tetap/inventaris/aset tak berwujud 100%) |
| `kpmm.rwa_frac.antar_kantor` | 1,00 | **konservatif** | tidak ada pos khusus; sebagai "aset lain" no. 26 = 100% |
| `kpmm.rwa_frac.lainnya` | 1,00 | benar | Lampiran II no. 26 (aset lain 100%) |

**Tidak ada bobot yang lebih rendah dari ketentuan**, sehingga tidak ada rasio
yang membesar secara keliru; nilai yang belum granular sudah konservatif (ATMR
lebih tinggi, rasio lebih rendah). Karena itu tidak ada **nilai** kunci yang
diubah — hanya perbaikan sitasi Pasal 13 lewat migrasi
`packages/db-migrations/000084_kpmm_config_verify.up.sql`.

### Penyambungan ke pelaporan OJK

Form 00.08 sandi **0101 (KPMM)** kini **terisi** dari modul KPMM yang sama:
kontrak `ojkreport.KPMMSource` (diimplementasikan `RepoSource.KPMMModalATMR`)
mengalir lewat builder ke `RasioKeuangan`; buku dan aktor diteruskan apa adanya
sehingga cakupan Form 00.08 sama dengan `GET /reports/kpmm`. Bila CKPN belum
dihitung atau ATMR belum lengkap, baris ditulis "-" beserta alasan, bukan 0. Ini
menggantikan catatan lama "belum disalurkan ke Form 00.08".

Yang **belum** tersambung/tersedia:
- **Modal pelengkap** (instrumen dengan persetujuan OJK, surplus revaluasi aset
  tetap, PPKA umum) belum dipisah dari data, sehingga total modal dan rasio adalah
  batas bawah (konservatif). Ketiga komponen kini disajikan eksplisit sebagai
  `ModalPelengkapInstrumen`/`SurplusRevaluasi`/`PPKAUmum` dengan `Tersedia=false`
  beserta alasannya (bukan nol). Sub-batas Pasal 10 ayat (2) dan Pasal 3 ayat (2)
  sudah ditegakkan `ojkreport.BatasModalPelengkap` dan hanya berlaku pada komponen
  yang terisi.
- **PPKA umum** (minimum 0,5% aset produktif lancar, POJK No. 1/2024 Pasal 19 ayat
  (2)) **ditunda**: pemetaan bagan akun masih `DRAF-BELUM-TERVERIFIKASI` dan kualitas
  aset produktif per pos belum tersimpan, sehingga tidak dipaksakan. Yang dibutuhkan:
  pemetaan COA aset produktif yang terverifikasi + kualitas per pos (lancar).
- **Klasifikasi kelas modal per COA** (`internal/ojkreport/coa_mapping.go`, atribut
  `ModalClass`): modal inti utama (modal disetor, laba, cadangan umum) sudah ditandai
  `INTI_UTAMA`; `INTI_TAMBAHAN`, `PELENGKAP`, dan `PENGURANG` dibiarkan kosong bila
  belum pasti (AYDA >1 tahun, pajak tangguhan, goodwill, disagio, surplus revaluasi,
  PPKA umum). Kode tanpa kelas tidak dijumlahkan sebagai modal, sehingga tidak ada
  modal fiktif; rinciannya disajikan pada `ModalKelasCOA`.
- **Pengurang modal inti lain** (AYDA/properti terbengkalai >1 tahun, pajak
  tangguhan, goodwill, disagio) belum dapat dihitung dari data agregat.
- **Bobot risiko kredit per agunan** serta AYDA/antar-bank granular (tabel di atas).
- **Tafsir pengurang PPKA–CKPN** per-kredit vs agregat masih setelan
  `kpmm.deduction_basis` (nilai awal `per_kredit`). Putuskan bersama OJK/akuntan
  sebelum dipakai untuk laporan resmi.
- **Pemetaan COA → pos OJK** yang dipakai ATMR masih
  `DRAF-BELUM-TERVERIFIKASI` (`coa_mapping.go`); COA bersaldo yang belum
  terpetakan menandai ATMR belum lengkap dan rasio tidak disajikan.
- **Invarian `lps_placements`**: satu baris per penempatan, `as_of` adalah keadaan
  terkini (diperbarui bank saat barisnya diubah); histori asesmen CKPN tersimpan di
  `pabl_ckpn_assessments`, bukan dengan menduplikasi baris. Aplikasi tidak punya jalur
  `INSERT` ke tabel ini, jadi tidak ada de-duplikasi yang perlu dilakukan.
- **Cakupan buku**: KPMM mengikuti pola laporan keuangan — `book` kosong =
  konsolidasi seluruh bank, `book` terisi = satu lini; tidak ada aturan cakupan
  baru. Untuk pelaporan OJK gunakan konsolidasi (tanpa `book`).
- **CKPN individual** (DCF/nilai realisasi agunan): tahap rancangan T1-T4 SUDAH dibangun
  dan teruji (DCF, NRV agunan, aturan MAX, input signifikansi), bawaan mati lewat
  `ckpn.individual.enabled`; tahap T5 belum tuntas. Rinciannya di
  `docs/CKPN-INDIVIDUAL-RANCANGAN.md`.
- **Perhitungan PD/LGD dari data historis** belum ada; bank mengisinya manual.
  **Pemilik keputusan: bank** (sistem tidak boleh menebak dari tarif PPKA untuk laporan resmi).
- **Pilihan kebijakan "tetap membentuk CKPN atas aset baik"** tersedia lewat saklar
  `ckpn.aset_baik.bentuk_ckpn` (bawaan `false`, `ckpn_service.go:395`); untuk penempatan
  pada bank lain lewat `ckpn.pabl.aset_baik_bentuk_ckpn`.
- **Akun pemulihan CKPN tersendiri** (bila bank memilih pendapatan, bukan balik beban)
  belum ada. **Pemilik keputusan: bank/DPS** (12.5.b vs 12.9; kode kini mengikuti 12.5.b).
- **Pelepasan CKPN saat hapus buku**: akunnya kini mengikuti saklar (CKPN `10950`/`11950`
  saat aktif; PPAP saat mati, `loan_service.go:1376-1419`), tetapi target `required_ckpn`
  baru dinolkan pada run CKPN berikutnya lewat `required_ckpn <> 0` (`ckpn_repo.go:56-58`).
  Ada jeda satu run; urutan pelepasan akun vs target belum menjadi kebijakan bank.
- **Kolom "CKPN aset baik/kurang baik/tidak baik"** (Form 06.00 XLIV-XLVI; Form 05.00
  XVII-XIX): OJK mendefinisikannya sebagai **stage 1/2/3** (Penjelasan Umum Kolom Bab II
  huruf P), dan penggolongan itu ditujukan bagi BPR yang **melakukan penawaran umum efek
  (SAK Indonesia)**. BPR dengan **SAK EP** menggolongkan CKPN atas **individual/kolektif**
  (kolom Jenis CKPN; sudah hidup). Sistem ini mengikuti SAK EP dan **tidak menyimpan status
  SICR/default**, jadi `required_ckpn` tidak dapat dipecah ke stage tanpa pemodelan baru
  (`ojkreport/form06.go:154-156`; Form 05 XVII-XIX kini memakai proxy kualitas:
  Baik=Lancar, Kurang Baik=Kurang Lancar, Tidak Baik=Macet).
  **Pemilik keputusan: pemilik sistem + bank/akuntan.** Dua jalan: (a) biarkan kolom stage
  belum tersedia karena tak berlaku untuk SAK EP, atau (b) ratifikasi proxy kualitas sebagai
  kebijakan setelah memutuskan celah DPK/Diragukan. Jangan paksakan angka sebelum diputuskan.

## (i) Mengisi parameter lewat API (tanpa SQL) — `ckpn-activation`

Kunci yang sebelumnya hanya bisa diisi SQL kini punya pintu berizin + teraudit:
`GET /api/v1/system/ckpn-activation` (izin `system:config:read`) dan
`PUT /api/v1/system/ckpn-activation` (izin `system:config`). Hanya bidang yang
dikirim yang diubah; setiap perubahan tercatat di `audit_logs`
(`UPDATE_CKPN_ACTIVATION`, nilai sebelum -> sesudah per kunci).

Bidang yang diterima (semua opsional): `pd_frac_gol_1..5`, `lgd_frac`,
`coa_expense_syariah`, `coa_reserve_syariah`, `parameters_status`
(SEMENTARA/FINAL), `parameters_temporary_since`, `ratification_ba_number`,
`ratification_ba_date`, `ratification_approved_by`, `ratification_pd_lgd_basis`,
`ratification_pd_lgd_from_bank`, `shadow_mode_enabled`, `ckpn_enabled`.

Penolakan (HTTP 422, pesan menyebut kunci):
- fraksi di luar 0..1 atau bukan angka (satuan FRAKSI, bukan persen);
- `parameters_status` selain SEMENTARA/FINAL;
- tanggal bukan `YYYY-MM-DD` atau di masa depan;
- `parameters_status=FINAL` tanpa bukti ratifikasi lengkap;
- `ckpn_enabled=true` selama masih ada penahan (parameter belum FINAL, PD/LGD
  belum sah, akun syariah belum dipetakan). Endpoint TIDAK pernah menyalakan
  sendiri; menyalakan adalah keputusan manusia.

Urutan aman untuk mengisi dalam SATU panggilan (atau bertahap): isi PD/LGD +
akun syariah + bukti ratifikasi + setel FINAL, dan baru setelah `enablement_gaps`
kosong setel `ckpn_enabled=true`. Penulisan `ckpn.parameters.status` selalu
dilakukan terakhir dalam transaksi karena trigger `000095` membaca bukti dari
tabel saat menulis FINAL.

Contoh (sesuaikan host dan token):

    curl -sS -X PUT https://<host>/api/v1/system/ckpn-activation \
      -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
      -d '{"pd_frac_gol_1":"0.0100","pd_frac_gol_2":"0.0500","pd_frac_gol_3":"0.1000",
           "pd_frac_gol_4":"0.5000","pd_frac_gol_5":"1.0000","lgd_frac":"0.4500",
           "coa_expense_syariah":"15901","coa_reserve_syariah":"11950",
           "ratification_ba_number":"BA-07/IX/2026","ratification_ba_date":"2026-08-01",
           "ratification_approved_by":"Direktur Utama + Akuntan","ratification_pd_lgd_basis":"PA BPR 12.6/12.7 data historis 2024","ratification_pd_lgd_from_bank":true,
           "parameters_status":"FINAL"}'

    # Setelah enablement_gaps kosong:
    curl -sS -X PUT https://<host>/api/v1/system/ckpn-activation \
      -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
      -d '{"ckpn_enabled":true}'

Halaman web `Pengaturan -> Aktivasi CKPN` (izin `system:config`) menyajikan form yang
sama. Endpoint di atas tetap berguna untuk otomasi; ia sudah terdaftar di
`docs/openapi/openapi.yaml` dan dijaga uji `openapi_guard_test.go`.
