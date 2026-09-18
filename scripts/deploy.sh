#!/usr/bin/env bash
# Deploy CBS Core ke VPS via SSH.
# Alamat VPS tidak disimpan di repo publik. Set dulu:
#   export CBS_VPS=user@host
set -euo pipefail

VPS="${CBS_VPS:-${VPS:-}}"
if [ -z "$VPS" ]; then
  echo "CBS_VPS belum di-set. Contoh: export CBS_VPS=user@host" >&2
  exit 1
fi
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
DEPLOY_DIR="$ROOT_DIR/deploy"

MODE="${1:-update}"

if [ "$MODE" = "setup" ]; then
  echo "==> [setup] Installing quadlet units & directories on VPS..."
  ssh "$VPS" "mkdir -p ~/.config/containers/systemd /srv/qouver/apps/cbs/{env,logs,monorepo,scripts}"
  scp -q "$DEPLOY_DIR/cbs-api.container" "$DEPLOY_DIR/cbs-web.container" "$VPS:~/.config/containers/systemd/"
  scp -q "$DEPLOY_DIR/deploy-vps.sh" "$VPS:/srv/qouver/apps/cbs/scripts/deploy-vps.sh"
  ssh "$VPS" "chmod +x /srv/qouver/apps/cbs/scripts/deploy-vps.sh"
  ssh "$VPS" "systemctl --user daemon-reload"
  echo "==> Setup finished. Make sure env files exist on VPS:"
  echo "    /srv/qouver/apps/cbs/env/cbs-prod.env"
  echo "    /srv/qouver/apps/cbs/env/cbs-web.env"
else
  echo "==> [update] Triggering deploy script on VPS..."
  ssh "$VPS" "/srv/qouver/apps/cbs/scripts/deploy-vps.sh cbs-core"
fi
