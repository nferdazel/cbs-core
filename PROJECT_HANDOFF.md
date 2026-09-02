# CBS Core — Project Handoff & Single Source of Truth (SSOT)

> **IMPORTANT FOR AI AGENTS & DEVELOPERS:** Read this document completely before touching any code. This document contains explicit architecture rules, repository constraints, security protocols, database schemas, and active task states.

---

## 1. Executive Summary & Market Positioning

**CBS Core** is a modern, high-performance, cloud-native Core Banking System designed specifically for **BPR/BPRS (Bank Perekonomian Rakyat / Syariah)** and **BMT / Koperasi Simpan Pinjam (KSP)** in Indonesia.

### Why BPR/BPRS & BMT/Koperasi?
- **Operational Reality:** Commercial banks require complex SWIFT/RTGS integrations, FX derivatives, and global treasury modules. BPR/BPRS and BMT/Koperasi focus on core retail banking: savings (tabungan/simpanan), time deposits (deposito/mudharabah), loans/financing (kredit/pembiayaan), and COA/General Ledger management.
- **Syariah Dual-Mode Capability:** Architecture supports conventional interest-based models as well as Syariah contracts (*Wadiah*, *Mudharabah*, *Murabahah*, *Musyarakah*).
- **Compliance Alignment:** Aligned with OJK regulations (POJK TI BPR, SLIK reporting readiness) and Kemenkop UKM standards (SAK ETAP / SAK EP / SAK Syariah).

---

## 2. Strict Rules of Engagement for AI Models

> [!CAUTION]
> **RULE #1: DO NOT GIT PUSH TO GITHUB**
> The GitHub repository `github.com/nferdazel/cbs-core` is PUBLIC. Code changes must be tested and optimized locally. **Do NOT run `git push`** unless the user explicitly requests it.

1. **Architecture Guardrails:** Maintain Clean Architecture in `apps/core-api/internal/`: `domain` (interfaces/models, NO DB imports) → `repository/postgres` → `service` → `handler/http`.
2. **Double-Entry Accounting Invariant:** Every financial movement MUST balance: $\sum \text{Debit} = \sum \text{Credit}$. Never bypass double-entry validation.
3. **Concurrency Locking:** All balance-modifying database queries MUST use `SELECT FOR UPDATE` inside a single `sql.Tx` with lexicographical row locking on account numbers to prevent deadlocks.
4. **Verification Requirement:** Always run `go build -v ./...` and `go test -v ./...` inside `apps/core-api` before declaring any task complete.

---

## 3. Repository Architecture & Tech Stack

Monorepo managed via **Turborepo** + **pnpm workspaces** + **Go Workspaces**:

```
cbs-core/
├── apps/
│   ├── core-api/               # Go (Golang) REST API Backend
│   │   ├── cmd/server/main.go  # Dependency Injection & Server entrypoint
│   │   └── internal/
│   │       ├── config/         # Environment variables configuration
│   │       ├── domain/         # Models, enums, permissions, interfaces
│   │       ├── handler/http/   # Chi Router HTTP handlers & responses
│   │       ├── middleware/     # JWT Auth & Permission Guard middlewares
│   │       ├── repository/     # PostgreSQL SQL implementations
│   │       └── service/        # Business logic & Double-entry transaction engine
│   └── backoffice-web/         # Next.js 15 (App Router, TS, Tailwind, shadcn/ui)
│       └── src/app/            # Operations Terminal & General Ledger Dashboard
├── packages/
│   ├── db-migrations/          # PostgreSQL DDL migrations (.sql)
│   └── shared-types/           # TypeScript DTOs shared across packages
├── deploy/                     # Podman Quadlet container definitions & deploy scripts
└── scripts/                    # Maintenance & local utility scripts
```

