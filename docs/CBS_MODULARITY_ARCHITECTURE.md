# CBS Core — Enterprise Modularity & Tiered Approval Architecture

## 1. Visi Modulitas: Parameter-Driven CBS

Di sistem perbankan enterprise (sekelas Temenos, Mambu, atau Thought Machine), **tidak ada aturan bisnis, produk, atau tier approval yang di-hardcode dalam binary Go/API**.

Semua perbedaan kebijakan antar-lembaga (BPR A vs BPR B vs BMT Syariah) ditangani via **4 Pilar Modulitas**:

```
 ┌─────────────────────────────────────────────────────────┐
 │                   CBS CORE API                          │
 └────────────────────────────┬────────────────────────────┘
                              │
  ┌───────────────────────────┼───────────────────────────┐
  │                           │                           │
  ▼                           ▼                           ▼
[Pilar 1: Product Engine]  [Pilar 2: Tiered Approval]  [Pilar 3: Strategy Calculators]
- Flexible Account Types   - Multi-Level Workflow      - Flat / Annuity / Sliding
- COA Mapping Matrix       - Configurable Tiers        - Murabahah / Mudharabah
- Fee & Interest Rules     - Role-Based Escalation     - Custom Penalty Plugins
```

---

## 2. Pilar 1: Parameter-Driven Banking Product Engine

Daripada men-hardcode tipe akun (seperti `SAVINGS`, `CHECKING`), CBS menyediakan **Banking Product Engine** dinamis di database.

### Database Schema Proposal (`banking_products`)
```sql
CREATE TABLE banking_products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(32) UNIQUE NOT NULL,      -- e.g. TAB-UTAMA, DEP-MUDHARABAH-6M, KRD-MIKRO
    name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL,         -- SAVINGS, DEPOSIT, LOAN, FINANCING
    is_syariah BOOLEAN NOT NULL DEFAULT FALSE,
    syariah_contract VARCHAR(32),          -- WADIAH, MUDHARABAH, MURABAHAH, NONE
    
    -- Dynamic COA Mapping (Link ke General Ledger)
    asset_gl_coa_id UUID REFERENCES chart_of_accounts(id),     -- e.g. Piutang Kredit / Kas
    liability_gl_coa_id UUID REFERENCES chart_of_accounts(id), -- e.g. Simpanan Nasabah
    income_gl_coa_id UUID REFERENCES chart_of_accounts(id),    -- e.g. Pendapatan Bunga / Margin
    expense_gl_coa_id UUID REFERENCES chart_of_accounts(id),   -- e.g. Beban Bunga / Bagi Hasil
    
    -- Configurable Product Attributes (JSONB)
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Example attributes JSONB:
    -- {
    --   "min_balance": 50000,
    --   "interest_rate_pa": 3.5,
    --   "nisbah_customer_percent": 70.0,
    --   "admin_fee_monthly": 5000,
    --   "early_closure_penalty": 25000
    -- }
    
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Hasilnya:**
- BPR A ingin buat "Tabungan Simpel" (Bunga 0%, Min saldo Rp 10.000) → **Cukup 1 baris di DB**.
- BMT B ingin buat "Simpanan Mudharabah 70:30" → **Cukup 1 baris di DB**.

---

## 3. Pilar 2: Tiered Approval Workflow Matrix

Perusahaan beda skala punya kebijakan persetujuan yang sangat berbeda. CBS menangani ini dengan **Dynamic Tiered Approval Engine**.

### Database Schema Proposal (`approval_policies`)
```sql
CREATE TABLE approval_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    module VARCHAR(64) NOT NULL,           -- TRANSACTIONS, LOANS, CUSTOMERS, COA
    action_type VARCHAR(64) NOT NULL,      -- DEPOSIT, WITHDRAWAL, LOAN_ORIGINATION, REVERSAL
    min_amount NUMERIC(28,4) NOT NULL DEFAULT 0,
    max_amount NUMERIC(28,4) NOT NULL,     -- 0 = Unlimited
    
    -- Tiered Steps Configuration (JSONB Array)
    approval_steps JSONB NOT NULL,
    -- Example JSONB:
    -- [
    --   {"step": 1, "role": "SUPERVISOR", "auto_assign": true},
    --   {"step": 2, "role": "CREDIT_MANAGER", "auto_assign": true},
    --   {"step": 3, "role": "DIRECTOR", "condition": "amount > 500000000"}
    -- ]
    
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Cara Kerja Evaluasi Tiered Approval:
1. Teller input penarikan Rp 750 Juta.
2. Engine mencocokkan nominal dengan `approval_policies`:
   - Tier 1: Needs `SUPERVISOR` approval.
   - Tier 2: Needs `CREDIT_MANAGER` / `BRANCH_MANAGER` approval.
   - Tier 3: Needs `DIRECTOR` approval.
3. Transaksi masuk ke antrean `maker_checker_requests` dengan `current_step = 1`.
4. Setelah Supervisor approve → `current_step = 2` (Esktalasi ke Branch Manager).
5. Setelah Director approve → Jurnal Keuangan diposting secara **Atomic**.

---

## 4. Pilar 3: Pattern Strategi (Go Interface Calculators)

Di level kode Go (`apps/core-api/internal/domain/`), kita menerapkan **Strategy Pattern**:

```go
// Generic Loan Calculator Interface
type LoanCalculator interface {
    CalculateSchedule(principal decimal.Decimal, rateOrMargin decimal.Decimal, termMonths int, startDate time.Time) ([]LoanSchedule, decimal.Decimal, decimal.Decimal)
}

// Implementasi Terpisah (Plugin-friendly):
// - FlatLoanCalculator
// - AnnuityLoanCalculator
// - SlidingLoanCalculator
// - MurabahahMarginCalculator
// - MudharabahProfitShareCalculator
```

Jika lembaga butuh perhitungan angsuran tipe baru (misalnya *Kredit Bunga Sliding* atau *Akad Istishna'*), **kita cukup buat 1 struct Go baru yang mengimplementasikan `LoanCalculator`**, tanpa perlu menyentuh core ledger transaction engine!

---

## 5. Summary Keunggulan Arsitektur Ini

| Komponen | Cara CBS Meng-handle Variasi Kebijakan |
|:---|:---|
| Tipe Akun & Produk | Dynamic `banking_products` table + JSONB attributes |
| COA & Jurnal Rules | Dynamic COA Mapping per-product |
| Maker-Checker & Approvals | Dynamic Tiered Approval Policy Engine (`approval_policies`) |
| Perhitungan Bunga/Margin | Strategy Pattern Interfaces (`LoanCalculator`, `FeeCalculator`) |
| Batas Limit Role Staff | Configurable via `system_config` per-branch / per-role |
