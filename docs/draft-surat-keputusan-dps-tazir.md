# RANCANGAN — Surat Keputusan Dewan Pengawas Syariah [Nama Bank]

## tentang Pengenaan Ta'zir dan Pengelolaan Dana Kebajikan atas Pembiayaan Syariah

> **Status: RANCANGAN KERJA — BELUM DISETUJUI.**
> Dokumen ini draf untuk ditinjau Dewan Pengawas Syariah (DPS) dan/atau penasihat
> hukum. Bukan keputusan yang berlaku, bukan nasihat hukum, dan bukan pernyataan
> resmi pembuat sistem. Bagian bertanda `[…]` (nomor, tanggal, nama, tempat, rujukan)
> tetap harus diisi manusia. Butir bertanda `[PILIHAN: …]` yang dulu diserahkan kini
> sudah **diisi bawaan** agar sistem dapat jalan; nilai bawaan itu **hanya sah setelah
> DPS menyetujuinya**, dan hanya DPS yang boleh mengubahnya (lihat catatan panel di
> akhir Pasal 3 dan bagian Catatan Pengisian).

**Nomor**: [nomor surat keputusan]
**Rapat DPS**: [nomor rapat], [tanggal rapat]
**Anggota DPS yang menandatangani**: [nama dan jabatan tiap anggota]
**Tanggal berlaku**: [tanggal berlaku]
**Acuan teknis sistem**: [sebutkan versi/berkas, bila perlu]

---

## Dasar Pertimbangan

**Menimbang:**

a. bahwa nasabah pembiayaan syariah yang mampu membayar namun menunda pembayaran
   merugikan bank dan tidak adil bagi nasabah yang tertib;

b. bahwa prinsip syariah menghendaki ketentuan ta'zir atas keterlambatan memenuhi
   keadilan, tidak menzalimi, dan tidak menjadi sumber pendapatan bank;

c. bahwa sistem CBS Core telah membukukan ta'zir ke akun 12500 Dana Kebajikan dan
   belum mengakuinya sebagai pendapatan, sebagai sikap sementara sampai DPS
   menetapkan perlakuan resminya;

d. bahwa parameter tarif, plafon, dan perlakuan akuntansi ta'zir berjalan di
   konfigurasi sistem, sehingga keputusan DPS dan nilai di sistem harus dijaga
   tetap sama;

e. bahwa [rujukan: fatwa/rondangan DSN-MUI atau ketentuan lain yang relevan];

**Memutuskan:**

---

## Pasal 1 — Pengenaan Ta'zir

1. Ta'zir keterlambatan dikenakan atas **pokok angsuran yang telah lewat jatuh
   tempo**, bukan atas pokok pembiayaan secara keseluruhan.
2. Ta'zir dihitung **per angsuran yang tertunggak**, hasil tiap angsuran
   dijumlahkan, dan berjalan **harian sampai angsuran dibayar**.
3. Tarif yang benar-benar ditagih **wajib sama dengan klausul yang tercantum di akad**.
   Selama DPS belum menetapkan tarif, nilai di sistem tetap **0** (tidak ada ta'zir
   diakru); sistem tidak pernah mengakru sebelum DPS dan akad menetapkannya. Bila akad
   memakai satuan bulan, nilai harian dihitung dari satuan akad itu (mis. **1%/bulan =
   0,33‰/hari**, bukan 1‰/hari). Angka **1‰/hari = ±3%/bulan hanya berlaku untuk bulan
   30 hari** dan **hanya contoh**, bukan ketetapan. Nilai berjalan tersimpan pada
   konfigurasi `loan.penalty.rate.daily.per_mille`.
4. Plafon akrual ta'zir per pembiayaan **10%** dari pokok angsuran tertunggak (bawaan
   sistem); nilai **0** berarti plafon dimatikan. Batas penjaga sistem: plafon **maksimum
   100%**, ditolak bila di atas itu. DPS memutuskan plafon resminya pada Pasal 1A/4.
5. Akrual ta'zir **dihentikan** bila nasabah **tidak mampu** atau karena *force majeure*
   menurut kriteria DPS (Fatwa DSN-MUI 17/2000: yang tidak mampu tidak boleh dikenai
   sanksi). Penghentian karena kolektibilitas NPL hanyalah **pengaman sistem**, bukan
   pemenuhan syarat syariah; ta'zir yang sudah terakru atas nasabah mampu tetap menjadi
   kewajibannya.
6. Ta'zir **hanya** dikenakan kepada nasabah yang **mampu tetapi sengaja menunda**
   pembayaran. Cara menilai kemampuan dan siapa yang memutuskan ditetapkan DPS (lihat
   Pasal 1A ayat (3)).
