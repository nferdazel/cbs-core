#!/usr/bin/env bash
# Pemeriksaan pra-deploy CBS Core. READ-ONLY: tidak ada INSERT/UPDATE/DDL di sini,
# hanya SELECT. Tujuannya menangkap prasyarat yang, bila terlewat, membuat migrasi
# atau deploy gagal separuh jalan (lihat docs/DEPLOY.md).
#
# Pemakaian:
#   scripts/preflight.sh                 # periksa DB container di mesin ini
#   scripts/preflight.sh --remote        # periksa DB produksi lewat SSH (cara migrate.sh)
#   scripts/preflight.sh --container X   # nama container postgres (default: qouver-postgres)
#
# Variabel lingkungan (mengikuti scripts/migrate.sh, bukan nama baru):
#   CBS_DB_CONTAINER  nama container (default qouver-postgres)
#   CBS_DB_NAME       nama database   (default cbs)
#   CBS_DB_OWNER      role pemilik    (default qouver)
#   CBS_MIGRATIONS    direktori migrasi (default packages/db-migrations)
#   CBS_SSH_HOST      host SSH untuk --remote (atau scripts/.ssh-host)
#   CBS_SSH_USER      user SSH untuk --remote (default sachiel)
#   CBS_SSH_KEY       private key untuk --remote (default ~/.ssh/id_ed25519)

set -euo pipefail

CONTAINER="${CBS_DB_CONTAINER:-qouver-postgres}"
DB_NAME="${CBS_DB_NAME:-cbs}"
DB_OWNER="${CBS_DB_OWNER:-qouver}"
MIGRATIONS_DIR="${CBS_MIGRATIONS:-packages/db-migrations}"
APP_ROLE="cbs_app"

if [[ -z "${CBS_SSH_HOST:-}" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ -f "$SCRIPT_DIR/.ssh-host" ]]; then
    CBS_SSH_HOST="$(tr -d '[:space:]' < "$SCRIPT_DIR/.ssh-host")"
  fi
fi
SSH_HOST="${CBS_SSH_HOST:-}"
SSH_USER="${CBS_SSH_USER:-sachiel}"
SSH_KEY="${CBS_SSH_KEY:-$HOME/.ssh/id_ed25519}"
REMOTE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remote) REMOTE=true; shift ;;
    --container) CONTAINER="$2"; shift 2 ;;
    *) echo "opsi tidak dikenal: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
  echo "[GAGAL] direktori migrasi tidak ditemukan: $MIGRATIONS_DIR" >&2
  echo "        Perbaiki: jalankan dari akar repo, atau set CBS_MIGRATIONS." >&2
  exit 1
fi

if [[ "$REMOTE" == true && -z "$SSH_HOST" ]]; then
  echo "[GAGAL] mode --remote butuh CBS_SSH_HOST atau scripts/.ssh-host." >&2
  echo "        Perbaiki: isi scripts/.ssh-host dengan host VPS (mode 600, gitignored)." >&2
  exit 1
fi

psql_exec() {
  # Sama seperti scripts/migrate.sh: psql dijalankan di dalam container sebagai
  # owner objek, memakai socket lokal (password container tidak selalu bisa TCP).
  # Cabang eksplisit dipakai agar array kosong tetap aman di bawah `set -u`.
  if [[ "$REMOTE" == true ]]; then
    ssh -o ConnectTimeout=10 -i "$SSH_KEY" "${SSH_USER}@${SSH_HOST}" \
      podman exec -i -u postgres "$CONTAINER" \
      psql -v ON_ERROR_STOP=1 -U "$DB_OWNER" -d "$DB_NAME" "$@"
  else
    podman exec -i -u postgres "$CONTAINER" \
      psql -v ON_ERROR_STOP=1 -U "$DB_OWNER" -d "$DB_NAME" "$@"
  fi
}

psql_query() {
  local sql="$1"
  shift
  printf '%s\n' "$sql" | psql_exec "$@" -tA -f -
}

problems=0
fail() {
  problems=$((problems + 1))
  echo "[GAGAL] $1" >&2
  shift
  for line in "$@"; do
    echo "        $line" >&2
  done
}

# ── 1. Koneksi database ────────────────────────────────────────────────────────
# Pesan galat psql ditangkap agar bisa dilaporkan, bukan sekadar "command failed".
conn_err="$(psql_query "SELECT 1" -q 2>&1 >/dev/null || true)"
if [[ -n "$conn_err" ]]; then
  echo "[GAGAL] koneksi database tidak bisa dibuka." >&2
  echo "        container=$CONTAINER db=$DB_NAME user=$DB_OWNER remote=$REMOTE" >&2
  echo "        Perbaiki: pastikan container berjalan (podman ps -a | grep $CONTAINER)," >&2
  echo "                  set CBS_DB_CONTAINER bila namanya berbeda," >&2
  echo "                  atau periksa CBS_SSH_HOST/CBS_SSH_KEY untuk mode --remote." >&2
  echo "        Detail: $conn_err" >&2
  exit 1
