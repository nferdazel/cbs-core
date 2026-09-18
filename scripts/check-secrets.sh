#!/usr/bin/env bash
# Cek cepat agar file sensitif tidak ikut ter-commit di repo publik.
# Pasang sebagai .git/hooks/pre-commit, atau jalankan manual: scripts/check-secrets.sh
set -uo pipefail

fail=0

# File yang tidak boleh ada di repo
tracked=$(git diff --cached --name-only --diff-filter=ACM)
for f in $tracked; do
  base=$(basename "$f")
  case "$base" in
    .env|*.env|.env.*) [ "$base" = ".env.example" ] || { echo "TERLARANG: $f (file env)"; fail=1; } ;;
    *.pem|*.key|id_rsa|id_ed25519) echo "TERLARANG: $f (kunci privat)"; fail=1 ;;
  esac
done

# Pola sensitif pada isi file yang distage
patterns='(\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53})'
if git diff --cached -U0 | grep -E '^\+' | grep -Eq "$patterns"; then
  echo "TERLARANG: hash bcrypt terdeteksi di perubahan yang distage"
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo ""
  echo "Commit dibatalkan. Perbaiki temuan di atas, atau jalankan dengan --no-verify bila benar-benar disengaja."
  exit 1
fi

echo "check-secrets: bersih"
