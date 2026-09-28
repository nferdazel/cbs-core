# Keputusan OJK — Pemilik Mendelegasikan (27 Sep 2026)

Pemilik sistem menyerahkan keputusan produk/regulasi yang tertunda kepada tim build.
Dokumen ini mencatat keputusan, bawaan konservatif, dan cara bank mengubahnya. Prinsip
yang dipegang di seluruh dokumen: **tidak mengarang angka/aturan regulasi**. Bila angkanya
tidak punya rujukan tertulis di repo, keputusannya adalah "sediakan mekanisme + parameter
yang diisi bank, default non-enforcing, laporan menulis `-`".

Rujukan: `docs/CELAH-LAPORAN-OJK.md` (triase), `docs/LAMPIRAN-OJK.md` (sumber sandi),
`docs/KEPUTUSAN.md` (keputusan lama), `docs/CKPN-SIAP-RILIS.md`.

## 1. BMPK

| Aspek | Keputusan | Bawaan | Cara bank mengubah |
|---|---|---|---|
| Basis batas | Batas **nominal per pihak** (`bmpk_limits.max_amount`) otoritatif | — (bank isi) | Isi `bmpk_limits` |
| Batas % modal | Ditunda: basis KPMM & persentase belum ada rujukannya | Tidak dihitung, kolom laporan "belum tersedia" | Dibangun bila Lampiran/POJK tersedia |
| Taksonomi pihak terkait | Teks bebas kebijakan bank | — | Isi `bmpk_related_parties.relationship_type` |
| Istilah resmi | Status internal netral (`DALAM_BATAS`/`MELAMPAUI_BATAS`) | Netral | Dipetakan bila istilah resmi (pelanggaran/pelampauan) ditetapkan |
| Nomor/sandi kolom Laporan BMPK | Tidak dikarang; kolom memakai penamaan internal | Belum tersedia | Dipetakan bila Lampiran diperoleh |

## 2. Jalur tulis data OJK

- **API-first**, bukan SQL/seed jangka panjang. Sudah tersedia:
  - `PUT /api/v1/ojk/loan-codes/{loanId}` (izin `system:config`) — **17 kolom Form 06**
    yang harus diisi bank (sandi inline, tanggal, nominal amortisasi/agunan/kelonggaran).
  - `PUT/GET /api/v1/ojk/placement-codes/{placementId}` + `GET .../reports/ojk/placements`
    (izin `system:config`) — **8 kolom Form 05** penempatan, termasuk baca nilai tersimpan.
  - `PUT /customers/{id}` — Form 06 IX/XVIII/XX.
  - `GET/PUT/DELETE /api/v1/reports/ojk/bmpk/master|related-parties|limits` — pihak
    terkait & batas BMPK (izin `system:config`, teraudit satu transaksi).
  - `GET/PUT/DELETE /api/v1/reports/ojk/kelembagaan/offices|management` — data
    kelembagaan (kantor + direksi/komisaris/pejabat).
  - `GET/PUT/DELETE /api/v1/reports/ojk/off-balance/items` — register rekening
    administratif (komitmen/kontinjensi).
  - `GET/PUT /api/v1/system/ckpn-pabl` (izin `system:config` / `:read`) — enam kunci
    `ckpn.pabl.*` (saklar + PD/LGD per golongan + aturan aset baik), sebelumnya SQL-only.
- Semuanya punya layar di `pengaturan/ojk` (Kelembagaan, Rekening Administratif,
  BMPK, Penempatan, plus OJK Reference Codes yang kini memuat 17 bidang kredit)
  sehingga **28 field** terisi tanpa SQL.
- **SQL/seed hanya untuk backfill sekali**; data berkelanjutan wajib lewat API teraudit.

## 3. CKPN

- `ckpn.enabled` tetap **false** sampai parameter `FINAL` + ratifikasi Direksi/akuntan lengkap
  dan mode bayangan berjalan. **Nilai SEMENTARA dilarang masuk laporan OJK** (aturan panel
  §1.5 — keputusan ini mengikat, jangan dilonggarkan ronde implementasi apa pun).
- `ckpn.pabl.enabled` menyala hanya setelah bank mengisi PD/LGD per golongan **dan**
  kesiapan CKPN utama terpenuhi (parameter `FINAL` + ratifikasi lengkap). Syarat itu
  ditegakkan mesin lewat `PUT /api/v1/system/ckpn-pabl` (gap ditolak `422`) sehingga
  Form 05 kolom XII/XXI tidak pernah membawa nilai dari parameter SEMENTARA.
- Akun pemulihan mengikuti keputusan 12.5.b; akun pendapatan tersendiri tidak dibangun.

