# Transkrip Struktur Form OJK — A4 / A5 / A6

Sumber: SEOJK No. 16/SEOJK.03/2024 (PDF resmi 528 hlmn) di `/tmp/ojk/s16b.pdf`,
dibaca dengan PyMuPDF (`fitz`). Status: **BACA-SAJA**, tidak ada perubahan kode/repo.

## Konvensi
- **`PDF #n`** = nomor halaman PDF 1-based (fitz indeks `n-1`); **cetak** = `n − 52`
  (terverifikasi: `PDF #298`→`- 246 -`, `PDF #189`→`- 137 -`, `PDF #293`→`- 241 -`).
- Halaman form daftar/rincian berurut: **`FORM xx – 1`** (judul + kepala tabel),
  **`– 2` (SANDI/KOLOM)**, **`– 3` (PENJELASAN)**. Form multi-halaman sering
  mengulang label `– 1` di halaman lanjutan (mis. `00.07`, `00.09`, `00.10`), jadi
  nomor terakhir bukan penanda halaman.
- Angka romawi (I, II, …) = **sandi kolom** yang tercetak di header.
- Klasifikasi isi: **sandi** (kode pilihan), **angka** (rupiah/nominal/persentase),
  **tanggal** (TT-BB-TTTT dsb.), **teks** (nama/alamat/uraian).
- Rentang halaman yang tercatat di `docs/CELAH-FORM-OJK.md` dipakai sebagai titik
  awal; setiap klaim di bawah disertai `PDF #page` hasil bacaan langsung.

---

# A4 — `BELUM DIMODELKAN`

## 1. FORM 00.01 — Data Kepemilikan BPR
- Nama persis: **FORM 00.01 – Data Kepemilikan BPR**. Form `PDF #72` (cetak 20);
  sandi `PDF #73` (cetak 21); penjelasan `PDF #74` (cetak 22). Rentang `72–74`.

### Kolom (8 kolom, urut) — `PDF #72`, sandi `PDF #73`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Nama | teks |
| II | Alamat | teks |
| III | Jenis | sandi pilihan (01/02/03/04) |
| IV | No. Identitas | teks (NIK/NPWP) |
| V | Status Pemegang Saham | sandi pilihan (01/02) |
| VI | Jumlah Nominal | angka (rupiah penuh) |
| VII | Persentase Kepemilikan | angka (%) |
| VIII | Status Perubahan | sandi pilihan (1/2/3/9) |

### Struktur baris
**Per-pemegang-saham** (satu baris = satu pemegang saham). **Tidak ada baris
JUMLAH** (`PDF #72` — tabel berakhir tanpa baris jumlah).

### Aturan penting (kutipan)
- **II Alamat / IV No. Identitas**: *"Dalam hal alamat lengkap pemegang saham BPR
  tidak diketahui, untuk kepemilikan kurang dari 2% (dua persen) kolom ini dapat
  dikosongkan."* (`PDF #74`) — sama untuk No. Identitas.
- **III Jenis**: 1. Perorangan **01**; 2. Badan Hukum **02**; 3. Pemerintah Daerah
  **03**; 4. Publik **04** (`PDF #73`).
- **V**: PSP **01**; Non PSP **02** (`PDF #73`). PSP = pemegang saham pengendali.
- **VIII Status Perubahan**: sandi **1** pemegang saham baru; **2** perubahan
  kepemilikan yang mengakibatkan perubahan PSP; **3** perubahan yang tidak
  mengakibatkan perubahan PSP; **9** tidak ada perubahan (`PDF #74`).
- Tidak ada aturan "hanya Desember" dan tidak ada ambang yang memicu form lain.

---

## 2. FORM 00.07 — Daftar Pinjaman yang Diterima
- Nama persis: **FORM 00.07 - Daftar Pinjaman yang Diterima**. Form I–IX `PDF #248`
  (cetak 196); lanjutan X–XV `PDF #249` (cetak 197); sandi `PDF #250–251`
  (cetak 198–199); penjelasan `PDF #252–254` (cetak 200–202). Rentang `248–254`.
- Catatan: `PDF #245–247` adalah Form **00.06**, bukan bagian 00.07.

### Kolom (15 kolom, urut) — `PDF #248` (I–IX) + `PDF #249` (X–XV)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | ID Pihak Lawan | sandi unik |
| II | Gol. Kreditur | sandi (Lampiran 02) |
| III | Sandi Bank | sandi (6 digit SPOJK) |
| IV | Lokasi Kreditur | sandi (Lampiran 03) |
| V | Jenis | sandi pilihan (10/20/31/32/41/42/99) |
| VI | Hubungan dengan Bank | sandi pilihan (12/20) |
| VII | Jangka Waktu | tanggal (2 sub: Tanggal Mulai, Tanggal Jatuh Tempo) |
| VIII | Suku Bunga | angka (%) + sandi (2 sub: Persentase, Cara Perhitungan 11/12/21/22) |
| IX | Plafon | angka (rupiah penuh) |
| X | Jenis Agunan yang Dijaminkan | sandi (Lampiran 01; tanpa agunan = 299) |
| XI | Nominal Agunan yang Dijaminkan | angka (rupiah penuh; tanpa agunan = 0) |
| XII | Baki Debet | angka (rupiah penuh) |
| XIII | Biaya Transaksi Belum Diamortisasi | angka (rupiah penuh) |
| XIV | Diskonto Belum Diamortisasi | angka (rupiah penuh) |
| XV | Baki Debet Neto | angka (rupiah penuh) |

### Sandi baris/kolom tetap
- **V Jenis** (`PDF #250`): Pinjaman Bilateral **10**; Pinjaman Sindikasi **20**;
  Pinjaman Khusus — dari Lembaga Pengayom **31**, dalam rangka Linkage **32**;
  Pinjaman dengan Persyaratan Tertentu — modal inti tambahan **41**, modal pelengkap
  **42**; Lainnya **99**.
- **VIII Cara Perhitungan** (`PDF #250`): Bunga Flat — Tetap **11**, Mengambang
  **12**; Bunga Tidak Flat — Tetap **21**, Mengambang **22**.
- **VI**: Terkait **12**; Tidak Terkait **20** (`PDF #250`).

