#!/bin/bash
set -euo pipefail

REPO="${1:-}"
if [ -z "$REPO" ]; then
  REPO="cbs-core"
fi

LOG="/srv/qouver/cbs/logs/deploy.log"
mkdir -p /srv/qouver/cbs/logs
echo "[$(date '+%Y-%m-%d %H:%M:%S')] deploy trigger: $REPO" | tee -a "$LOG"

switch_local_image() {
  echo "==> switching quadlet to local image + Pull=never" | tee -a "$LOG"
  sed -i 's|^Image=.*|Image=localhost/cbs-api:local|' ~/.config/containers/systemd/cbs-api.container
  sed -i 's|^Pull=.*|Pull=never|' ~/.config/containers/systemd/cbs-api.container
  if ! grep -q '^Pull=' ~/.config/containers/systemd/cbs-api.container; then
    sed -i '/^Image=/a Pull=never' ~/.config/containers/systemd/cbs-api.container
  fi
  systemctl --user daemon-reload 2>&1 | tee -a "$LOG"
}

if [ "$REPO" = "cbs-core" ]; then
  MONO_DIR="/srv/qouver/cbs/monorepo"
  IS_FIRST=0
  if [ -d "$MONO_DIR/.git" ]; then
    cd "$MONO_DIR"
    git remote set-url origin git@github.com:nferdazel/cbs-core.git 2>&1 | tee -a "$LOG" || true
    OLD_REV=$(git rev-parse HEAD 2>/dev/null || echo "")
    git pull origin main 2>&1 | tee -a "$LOG"
    NEW_REV=$(git rev-parse HEAD 2>/dev/null || echo "")
    if [ "$OLD_REV" != "$NEW_REV" ] && [ -n "$OLD_REV" ]; then
      CHANGED_FILES=$(git diff --name-only "$OLD_REV" "$NEW_REV" 2>/dev/null || echo "apps/api/ apps/web/")
    else
      CHANGED_FILES="apps/api/ apps/web/"
    fi
  else
    echo "cloning cbs-core -> $MONO_DIR" | tee -a "$LOG"
    git clone git@github.com:nferdazel/cbs-core.git "$MONO_DIR" 2>&1 | tee -a "$LOG"
    CHANGED_FILES="apps/api/ apps/web/"
    IS_FIRST=1
  fi

  # 1. Deploy Go Core API Backend if apps/api/ changed or first run
  if echo "$CHANGED_FILES" | grep -q "^apps/api/" || [ "$IS_FIRST" -eq 1 ]; then
    echo "==> [cbs-core] deploying API (apps/api)" | tee -a "$LOG"
    cd "$MONO_DIR/apps/api"
    podman build -t localhost/cbs-api:local . 2>&1 | tail -20 | tee -a "$LOG"
    switch_local_image
    systemctl --user restart cbs-api 2>&1 | tee -a "$LOG"
    sleep 3
    systemctl --user is-active cbs-api 2>&1 | tee -a "$LOG"
    curl -s http://127.0.0.1:8082/healthz 2>&1 | tee -a "$LOG"
  fi

  # Deploy Next.js Web Frontend if apps/web/ changed or first run
  if echo "$CHANGED_FILES" | grep -q "^apps/web/" || [ "$IS_FIRST" -eq 1 ]; then
    echo "==> [cbs-core] deploying Web (apps/web)" | tee -a "$LOG"
    cd "$MONO_DIR/apps/web"
    pnpm install --frozen-lockfile 2>&1 | tail -5 | tee -a "$LOG"
    NEXT_PUBLIC_API_BASE_URL=https://api.qouver.com/cbs/v1 pnpm build 2>&1 | tail -5 | tee -a "$LOG"
    mkdir -p /srv/qouver/cbs/web/
    rsync -az out/ /srv/qouver/cbs/web/ 2>&1 || rsync -az .next/ /srv/qouver/cbs/web/ 2>&1 | tee -a "$LOG"
    restorecon -RF /srv/qouver/cbs/web/ 2>&1 | tee -a "$LOG"
    echo "==> [cbs-core] Web build complete" | tee -a "$LOG"
  fi
fi

echo "[$(date)] done $REPO" | tee -a "$LOG"