## 4. Butir DK (definisi) — diputuskan

| Butir | Keputusan |
|---|---|
| XLVII / Form 05 XX Klasifikasi Aset Keuangan | **Sediakan kolom** (migrasi `000108`), **tanpa auto-klasifikasi**; bank mengisi. |
| XLIV–XLVI CKPN per stage | Tetap **belum tersedia**: stage 1/2/3 ditujukan BPR berpasar modal (SAK Indonesia); instalasi ini SAK EP tanpa model SICR/default. |
| XXXVI Pendapatan Bunga Dalam Penyelesaian | Tetap **belum tersedia**: sistem memakai basis kas untuk NPL; tidak diturunkan dari kolektibilitas (akan menggandakan kolom XXXV). |
| III No. Identitas (NIK/NPWP) | **Tidak dipaparkan** sebagai keluaran laporan (keputusan privasi). |

## 5. Butir KB (kondisional bank) — kebijakan

- Bank **bukan peserta** KUR/LPBBTI/Laku Pandai secara bawaan; partisipasi dikonfirmasi saat
  onboarding. Selama bukan peserta, kolom terkait (VI, XXXIX, XL, XLIII; Laporan Laku Pandai,
  Form 00.15) ditulis `-` dan itu **sah, bukan cacat data**.
- Saklar `bank.participates.*` **sengaja tidak di-seed** sampai modul KUR/LPBBTI/Laku Pandai
  benar-benar dibangun dan ada kode yang membacanya. Kunci config tanpa pembaca hanya menjadi
  sampah; menaruh namanya di string dokumentasi akan meloloskan invarian config secara semu.

## 6. Di luar cakupan build

- **Tarif ta'zir**: milik DPS bank (sistem hanya menyediakan bawaan `0`, plafon penjaga, akun,
  gerbang klausul akad). Jangan diisi mesin.
- **Hapus buku**: sudah ada (POJK 1/2024 Pasal 42–43); gerbang pelepasan cadangan jangan diubah.
- **Dekom**: tidak dimodelkan, bukan objek otomasi.
- **Laporan LX** (publikasi keuangan, bukti pengumuman, TPPU/TPPT sebagai dokumen, Form 00.13):
  berkas manual dari luar sistem.

## 7. Yang TIDAK boleh diotomasi/ditebak mesin

1. Angka PD/LGD sah & status `FINAL` CKPN.
2. Sandi Bank 6 digit APOLO/SPOJK dan nama/sandi kolom resmi Laporan BMPK.
3. Taksonomi pihak terkait & batas BMPK.
4. Istilah hukum "pelanggaran/pelampauan", tarif ta'zir, kebijakan hapus buku/dekom.
5. Klasifikasi SAK EP, stage CKPN, pendapatan bunga dalam penyelesaian.
6. Membuka NIK/NPWP ke keluaran laporan.

## 8. Form 09.00 — Rincian Aset Lainnya

- **Sumber nilai = saldo COA saja**, lewat `COAMapping09Draft` terpisah dari
  `COAMappingDraft` (satu kode COA = satu entri, jadi pemetaan rincian tidak boleh
  menimpa pemetaan neraca). Konsekuensinya total Form 09.00 selalu **sama persis** dengan
  pos Aset Lainnya (`1299000000`) Form 01.00 — dijaga test invarian.
- Pos 1.a (Penempatan pada Bank Lain), 1.c (Surat Berharga), dan 1.d (Lainnya) ditulis
  `-` + alasan: tidak ada akun COA untuk akruan pos itu. **Data Form 05.00/06.00 tidak
  dipakai** meski Form 05.00 punya kolom akruan — memakainya membuat total Form 09.00
  meleset dari Form 01.00, dan kekonsistenan dua form resmi lebih penting daripada
  mengisi satu baris.
- Enam akun baru `10310`–`10360` (migrasi `000114`) menyediakan tempat mencatat premi
  LPS dibayar di muka, uang muka pajak, aset pajak tangguhan, biaya dibayar di muka,
  tagihan asuransi, dan uang muka kegiatan operasional; keduanya juga dipetakan ke pos
  Aset Lainnya Form 01.00.
- Pemetaan tetap `DRAF-BELUM-TERVERIFIKASI` — status yang sama dengan Form 01.00/02.00
  yang sejak lama buildable; verifikasi bank/akuntan tetap diperlukan, tetapi bukan lagi
  penghalang pembangunan form.

Semua butir implementable kini dikerjakan; tidak ada lagi celah kode — sisa hanya
keputusan milik bank (verifikasi pemetaan, ratifikasi parameter, partisipasi program;
lihat `CELAH-LAPORAN-OJK.md §4`).