### Struktur baris
**Per-pinjaman/kreditur** (satu baris = satu pinjaman yang diterima). Ada baris
**JUMLAH** (`PDF #248`). Definisi: *"pinjaman yang diterima dari bank, Bank
Indonesia dan/atau pihak ketiga bukan bank dengan kewajiban pembayaran kembali
berdasarkan persyaratan perjanjian utang piutang."* (`PDF #252`).

### Aturan penting (kutipan)
- **X**: *"Dalam hal tidak terdapat agunan yang diserahkan, kolom jenis agunan yang
  dijaminkan diisi dengan sandi lainnya (299)."* (`PDF #253`)
- **XI**: *"Dalam hal tidak terdapat agunan yang diserahkan, kolom nilai agunan diisi
  dengan 0 (nol)."* (`PDF #253`)
- **IX Plafon**: untuk pinjaman berangsuran pokok = *"posisi plafon pada tanggal
  laporan"* (plafon menurun) (`PDF #253`).
- **XV Baki Debet Neto**: *"baki debet pinjaman setelah dikurangi dengan diskonto dan
  biaya transaksi belum diamortisasi."* (`PDF #254`)

---

## 3. FORM 04.00 — Daftar Surat Berharga
- Nama persis: **FORM 04.00 – Daftar Surat Berharga**. Form I–VI `PDF #129`
  (cetak 77); lanjutan VII–XV `PDF #130` (cetak 78); lanjutan XVI–XXV `PDF #131`
  (cetak 79); sandi `PDF #132–133` (cetak 80–81); penjelasan `PDF #134–136`
  (cetak 82–84). Rentang `129–134` (konten sampai 136).

### Kolom (25 kolom, urut) — `PDF #129–131`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Klasifikasi | sandi pilihan (1/2) |
| III | Suku Bunga | angka (%) |
| IV | Jangka Waktu | tanggal (2 sub: Tanggal Mulai, Tanggal Jatuh Tempo) |
| V | Nominal | angka (rupiah penuh) |
| VI | Nominal yang Dijaminkan | angka (rupiah penuh) |
| VII | Biaya Perolehan | angka (rupiah penuh) |
| VIII | Diskonto/Premium Belum Diamortisasi | angka (rupiah penuh) |
| IX | Biaya Transaksi Belum Diamortisasi | angka (rupiah penuh) |
| X | Laba/Rugi Belum Direalisasi | angka (rupiah penuh) |
| XI | Biaya Perolehan Diamortisasi/Nilai Wajar | angka (rupiah penuh) |
| XII | Nomor Surat Berharga | teks (ISIN) |
| XIII | ID Pihak Lawan | sandi unik |
| XIV | Jenis | sandi pilihan (1/2/3) |
| XV | Kualitas | sandi pilihan (1/3/5) |
| XVI | Cadangan Kerugian Penurunan Nilai | angka (rupiah penuh) |
| XVII | Lembaga Pemeringkat | sandi (Lampiran 08; tanpa peringkat = 9) |
| XVIII | Peringkat Surat Berharga | sandi (Lampiran 09; tanpa peringkat = 99) |
| XIX | Tanggal Pemeringkatan | tanggal |
| XX | Tanggal Penerbitan | tanggal |
| XXI | Cadangan Kerugian Penurunan Nilai Aset Baik | angka (rupiah penuh) |
| XXII | Cadangan Kerugian Penurunan Nilai Aset Kurang Baik | angka (rupiah penuh) |
| XXIII | Cadangan Kerugian Penurunan Nilai Aset Tidak Baik | angka (rupiah penuh) |
| XXIV | Klasifikasi Aset Keuangan | sandi pilihan (1/2/3) |
| XXV | Jenis CKPN | sandi pilihan (1/2) |

### Sandi baris/kolom tetap (`PDF #132–133`)
- **II Klasifikasi**: Tersedia untuk dijual **1**; Dimiliki hingga jatuh tempo **2**.
- **XIV Jenis**: diterbitkan Bank Indonesia **1**; Pemerintah **2**; Pemerintah
  Daerah **3**.
- **XV Kualitas**: Lancar **1**; Kurang Lancar **3**; Macet **5** (hanya tiga ini
  tercetak di form; Bab II mendefinisikan 1–5, `PDF #63`).
- **XXIV Klasifikasi Aset Keuangan**: nilai wajar melalui laba rugi **1**; nilai wajar
  melalui penghasilan komprehensif lain **2**; biaya perolehan diamortisasi **3**.
- **XXV Jenis CKPN**: Individual **1**; Kolektif **2**.

### Struktur baris
**Per-surat-berharga** (satu baris = satu surat berharga yang dimiliki). Ada baris
**JUMLAH** (`PDF #129`). Definisi: *"Surat berharga yaitu surat berharga yang
diterbitkan oleh Bank Indonesia, Pemerintah, atau Pemerintah Daerah…"* (`PDF #134`).

### Aturan penting (kutipan)
- **XXIV**: *"Kolom ini hanya diisi apabila BPR melakukan penawaran umum efek di
  pasar modal."* (`PDF #135`)
- **XVII**: *"Dalam hal pihak lawan tidak memiliki peringkat atau memiliki peringkat
  yang diterbitkan oleh lembaga pemeringkat yang tidak diakui Otoritas Jasa Keuangan
  diisi dengan sandi 9."* (`PDF #135`)
- **XVIII**: sama, *"diisi dengan sandi 99."* (`PDF #135`)
- **VIII**: *"nilai diskonto atau premium yang belum diamortisasi…"* (`PDF #134`);
  **XI**: nilai nominal − diskonto belum diamortisasi + premium belum diamortisasi +
  biaya transaksi belum diamortisasi, atau nilai wajar (`PDF #135`).
- **XXV Jenis CKPN**: berlaku aturan Bab II — untuk posisi **Desember 2024** diisi
  Kolektif (sandi 2) (`PDF #65`).

---

## 4. FORM 07.00 — Daftar Agunan yang Diambil Alih
- Nama persis: **FORM 07.00 – Daftar Agunan yang Diambil Alih**. Form `PDF #179`
  (cetak 127); sandi `PDF #180` (cetak 128); penjelasan `PDF #181` (cetak 129).
  Cross-ref posisi keuangan `PDF #104` (cetak 52). Rentang `179–181, 104`.

