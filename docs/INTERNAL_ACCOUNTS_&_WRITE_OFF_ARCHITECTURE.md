# Manajemen Rekening Internal, Antar Kantor (RAK), & Hapus Buku (Write-off)

## 1. Manajemen Rekening Internal (Single-Branch vs Multi-Cabang)

Di Core Banking System (CBS) perbankan Indonesia (BPR/BPRS & BMT), **Rekening Internal (GL Accounts)** diklasifikasikan ke dalam 4 kategori utama:

### A. Rekening Kas Vault & Sub-Kasir Teller (Till Accounts)
* `10100` — **Kas Vault Utama (Main Branch Vault Cash)**: Tempat penyimpanan fisik brankas utama bank.
* `10101-{STAFF_ID}` — **Kasir Teller (Teller Till Accounts)**: Rekening fisik laci kasir masing-masing Teller.
* **Operasional EOD Kasir:** Saat awal shift, Kasir menerima Modal Kas (`GL-VAULT` $\rightarrow$ `GL-TELLER`). Saat EOD, Teller menyetorkan sisa kas kembali (`GL-TELLER` $\rightarrow$ `GL-VAULT`).

### B. Rekening Antar Kantor / RAK (Branch Inter-Office Settlement)
Untuk BPR/BMT yang memiliki Kantor Cabang (KC) dan Kantor Kas (KK):
* `10800` — **RAK Aktiva (Inter-office Asset Account)**: Saldo tagihan antar-cabang.
* `20800` — **RAK Pasiva (Inter-office Liability Account)**: Saldo kewajiban antar-cabang.
* *Contoh Trx:* Nasabah Cabang A tarik tunai di Teller Cabang B:
  - **Di Cabang B:** Debit `20800 (RAK Pasiva Cabang A)` $\rightarrow$ Kredit `10101 (Kas Teller Cabang B)`.
  - **Di Cabang A:** Debit `20100 (Tabungan Nasabah)` $\rightarrow$ Kredit `10800 (RAK Aktiva Cabang B)`.

### C. Rekening Pelantara / Suspense Accounts
* `10999` — **Rekening Perantara Pencairan (Disbursement Suspense Account)**: Digunakan saat proses *clearing* / pencairan pinjaman sebelum masuk ke rekening tabungan nasabah.

---

## 2. Jurnal Akuntansi Pencairan Pinjaman (Disbursement Source Account)

Dari rekening mana pencairan pinjaman didebet?

### Standar Jurnal Double-Entry Pencairan Kredit:
 Saat kredit disetujui dan dicairkan (`DisburseLoan`), sistem mencatat:
- **DEBIT:** `10300 - Piutang Pembiayaan / Loan Receivable` (Aset Pinjaman Bertambah)
- **KREDIT:** `20100 - Simpanan Nasabah / Customer Savings Account` (Kewajiban Bank Bertambah)

> **Catatan:** Uang tunai Kas Vault (`10100`) **TIDAK BERKURANG** saat pencairan kredit, karena dana masuk sebagai saldo simpanan di rekening tabungan nasabah. Kas Vault baru berkurang ketika nasabah melakukan **Penarikan Tunai (`Withdrawal`)** di teller.

---

## 3. Akuntansi Hapus Buku (Write-off) & Recovery Kredit Macet

Ketika kredit berada di status **Kolektibilitas 5 (MACET)** dan debitur tidak lagi mampu membayar:

### A. Pengakuan Cadangan Kerugian (PPAP / PPKA Periodik)
* **DEBIT:** `50200 - Beban Penyisihan PPAP/CKPN` (Beban Laba Rugi Bertambah)
* **KREDIT:** `10900 - Cadangan PPAP/CKPN` (Kontra-Aset Bertambah)

### B. Eksekusi Hapus Buku (Write-Off / `WriteOffLoan`)
Hapus buku adalah tindakan administratif mengeluarkan pinjaman macet dari Neraca (*On-Balance Sheet*):
* **DEBIT:** `10900 - Cadangan PPAP/CKPN` (Mengurangi Cadangan Kerugian)
* **KREDIT:** `10300 - Piutang Pembiayaan / Loan Receivable` (Mengurangi Aset Pinjaman)
* **Jurnal Memoria / Kontinjensi (Off-Balance Sheet):**
  - **DEBIT:** `90100 - Rekening Kontinjensi Tagihan Kredit Hapus Buku` (Pencatatan Ekstrakomptabel untuk penagihan susulan oleh AO).

### C. Penerimaan Kembali Kredit Hapus Buku (Recovery / `RecoverWrittenOffLoan`)
Jika di kemudian hari debitur yang di-hapus buku membayar sebagian atau seluruh tunggakannya:
* **DEBIT:** `10100 - Kas Vault / Rekening Nasabah` (Aset Kas Bertambah)
* **KREDIT:** `40900 - Pendapatan Recovery Hapus Buku` (Pendapatan Lain-lain Bertambah)
