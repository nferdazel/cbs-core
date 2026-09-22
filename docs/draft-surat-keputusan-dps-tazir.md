# RANCANGAN — Surat Keputusan Dewan Pengawas Syariah [Nama Bank]

## tentang Pengenaan Ta'zir dan Pengelolaan Dana Kebajikan atas Pembiayaan Syariah

> **Status: RANCANGAN KERJA — BELUM DISETUJUI.**
> Dokumen ini draf untuk ditinjau Dewan Pengawas Syariah (DPS) dan/atau penasihat
> hukum. Bukan keputusan yang berlaku, bukan nasihat hukum, dan bukan pernyataan
> resmi pembuat sistem. Seluruh bagian bertanda `[…]` dan `[PILIHAN: …]` harus
> diputuskan dan diisi manusia sebelum dokumen ini punya akibat apa pun.

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
3. Tarif yang berlaku saat ini **1‰ (satu per mil) per hari** dari pokok angsuran
   tertunggak, setara sekitar **0,1% per hari atau ±3% per bulan**. Nilai ini
   tersimpan pada konfigurasi `loan.penalty.rate.daily.per_mille`.
4. Plafon akrual ta'zir per pembiayaan **10%** dari pokok angsuran tertunggak;
   nilai **0** berarti plafon dimatikan. Nilai ini tersimpan pada konfigurasi
   `loan.penalty.cap.percent`.
5. Akrual ta'zir **dihentikan** saat pembiayaan digolongkan NPL.
6. `[PILIHAN: apakah ta'zir hanya dikenakan kepada nasabah yang mampu tetapi
   sengaja menunda pembayaran. Bila ya, DPS perlu menetapkan cara menilai
   kemampuan nasabah dan siapa yang memutuskan.]`
7. `[PILIHAN: apakah pembiayaan yang direstrukturisasi dikecualikan dari ta'zir,
   seluruhnya atau hanya selama masa restrukturisasi. DPS menetapkan.]`

## Pasal 2 — Perlakuan Dana Kebajikan

1. Ta'zir **bukan pendapatan bank** dan tidak boleh diperlakukan sebagai
   pendapatan dalam laporan keuangan.
2. Pembukuan ta'zir dilakukan ke akun **12500 Dana Kebajikan**; akun kredit
   ditentukan oleh pemetaan produk syariah dan dibaca dari konfigurasi
   `loan.penalty.syariah.social_fund.coa` dengan bawaan `12500`.
3. **Dana kebajikan belum boleh diakui sebagai pendapatan** dan belum boleh
   disalurkan sebelum DPS menetapkan perlakuan dan tata cara penyalurannya.
4. Saldo akun 12500 dilaporkan pada ringkasan tutup hari sebagai
   `social_fund_balance`, sehingga perkembangannya dapat diawasi.

## Pasal 3 — Penyaluran Dana Kebajikan

1. `[PILIHAN: siapa yang menetapkan penerima dana kebajikan — DPS, DPS bersama
   Direksi, atau pejabat yang ditunjuk. Sebutkan nama jabatan, bukan nama orang.]`
2. `[PILIHAN: frekuensi penyaluran, misalnya setiap kuartal, dan tenggat
   pelaporannya ke DPS.]`
3. `[PILIHAN: kriteria penerima dan bentuk penyaluran yang dibenarkan — DPS
   menetapkan.]`
4. `[PILIHAN: bentuk dan masa simpan bukti pertanggungjawaban penyaluran, misalnya
   berita acara, kuitansi penerimaan, atau laporan bulanan. DPS menetapkan.]`
5. `[PILIHAN: apa yang dilakukan atas saldo yang belum tersalur, dan siapa yang
   melaporkannya secara berkala ke DPS.]`

## Pasal 4 — Peninjauan

1. Surat keputusan ini ditinjau **secara berkala, sekurang-kurangnya sekali
   setahun**.
2. Surat keputusan ini **wajib ditinjau ulang setiap kali parameter di sistem
   berubah**, termasuk tarif harian, plafon, aturan penghentian pada NPL, dan
   akun pembukuan ta'zir.
3. Tarif dan plafon tersimpan di konfigurasi sistem
   (`loan.penalty.rate.daily.per_mille`, `loan.penalty.cap.percent`). Nilai di
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
- **Keputusan yang sengaja tidak diambil di rancangan ini** dan diserahkan penuh
  kepada DPS: (a) apakah ta'zir hanya dikenakan kepada nasabah mampu yang sengaja
  menunda; (b) apakah pembiayaan yang direstrukturisasi dikecualikan; (c) siapa
  yang menetapkan penerima dana kebajikan; (d) frekuensi penyaluran; (e) bentuk
  dan masa simpan bukti pertanggungjawaban; (f) perlakuan atas saldo yang belum
  tersalur.
- **Angka yang dipakai (1‰ per hari, plafon 10%, akun 12500) adalah keadaan
  sistem saat ini**, bukan usulan nilai. Bila DPS menetapkan angka lain, nilai di
  sistem harus diubah agar sejalan (lihat Pasal 4).
- Rancangan ini **bukan nasihat hukum**. Sebelum diberlakukan, mintakan tinjauan
  DPS dan/atau penasihat hukum bank.