### Stack Details
- **Backend:** Go 1.22+, `go-chi/chi/v5`, `shopspring/decimal`, `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `jackc/pgx/v5`
- **Frontend:** Next.js 15 (Standalone mode), React 19, TypeScript, Tailwind CSS
- **Database:** PostgreSQL 16/18 with row-level locking and `NUMERIC(28,4)` precision
- **Deployment:** Rocky Linux 9.8, Podman rootless Quadlet systemd services, Caddy Reverse Proxy with SSL

---

## 4. RBAC & Security Matrix

CBS uses a **Fixed 7-Role Staff Hierarchy** with granular permissions:

| Role | Primary Function | Default Limit |
|:---|:---|:---:|
| `SUPERADMIN` | Complete system control & Superuser setup | Unlimited |
| `ADMIN` | Staff management, COA setup, System config | Configurable |
| `SUPERVISOR` | Maker-Checker approvals, reversals, credit committee approval | Rp 500.000.000 |
| `TELLER` | Over-the-counter deposit/withdraw/transfer, account opening | Rp 50.000.000 |
| `CS` | Customer registration (CIF), customer data updates | No financial ops |
| `AO` (Account Officer) | Loan/Financing origination (Kredit/Pembiayaan), field collections | Rp 250.000.000 |
| `AUDITOR` | Read-only compliance & export capabilities across all modules | Read-only |

### Permissions Structure
- **Users:** `users:create`, `users:read`, `users:update`, `users:delete`
- **Customers (CIF):** `customers:create`, `customers:read`, `customers:update`
- **Accounts:** `accounts:open`, `accounts:read`, `accounts:freeze`, `accounts:close`
- **Transactions:** `transactions:deposit`, `transactions:withdraw`, `transactions:transfer`, `transactions:reverse`
- **Loans & Financing (BPR/BMT):** `loans:apply`, `loans:read`, `loans:approve`
- **Collections:** `collections:input`
- **Maker-Checker:** `maker_checker:approve`, `maker_checker:reject`
- **Ledger & COA:** `ledger:read`, `coa:manage`
- **Audit & System:** `audit_logs:read`, `reports:export`, `system:config`

### Authentication Architecture
- **Access Token:** JWT (HS256), 15-minute TTL, containing `uid`, `username`, `role`, `branch`, `sid`.
- **Refresh Token:** Cryptographically random 32-byte opaque token, stored as SHA-256 hash in `staff_sessions` table, 8-hour TTL (1 work shift). Automatic rotation on refresh.
- **Brute Force Defense:** Auto-lock account for 15 minutes after 5 consecutive failed login attempts.
- **Password Policy:** Minimum 8 characters; requires uppercase, lowercase, digit, and special character.

---

## 5. Database Schema Overview

Located in `packages/db-migrations/`:

1. `000001_init_cbs_schema.up.sql`:
   - `chart_of_accounts` (COA Code, Name, Account Type: ASSET/LIABILITY/EQUITY/REVENUE/EXPENSE)
   - `customers` (CIF Number, Full Name, Identity Number / NIK, Phone, Address, Type: INDIVIDUAL/CORPORATE)
   - `accounts` (Account Number, Customer ID, COA ID, Balance, Currency, Status, Version for Optimistic Lock)
   - `journal_entries` (Entry Number, Transaction Date, Ref Number, Description, Created By)
   - `journal_lines` (Entry ID, Account ID, Debit, Credit)
   - `maker_checker_requests` (Request ID, Maker ID, Checker ID, Status: PENDING/APPROVED/REJECTED, Amount)
   - `audit_logs` (Action, Target, User ID, Role, IP Address, Metadata)

2. `000002_staff_auth.up.sql`:
   - `staff_users` (Employee ID, Username, Email, Password Hash, Role, Branch Code, Lock Status)
   - `staff_sessions` (Refresh Token Hash, IP, User Agent, Expires At, Revoked At)
   - `system_config` (Key-Value threshold store for Maker-Checker and Role Limits)
   - Seeded Superadmin: `superadmin` / `Admin@CBS2026!`

3. `000003_loans_schema.up.sql`:
   - `loans` (Loan Number, Customer ID, Disbursement Account, Type: Flat/Annuity/Murabahah/Mudharabah, Status, Plafond Principal, Rate/Margin, Total Payable, Term, AO ID, Approved By)
   - `loan_schedules` (Installment No, Due Date, Principal, Interest/Margin, Total, Payment Status: Pending/Paid/Overdue)

---

## 6. Infrastructure & Deployment Reference

- **Production VPS:** Rocky Linux 9.8 at `43.133.148.191` (User: `sachiel`, Sudo: `REDACTED`)
- **Containers:** Podman Quadlet (`cbs-api.container` on port 8095, `cbs-web.container` on port 3005)
- **Database:** Container `qouver-postgres` (PostgreSQL), DB Name `cbs`, User `cbs_app`
- **Reverse Proxy Routing (Caddy):**
  - `https://api.qouver.com/cbs/*` → Proxied to local port `8095` (Prefix stripped)
  - `https://cbs.qouver.com` → Proxied to local port `3005` (Backoffice UI)
- **Webhook CD:** Triggers `/srv/qouver/cbs/scripts/deploy-vps.sh` on port `9000` via endpoint `https://api.qouver.com/hooks/cbs-core`.

---

## 7. Current Project Status & Next Roadmap

### Completed ✅
- Clean Architecture Go Backend with Double-Entry Transaction Engine
- **19/19 Go Unit Tests PASS** (Double-entry balance, Auth service, Role-Permission checks, Loan Schedules, Trial Balance, Balance Sheet, Income Statement, SLIK OJK, Dukcapil NIK, EOD & EOY Closing Entries)
- Staff User Management & JWT + Session Auth with RBAC Middlewares (7 Roles: SUPERADMIN, ADMIN, SUPERVISOR, TELLER, CS, AO, AUDITOR)
- **Banking Business Date & Batch Processing Engine:**
  - `GET /api/v1/system/business-date` (System Business Date control)
  - `POST /api/v1/batch/eod` (End of Day closing & business date advancing)
  - `POST /api/v1/batch/eom` (End of Month admin fee & interest distribution)
  - `POST /api/v1/batch/eoy` (End of Year Tutup Buku Akhir Tahun & Closing Double-Entry Journal to Retained Earnings)
- **Loans & Financing Engine:** Origination, Repayment Schedule generator (Flat & Murabahah Margin), Approval, Disbursement, & Installment payments
- **Financial Statement Reports Engine:** Trial Balance (Neraca Saldo), Balance Sheet (Laporan Posisi Keuangan / Neraca), and Income Statement (Laporan Laba Rugi)
- **Mobile Collector / Field Collection Engine (`Jemput Bola`):** Mobile API for AO/Collectors doing daily market collections with geolocation & receipt logging
- **Third-Party Integration Gateway Layer:** SLIK OJK / CBAS Debtor check, Dukcapil NIK verification API, SMS & WhatsApp Notification Gateways
- **Maker-Checker Workflow:** Supervisor pending approval queue & approval/rejection HTTP handlers
- Database Schemas `000001`, `000002`, and `000003` written & verified
- Interactive Backoffice Dashboard & **Login Screen** (`/login`) in Next.js 15 (compiles cleanly in 2.8s)

### Next Action Items (Roadmap for BMT / BPR focus) 🚀
1. **Backoffice UI Reports Tab:** Render Financial Statements (Neraca & Laba Rugi) in Next.js Backoffice UI.
2. **Push to Production:** Trigger `git push` to deploy to VPS when ready for live launch.
