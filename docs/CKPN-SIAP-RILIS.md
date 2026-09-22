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
| `ckpn.coa.expense` | COA beban CKPN (global) | kode COA | `50301` | `packages/db-migrations/000069_ckpn_coa_config.up.sql:33` |
| `ckpn.coa.reserve` | COA cadangan CKPN (global) | kode COA | `10950` | `000069_ckpn_coa_config.up.sql:35` |
| `ckpn.coa.expense.syariah` | COA beban CKPN unit syariah | kode COA | **kosong** | `packages/db-migrations/000071_ckpn_coa_syariah.up.sql:24` |
| `ckpn.coa.reserve.syariah` | COA cadangan CKPN unit syariah | kode COA | **kosong** | `000071_ckpn_coa_syariah.up.sql:26` |

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

## (c) Kebijakan yang harus diputuskan bank/DPS (jangan ditebak sistem)

1. **Perlakuan aset baik.** Aset yang memenuhi kriteria aset baik (tidak menunggak
   >7 hari dan tidak pernah direstrukturisasi — PA BPR 12.3.a.1.c) **boleh tidak
   dibentuk CKPN** (PA BPR 12.3.a.2.a). Dasar hukumnya adalah pilihan kebijakan
   akuntansi, bukan kewajiban. Mesin saat ini **selalu** mengecualikan aset baik pada
   jalur resmi (`domain/ckpn.go:193`); cara "tetap membentuk CKPN atas aset baik" belum
   dapat dijadikan kebijakan resmi (hanya ada sebagai basis pembanding `setara PPKA`
   pada mode bayangan). Putuskan opsi mana yang dipakai bank, lalu sesuaikan.
2. **PD dan LGD** per golongan/kategori dari data historis bank (3 tahun); sistem tidak
   menghitungnya (PA BPR 12.6-12.7).
3. **Pengurang modal inti PPKA–CKPN.** PA BPR 1.1.6: bila PPKA > CKPN, selisihnya
   menjadi pengurang modal inti. Kode menjumlahkan selisih **per kredit**
   `Σ max(PPKA_i − CKPN_i, 0)` (`ckpn_service.go:227-229`). Alternatif tafsir adalah
   selisih **agregat** total PPKA − total CKPN. Putuskan bersama OJK/akuntan sebelum
   angkanya dipakai untuk KPMM; jangan menganggap pilihan yang ada sudah final.
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

## (f) Apa yang belum dibangun

- **KPMM/ATMR belum dihitung**; rasio KPMM ditandai TIDAK TERSEDIA
  (`ojkreport/ratios.go:74-79`). Akibatnya pengurang modal inti PPKA–CKPN baru
  dilaporkan sebagai angka pada ringkasan EOD (`batch_process_service.go:508`), belum
  diterapkan pada perhitungan modal. Usulan urutan: (1) putuskan per-kredit vs agregat;
  (2) susun komponen modal & ATMR sesuai POJK KPMM BPR; (3) baru terapkan pengurang.
  **Jangan membangun modul KPMM sebelum keputusan bank.**
- **CKPN individual** (DCF/nilai realisasi agunan) belum ada (`domain/ckpn.go:146-152`).
- **Perhitungan PD/LGD dari data historis** belum ada; bank mengisinya manual.
- **Pilihan kebijakan "tetap membentuk CKPN atas aset baik"** belum tersedia di jalur
  resmi.
- **Akun pemulihan CKPN tersendiri** (bila bank memilih pendapatan, bukan balik beban)
  belum ada.
- **Pelepasan CKPN saat hapus buku**: akunnya kini mengikuti saklar (CKPN `10950`/`11950`
  saat aktif; PPAP saat mati, `loan_service.go:1376-1419`), tetapi target `required_ckpn`
  baru dinolkan pada run CKPN berikutnya lewat `required_ckpn <> 0` (`ckpn_repo.go:56-58`).
  Ada jeda satu run; urutan pelepasan akun vs target belum menjadi kebijakan bank.
- **Pemisahan CKPN per golongan kualitas (stage 1/2/3)** untuk pelaporan belum
  disimpan; `required_ckpn` hanya total per kredit (`ojkreport/form06.go:152-156`).