fi

# ── 2. Role aplikasi cbs_app ───────────────────────────────────────────────────
role_exists="$(psql_query "SELECT 1 FROM pg_roles WHERE rolname = '$APP_ROLE'" -q)"
if [[ "$role_exists" != "1" ]]; then
  fail "role aplikasi '$APP_ROLE' belum ada." \
    "Migrasi 000012_app_role_grants akan gagal tanpa role ini." \
    "Perbaiki (jalankan sekali sebagai owner/superuser; ganti <rahasia>):" \
    "  podman exec -u postgres $CONTAINER psql -U $DB_OWNER -d $DB_NAME \\" \
    "    -c \"CREATE ROLE $APP_ROLE LOGIN PASSWORD '<rahasia>';\""
fi

has_migrations_table="$(psql_query "SELECT to_regclass('public.schema_migrations') IS NOT NULL" -q)"
has_system_config="$(psql_query "SELECT to_regclass('public.system_config') IS NOT NULL" -q)"

# ── 3. schema_migrations vs berkas migrasi di repo ─────────────────────────────
missing=()
if [[ "$has_migrations_table" != "t" ]]; then
  # Belum ada tabel pelacak: semua migrasi dianggap belum diterapkan.
  for file in "$MIGRATIONS_DIR"/*.up.sql; do
    [[ -e "$file" ]] || continue
    missing+=("$(basename "$file")")
  done
  fail "tabel schema_migrations belum ada; seluruh migrasi belum diterapkan." \
    "Perbaiki: jalankan scripts/migrate.sh (lokal) atau scripts/migrate.sh --remote (produksi)."
else
  applied="$(psql_query "SELECT filename FROM schema_migrations ORDER BY filename" -q)"
  for file in "$MIGRATIONS_DIR"/*.up.sql; do
    [[ -e "$file" ]] || continue
    name="$(basename "$file")"
    if ! printf '%s\n' "$applied" | grep -qxF "$name"; then
      missing+=("$name")
    fi
  done
  if (( ${#missing[@]} > 0 )); then
    fail "schema_migrations tertinggal ${#missing[@]} migrasi dari berkas di repo." \
      "Migrasi yang belum diterapkan:" \
      "${missing[@]}" \
      "Perbaiki: jalankan scripts/migrate.sh (lokal) atau scripts/migrate.sh --remote (produksi)."
  fi
fi

# ── 4. Baris system.business_date ──────────────────────────────────────────────
if [[ "$has_system_config" != "t" ]]; then
  fail "tabel system_config belum ada (migrasi dasar belum lengkap)." \
    "Perbaiki: selesaikan migrasi lebih dulu: scripts/migrate.sh (--remote untuk produksi)."
else
  business_date="$(psql_query "SELECT 1 FROM system_config WHERE key = 'system.business_date'" -q)"
  if [[ "$business_date" != "1" ]]; then
    fail "baris 'system.business_date' belum ada di system_config." \
      "Tanpa baris ini, semua pembatalan transaksi dianggap lintas hari dan wajib persetujuan pejabat." \
      "Perbaiki: jalankan tutup hari sekali lewat aplikasi, atau isi barisnya (tanggal format YYYY-MM-DD):" \
      "  podman exec -u postgres $CONTAINER psql -U $DB_OWNER -d $DB_NAME \\" \
      "    -c \"INSERT INTO system_config (key, value, description) VALUES ('system.business_date', CURRENT_DATE::text, 'Tanggal bisnis berjalan') ON CONFLICT (key) DO NOTHING;\""
  fi
fi

# ── 5. Owner skema public ──────────────────────────────────────────────────────
schema_owner="$(psql_query "SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'public'" -q)"
if [[ -n "$schema_owner" && "$schema_owner" != "$DB_OWNER" ]]; then
  fail "owner skema public adalah '$schema_owner', bukan '$DB_OWNER'." \
    "Migrasi DDL mengubah objek milik owner; owner yang salah merusak grant dan DDL." \
    "Perbaiki (sebagai superuser): ALTER SCHEMA public OWNER TO $DB_OWNER;"
fi

# ── Ringkasan ──────────────────────────────────────────────────────────────────
if (( problems > 0 )); then
  echo >&2
  echo "preflight GAGAL: $problems masalah ditemukan. Jangan lanjutkan migrasi/deploy." >&2
  exit 1
fi

echo "preflight OK: koneksi, role $APP_ROLE, migrasi, tanggal bisnis, dan owner skema siap."
