# CBS Core — Project Handoff & Single Source of Truth (SSOT)

> **IMPORTANT FOR AI AGENTS & DEVELOPERS:** Read this document completely before touching any code. This document contains explicit architecture rules, repository constraints, security protocols, database schemas, and active task states.

---

## 1. Executive Summary & Market Positioning

**CBS Core** is a modern, high-performance, cloud-native Core Banking System designed specifically for **BPR/BPRS (Bank Perekonomian Rakyat / Syariah)** and **BMT / Koperasi Simpan Pinjam (KSP)** in Indonesia.

### Why BPR/BPRS & BMT/Koperasi?
- **Operational Reality:** Commercial banks require complex SWIFT/RTGS integrations, FX derivatives, and global treasury modules. BPR/BPRS and BMT/Koperasi focus on core retail banking: savings (tabungan/simpanan), time deposits (deposito/mudharabah), loans/financing (kredit/pembiayaan), and COA/General Ledger management.
- **Syariah Dual-Mode Capability:** Architecture supports conventional interest-based models as well as Syariah contracts (*Wadiah*, *Mudharabah*, *Murabahah*, *Musyarakah*).
- **Compliance Alignment:** Aligned with OJK regulations (POJK No. 1 Tahun 2024, POJK TI BPR, SLIK reporting readiness), UU No. 27 Tahun 2022 (UU PDP), and Kemenkop UKM standards (SAK ETAP / SAK EP / SAK Syariah).

---

## 2. Strict Rules of Engagement for AI Models

> [!IMPORTANT]
> **RULE #1: GIT PUSH TO GITHUB IS AUTHORIZED FOR CI/CD**
> The GitHub repository `github.com/nferdazel/cbs-core` is connected to the production VPS webhook deployment pipeline. Always run `go test -v ./...` and verify clean builds in `apps/api` and `apps/web` before pushing to `origin main`.