### Kolom (8 kolom, urut) — `PDF #179`, sandi `PDF #180`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Jenis Agunan | sandi pilihan (01/02/03/04/05/99) |
| III | Alamat Agunan | teks |
| IV | Tanggal Pengambilalihan | tanggal |
| V | Nilai Pengakuan Awal | angka (rupiah penuh) |
| VI | Akumulasi Kerugian Penurunan Nilai | angka (rupiah penuh) |
| VII | Jumlah | angka (rupiah penuh) |
| VIII | Nilai Bersih Yang Dapat Direalisasikan | angka (rupiah penuh) |

### Sandi baris/kolom tetap (`PDF #180`)
**II Jenis Agunan**: Emas perhiasan **01**; Tanah dan/atau bangunan **02**;
Resi Gudang **03**; Tempat usaha (los/kios/lapak) **04**; Kendaraan bermotor, kapal,
perahu bermotor, alat berat, mesin yang menjadi satu kesatuan dengan tanah **05**;
Lainnya **99**.

### Struktur baris
**Per-AYDA** (satu baris = satu agunan yang diambil alih). Ada baris **JUMLAH**
(`PDF #179`).

### Aturan penting (kutipan)
- Definisi: *"AYDA yaitu aset yang diperoleh BPR untuk penyelesaian kredit, baik
  melalui pelelangan, atau di luar pelelangan berdasarkan penyerahan secara sukarela
  oleh pemilik agunan … dalam hal debitur telah dinyatakan macet."* (`PDF #181`)
- **V**: *"nilai wajar AYDA setelah dikurangi estimasi biaya penjualan (nilai realisasi
  bersih/net realizeable value) pada saat agunan diambil alih, paling tinggi sebesar
  baki debet kredit debitur."* (`PDF #181`)
- **VI**: *"nilai realisasi bersih posisi laporan atas AYDA dikurangi nilai pengakuan
  awal. Kerugian … dapat dipulihkan kembali paling tinggi sebesar akumulasi kerugian
  penurunan nilai yang telah diakui."* (`PDF #181`)
- **VII Jumlah**: *"nilai yang lebih rendah dari nilai realisasi bersih (net realizable
  value) posisi laporan atau nilai tercatat."* (`PDF #181`)
- Cross-ref: *"Pos ini dirinci pada Form 07.00 – Daftar Agunan yang Diambil Alih."*
  (`PDF #104`)

---

## 5. FORM 16.00 — Daftar Penyertaan Modal
- Nama persis: **FORM 16.00 – Daftar Penyertaan Modal**. Form I–IX `PDF #223`
  (cetak 171); lanjutan X–XV `PDF #224` (cetak 172); sandi `PDF #225–226`
  (cetak 173–174); penjelasan `PDF #227–228` (cetak 175–176). Rentang `223–227`
  (konten sampai 228).

### Kolom (15 kolom, urut) — `PDF #223` (I–IX) + `PDF #224` (X–XV)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | No. Register | teks (kode unik, mandatory) |
| III | ID Pihak Lawan | sandi unik |
| IV | Metode Penyertaan | sandi pilihan (1/2) |
| V | Kualitas | sandi pilihan (1/3/4/5) |
| VI | Tujuan Penyertaan | sandi pilihan (1/9) |
| VII | Tanggal Mulai | tanggal |
| VIII | Persentase Penyertaan | angka (%) |
| IX | Nominal | angka (rupiah penuh) |
| X | Jumlah Bulan Laporan | angka (rupiah penuh) |
| XI | Cadangan Kerugian Penurunan Nilai | angka (rupiah penuh) |
| XII | Cadangan Kerugian Penurunan Nilai Aset Baik | angka (rupiah penuh) |
| XIII | Cadangan Kerugian Penurunan Nilai Aset Kurang Baik | angka (rupiah penuh) |
| XIV | Cadangan Kerugian Penurunan Nilai Aset Tidak Baik | angka (rupiah penuh) |
| XV | Jenis CKPN | sandi pilihan (1/2) |

### Sandi baris/kolom tetap (`PDF #225`)
- **IV Metode Penyertaan**: Biaya Perolehan **1**; Metode Ekuitas **2**.
- **V Kualitas**: Lancar **1**; Kurang Lancar **3**; Diragukan **4**; Macet **5**.
- **VI Tujuan Penyertaan**: Dalam Rangka Investasi – Penyertaan pada Lembaga
  Penunjang **1**; Lainnya **9**.
- **XV Jenis CKPN**: Individual **1**; Kolektif **2**.

### Struktur baris
**Per-penyertaan modal** (satu baris = satu penyertaan kepada satu pihak lawan).
**Tidak ada baris JUMLAH** (`PDF #223–224`).

### Aturan penting (kutipan)
- Definisi: *"Yang dilaporkan pada form ini yaitu semua penyertaan modal yang
  dilakukan oleh BPR sesuai dengan ketentuan peraturan perundang-undangan."*
  (`PDF #227`)
- **II No. Register**: *"Kode register penyertaan modal harus unik, 1 (satu) kode
  register digunakan untuk 1 (satu) penyertaan modal kepada 1 (satu) pihak lawan …
  tidak boleh digunakan untuk penyertaan modal kepada pihak lain (no reuse atau no
  recycle) … Kolom ini bersifat mandatory."* (`PDF #227`)
- **VIII**: *"persentase penyertaan modal BPR pada pihak lawan terhadap modal
  Lembaga Penunjang."* (`PDF #228`)
- **X Jumlah Bulan Laporan**: *"nilai tercatat penyertaan modal pada bulan laporan."*
  (`PDF #228`)
- **XV**: berlaku aturan Bab II Desember 2024 (Kolektif sandi 2) (`PDF #65`).

---

## 6. FORM 17.00 — Daftar Properti Terbengkalai
- Nama persis: **FORM 17.00 – Daftar Properti Terbengkalai**. Form `PDF #229`
  (cetak 177); sandi `PDF #230` (cetak 178); penjelasan `PDF #231–232`
  (cetak 179–180). Rentang `229–231` (konten sampai 232).

