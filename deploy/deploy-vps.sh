#!/usr/bin/env bash
set -euo pipefail

REPO="${1:-cbs-core}"
LOG="/srv/qouver/apps/cbs/logs/deploy.log"
mkdir -p /srv/qouver/apps/cbs/logs
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deploy trigger received for: $REPO" | tee -a "$LOG"

MONO_DIR="/srv/qouver/apps/cbs/monorepo"
IS_FIRST=0

if [ -d "$MONO_DIR/.git" ]; then
  cd "$MONO_DIR"
  git remote set-url origin github-cbs:nferdazel/cbs-core.git 2>&1 | tee -a "$LOG" || true
  OLD_REV=$(git rev-parse HEAD 2>/dev/null || echo "")
  git fetch origin main && git reset --hard origin/main 2>&1 | tee -a "$LOG"
  NEW_REV=$(git rev-parse HEAD 2>/dev/null || echo "")
  if [ "$OLD_REV" != "$NEW_REV" ] && [ -n "$OLD_REV" ]; then
    CHANGED_FILES=$(git diff --name-only "$OLD_REV" "$NEW_REV" 2>/dev/null || echo "apps/api/ apps/web/")
  else
    CHANGED_FILES="apps/api/ apps/web/"
  fi
else
  echo "Cloning cbs-core -> $MONO_DIR" | tee -a "$LOG"
  git clone github-cbs:nferdazel/cbs-core.git "$MONO_DIR" 2>&1 | tee -a "$LOG"
  CHANGED_FILES="apps/api/ apps/web/"
  IS_FIRST=1
fi

# 1. Deploy Core API if apps/api/ changed
if echo "$CHANGED_FILES" | grep -q "^apps/api/" || [ "$IS_FIRST" -eq 1 ]; then
  echo "==> [cbs-core] Building & restarting Core API..." | tee -a "$LOG"
  cd "$MONO_DIR/apps/api"
  podman build -t localhost/cbs-api:local . 2>&1 | tail -15 | tee -a "$LOG"
  systemctl --user restart cbs-api 2>&1 | tee -a "$LOG"
  sleep 2
  systemctl --user is-active cbs-api 2>&1 | tee -a "$LOG"
fi

# 2. Deploy Backoffice Web if apps/web/ or packages/ changed
if echo "$CHANGED_FILES" | grep -q "^apps/web/\|^packages/" || [ "$IS_FIRST" -eq 1 ]; then
  echo "==> [cbs-core] Building & restarting Backoffice Web..." | tee -a "$LOG"
  cd "$MONO_DIR"
  podman build -f apps/web/Dockerfile -t localhost/cbs-web:local . 2>&1 | tail -15 | tee -a "$LOG"
  systemctl --user restart cbs-web 2>&1 | tee -a "$LOG"
  sleep 2
  systemctl --user is-active cbs-web 2>&1 | tee -a "$LOG"
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deployment completed successfully for $REPO" | tee -a "$LOG"