1. **Architecture Guardrails:** Maintain Clean Architecture in `apps/api/internal/`: `domain` (interfaces/models, NO DB imports) → `repository/postgres` → `service` → `handler/http`.
2. **Double-Entry Accounting Invariant:** Every financial movement MUST balance: $\sum \text{Debit} = \sum \text{Credit}$. Never bypass double-entry validation.
3. **Single Financial Rounding Function:** Use `utils.RoundMoney(val)` (Banker's Rounding IEEE 754 to exact Rupiah) for all monetary rounding. Never use arbitrary rounding methods.
4. **Concurrency Locking:** All balance-modifying database queries MUST use `SELECT FOR UPDATE` inside a single `sql.Tx` with lexicographical row locking on account numbers to prevent deadlocks.
5. **Verification Requirement:** Always run `go build ./...` and `go test -v ./...` inside `apps/api` before declaring any backend task complete.

---

## 3. Repository Architecture & Tech Stack

Monorepo managed via **Turborepo** + **pnpm workspaces** + **Go Workspaces** (Standardized pattern matching SDS & Skyward monorepos):

```
cbs-core/
├── apps/
│   ├── api/                 # Clean Architecture Go Core Banking REST API Server
│   │   ├── cmd/server/      # Application Entrypoint (main.go)
│   │   ├── internal/        # Domain, Service, Repository, Handler, Middleware, Utils
│   │   └── Dockerfile       # Production Multi-Stage Go Build (golang:1.26-alpine)
│   └── web/                 # Next.js 15 Fintech Backoffice Web Application
│       ├── src/app/         # App Router pages (Dashboard & /login)
│       └── Dockerfile       # Production Multi-Stage Node.js Build (node:22-alpine)
├── deploy/                  # Production VPS Deployment Assets & Automation
│   ├── deploy-vps.sh        # Differential Webhook Deployment Script
│   ├── cbs-api.container    # Podman Quadlet Container Unit (Port 8095)
│   ├── Caddyfile.cbs.qouver.com # Caddy Reverse Proxy & Domain Route Config
│   └── webhook.json         # GitHub HMAC Webhook Definition
├── packages/
│   ├── db-migrations/       # PostgreSQL Migrations (000001, 000002, 000003)
│   └── shared-types/        # TypeScript Definitions for Web-API Contracts
├── docs/                    # Architectural Specifications & Expert Audit Reports
├── docker-compose.yml       # Local PostgreSQL & Redis Infrastructure
├── Makefile                 # Developer CLI Automation Shortcuts
├── pnpm-workspace.yaml      # Monorepo Workspace Configuration
└── go.work                  # Go Workspace Configuration
```

### Stack Details
- **Backend:** Go 1.25+, `go-chi/chi/v5`, `shopspring/decimal`, `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `jackc/pgx/v5`
- **Frontend:** Next.js 15 (Standalone mode), React 19, TypeScript, Tailwind CSS
- **Database:** PostgreSQL 18 with row-level locking and `NUMERIC(28,4)` precision
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

### Authentication Architecture
- **Access Token:** JWT (HS256), 15-minute TTL, containing `uid`, `username`, `role`, `branch`, `sid`.
- **Refresh Token:** Cryptographically random 32-byte opaque token, stored as SHA-256 hash in `staff_sessions` table, 8-hour TTL (1 work shift). Automatic rotation on refresh.
- **Brute Force Defense:** Auto-lock account for 15 minutes after 5 consecutive failed login attempts.
- **Password Policy:** Minimum 8 characters; requires uppercase, lowercase, digit, and special character.

---

## 5. Database Schema & Migration Status

Database: PostgreSQL 18 in `qouver-postgres` container on VPS. Database Name: **`cbs`** (User: `cbs_app`).

Executed Migrations (`packages/db-migrations/`):

1. `000001_init_cbs_schema.up.sql`:
   - `chart_of_accounts`, `customers`, `accounts`, `journal_entries`, `journal_lines`, `idempotency_keys`, `maker_checker_requests`, `audit_logs`
2. `000002_staff_auth.up.sql`:
   - `staff_users`, `staff_sessions`, `system_config` (Seeded default 7 roles & Superadmin user)
3. `000003_loans_schema.up.sql`:
   - `loans`, `loan_schedules`

**Total Live Production Tables: 13 Tables ✅**

---

## 6. Live Production Infrastructure & Deployment Details

- **Production Server:** Rocky Linux 9.8 VPS at `43.133.148.191` (User: `sachiel`, Sudo: `REDACTED`)
- **Live Domains & Gateways:**
  - **Backoffice Web UI:** **`https://cbs.qouver.com`** (HTTP/2 200 OK ✅, proxied to Podman container `cbs-web` on port `3005`)
  - **API Gateway:** **`https://api.qouver.com/cbs/v1/*`** (HTTP/2 200 OK ✅, proxied to Podman container `cbs-api` on port `8095`)
- **Containers:** Podman Quadlet (`cbs-api.container` & `cbs-web.container`)
- **Reverse Proxy Routing (Caddy):** Automatic SSL via Let's Encrypt / Caddy Gateway
- **Continuous Deployment:** Script `/srv/qouver/cbs/scripts/deploy-vps.sh` triggered via GitHub Webhook on port `9000` (`/hooks/cbs-deploy`) or SSH `github-cbs`.

---

## 7. Current Project Status & Verified Achievements

### Completed ✅
- **26/26 Go Unit Tests PASSING PERFECTLY** (100% Green)
- Monorepo directory layout standardized to `apps/api` and `apps/web` (matching SDS & Skyward patterns).
- Single canonical financial rounding function `utils.RoundMoney(val)` (IEEE 754 Banker's Rounding to exact Rupiah).
- Banking Business Date & Batch Processing Engine (EOD, EOM, EOY Tutup Buku Akhir Tahun Closing Entries).
- Document & PDF Printable Generator Engine (`/api/v1/documents/*`) for Slip Setoran/Penarikan Teller, Surat Perjanjian Kredit / Akad Pembiayaan, and 58mm/80mm ESC/POS Thermal Receipts.
- Accounting Normal Balance Rules (Asset/Expense DEBIT +, Liability/Equity/Revenue CREDIT +) & Lexicographical Row Locking in `PostCompoundJournal` (100% Deadlock Elimination).
- Kolektibilitas OJK POJK No. 1 Tahun 2024 (Kol 1-5 matrix, Accrual vs Cash Basis, PPAP Rates, Credit Restructuring Engine).
- Live Production Deployment on VPS `43.133.148.191` under `https://cbs.qouver.com` with Caddy SSL and Podman Quadlet containers.
