# Core Banking System (CBS) Monorepo

Enterprise-grade, Future-Proof **Core Banking System (CBS)** featuring a strict **Double-Entry General Ledger**, CIF/Customer management, Account lifecycle, and an interactive Backoffice Dashboard.

---

## 🏛️ Architecture Overview

```
cbs-core/
├── apps/
│   ├── core-api/           # [Go] Clean Architecture REST API & Double-Entry Engine
│   │   ├── cmd/server/     # Server Entrypoint
│   │   └── internal/       # domain, repository, service, http handlers
│   └── backoffice-web/     # [Next.js 15] Operations Portal & Ledger Visualizer
│
├── packages/
│   ├── db-migrations/      # PostgreSQL DDL migrations (ACID ledger schema)
│   └── shared-types/       # TypeScript shared DTO contracts
│
├── docker-compose.yml      # PostgreSQL 16 & Redis local environment
├── Makefile                # Fast developer task runner
└── go.work                 # Go Workspace
```

---

## 🚀 Quick Start

### 1. Start Database & Cache
```bash
docker compose up -d
# or make docker-up
```

### 2. Run Go Backend API (`:8080`)
```bash
make dev-api
# or: cd apps/core-api && go run cmd/server/main.go
```

### 3. Run Next.js Backoffice Portal (`:3000`)
```bash
make dev-web
# or: pnpm --filter backoffice-web dev
```

### 4. Run Double-Entry Unit Tests
```bash
make test-api
# or: cd apps/core-api && go test -v ./...
```

---

## 💎 Key Features Built

- **Strict Double-Entry Ledger Engine:** Every financial transaction requires $\Sigma \text{Debits} = \Sigma \text{Credits}$ with arbitrary-precision arithmetic (`shopspring/decimal`).
- **PostgreSQL Row Locking & Optimistic Concurrency:** Protects account balances against race conditions and overdrafts with `SELECT ... FOR UPDATE` & versioning.
- **Idempotency Ready:** Financial API endpoints enforce unique idempotency keys to prevent duplicate transactions during network retries.
- **CIF & Account Management:** Full lifecycle support for customer identification files, savings, and internal GL accounts.
