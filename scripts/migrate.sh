#!/usr/bin/env bash
# Menerapkan migrasi SQL ke database CBS secara aman.
#
# Karakteristik yang penting untuk perbankan:
#   - ON_ERROR_STOP=1: satu error menggagalkan seluruh file, tidak ada migrasi separuh jadi.
#   - Satu transaksi per file (-1): file yang gagal di-rollback utuh.
#   - Dijalankan sebagai owner tabel (default: qouver), karena DDL mengubah objek milik owner.
#
# Pemakaian:
#   scripts/migrate.sh                 # terapkan semua file .up.sql yang belum tercatat
#   scripts/migrate.sh --dry-run       # tampilkan file yang akan diterapkan
#   scripts/migrate.sh --remote        # jalankan psql di VPS lewat SSH (DB live ada di sana)
#   scripts/migrate.sh --container X   # nama container postgres (default: qouver-postgres)
#
# Variabel lingkungan:
#   CBS_DB_CONTAINER  nama container (default qouver-postgres)
#   CBS_DB_NAME       nama database   (default cbs)
#   CBS_DB_OWNER      role pemilik    (default qouver)
#   CBS_MIGRATIONS    direktori migrasi (default packages/db-migrations)
#   CBS_SSH_HOST      host SSH untuk --remote (wajib; lihat catatan di bawah)
#   CBS_SSH_USER      user SSH untuk --remote (default sachiel)
#   CBS_SSH_KEY       private key untuk --remote (default ~/.ssh/id_ed25519)
#
# Host SSH sengaja TIDAK punya nilai default di repo ini karena repo bersifat publik.
# Isi lewat variabel lingkungan CBS_SSH_HOST, atau simpan sekali di scripts/.ssh-host
# (berkas lokal, diabaikan git).

set -euo pipefail

CONTAINER="${CBS_DB_CONTAINER:-qouver-postgres}"
DB_NAME="${CBS_DB_NAME:-cbs}"
DB_OWNER="${CBS_DB_OWNER:-qouver}"
MIGRATIONS_DIR="${CBS_MIGRATIONS:-packages/db-migrations}"
if [[ -z "${CBS_SSH_HOST:-}" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ -f "$SCRIPT_DIR/.ssh-host" ]]; then
    CBS_SSH_HOST="$(tr -d '[:space:]' < "$SCRIPT_DIR/.ssh-host")"
  fi
fi
SSH_HOST="${CBS_SSH_HOST:?isi CBS_SSH_HOST atau scripts/.ssh-host dengan host SSH VPS}"
SSH_USER="${CBS_SSH_USER:-sachiel}"
SSH_KEY="${CBS_SSH_KEY:-$HOME/.ssh/id_ed25519}"
DRY_RUN=false
REMOTE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --remote) REMOTE=true; shift ;;
    --container) CONTAINER="$2"; shift 2 ;;
    *) echo "opsi tidak dikenal: $1" >&2; exit 2 ;;
  esac
done

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
  echo "direktori migrasi tidak ditemukan: $MIGRATIONS_DIR" >&2
  exit 1
fi

remote_prefix=()
if [[ "$REMOTE" == true ]]; then
  remote_prefix=(ssh -o ConnectTimeout=10 -i "$SSH_KEY" "${SSH_USER}@${SSH_HOST}")
fi

psql_exec() {
  # -u postgres menjalankan psql sebagai owner objek di dalam container.
  # Kolom password container tidak selalu bisa login lewat TCP, jadi socket lokal dipakai.
  "${remote_prefix[@]}" podman exec -i -u postgres "$CONTAINER" \
    psql -v ON_ERROR_STOP=1 -U "$DB_OWNER" -d "$DB_NAME" "$@"
}

# psql_query mengirim SQL lewat stdin. Dipakai untuk query dengan tanda kurung/spasi,
# karena meneruskan lewat -c saat --remote akan diurai ulang oleh shell SSH.
psql_query() {
  local sql="$1"
  shift
  printf '%s\n' "$sql" | psql_exec "$@" -f -
}

# Tabel pelacak dibuat sekali; aman dijalankan berulang.
psql_query "
  CREATE TABLE IF NOT EXISTS schema_migrations (
    filename    TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );" -q >/dev/null

applied=0
skipped=0

for file in "$MIGRATIONS_DIR"/*.up.sql; do
  [[ -e "$file" ]] || continue
  name="$(basename "$file")"

  if [[ "$(psql_query "SELECT 1 FROM schema_migrations WHERE filename = '$name'" -tA)" == "1" ]]; then
    skipped=$((skipped + 1))
    continue
  fi

  if [[ "$DRY_RUN" == true ]]; then
    echo "akan diterapkan: $name"
    continue
  fi

  echo "menerapkan: $name"
  # Satu transaksi per file: gagal berarti rollback utuh, tidak menyisakan migrasi separuh.
  psql_exec -q -1 -f - < "$file"
  psql_query "INSERT INTO schema_migrations (filename) VALUES ('$name') ON CONFLICT DO NOTHING" -q >/dev/null
  applied=$((applied + 1))
done

if [[ "$DRY_RUN" == true ]]; then
  echo "dry-run selesai."
else
  echo "selesai: $applied diterapkan, $skipped dilewati."
fi