### Kolom (10 kolom, urut) — `PDF #229`, sandi `PDF #230`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | No. Register | teks (kode unik, mandatory) |
| III | Jenis Properti Terbengkalai | sandi pilihan (1/2/3) |
| IV | Alamat Properti Terbengkalai | teks |
| V | Koordinat | teks (koordinat) |
| VI | Tanggal Penetapan | tanggal |
| VII | Biaya Perolehan atau Nilai Wajar | angka (rupiah penuh) |
| VIII | Akumulasi Penyusutan atau Amortisasi | angka (rupiah penuh) |
| IX | Jumlah | angka (rupiah penuh) |
| X | Metode Pengukuran | sandi pilihan (1/2) |

### Sandi baris/kolom tetap (`PDF #230`)
- **III Jenis Properti Terbengkalai**: Tanah **1**; Bangunan **2**;
  Tanah dan Bangunan **3**.
- **X Metode Pengukuran**: Model Biaya **1**; Model Revaluasi (Nilai Wajar) **2**.

### Struktur baris
**Per-properti terbengkalai** (satu baris = satu properti). **Tidak ada baris
JUMLAH** (`PDF #229`).

### Aturan penting (kutipan)
- Definisi: *"properti terbengkalai adalah aset tetap dalam bentuk properti yang
  dimiliki BPR namun tidak digunakan untuk kegiatan usaha BPR yang berkaitan
  operasional BPR…"* (`PDF #231`)
- **II No. Register**: unik, 1 register untuk 1 properti, *"no reuse atau no
  recycle"*, *"bersifat mandatory."* (`PDF #231`)
- **VIII**: *"Dalam hal properti terbengkalai mengalami penurunan nilai … maka jumlah
  penurunan nilai tersebut juga dilaporkan pada kolom ini."* (`PDF #231`)
- **IX Jumlah**: *"nilai pengakuan awal dikurangi dengan akumulasi penyusutan atau
  amortisasi termasuk akumulasi kerugian penurunan nilai apabila ada."* (`PDF #232`)
- Cross-ref: *"Pos ini dirinci pada Form 17.00- Daftar Properti Terbengkalai."*
  (`PDF #104`)

---

## 7. FORM 18.00 — Daftar Aset Keuangan Lainnya
- Nama persis: **FORM 18.00 – Daftar Aset Keuangan Lainnya**. Form I–IX `PDF #233`
  (cetak 181); lanjutan X–XV `PDF #234` (cetak 182); sandi `PDF #235–236`
  (cetak 183–184); penjelasan `PDF #237–238` (cetak 185–186). Rentang `233–237`
  (konten sampai 238).

### Kolom (15 kolom, urut) — `PDF #233` (I–IX) + `PDF #234` (X–XV)
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | No. Rekening | teks (unik per rekening) |
| III | ID Pihak Lawan | sandi unik |
| IV | Jenis | sandi pilihan (10/99) |
| V | Tanggal Mulai | tanggal (TT-MM-TTTT) |
| VI | Tanggal Jatuh Tempo | tanggal (TT-MM-TTTT) |
| VII | Suku Bunga | angka (%) |
| VIII | Nominal | angka (rupiah penuh) |
| IX | Nilai Agunan yang Dapat Diperhitungkan | angka (rupiah penuh) |
| X | Cadangan Kerugian Penurunan Nilai | angka (rupiah penuh) |
| XI | Cadangan Kerugian Penurunan Nilai Aset Baik | angka (rupiah penuh) |
| XII | Cadangan Kerugian Penurunan Nilai Aset Kurang Baik | angka (rupiah penuh) |
| XIII | Cadangan Kerugian Penurunan Nilai Aset Tidak Baik | angka (rupiah penuh) |
| XIV | Klasifikasi Aset Keuangan | sandi pilihan (1/2/3) |
| XV | Jenis CKPN | sandi pilihan (1/2) |

### Sandi baris/kolom tetap (`PDF #235`)
- **IV Jenis**: Tagihan fraud **10**; Tagihan lainnya **99**.
- **XIV Klasifikasi Aset Keuangan**: nilai wajar melalui laba rugi **1**; nilai wajar
  melalui penghasilan komprehensif lain **2**; biaya perolehan diamortisasi **3**.
- **XV Jenis CKPN**: Individual **1**; Kolektif **2**.

### Struktur baris
**Per-rekening aset keuangan lainnya** (satu baris = satu rekening unik).
**Tidak ada baris JUMLAH** (`PDF #233–234`).

### Aturan penting (kutipan)
- Definisi: *"semua aset keuangan lainnya yang dimiliki oleh BPR yang memenuhi
  definisi aset keuangan sesuai dengan standar akuntansi keuangan."* (`PDF #237`)
- **II**: *"setiap rekening aset keuangan lainnya diisi dengan 1 (satu) nomor rekening
  yang unik (tidak boleh sama) untuk setiap rekening."* (`PDF #237`)
- **IX**: *"nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA."*
  (`PDF #237`)
- **XV**: berlaku aturan Bab II Desember 2024 (Kolektif sandi 2) (`PDF #65`).
- Format tanggal form ini **`TT-MM-TTTT`** (bukan `TT-BB-TTTT`).

---

## 8. FORM 00.17 — Laporan Perubahan Ekuitas
- Nama persis: **FORM 00.17 – Laporan Perubahan Ekuitas**. Form `PDF #293`
  (cetak 241); penjelasan `PDF #294` (cetak 242). Rentang `293–294`.

### Kolom (12 kolom, urut) — `PDF #293`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Nama Rekening | teks |
| II | Sandi | sandi pos (8 digit) |
| III | Modal Disetor | angka (rupiah penuh) |
| IV | Tambahan Modal | angka (rupiah penuh) |
| V | Modal Sumbangan | angka (rupiah penuh) |
| VI | DSM Ekuitas | angka (rupiah penuh) |
| VII | Laba/Rugi yang Belum Direalisasi | angka (rupiah penuh) |
| VIII | Surplus Revaluasi Aset Tetap | angka (rupiah penuh) |
| IX | Cadangan Tujuan | angka (rupiah penuh) |
| X | Cadangan Umum | angka (rupiah penuh) |
| XI | Saldo Laba yang Belum Ditentukan | angka (rupiah penuh) |
| XII | Jumlah | angka (rupiah penuh) |

