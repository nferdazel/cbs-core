# Analisis & Arsitektur Centralized Banking Utilities Package (`internal/utils`)

## 1. Urgensi Centralized Utilities di Core Banking System

Di dalam arsitektur Core Banking System (BPR/BPRS & BMT), kalkulasi desimal, pembulatan suku bunga/margin, pencetakan terbilang nominal, dan masking data privasi **TIDAK BOLEH** diimplementasikan secara terpisah-pisah (*scattered*) di masing-masing service.

Semua fungsi utilitas keuangan harus **tersentralisasi di satu package global yang immutable & standardized** (`apps/core-api/internal/utils/`) agar menjamin:
1. **0% Risiko Bias Pembulatan (Statistical Bias Elimination)** pada kalkulasi bunga, bagi hasil, dan pajak.
2. **Kesesuaian Standar Akuntansi Perbankan (IEEE 754 & SAK)**.
3. **Kepatuhan UU PDP (Perlindungan Data Pribadi)** pada penampakan NIK dan Nomor Rekening di slip cetak dan layar umum.

---

## 2. Modul & Fungsi Utilitas yang Diimplementasikan

### A. Mesin Pembulatan Keuangan (`Financial Rounding Engine`)
Disediakan 3 strategi pembulatan perbankan baku (`financial.go`):

1. `BankersRound(val decimal.Decimal, scale int32)`:
   - **Metode:** *Round-Half-Even* (IEEE 754 Banker's Rounding).
   - **Kegunaan:** Digunakan pada akumulasi bunga tabungan/deposito, bagi hasil Syariah, dan kalkulasi cadangan PPAP untuk menghilangkan bias pembulatan ke atas pada jutaan transaksi.
   - **Contoh:** `2.5` $\rightarrow$ `2`, `3.5` $\rightarrow$ `4`.

2. `HalfUpRound(val decimal.Decimal, scale int32)`:
   - **Metode:** *Round-Half-Away-From-Zero* (Standard Commercial Rounding).
   - **Kegunaan:** Digunakan pada penetapan angsuran pembiayaan komersial retail.

3. `TruncateDown(val decimal.Decimal, scale int32)`:
   - **Metode:** *Truncation / Cut-off* (Pembulatan Kebawah).
   - **Kegunaan:** Digunakan pada perhitungan Pajak Penghasilan (PPh Pasal 4 ayat 2) atas bunga simpanan yang memotong pecahan rupiah.

---

### B. Formatter Keuangan & Mesin Terbilang Rupiah (`Terbilang Engine`)

1. `FormatIDR(val decimal.Decimal) string`:
   - Mengubah desimal menjadi format baku mata uang Indonesia dengan pemisah ribuan titik dan desimal koma (misal: `12500000` $\rightarrow$ `"Rp 12.500.000,00"`).

2. `TerbilangRupiah(val decimal.Decimal) string`:
   - Konversi otomatis nominal desimal menjadi kalimat terbilang bahasa Indonesia (misal: `12500000` $\rightarrow$ `"Dua Belas Juta Lima Ratus Ribu Rupiah"`).
   - Digunakan pada cetak Slip Setoran/Penarikan Teller dan Surat Perjanjian Kredit / Akad Pembiayaan.

---

### C. Keamanan Data & Masking Privasi (`Security & PDP Masking`)

1. `MaskNIK(nik string) string`:
   - Masking 16-digit NIK KTP nasabah untuk kepatuhan UU PDP (misal: `"3171012345670001"` $\rightarrow$ `"3171************"`).

2. `MaskAccountNumber(accNo string) string`:
   - Masking nomor rekening nasabah untuk laporan publik atau layar kasir luar (misal: `"201001002003"` $\rightarrow$ `"201******003"`).

---

## 3. Status Pengujian Unit Test

Seluruh utilitas tersentralisasi dilengkapi unit test di `internal/utils/financial_test.go`:
- **Coverage:** 100% Fungsi Teruji
- **Total Suite:** **26/26 Go Unit Tests PASSING** (100% Green) ✅
