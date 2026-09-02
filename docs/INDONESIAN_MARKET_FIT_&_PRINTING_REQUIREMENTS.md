# Analisis Pasar Indonesia & Kebutuhan Cetak (Printing Slips) di BPR/BPRS & BMT

## Part 1: Brainstorming Kebutuhan Cetak (Print Slips & Documents)

Di operasional BPR/BPRS dan BMT/Koperasi di Indonesia, **bukti fisik dan cetakan kertas (paper proof) masih merupakan syarat wajib** untuk audit, keabsahan nasabah, dan regulasi OJK / Kemenkop UKM.

### 1. Cetakan Teller Harian (Teller Transaction Slips)
* **Slip Setoran Tunai (2 Ply):**
  * Ply 1: Untuk Nasabah (Bukti setoran).
  * Ply 2: Untuk Teller (Voucher lampiran kas opname).
  * *Komponen:* No. Ref, Tanggal/Jam, Nama Teller, No. Rekening & Nama Nasabah, Nominal Angka & **Terbilang** (misal: *Lima Juta Rupiah*), Tanda Tangan Nasabah & Teller.
* **Slip Penarikan Tunai (2 Ply):**
  * Memiliki blok verifikasi Tanda Tangan Nasabah (dicocokkan dengan spesimen KTP/Buku Tabungan).
* **Slip Pemindahbukuan / Transfer Internal (1-2 Ply).**

### 2. Cetak Buku Tabungan (Passbook Printing - Epson PLQ 20/30)
* Nasabah BPR & BMT di daerah sangat menyukai bukti fisik di **Buku Tabungan**.
* **Mekanisme Cetak Buku:**
  * Cetak berbasis baris (Line Printing) ke printer Dot-Matrix khusus (Passbook Printer Epson PLQ-20 / Olivetti PR2 Plus).
  * *Format Baris:* `[Tanggal] [Kode Trx] [Debet] [Kredit] [Saldo] [Teller ID]`.

### 3. Dokumen Pembiayaan / Kredit (Loan Documents)
* **Cetak Akad Pembiayaan / Perjanjian Kredit:**
  * Dokumen hukum perjanjian pinjaman (termasuk kolom e-Meterai / Meterai fisik 10.000, jaminan/agunan, dan syarat ketentuan).
* **Cetak Akad Syariah (Wadiah / Murabahah / Mudharabah):**
  * Spesifik untuk BMT / BPRS yang menyantumkan rincian Ijab Qabul & Margin Jual Beli.
* **Tabel Jadwal Angsuran (Repayment Schedule Printout):**
  * Cetakan rincian angsuran per bulan (Pokok + Bunga/Margin) yang diserahkan ke nasabah saat pencairan.
* **Kuitansi Pencairan & Kuitansi Angsuran.**

### 4. Resi Thermal Mobile Collector (Jemput Bola Pasar)
* Cetakan struk kecil ukuran 58mm / 80mm dari Printer Thermal Bluetooth yang dibawa AO/Collector ke pasar.
* *Komponen:* Nama BMT/BPR, Nama Collector, No. Resi `MBL-...`, Nama Nasabah, Saldo Setelah Trx, Koordinat GPS.

### 5. Cetakan End of Day (EOD Kas Opname Supervisor)
* **Berita Acara Kas Opname Teller:** Diakhir hari, Teller mencetak lembar rincian uang fisik brankas (pecahan 100k, 50k, 20k) untuk dicocokkan dengan saldo sistem.

---

## Part 2: Review Product Market Fit CBS Kita di Pasar Indonesia

Bagaimana posisi CBS Core yang sudah kita bangun jika diadu dengan kompetitor software BPR/BMT di pasar Indonesia saat ini?

### 🚀 Keunggulan Utama (Competitive Edge) CBS Kita:
1. **Modern Web-Cloud Monorepo vs Software Legacy Desktop (FoxPro/Delphi):**
   * 85-90% BPR/BMT di Indonesia masih memakai aplikasi desktop tua (FoxPro, Access, Delphi) yang di-install per PC dan rawan crash/virus.
   * CBS kita berbasis **Web Modern (Go + Next.js 15)** yang bisa diakses via browser dari cabang manapun tanpa install ulang.
2. **Core Ledger Double-Entry Atomic:**
   * Garansi saldo 100% seimbang. Tidak ada risiko selisih kas akibat terputus koneksi.
3. **Dual Engine BPR (Konvensional) & BMT (Syariah):**
   * Satu sistem bisa melayani BPR (bunga flat/anuitas) sekaligus BMT/BPRS (Wadiah, Mudharabah, Murabahah).
4. **Built-in Mobile Collector (Jemput Bola):**
   * Software lain butuh pihak ketiga (third party) dengan integrasi sync yang rumit. CBS kita punya API Mobile Collector terintegrasi bawaan.
5. **Keamanan & Standar OJK (Audit Trail & 7 Role RBAC):**
   * Memenuhi standar POJK TI BPR untuk audit log dan Maker-Checker 4-Eyes Principle.

---

## 📋 Fitur yang Perlu Dilengkapi untuk "Perfect Market Fit":

1. **Komponen Modal Print / PDF Generator (Slip & Kuitansi):**
   * Menambahkan tampilan cetak popup (Print Preview PDF / HTML Print) di frontend Next.js untuk Slip Teller & Kuitansi Angsuran.
2. **Fungsi Terbilang dalam Bahasa Indonesia:**
   * Konversi angka nominal ke kalimat teks (contoh: `Rp 5.000.000` $\rightarrow$ *"Lima Juta Rupiah"*).
3. **Modul Batch Processing EOD / EOM (End of Day / Month):**
   * Potong biaya admin tabungan otomatis tiap akhir bulan.
   * Distribusi bunga / nisbah bagi hasil otomatis tiap akhir bulan.
4. **Exporter Laporan SLIK OJK (Sistem Layanan Informasi Keuangan):**
   * Export data debitur ke format file `.txt` / `.csv` standar OJK SLIK.