### Struktur baris: TETAP (17 baris baku × 3 tahun) — `PDF #293`
| Nama Rekening | Sandi |
|---|---|
| Saldo per 31 Des Tahun T-2 | 10000000 |
| Dividen | 10100000 |
| Pembentukan Cadangan | 10200000 |
| Setoran Modal | 10300000 |
| Laba/Rugi yang Belum Direalisasi | 10400000 |
| Revaluasi Aset Tetap | 10500000 |
| Laba/Rugi Periode Berjalan | 10600000 |
| Pos Penambah/Pengurang Lainnya | 19900000 |
| Saldo per 31 Des Tahun T-1 | 20000000 |
| Dividen | 20100000 |
| Pembentukan Cadangan | 20200000 |
| Setoran Modal | 20300000 |
| Laba/Rugi yang Belum Direalisasi | 20400000 |
| Revaluasi Aset Tetap | 20500000 |
| Laba/Rugi Periode Berjalan | 20600000 |
| Pos Penambah/Pengurang Lainnya | 29900000 |
| Saldo per 31 Des Tahun T | 30000000 |

Tidak ada baris JUMLAH terpisah; kolom XII "Jumlah" adalah penjumlahan per baris.

### Aturan penting (kutipan)
- **Hanya Desember**: *"Form ini hanya disampaikan untuk laporan posisi bulan
  Desember."* (`PDF #293`)
- Definisi: *"Laporan perubahan ekuitas adalah laporan keuangan yang menyajikan laba
  rugi dan penghasilan komprehensif lain untuk suatu periode … Penyusunan …
  mengacu pada standar akuntansi keuangan yang berlaku."* (`PDF #294`)

---

# A5 — `KONDISIONAL`

## 9. FORM 09.01 — Rincian Aset Lainnya – Lain-lain
- Nama persis: **FORM 09.01 – Rincian Aset Lainnya – Lain-lain**. Form `PDF #189`
  (cetak 137); penjelasan `PDF #190` (cetak 138). Rentang `188–190`; form sebenarnya
  di `#189` (`#188` adalah penjelasan Form 09.00).

### Kolom (3 kolom, urut) — `PDF #189`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Uraian | teks |
| III | Jumlah | angka (rupiah penuh) |

### Struktur baris
**Per-rincian bebas** (satu baris = satu uraian aset dari pos "Lainnya"). Ada baris
**JUMLAH** (`PDF #189`).

### Kapan induk mewajibkan (aturan pemicu)
- Induk: **Form 09.00 – Rincian Aset Lainnya**, pos **"Lainnya"** (`PDF #188`).
- *"Jika jumlah pos ini melebihi 25% (dua puluh lima persen) dari jumlah pos aset
  lainnya maka pos tersebut dilaporkan pada Form 09.01 - Rincian Aset Lainnya –
  Lain-Lain."* (`PDF #188`, penjelasan 09.00)
- *"Rincian aset lainnya – lain-lain dilaporkan oleh BPR apabila pos lainnya pada Form
  09.00 … melebihi 25% (dua puluh lima persen) dari jumlah aset lainnya."*
  (`PDF #190`)
- **Syarat numerik**: *"Kolom [III Jumlah] … harus sama dengan jumlah pos Lainnya pada
  Form 09.00 – Rincian Aset Lainnya."* (`PDF #190`) → form ini **menunggu induk
  (09.00)** dan hanya terbit saat ambang 25% terlampaui.

---

## 10. FORM 14.01 — Rincian Liabilitas Lainnya – Lain-lain
- Nama persis: **FORM 14.01 – Rincian Liabilitas Lainnya – Lain-lain**. Form
  `PDF #216` (cetak 164); penjelasan `PDF #217` (cetak 165). Rentang `216–217`.

### Kolom (3 kolom, urut) — `PDF #216`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Uraian | teks |
| III | Jumlah | angka (rupiah penuh) |

### Struktur baris
**Per-rincian bebas** (satu baris = satu uraian liabilitas dari pos "Lainnya"). Ada
baris **JUMLAH** (`PDF #216`).

### Kapan induk mewajibkan (aturan pemicu)
- Induk: **Form 14.00 – Rincian Liabilitas Lainnya**, pos **"Lainnya"** (`PDF #215`).
- *"Jika jumlah pos ini melebihi 25% (dua puluh lima persen) dari jumlah pos liabilitas
  lainnya maka pos tersebut dilaporkan pada Form 14.01 …"* (`PDF #215`)
- *"Rincian liabilitas lainnya – lain-lain dilaporkan apabila pos lainnya pada Form
  14.00 … melebihi 25% (dua puluh lima persen) dari jumlah liabilitas lainnya."*
  (`PDF #217`)
- **III Jumlah**: disebut *"sebesar liabilitas BPR yang harus diselesaikan"*
  (`PDF #217`) — **tidak** setegas 09.01 yang eksplisit "harus sama dengan Form
  09.00" (lihat Anomali).

---

## 11. FORM 00.09 — Data Anggota Direksi dan Anggota Dewan Komisaris BPR yang Berhenti Menjabat
- Nama persis: **FORM 00.09 – Data Anggota Direksi dan Anggota Dewan Komisaris BPR
  yang Berhenti Menjabat**. Form I–V `PDF #258` (cetak 206) + VI–X `PDF #259`
  (cetak 207); sandi `PDF #260–261` (cetak 208–209); penjelasan `PDF #262–263`
  (cetak 210–211). Rentang `258–263`.

### Kolom (10 kolom, urut) — `PDF #258` + `PDF #259`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Nama | teks |
| II | NIK | teks |
| III | Jabatan | sandi pilihan (110/120/210/220) |
| IV | Tanggal Mulai Menjabat | tanggal |
| V | Keanggotaan Komite (sub: Komite Audit, Komite Pemantau Risiko, Komite Remunerasi dan Nominasi, Komite Manajemen Risiko) | sandi pilihan per sub (00/01/02) |
| VI | Membawahkan Fungsi Kepatuhan (Ya/Tidak) | sandi pilihan (1/2) |
| VII | Komisaris Independen (Ya/Tidak) | sandi pilihan (1/2) |
| VIII | Keterangan Penyebab Berhenti Menjabat | sandi pilihan (1/2/3) |
| IX | Tanggal Berhenti Menjabat | tanggal |
| X | Alasan Mengundurkan Diri/Pemberhentian | teks |

