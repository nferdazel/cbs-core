# Onboarding Bank — dari instalasi ke siap lapor OJK

> Dokumen ini adalah **peta jalan**, bukan sumber angka. Ia menyebut **apa** yang harus
> diisi bank dan **ke mana** mencarinya; nilai/nama/keputusan tetap di dokumen kanonik
> masing-masing (`INDEX.md`). Tidak ada angka yang boleh diarang di sini.

Tujuan: setelah mengikuti daftar ini, bank tahu persis apa yang masih kosong dan siapa
yang berwenang mengisinya. Sistem dirancang **gagal-aman**: selama sesuatu belum diisi,
laporan yang bergantung padanya **menolak** atau menandai "belum tersedia" — bukan
menampilkan angka tebakan.

---

## A. Prasyarat teknis (sudah berjalan)

1. Deploy & migrasi: ikuti `DEPLOY.md` hingga `migrate.sh` bersih.
2. Verifikasi pasca-deploy: `DEPLOY.md` §4 (kesehatan API, jumlah `schema_migrations`).

## B. Identitas & kebijakan bank

| Yang diisi | Di mana | Dokumen kanonik |
|---|---|---|
| Nama/alamat/telepon/NPWP bank (Form 00.00) | profil bank | `KEPUTUSAN-OJK.md` |
| Parameter operasional (tanggal bisnis, sandi bank APOLO) | konfigurasi sistem | `KEPUTUSAN.md` |

## C. Parameter CKPN / PD / LGD (WAJIB sebelum CKPN menyala)

Ini **tidak boleh ditebak sistem**. Urutannya:

1. Sediakan angka PD per golongan & LGD sebagai **fraksi** (`ckpn.pd_frac.gol_1..5`,
   `ckpn.lgd_frac`) — dasar rumus di `CKPN-SIAP-RILIS.md` §(a), angka kanonik di
   `KEPUTUSAN-PANEL-RISIKO-CKPN.md §1.2`.
2. Jalankan **mode bayangan**, periksa `ParameterGaps` kosong.
3. **Ratifikasi** Direksi + akuntan (DPS untuk BPRS): isi bukti berita acara
   (`ckpn.ratification.*`). Tanpa bukti lengkap, perubahan ke status `FINAL` **ditolak**
   (ditegakkan trigger DB, bukan hanya API).
   > **Terbukti di produksi (1 Okt 2026):** percobaan `UPDATE system_config SET
   > value='FINAL' WHERE key='ckpn.parameters.status'` tanpa bukti ditolak oleh trigger
   > `ckpn_ratification_guard` dengan pesan yang menyebut kelima bukti yang kurang,
   > dan status tetap `SEMENTARA`. Sistem tidak meratifikasi atas nama bank.
4. Setel `ckpn.parameters.status=FINAL`, lalu nyalakan `ckpn.enabled`.
   Prosedur lengkap & verifikasi: `CKPN-SIAP-RILIS.md` §(e), §(g).

> Selama `SEMENTARA`, ekspor OJK (`/reports/ojk/monthly`, `/bmpk`, `/perbedaan-kualitas`)
> **ditolak** dengan alasan & langkah perbaikan. Laporan tetap memakai PPKA.

## D. Pemetaan COA → pos laporan OJK

1. Buka `/reports/ojk/mapping` (izin `reports:export`): daftar kode COA yang belum
   terpetakan (`unmapped_coa`) & baris keputusan.
2. Untuk tiap baris, putuskan lewat `/reports/ojk/mapping/decision` (izin penulisan
   terkait). Catatan wajib untuk keputusan "dicatat".
3. Dokumen kanonik konversi: `KEPUTUSAN.md` (bagian konversi COA laporan).

## E. Data register OJK

Semua register (00.01, 00.05, 00.06, 00.11, 00.16, 15.00, 18.00, dst.) tersedia lewat
endpoint register masing-masing (izin `system:config` untuk tulis). Isi sesuai data bank;
sistem menolak/`"-"` kolom yang belum ada sumbernya, bukan menulis nol.

## F. Program & saklar khusus (bila berlaku)

- KUR / LPBBTI / Laku Pandai — keputusan partisipasi bank (`KEPUTUSAN-OJK.md`).
- Tarif ta'zir / dana kebajikan (BPRS) — `KEPUTUSAN-PANEL-RISIKO-CKPN.md §2`,
  naskah SK: `draft-surat-keputusan-dps-tazir.md`.
- Saklar bobot agunan, basis deduksi KPMM — `CKPN-SIAP-RILIS.md §(f)`.

## G. Verifikasi terakhir sebelum lapor

- `CKPN-SIAP-RILIS.md §(e)` langkah verifikasi (ringkasan parameter, mode bayangan).
- Ekspor `/reports/ojk/monthly?period=YYYY-MM` harus **berhasil** (bukan 422) setelah
  butir C selesai; bila 422, pesannya menyebut langkah perbaikan.
- Peta menyeluruh kelengkapan form: `CELAH-FORM-OJK.md`; kelengkapan kolom:
  `CELAH-LAPORAN-OJK.md`.

---

## Yang **tidak** boleh dilakukan sistem (batas peran)

- Sistem **tidak meratifikasi** atas nama bank. Status `FINAL` hanya sah dengan bukti
  berita acara bertanda tangan.
- Sistem **tidak mengisi** angka PD/LGD, daftar COA, atau data register. Itu data bank.
- Sistem **tidak menebak** kolom yang belum ada sumbernya: ditulis `-` beserta alasan.
