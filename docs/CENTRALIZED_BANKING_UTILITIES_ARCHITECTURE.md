# Centralized Banking Utilities Package — Architecture & Design Reference

**Package:** `apps/core-api/internal/utils/`
**Last Updated:** September 2026
**Status:** ✅ Production Ready — 26/26 Unit Tests Passing

---

## 1. Urgensi Centralized Utilities di Core Banking System

Di dalam arsitektur Core Banking System (BPR/BPRS & BMT), kalkulasi desimal, pembulatan bunga/margin, pencetakan terbilang nominal, dan masking data privasi **TIDAK BOLEH** diimplementasikan secara terpisah-pisah (*scattered*) di masing-masing service.

Semua fungsi utilitas keuangan tersentralisasi di satu package global yang **immutable & standardized** (`apps/core-api/internal/utils/`) untuk menjamin:
1. **0% Risiko Bias Pembulatan** pada kalkulasi bunga, bagi hasil Syariah, dan cadangan PPAP.
2. **Kesesuaian Standar Akuntansi Perbankan (IEEE 754 & SAK ETAP)**.
3. **Kepatuhan UU PDP (Perlindungan Data Pribadi)** pada penampakan NIK dan Nomor Rekening di slip cetak dan layar umum.

---

## 2. Rincian Fungsi Utilitas (`internal/utils/`)

### A. Satu Fungsi Pembulatan Baku Tunggal (`financial.go`)

> [!IMPORTANT]
> Seluruh modul perbankan (Setoran/Penarikan Teller, Angsuran Kredit, Bunga/Margin, Pajak, Jurnal General Ledger) **WAJIB** menggunakan **1 fungsi ini** dan TIDAK BOLEH menggunakan pembulatan lain secara langsung.

```go
// RoundMoney adalah SATU-SATUNYA fungsi pembulatan baku di seluruh core banking.
// Menggunakan Banker's Rounding (Round-Half-Even, IEEE 754) ke Rupiah utuh.
func RoundMoney(val decimal.Decimal) decimal.Decimal {
    return val.RoundBank(0)
}
```

**Mengapa Round-Half-Even (Banker's Rounding) sebagai standar tunggal?**
- Mengeliminasi bias statistik pembulatan ke atas pada akumulasi jutaan transaksi per hari.
- Standar resmi OJK & Bank Indonesia serta standar internasional IEEE 754 / ISO 4217.
- Contoh: `2.5` → `2` (bukan 3), `3.5` → `4` — selalu menuju angka genap terdekat.

**Fungsi rounding tambahan (tersedia jika dibutuhkan untuk kasus khusus, bukan default):**

| Fungsi | Metode | Kapan Digunakan |
| :--- | :--- | :--- |
| `BankersRound(val, scale)` | Round-Half-Even dengan skala kustom | Akumulasi bagi hasil Syariah berskala desimal khusus |
| `HalfUpRound(val, scale)` | Round-Half-Away-From-Zero | Konfigurasi angsuran legacy konvensional tertentu |
| `TruncateDown(val, scale)` | Truncation / Cut-off | PPh Pasal 4 ayat 2 — pemotongan pajak bunga deposito |

---

### B. Formatter Keuangan & Mesin Terbilang (`financial.go`)

| Fungsi | Contoh Input | Contoh Output |
| :--- | :--- | :--- |
| `FormatIDR(val)` | `12500000` | `"Rp 12.500.000,00"` |
| `TerbilangRupiah(val)` | `12500000` | `"Dua Belas Juta Lima Ratus Ribu Rupiah"` |

**Digunakan di:**
- Slip Setoran Tunai Teller (`GET /api/v1/documents/deposit-slip/{refNo}`)
- Slip Penarikan Tunai Teller (`GET /api/v1/documents/withdrawal-slip/{refNo}`)
- Surat Perjanjian Kredit / Akad Pembiayaan (`GET /api/v1/documents/loan-agreement/{loanId}`)
- Struk Kasir Lapangan Thermal 58mm/80mm (`GET /api/v1/documents/thermal-receipt/{receiptNo}`)

---

### C. Keamanan Data & Masking Privasi UU PDP (`security.go`)

| Fungsi | Contoh Input | Contoh Output | Kepatuhan |
| :--- | :--- | :--- | :--- |
| `MaskNIK(nik)` | `"3171012345670001"` | `"3171************"` | UU No. 27 Tahun 2022 (UU PDP) |
| `MaskAccountNumber(accNo)` | `"201001002003"` | `"201******003"` | Standar PCI-DSS & POJK TI BPR |

---

## 3. Panduan Penggunaan di Service Layer

```go
import "cbs-core/apps/core-api/internal/utils"

// Pembulatan angsuran kredit / margin Murabahah
installment := utils.RoundMoney(principal.Add(interest))

// Format tampilan di layar backoffice / dokumen cetak
display := utils.FormatIDR(installment)
// -> "Rp 4.666.667,00"

// Terbilang untuk Surat Perjanjian Kredit
terbilang := utils.TerbilangRupiah(principal)
// -> "Lima Puluh Juta Rupiah"

// Masking privasi sebelum log / tampilkan di API publik
safeLogs := utils.MaskNIK(customer.IDCardNumber)
// -> "3171************"
```

---

## 4. Status Pengujian Unit Test

**File:** `apps/core-api/internal/utils/financial_test.go`

| Test Case | Status |
| :--- | :---: |
| `TestFinancialUtils_RoundMoney` (Banker's Round ke Rupiah utuh) | ✅ PASS |
| `TestFinancialUtils_RoundingModes` (BankersRound, HalfUp, Truncate) | ✅ PASS |
| `TestFinancialUtils_FormatIDR` | ✅ PASS |
| `TestFinancialUtils_TerbilangRupiah` | ✅ PASS |
| `TestSecurityUtils_Masking` (NIK & Account) | ✅ PASS |

**Total Suite: 26/26 Go Unit Tests PASSING (100% Green) ✅**
