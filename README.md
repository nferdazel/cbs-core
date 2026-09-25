# CBS Core — Core Banking System

CBS Core is a core banking system for Indonesian rural banks (**BPR** / **BPRS**),
covering conventional and syariah (UUS) business: tabungan, deposito, and
kredit/pembiayaan, on top of a strict double-entry general ledger with
maker-checker approval, EOD/EOM/EOY batch processing, PPAP/PPKA collectibility
and CKPN provisioning, and OJK reporting data. Design decisions are driven by
Indonesian banking regulation — notably **POJK No. 1/2024** (asset quality),
and **POJK No. 23/2024** with **SEOJK No. 16/2024** (OJK reporting). The
regulatory basis, product decisions, and open debt live in
[`docs/KEPUTUSAN.md`](docs/KEPUTUSAN.md); deployment prerequisites live in
[`docs/DEPLOY.md`](docs/DEPLOY.md). Not sure which document is authoritative for a
topic? Start at [`docs/INDEX.md`](docs/INDEX.md) (one topic, one canonical place).
Where code and docs disagree, the code and migrations are the source of truth.

## Repository layout

```
cbs-core/
├── apps/
│   ├── api/                    # Go REST API (module path: cbs-core/apps/core-api)
│   │   ├── cmd/server/         # API entrypoint
│   │   ├── cmd/backfill-name-tokens/
│   │   ├── cmd/reindex-customer-indexes/
│   │   └── internal/           # domain, repository, service, handler, middleware, config, crypto, ojkreport
│   └── web/                    # Next.js 15 backoffice (package name: web)
│       └── src/app/(dashboard)/  # rekening, transaksi, kredit, pembiayaan, deposito, ppap, laporan, ...
├── packages/
│   ├── db-migrations/          # 98 up-only PostgreSQL migrations (*.up.sql)
│   └── shared-types/           # @cbs/shared-types — shared TypeScript contracts
├── deploy/                     # Podman Quadlet units (cbs-api, cbs-web) and Caddy config
├── scripts/                    # migrate.sh, preflight.sh, check-secrets.sh, deploy.sh
├── docs/                       # peta dokumen: docs/INDEX.md
├── docker-compose.yml          # local Postgres + Redis (see caveats below)
├── Makefile
├── go.work                     # Go workspace (use ./apps/api), Go 1.25.0
├── pnpm-workspace.yaml         # pnpm workspace root
└── turbo.json
```

Toolchain as recorded in the repo:

| Component | Version | Source |
|---|---|---|
| Go | 1.25.0 | `apps/api/go.mod`, `go.work` (API image builder is `golang:1.26-alpine`) |
| Node | 22 (container image) | `apps/web/Dockerfile`; no `.nvmrc` or `engines` field is pinned |
| pnpm | 11.25.0 | `package.json` `packageManager` |
| PostgreSQL | 16 locally (`docker-compose.yml`), **18 in production** | `docker-compose.yml`, `docs/DEPLOY.md` |
| Turbo | ^2.4.4 | `package.json` |

## Running locally

### 1. Database and migrations

`docker-compose.yml` starts PostgreSQL 16 and Redis. Two limitations are real
and should not be mistaken for a complete dev setup:

- It only applies migration `000001_init_cbs_schema.up.sql` (initdb mount). A
  fresh database from Compose is therefore **not** fully migrated.
- It starts a Redis container (`cbs-redis`) that **no application code uses**.

```bash
# Start Postgres only; Compose also defines an unused Redis service.
docker compose up -d postgres
# or: make docker-up   (starts both services)
```

Apply the full migration set with the repo's own script:

```bash
# Defaults: container qouver-postgres, database cbs, owner qouver.
# Override the three variables to match the Compose database above:
CBS_DB_CONTAINER=cbs-postgres CBS_DB_NAME=cbs_db CBS_DB_OWNER=cbs_user \
  scripts/migrate.sh

scripts/migrate.sh --dry-run            # list pending migrations (still needs a reachable DB)
scripts/migrate.sh --container NAME     # target another postgres container
scripts/migrate.sh --remote             # run psql on the production host over SSH
```

`scripts/migrate.sh` invokes `psql` inside a container through **`podman exec`**
(the script is written for the Podman host used in production) and tracks applied
files in `schema_migrations`, one transaction per file. On a Docker-only machine
you would run the same `*.up.sql` files with `psql` directly, or use a Podman
container. Migrations are **up-only**: there are no `*.down.sql` files and no
rollback path beyond restoring a backup.

### 2. API

Defaults in `apps/api/internal/config/config.go` match the database defined in
`docker-compose.yml` (`localhost:5432`) and listen on `:8080`.

```bash
cp .env.example .env    # then edit — see the warning below
make dev-api            # or: cd apps/api && go run cmd/server/main.go
```

Warning about `.env.example`: it sets `APP_ENV=production` and does **not**
define `ENCRYPTION_MASTER_KEY`. With `APP_ENV=production` the API refuses to
start unless `ENCRYPTION_MASTER_KEY` (and a non-empty `JWT_SECRET`) are present.
For local development set `APP_ENV=development` and provide a base64-encoded
32-byte `ENCRYPTION_MASTER_KEY` if you need customer endpoints.

