# Panduan Deployment & Konfigurasi Domain (`cbs.qouver.com`)

**Target Server:** Rocky Linux 9.8 VPS / Container Engine (Podman / Docker)  
**Domain Utama:** `cbs.qouver.com`  
**SSL Certificate:** Automatic via Caddy / Let's Encrypt  

---

## 1. Arsitektur Port & Service Deployment

| Service Name | Internal Container Port | Exposed Host Port | Subdomain / Path Route |
| :--- | :---: | :---: | :--- |
| **Next.js Backoffice Web** | `3000` | `3000` | `cbs.qouver.com/` |
| **Go Core API Backend** | `8080` | `8080` | `cbs.qouver.com/api/*` |
| **PostgreSQL 16 Database** | `5432` | `5432` (Internal) | `localhost:5432` |
| **Redis Cache & Session** | `6379` | `6379` (Internal) | `localhost:6379` |

---

## 2. Konfigurasi Caddy Reverse Proxy (`Caddyfile`)

Caddy secara otomatis mengurus sertifikat SSL (HTTPS) dan pemetaan route untuk `cbs.qouver.com`:

```caddyfile
cbs.qouver.com {
    # Forward API requests to Go Core API Backend
    handle /api/* {
        reverse_proxy localhost:8080
    }

    # Forward documents & print requests to Core API
    handle /documents/* {
        reverse_proxy localhost:8080
    }

    # Forward web requests to Next.js Backoffice Web
    handle {
        reverse_proxy localhost:3000
    }

    # Security Headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "SAMEORIGIN"
        X-XSS-Protection "1; mode=block"
    }

    encode gzip zstd
}
```

---

## 3. Environment Variables Produksi (`.env.production`)

```env
# General
NODE_ENV=production
ENV=production
PORT=8080

# PostgreSQL Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=cbs_prod_user
DB_PASSWORD=YOUR_STRONG_SECURE_PASSWORD
DB_NAME=cbs_prod_db
DB_SSL_MODE=disable

# Security & Authentication
JWT_SECRET=YOUR_SUPER_SECRET_HMAC_SHA256_KEY_MIN_32_CHARS
SESSION_TTL_HOURS=8
REFRESH_TOKEN_TTL_DAYS=7

# Backoffice Web Frontend
NEXT_PUBLIC_API_BASE_URL=https://cbs.qouver.com/api/v1
```

---

## 4. Langkah Eksekusi Deploy / Webhook Auto-Deploy

Saat webhook GitHub menerima push pada branch `main`:

```bash
# 1. Pull latest code from GitHub
git pull origin main

# 2. Build & Restart Containers via Docker Compose / Podman
docker compose -f docker-compose.prod.yml up -d --build

# 3. Running Database Migrations
golang-migrate -path packages/db-migrations/ -database "postgres://cbs_prod_user:PASSWORD@localhost:5432/cbs_prod_db?sslmode=disable" up
```

---

## 5. Status Deployment
* **GitHub Repository:** `git@github.com:nferdazel/cbs-core.git` ✅ (Pushed)
* **Branch:** `main` ✅
* **Passing Unit Tests:** **26/26 Green** ✅