### Sandi tetap (`PDF #260–261`)
- **III Jabatan**: Direktur Utama **110**; Direktur **120**; Komisaris Utama **210**;
  Komisaris **220**.
- **V Keanggotaan Komite**: Tidak Menjabat **00**; Ketua **01**; Anggota **02**.
- **VI**: Ya **1**; Tidak **2**. **Bagi anggota dewan komisaris kolom ini
  dikosongkan** (`PDF #263`).
- **VII**: Ya **1**; Tidak **2**.
- **VIII**: Pengunduran Diri **1**; Pemberhentian **2**; Meninggal Dunia **3**.

### Struktur baris
**Per-orang yang berhenti** (satu baris = satu direksi/DK). **Tidak ada baris
JUMLAH**.

### Kapan induk mewajibkan (aturan pemicu)
- Bukan turunan form angka; dipicu **peristiwa**: *"Form ini diisi dalam hal terdapat
  anggota direksi dan/atau anggota dewan komisaris yang diberhentikan, mengundurkan
  diri atau meninggal dunia…"* (`PDF #262`)
- Artinya: dapat dibangun dari `bank_management` (event `ended_at`) tanpa menunggu
  form lain, tetapi hanya muncul bila ada peristiwa.

---

## 12. FORM 00.10 — Data Pejabat Eksekutif BPR yang Berhenti Menjabat
- Nama persis: **FORM 00.10 – Data Pejabat Eksekutif BPR yang Berhenti Menjabat**.
  Form `PDF #264` (cetak 212) + `PDF #265` (cetak 213); sandi `PDF #266–267`
  (cetak 214–215); penjelasan `PDF #268–269` (cetak 216–217). Rentang `264–269`.

### Kolom (9 kolom, urut) — `PDF #264` + `PDF #265`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Nama | teks |
| II | NIK | teks |
| III | Jabatan (header dipecah 5 sub-kolom: Kepatuhan, Manajemen Risiko, Audit Intern, APU dan PPT, Lainnya) | sandi pilihan (00/01/02) |
| IV | Tanggal Mulai Menjabat | tanggal |
| V | Surat Pengangkatan (sub: No., Tanggal) | teks + tanggal |
| VI | Keanggotaan Komite (sub: Komite Audit, Komite Pemantau Risiko, Komite Remunerasi dan Nominasi, Komite Manajemen Risiko) | sandi pilihan per sub (00/01/02) |
| VII | Keterangan Penyebab Berhenti Menjabat | sandi pilihan (1/2/3) |
| VIII | Alasan Mengundurkan Diri/Pemberhentian | teks |
| IX | Surat Pemberhentian (sub: No., Tanggal) | teks + tanggal |

### Sandi tetap (`PDF #266`)
- **III Jabatan**: Tidak Menjabat **00**; Kepala Satuan Kerja **01**;
  Pejabat Eksekutif **02**.
- **VI Keanggotaan Komite**: Tidak Menjabat **00**; Ketua **01**; Anggota **02**.
- **VII**: Pengunduran Diri **1**; Pemberhentian **2**; Meninggal Dunia **3**.

### Struktur baris
**Per-pejabat eksekutif yang berhenti** (satu baris = satu orang). **Tidak ada baris
JUMLAH**.

### Kapan induk mewajibkan (aturan pemicu)
- Bukan turunan form angka; dipicu **peristiwa**: *"Form ini diisi dalam hal terdapat
  kepala satuan kerja atau pejabat eksekutif BPR yang diberhentikan oleh BPR,
  mengundurkan diri, atau meninggal dunia…"* (`PDF #268`)

---

## 13. FORM 00.12 — Data Penutupan Kantor dan Terminal Perbankan Elektronik (TPE)
- Nama persis: **FORM 00.12 – Data Penutupan Kantor dan Terminal Perbankan
  Elektronik (TPE)**. Form `PDF #276` (cetak 224); sandi `PDF #277–278`
  (cetak 225–226); penjelasan `PDF #279–280` (cetak 227–228). Rentang `276–280`.

### Kolom (8 kolom, urut) — `PDF #276`, sandi `PDF #277–278`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Jenis | sandi pilihan (01–08, 99) |
| II | Sandi Kantor/Kode Kantor | sandi/teks |
| III | Sandi Kantor Induk | sandi |
| IV | Nama Kantor | teks |
| V | Koordinat | teks |
| VI | Alamat | teks |
| VII | Tanggal Pelaksanaan | tanggal |
| VIII | Lokasi | sandi (Lampiran 03) |

### Sandi tetap (`PDF #277`)
**I Jenis**: Kantor Cabang **01**; Kantor Kas **02**; Kas Keliling **03**; Titik
Pembayaran **04**; ATM **05**; EDC **06**; Kantor Wilayah **07**; Sentra Keuangan
Khusus **08**; Lainnya **99**.

### Struktur baris
**Per-kantor/TPE yang ditutup** (satu baris = satu penutupan). **Tidak ada baris
JUMLAH**.

### Kapan induk mewajibkan (aturan pemicu)
- Bukan turunan form angka; dipicu **peristiwa**: *"Form ini diisi dalam hal terdapat
  penutupan kantor dan/atau transaksi perbankan elektronik (TPE)…"* (`PDF #279`)
- *"Untuk TPE kolom [IV Nama Kantor] dikosongkan."* (`PDF #279`)
- **VI Alamat**: *"Untuk TPE kolom ini dikosongkan"* (implisit lewat butir penjelasan;
  lihat `PDF #279`).

---

## 14. FORM 03.00 — Daftar Kas dalam Valuta Asing
- Nama persis: **FORM 03.00 – Daftar Kas dalam Valuta Asing**. Form `PDF #126`
  (cetak 74); sandi `PDF #127` (cetak 75); penjelasan `PDF #128` (cetak 76).
  Cross-ref posisi keuangan `PDF #102` (cetak 50). Rentang `126–128, 102`.