- Health check: `curl http://localhost:8080/healthz`
- API routes are under `/api/v1/...`

### 3. Web backoffice

`apps/web` is a pnpm workspace package. It talks to the API through a Next.js
same-origin rewrite (`/api/:path*` → `INTERNAL_API_URL`, default
`http://cbs-api:8080`), so set that variable for a local API.

```bash
pnpm install
INTERNAL_API_URL=http://localhost:8080 make dev
# or: cd apps/web && INTERNAL_API_URL=http://localhost:8080 pnpm dev
```

The web app listens on `http://localhost:3000`.

### Ports

| Service | Local | Production (host → container) |
|---|---|---|
| API | `8080` | `localhost:8095` → `8080` |
| Web | `3000` | `localhost:3005` → `3000` |
| PostgreSQL | `5432` | container-internal, behind a reverse proxy |

## Tests

```bash
cd apps/api && go test ./...   # or: make test-api  (runs with -v)
```

Unit tests need no database and pass with `go test ./...`.

Integration tests are skipped unless `CBS_TEST_DB_DSN` is set. Point it at a
disposable, migrated database (never production):

```bash
# Use the local database credential from docker-compose.yml.
CBS_TEST_DB_DSN='postgres://cbs_user:<password>@localhost:5432/cbs_db?sslmode=disable' \
  go test ./...
```

Known caveat: when all integration tests share one database,
`TestIntegrasiOJKFormDaftarDanNPL` can fail because it asserts a bank-wide NPL
ratio exactly while other tests insert loans into the same database; it passes
in isolation. This is a test-isolation weakness, not a feature collision.

## Status

This is a working system in active use, but it is **not** "enterprise-grade" or
"production-ready" by assertion: there is no CI in the repository, and the open
items below are release gates. Verify current status against
[`docs/KEPUTUSAN.md`](docs/KEPUTUSAN.md) before relying on any claim.

**Implemented and exercised**

- Double-entry ledger engine (`Σ debits = Σ credits`) with `shopspring/decimal`
  arithmetic, explicit SQL (no ORM), and PostgreSQL row locking.
- Customer (CIF) management with field-level AES-256-GCM envelope encryption and
  a blind index for search; bcrypt passwords; httpOnly session cookies with CSRF
  double-submit; RBAC and branch scoping; login rate limiting.
- Accounts (savings/current/GL), teller transactions with idempotency keys,
  time deposits, conventional loans and syariah financing mapping, maker-checker
  workflow, EOD/EOM/EOY batch, PPAP/PPKA collectibility rules, CKPN, collateral,
  OJK report data, and an append-only `audit_logs`.
- The migration chain from zero is clean and idempotent, and the API boots
  against a migrated schema and serves `/healthz` and login.

**Present but not real integrations**

- `POST /api/v1/integrations/slik/check` and `/dukcapil/verify` exist, but are
  backed by **mock** gateways (`NewMockSLIKGateway`, `NewMockDukcapilGateway`).
  There is no live Dukcapil or SLIK connection.
- Document endpoints (deposit/withdrawal slips, loan agreement, thermal receipt,
  passbook) return **HTML print views**, not server-rendered PDF. There is no PDF
  printer integration.

**Known gaps / not finished**

- Per-role transaction limits and approval thresholds are **not in effect**: the
  `limit.*` config keys are never seeded, so defaults apply to every role. This
  is flagged as a release blocker in `docs/KEPUTUSAN.md`.
- CKPN production parameters (`ckpn.pd.*`, `ckpn.lgd_frac`) are unset, so credit
  computations fail with a clear error instead of producing numbers; the
  PD/LGD methodology needs bank data.
- The COA → OJK account mapping is still a draft, so reports are not ready for
  APOLO submission.
- Backups exist on the production host only (no offsite copy); recovery was
  drilled once, not on a schedule.

**Switches intentionally left off** (see `docs/KEPUTUSAN.md`)

`ppap.collateral.enabled`, `ckpn.enabled`, `loan.restructure.loss.enabled`, the
Pasal 23 (LPS placement) module, and password expiry
(`auth.password_expiry_days = 0`).

## Contributing

- Comments and docs are written in **Indonesian**, and they explain *why* a
  decision was made, not just what the code does. Keep that style.
- **Migrations are up-only.** Add a new `NNNNNN_description.up.sql`; never edit a
  migration that has been applied, and never add `.down.sql`. Apply them with
  `scripts/migrate.sh`. A mistake in production is recovered from a backup.
- Run the tests affected by your change (`go test ./...` in `apps/api`).
- Never commit secrets, credentials, host names, or IP addresses. Install
  `scripts/check-secrets.sh` as a pre-commit hook, or run it manually.
- **Do not push without the owner's approval.** Pushing drives the production
  deploy, and migrations cannot be rolled back.
