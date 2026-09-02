# Analisis Regulasi Perbankan Indonesia, Tanggal Bisnis, & Mesin EOY/EOD/EOM

## 1. Konsep Tanggal Bisnis Perbankan (Business Date vs System Date)

Di operasional perbankan (BPR/BPRS & BMT), **pencatatan jurnal keuangan TIDAK BOLEH mengandalkan `time.Now()` jam OS/server secara langsung**.

### Mengapa Tanggal Bisnis Perbankan Sangat Vital?
1. **End of Day (EOD) Cut-Off:**
   - Ketika transaksi kasir ditutup jam 17:00 WIB dan Supervisor mengeksekusi **EOD (End of Day)**, `business_date` sistem secara resmi bergeser dari `2026-09-02` ke `2026-09-03`.
   - Transaksi yang masuk setelah jam EOD (*after-hours transaction*) akan dicatat dengan `business_date` tanggal bisnis berikutnya (`2026-09-03`), meskipun jam fisik server masih jam `18:00 WIB` di tanggal 2.
2. **Kesesuaian Audit Trail & Pelaporan OJK:**
   - Seluruh saldo laporan harian, neraca saldo, dan pelaporan SLIK OJK mengacu ke `business_date` yang dikontrol secara terpusat.

---

## 2. Siklus Batch Processing (EOD, EOM, EOY)

CBS yang kita bangun menyediakan **Batch Processing Engine** resmi:

### A. End of Day (EOD - Tutup Kas Harian)
* `POST /api/v1/batch/eod`
* Mengunci sistem dari transaksi kasir harian (`status = IN_EOD_PROCESSING`), memverifikasi saldo akhir Teller, dan memajukan `business_date` ke tanggal bisnis berikutnya secara atomic.

### B. End of Month (EOM - Tutup Buku Bulanan)
* `POST /api/v1/batch/eom`
* **Pemotongan Biaya Administrasi Bulanan:** Eksekusi auto-debet biaya admin tabungan nasabah.
* **Perhitungan & Distribusi Bunga / Bagi Hasil:**
  * BPR Konvensional: Perhitungan & pengkreditan bunga tabungan bulanan.
  * BMT / BPRS Syariah: Perhitungan Nisbah Bagi Hasil Mudharabah berdasarkan saldo rata-rata bulanan.

### C. End of Year (EOY - Tutup Buku Akhir Tahun)
* `POST /api/v1/batch/eoy`
* **Jurnal Penutup Akhir Tahun (Closing Entries):**
  1. Menghitung Total Pendapatan (*Revenues*) & Total Beban (*Expenses*) selama 1 tahun fiskal.
  2. Otomatis memindahkan Laba/Rugi Bersih (*Net Income*) ke rekening **Laba Ditahan / Retained Earnings (COA 30201)** di kelompok Ekuitas.
  3. Memisahkan saldo akun riil (Aset, Kewajiban, Ekuitas) untuk menjadi Saldo Awal di tahun fiskal baru.

---

## 3. Matriks Kepatuhan Regulasi Indonesia (OJK & Kemenkop UKM)

| Regulasi | Persyaratan Utama | Implementasi di CBS Core Kita |
|:---|:---|:---|
| **POJK TI BPR** | User Access Control, Immutable Audit Logs, Disaster Recovery | 7 Role RBAC, Log Audit Trail `staff_user_id` & IP, Quadlet Podman container. |
| **POJK Kualitas Aset & PPAP** | Klasifikasi Kolektibilitas Kredit 1 (Lancar) s/d 5 (Macet) & Pembentukan Penyisihan | Modul Integrasi SLIK OJK & Kolom Kolektibilitas `loan_schedules`. |
| **Kemenkop UKM (BMT/KSP)** | SAK ETAP / SAK Syariah (Akad Wadiah, Mudharabah, Murabahah) | Engine Jurnal Double-Entry Multi-Akad & Schedule Generator Murabahah Margin. |
| **PPh Pasal 4 ayat 2** | Pajak Bunga Tabungan/Deposito 20% | Parameterized Tax Deduction Engine saat EOM. |
