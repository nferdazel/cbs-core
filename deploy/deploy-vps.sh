#!/usr/bin/env bash
set -euo pipefail

REPO="${1:-cbs-monorepo}"
LOG="/srv/qouver/cbs/logs/deploy.log"
mkdir -p /srv/qouver/cbs/logs
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deploy trigger received for: $REPO" | tee -a "$LOG"

MONO_DIR="/srv/qouver/cbs/monorepo"
IS_FIRST=0

if [ -d "$MONO_DIR/.git" ]; then
  cd "$MONO_DIR"
  OLD_REV=$(git rev-parse HEAD 2>/dev/null || echo "")
  git pull origin main 2>&1 | tee -a "$LOG"
  NEW_REV=$(git rev-parse HEAD 2>/dev/null || echo "")
  if [ "$OLD_REV" != "$NEW_REV" ] && [ -n "$OLD_REV" ]; then
    CHANGED_FILES=$(git diff --name-only "$OLD_REV" "$NEW_REV" 2>/dev/null || echo "apps/core-api/ apps/backoffice-web/")
  else
    CHANGED_FILES="apps/core-api/ apps/backoffice-web/"
  fi
else
  echo "Cloning cbs-monorepo -> $MONO_DIR" | tee -a "$LOG"
  git clone git@github.com:nferdazel/cbs-core.git "$MONO_DIR" 2>&1 | tee -a "$LOG"
  CHANGED_FILES="apps/core-api/ apps/backoffice-web/"
  IS_FIRST=1
fi

# 1. Deploy Core API if apps/core-api/ changed
if echo "$CHANGED_FILES" | grep -q "^apps/core-api/" || [ "$IS_FIRST" -eq 1 ]; then
  echo "==> [cbs-monorepo] Building & restarting Core API..." | tee -a "$LOG"
  cd "$MONO_DIR/apps/core-api"
  podman build -t localhost/cbs-api:local . 2>&1 | tail -15 | tee -a "$LOG"
  systemctl --user restart cbs-api 2>&1 | tee -a "$LOG"
  sleep 2
  systemctl --user is-active cbs-api 2>&1 | tee -a "$LOG"
fi

# 2. Deploy Backoffice Web if apps/backoffice-web/ or packages/ changed
if echo "$CHANGED_FILES" | grep -q "^apps/backoffice-web/\|^packages/" || [ "$IS_FIRST" -eq 1 ]; then
  echo "==> [cbs-monorepo] Building & restarting Backoffice Web..." | tee -a "$LOG"
  cd "$MONO_DIR"
  podman build -f apps/backoffice-web/Dockerfile -t localhost/cbs-web:local . 2>&1 | tail -15 | tee -a "$LOG"
  systemctl --user restart cbs-web 2>&1 | tee -a "$LOG"
  sleep 2
  systemctl --user is-active cbs-web 2>&1 | tee -a "$LOG"
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Deployment completed successfully for $REPO" | tee -a "$LOG"