### Kolom (5 kolom, urut) — `PDF #126`, sandi `PDF #127`
| Sandi | Nama kolom persis | Jenis isi |
|---|---|---|
| I | Sandi Kantor | sandi |
| II | Jenis Valuta Asing | sandi (Lampiran 04) |
| III | Nominal | angka (2 desimal, original currency) |
| IV | Kurs Tengah (Rp) | angka (2 desimal) |
| V | Nilai Rupiah | angka (rupiah penuh) |

### Struktur baris
**Per-jenis valuta asing** (satu baris = satu jenis valas yang diperdagangkan).
Ada baris **JUMLAH** (`PDF #126`).

### Kapan induk mewajibkan (aturan pemicu)
- Bukan turunan form angka; dipicu **status/keadaan**: *"Kas dalam valuta asing
  (valas) yaitu uang kertas asing, uang logam asing, dan cek pelawat (travellers
  cheque) yang masih berlaku yang dimiliki BPR **sebagai pedagang valuta asing**."*
  (`PDF #128`) → terbit hanya bila BPR berstatus PVA.
- Cross-ref: *"Pos ini dirinci pada Form 03.00 – Daftar Kas dalam Valuta Asing."*
  (`PDF #102`)

### Aturan penting (kutipan)
- **III**: *"nilai valuta asing (original currency) sebelum dirupiahkan yang dimiliki
  BPR pada tanggal laporan."* (`PDF #128`)
- **IV**: *"kurs tengah yang tersedia di sistem Bank Indonesia pada tanggal laporan.
  Apabila kurs tengah tidak tersedia, nilai yang dilaporkan sebesar kurs beli ditambah
  kurs jual … dibagi dua (rata-rata)."* (`PDF #128`)
- **V**: *"hasil perkalian dari nominal dengan kurs tengah."* (`PDF #128`)

---

# A6 — `DOKUMEN`

## 15. FORM 00.19 — Struktur Organisasi BPR
- Nama persis: **FORM 00.19 – Struktur Organisasi BPR**. `PDF #298` (cetak 246).
  Terdaftar di daftar Laporan Gabungan `PDF #60` (cetak 8).
- **Bukan tabel angka.** Isi peraturan: *"Struktur organisasi BPR disampaikan oleh BPR
  dalam bentuk portable document format (.pdf) kepada Otoritas Jasa Keuangan.
  Struktur organisasi BPR yang dilaporkan mencakup susunan hierarki seluruh jaringan
  kantor yang dimiliki oleh BPR, divisi atau satuan kerja, dan nama pegawai tetap atau
  tidak tetap BPR."* (`PDF #298`)
- **Dari data apa** (per triase `docs/CELAH-FORM-OJK.md`): bahan sudah ada —
  `bank_offices`, `bank_management`, `branches` → PDF-nya bisa dirakit otomatis.

---

## 16. FORM 00.20 — Struktur Kelompok Usaha BPR
- Nama persis: **FORM 00.20 – Struktur Kelompok Usaha BPR**. `PDF #299` (cetak 247).
  Terdaftar `PDF #60` (cetak 8).
- **Bukan tabel angka.** Isi peraturan: *"Struktur kelompok usaha BPR disampaikan oleh
  BPR dalam bentuk portable document format (.pdf) … mencakup seluruh pihak yang
  terkait dengan BPR dari segi pengendalian sampai dengan ultimate shareholder sesuai
  dengan Peraturan Otoritas Jasa Keuangan mengenai Penilaian Kemampuan dan Kepatutan
  bagi Pihak Utama Lembaga Jasa Keuangan."* (`PDF #299`)
- **Dari data apa**: hanya satu string `ojk.report.ultimate_shareholders` (triase) —
  belum cukup untuk merakit PDF; tetap manual.

---

## 17. FORM 00.21 — Laporan Dokumen Penilaian Risiko TPPU, TPPT, dan/atau PPSPM
- Nama persis: **FORM 00.21 – Laporan Dokumen Penilaian Risiko TPPU, TPPT, dan/atau
  PPSPM**. `PDF #300` (cetak 248). Terdaftar `PDF #59` (cetak 7) dan `PDF #60`
  (cetak 8). Ketentuan koreksi khusus di `PDF #60` (cetak 8).
- **Bukan tabel angka.** Isi peraturan: *"Laporan dokumen penilaian risiko TPPU, TPPT,
  dan/atau PPSPM disampaikan oleh BPR dalam bentuk portable document format (.pdf).
  Penyusunan Laporan dimaksud sesuai dengan Peraturan Otoritas Jasa Keuangan mengenai
  penerapan program anti pencucian uang, pencegahan pendanaan terorisme, dan pencegahan
  pendanaan proliferasi senjata pemusnah massal di sektor jasa keuangan."* (`PDF #300`)
- Ketentuan koreksi: bila koreksi laporan hanya terkait Form 00.21, *"BPR melampirkan
  surat pernyataan … disampaikan dalam bentuk portable document format (.pdf) pada
  Form 00.13 – Dokumen Pendukung."* (`PDF #60`)
- **Dari data apa**: disusun manual di luar sistem (`definitions.go:105`,
  `Buildable=false`) — tetap manual.

---

# Aturan umum lintas form (Bab II) yang dipakai
`PDF #61–66` (cetak 9–14):
- **J. Kualitas** (`PDF #63`): 1 lancar, 2 dalam perhatian khusus, 3 kurang lancar,
  4 diragukan, 5 macet.
- **K. Jangka Waktu** (`PDF #64`): *"Untuk aset atau liabilitas keuangan yang tidak
  memiliki jatuh tempo, tanggal jatuh tempo dikosongkan."*
- **P. CKPN** (`PDF #65`): CKPN individual/kolektif untuk SAK EP; CKPN aset baik
  (stage 1), kurang baik (stage 2), tidak baik (stage 3).
- **Q. Jenis CKPN** (`PDF #65`): *"Untuk laporan berkala bulanan posisi bulan Desember
  2024, kolom Jenis CKPN diisi dengan CKPN Kolektif (sandi 2)."* Berlaku untuk form
  yang punya kolom Jenis CKPN: `04.00` XXV, `16.00` XV, `18.00` XV.