7. Pembiayaan yang **direstrukturisasi dikecualikan** dari ta'zir **seluruhnya selama
   masa restrukturisasi** (konsesi menunjukkan nasabah tertekan, bukan menunda). Setelah
   masa restrukturisasi berakhir, ketentuannya kembali mengikuti Pasal 1.

### Pasal 1A — Syarat Pencantuman Klausul dalam Akad

1. Ta'zir **tidak diakru** sebelum bank menyatakan bahwa klausul ta'zir **sudah
   dicantumkan pada akad** pembiayaan. Penanda sistem `loan.penalty.syariah.akad_disclosed`
   harus `true`, dan setiap pembiayaan wajib memiliki **nomor akad** (`loans.akad_number`)
   sebagai buktinya. Tanpa keduanya, ta'zir = 0 dan alasannya dicatat — sistem menolak
   mengakru, bukan diam-diam menghitung.
2. Penetapan `akad_disclosed=true` adalah wewenang pejabat berizin dan hanya boleh
   dilakukan setelah rumusan klausul disetujui DPS.
3. Penilaian "mampu" dilakukan per nasabah/pembiayaan dan **disimpan agar dapat
   diaudit**; nasabah tidak mampu/*force majeure* tidak dikenai ta'zir.
4. Acuan kewajiban pengungkapan: POJK 24/2024 (ta'zir/ta'widh wajib diungkapkan dalam
   informasi produk sebelum penandatanganan akad) dan Fatwa DSN-MUI No. 17/DSN-MUI/IX/2000.

## Pasal 2 — Perlakuan Dana Kebajikan

1. Ta'zir **bukan pendapatan bank** dan tidak boleh diperlakukan sebagai
   pendapatan dalam laporan keuangan.
2. Pembukuan ta'zir mengikuti **pemetaan jurnal EVENT `loan_penalty`** produk syariah.
   Kunci konfigurasi `loan.penalty.syariah.social_fund.coa` adalah **bawaan tujuan**
   (12500 Dana Kebajikan); ia bukan satu-satunya penentu akun. Kode **menolak** pemetaan
   yang mengkredit akun pendapatan (mis. 40500) untuk pembiayaan syariah: selama pemetaan
   produk tidak menunjuk dana kebajikan, ta'zir tidak diakru, bukan diakui sebagai
   pendapatan.
3. **Dana kebajikan belum boleh diakui sebagai pendapatan** dan belum boleh
   disalurkan sebelum DPS menetapkan perlakuan dan tata cara penyalurannya.
4. Saldo akun 12500 dilaporkan pada ringkasan tutup hari sebagai
   `social_fund_balance`, sehingga perkembangannya dapat diawasi.
5. Saldo akun 12500 yang tidak bergerak **lebih dari 2 kuartal (6 bulan)** memicu
   **peringatan tingkat tinggi** pada tutup hari dan wajib dilaporkan ke DPS pada rapat
   terdekat. Sistem **tidak pernah** mengakui saldo itu sebagai pendapatan.

## Pasal 3 — Penyaluran Dana Kebajikan

1. Penerima dana kebajikan ditetapkan **DPS** (dapat bersama Direksi atau pejabat yang
   DPS tunjuk); kriteria penerima adalah **dana sosial** (fakir/miskin/yatim/dhuafa/
   kepentingan umum) dan **dilarang** menjadi pendapatan atau biaya operasional bank.
2. Penyaluran dilakukan **sekurang-kurangnya sekali per kuartal**, dengan laporan ke DPS
   pada rapat terdekat setelah penyaluran.
3. Bentuk penyaluran mengikuti kebijakan sosial bank yang disetujui DPS dan tidak
   bertentangan dengan prinsip syariah.
4. Bukti pertanggungjawaban berupa **berita acara dan kuitansi** penerimaan, disimpan
   sekurang-kurangnya sesuai kebijakan arsip bank.
5. Perlakuan atas saldo yang belum tersalur (termasuk pemindahan ke dana sosial lain)
   ditetapkan DPS dan dituangkan dalam **berita acara DPS** sebelum dijalankan.
6. Butir 1–5 di atas adalah **bawaan**; DPS dapat mengubahnya **hanya lewat keputusan
   DPS** (bukan perubahan sistem oleh vendor). Setiap perubahan wajib disesuaikan dengan
   nilai di sistem agar keduanya sama.

> **Catatan panel (bukan bagian keputusan DPS):** butir-butir di atas yang bersifat
> bawaan sistem dapat diubah hanya oleh DPS bank. Yang **hanya boleh diisi/ditetapkan
> DPS** adalah: (a) apakah ta'zir dikenakan; (b) tarif yang benar-benar ditagih sesuai
> akad; (c) kriteria "mampu" dan mekanisme penilaiannya; (d) penerima dan bentuk
> penyaluran dana kebajikan; (e) rumusan klausul akad; (f) perlakuan atas saldo tak
> tersalur. Sistem/vendor hanya menetapkan bawaan 0, plafon penjaga, akun tujuan,
> peringatan, dan pelaporan.

## Pasal 4 — Peninjauan

1. Surat keputusan ini ditinjau **secara berkala, sekurang-kurangnya sekali
   setahun**.
2. Surat keputusan ini **wajib ditinjau ulang setiap kali parameter di sistem
   berubah**, termasuk tarif harian, plafon, aturan penghentian pada NPL, dan
   akun pembukuan ta'zir.
3. Tarif dan plafon tersimpan di konfigurasi sistem
   (`loan.penalty.rate.daily.per_mille`, `loan.penalty.cap_pct`). Nilai di
   sistem dan ketetapan DPS **harus selalu sama**; setiap perbedaan diselesaikan
   secara tertulis dengan menyesuaikan salah satu, bukan dibiarkan.
4. Hasil peninjauan dituangkan dalam berita acara atau keputusan baru.
   `[rujukan: format berita acara DPS yang berlaku di bank]`

---

## Penutup

Ditetapkan di [tempat] pada [tanggal].

Menyetujui,

| Nama | Jabatan | Tanda tangan |
|---|---|---|
| [nama anggota DPS] | [jabatan] | |
| [nama anggota DPS] | [jabatan] | |
| [nama anggota DPS] | [jabatan] | |

---

## Catatan Pengisian (bukan bagian keputusan)

- **Placeholder yang harus diisi manusia**: `[Nama Bank]`, `[nomor rapat]`,
  `[tanggal rapat]`, `[nomor surat keputusan]`, `[nama dan jabatan anggota DPS]`,
  `[tanggal berlaku]`, `[tempat]`, dan seluruh `[rujukan: …]`.
- **Perbaikan putaran panel ini** (lihat `docs/KEPUTUSAN-PANEL-RISIKO-CKPN.md` §2.2):
  Pasal 1.3 menyatakan tarif wajib sama dengan akad (1‰/hari hanya contoh 30 hari);
  Pasal 1.5 menghentikan ta'zir karena **tidak mampu/force majeure**, bukan karena NPL;
  Pasal 1.6–1.7 diisi (mampu-yang-menunda; restrukturisasi dikecualikan selama masa
  restrukturisasi); Pasal 2.2 menyatakan akun mengikuti pemetaan jurnal EVENT
  `loan_penalty` (kunci config hanya bawaan, kode menolak akun pendapatan); Pasal 3
  diisi bawaan; dan **Pasal 1A baru** mewajibkan klausul akad sebelum ta'zir diakru.
- **Keputusan yang dahulu diserahkan penuh kepada DPS kini punya bawaan** yang hanya
  sah setelah DPS menyetujuinya: (a) ta'zir hanya untuk nasabah mampu yang sengaja
  menunda; (b) restrukturisasi dikecualikan selama masa restrukturisasi; (c) penerima
  dana kebajikan ditetapkan DPS; (d) penyaluran sekurang-kurangnya per kuartal;
  (e) bukti berita acara + kuitansi; (f) perlakuan saldo belum tersalur lewat berita
  acara DPS. **Hanya DPS** yang boleh mengubah butir-butir ini.
- **Angka bawaan sistem**: tarif harian `loan.penalty.rate.daily.per_mille = 0` (tidak
  ada ta'zir diakru sampai DPS dan akad menetapkan), plafon `loan.penalty.cap_pct = 10`
  (maksimum penjaga 100), akun dana kebajikan 12500. **0 adalah angka yang benar sebagai
  bawaan** karena ta'zir tidak boleh ditagih tanpa dasar akad/keputusan DPS; konsekuensinya
  ta'zir tidak diakru dan saldo 12500 tidak bertambah sampai tarif ditetapkan. Bila DPS
  menetapkan angka lain, nilai di sistem harus diubah agar sejalan (lihat Pasal 4).
- **Yang hanya boleh diisi DPS**: butir (a)–(f) di atas; sedangkan nomor/tanggal/rapat
  dan nama pengesah adalah data administrasi bank. Sistem tidak boleh mengarang identitas.
- Rancangan ini **bukan nasihat hukum**. Sebelum diberlakukan, mintakan tinjauan
  DPS dan/atau penasihat hukum bank.
