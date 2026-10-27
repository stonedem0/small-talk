#!/usr/bin/env bash
#
# Runs on the server (as the app user). Swaps in a freshly uploaded build,
# restarts the services, and rolls back to the previous build if /health fails.
#
# Usage: deploy.sh <staging-dir>
#   <staging-dir> contains: app, directory, client-dist/
#
# Previous builds are kept next to the live ones as *.prev so a bad deploy
# can be undone by running `deploy.sh --rollback`.

set -euo pipefail

ROOT=${INSTALL_DIR:-/home/ubuntu/small-talk}
APP_BIN="$ROOT/apps/app/server/app"
DIR_BIN="$ROOT/apps/directory/directory"
CLIENT_DIST="$ROOT/apps/app/client/dist"
APP_URL=${APP_URL:-http://127.0.0.1:8080/health}
DIR_URL=${DIR_URL:-http://127.0.0.1:8081/health}
DIR_ENV=${DIR_ENV:-$ROOT/apps/directory/directory.env}

log() { echo "[deploy] $*"; }

swap() {
  local live="$1" new="$2"
  rm -rf "$live.prev"
  [ -e "$live" ] && mv "$live" "$live.prev"
  mv "$new" "$live"
}

restore() {
  local live="$1"
  [ -e "$live.prev" ] || { log "no previous build for $live"; return 1; }
  rm -rf "$live.bad"
  [ -e "$live" ] && mv "$live" "$live.bad"
  mv "$live.prev" "$live"
}

restart() {
  sudo systemctl restart app.service directory.service
}

# The directory's /health sits behind the internal key when one is configured.
internal_key() {
  [ -r "$DIR_ENV" ] && sed -n 's/^INTERNAL_API_KEY=//p' "$DIR_ENV" | head -n1 || true
}

wait_healthy() {
  local key
  key=$(internal_key)
  for _ in $(seq 1 20); do
    if curl -fsS -o /dev/null "$APP_URL" &&
       curl -fsS -o /dev/null -H "X-Internal-Key: $key" "$DIR_URL"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

if [ "${1:-}" = "--rollback" ]; then
  log "rolling back"
  restore "$APP_BIN"
  restore "$DIR_BIN"
  restore "$CLIENT_DIST" || true
  restart
  wait_healthy && log "rollback ok" || { log "rollback is unhealthy too"; exit 1; }
  exit 0
fi

STAGE=${1:?usage: deploy.sh <staging-dir> | --rollback}
for f in app directory client-dist; do
  [ -e "$STAGE/$f" ] || { echo "missing $STAGE/$f" >&2; exit 1; }
done
chmod +x "$STAGE/app" "$STAGE/directory"

log "swapping in new build"
swap "$APP_BIN" "$STAGE/app"
swap "$DIR_BIN" "$STAGE/directory"
swap "$CLIENT_DIST" "$STAGE/client-dist"

log "restarting services"
restart

if wait_healthy; then
  log "healthy"
  rm -rf "$STAGE"
else
  log "unhealthy after deploy, rolling back"
  restore "$APP_BIN"
  restore "$DIR_BIN"
  restore "$CLIENT_DIST" || true
  restart
  wait_healthy && log "rolled back, previous build is serving" || log "still unhealthy after rollback"
  exit 1
fi