- **R. ID Pihak Lawan** (`PDF #65–66`): harus = CIF SLIK bila ada; unik; no reuse;
  *"Kolom ID Pihak lawan harus diisi (mandatory)."*
- **Golongan Kreditur** (`PDF #63`): Bank Indonesia / bank / pihak ketiga bukan bank
  yang memberi fasilitas pinjaman kepada BPR.
- **Sandi Bank** (`PDF #61`): 6 digit untuk BPR/BPRS; mengacu SPOJK untuk bank umum.

# Form yang tidak ditemukan di PDF
- **Tidak ada.** Seluruh form yang diminta (A4: `04.00`, `07.00`, `16.00`, `17.00`,
  `18.00`, `00.07`, `00.01`, `00.17`; A5: `09.01`, `14.01`, `00.09`, `00.10`, `00.12`,
  `03.00`; A6: `00.19`, `00.20`, `00.21`) ditemukan di `s16b.pdf` dan tercantum di
  tabel triase `docs/CELAH-FORM-OJK.md`.
- `08.00` dan `10.00` (A4) **tidak dikerjakan ulang** karena sudah ditranskrip di
  `docs/transkrip-form-08-10-11-12-14.md`.

# Anomali / perlu konfirmasi
1. **`00.07` label halaman ganda.** Form dan lanjutannya sama-sama berlabel
   `FORM 00.07 - 1` (`PDF #248` dan `#249`); sandi `– 2`, penjelasan `– 3`. Rentang
   `248–254` (bukan 248–250). Konvensi label halaman jangan dipakai sebagai penanda.
2. **`04.00` label `– 1` di tiga halaman** (`#129–131`); `XV Kualitas` hanya mencetak
   sandi **1, 3, 5** — melompati 2 (dalam perhatian khusus) dan 4 (diragukan) yang ada
   di Bab II (`PDF #63`). Perlu konfirmasi apakah sengaja.
3. **`16.00` salah nomor romawi.** Header halaman `#224` mencetak `XII` **dua kali**
   (kolom "Cadangan Kerugian Penurunan Nilai Aset Baik" dan "…Kurang Baik");
   seharusnya `XIII`. Tabel sandi `#225` sudah benar (`XII`, `XIII`, `XIV`, `XV`).
   Ikuti tabel sandi, bukan header.
4. **`16.00`/`17.00`/`18.00` tanpa baris JUMLAH** (`PDF #223–224`, `#229`, `#233–234`),
   sedangkan `00.07`/`04.00`/`07.00` punya `JUMLAH`. Pastikan eksportir tidak
   menambahkan baris jumlah untuk tiga form ini.
5. **`00.10` kolom III Jabatan dipecah 5 sub-kolom** (Kepatuhan, Manajemen Risiko,
   Audit Intern, APU dan PPT, Lainnya) menurut garis tabel (`PDF #264`,
   `find_tables`), tetapi tabel sandi (`PDF #266`) hanya memberi satu set sandi
   `00/01/02` (Tidak Menjabat/Kepala Satuan Kerja/Pejabat Eksekutif) — tidak ada
   penjelasan bagaimana sandi dipetakan per fungsi. **Perlu konfirmasi OJK** sebelum
   dibangun. Pola serupa ada di Form 00.03 (di luar cakupan ini).
6. **`00.09` kolom V Keanggotaan Komite** dipecah 4 sub-kolom komite (`PDF #258`),
   masing-masing bersandi `00/01/02` menurut tabel sandi (`PDF #260`) — konfirmasi
   bahwa tiap sub-kolom komite memang diisi sandi tersendiri.
7. **`14.01` vs `09.01` tidak simetris.** `09.01` III Jumlah: *"harus sama dengan jumlah
   pos Lainnya pada Form 09.00"* (`PDF #190`); `14.01` III Jumlah hanya disebut
   *"sebesar liabilitas BPR yang harus diselesaikan"* (`PDF #217`) tanpa menyebut Form
   14.00. Perlu konfirmasi apakah tetap harus sama dengan pos Lainnya Form 14.00.
8. **`18.00` format tanggal `TT-MM-TTTT`** (`PDF #235`) berbeda dari mayoritas form
   yang memakai `TT-BB-TTTT`. Pastikan parser tanggal menangani keduanya.
9. **`00.17` typo header** *"Laba/Rugi yang Belum Direaliasasi"* (`PDF #293`) dan hanya
   boleh terbit posisi Desember (`PDF #293`). Bangun khusus bulan Desember.
10. **`03.00` tidak punya tabel sandi valas sendiri** — hanya merujuk Lampiran 04
    (`PDF #127`), dan triase mencatat pos `1101020000` sengaja tanpa sumber COA.
    Sumber data valas belum ada.
11. **Rentang triase `09.01` "190, 188–189" sedikit menyesatkan**: form 09.01 ada di
    `PDF #189`, penjelasannya di `#190`; `#188` adalah penjelasan Form 09.00. Form
    09.00 di `#187`.
12. **`00.12` III Sandi Kantor Induk** dijelaskan sebagai *"3 (tiga) angka sandi kantor
    pusat dan kantor cabang"* (`PDF #279`) sedangkan sandi kantor pada form lain
    (`00.11`) juga 3 angka — konsisten, tetapi sumber `bank_offices.code` belum
    menjamin pemetaan induk. Perlu keputusan.
13. **`07.00` cross-ref ganda**: penjelasan pos AYDA di `PDF #104` dan form di
    `#179–181`; pastikan tidak tertukar saat implementasi.
14. **`04.00` typo** *"Sertfikat"*, *"kompehensif"*, dan **`16.00` typo** *"perolahan"*
    (`PDF #135`, `#228`) — hanya catatan kualitas teks PDF, bukan data.

## Referensi halaman mentah hasil ekstraksi
`/tmp/ojk/raw/`: `00.01.txt`, `00.07.txt`, `04.00.txt`, `07.00.txt`, `16.00.txt`,
`17.00.txt`, `18.00.txt`, `00.17.txt`, `09.01.txt`, `14.01.txt`, `00.09.txt`,
`00.10.txt`, `00.12.txt`, `03.00.txt`, `00.19.txt`, `00.20.txt`, `00.21.txt`,
`00.21b.txt`, `bab2-63-68.txt`.
