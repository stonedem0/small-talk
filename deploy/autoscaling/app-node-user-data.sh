#!/usr/bin/env bash
#
# Minimal bootstrap script for Auto Scaling app nodes.
# Assumptions:
#   - The Launch Template/user-data exports the required env vars before calling this script.
#   - A tarball containing the repo (or at least apps/app/server + deploy/systemd files) is accessible via SMALL_TALK_TARBALL_URL.
#   - Systemd is available.

set -euo pipefail

log() {
  echo "[small-talk bootstrap] $*"
}

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    echo "missing required env var: $name" >&2
    exit 1
  fi
}

require_env SMALL_TALK_TARBALL_URL
require_env JWT_SECRET
require_env REFRESH_JWT_SECRET
require_env DIRECTORY_URL
require_env REDIS_ADDR
require_env CORS_ORIGINS

APP_USER=${APP_USER:-ubuntu}
APP_GROUP=${APP_GROUP:-$APP_USER}
INSTALL_DIR=${INSTALL_DIR:-/home/${APP_USER}/small-talk}
SERVER_DIR="$INSTALL_DIR/apps/app/server"
SYSTEMD_DIR=/etc/systemd/system
ENV_FILE=${ENV_FILE:-$SERVER_DIR/app.env}

# Fetch metadata for APP_ID / WS_PUBLIC_URL defaults
fetch_md() {
  local path="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -H "X-aws-ec2-metadata-token: ${IMDS_TOKEN}" "http://169.254.169.254/latest/meta-data/${path}"
  else
    wget -qO- --header "X-aws-ec2-metadata-token: ${IMDS_TOKEN}" "http://169.254.169.254/latest/meta-data/${path}"
  fi
}

if command -v curl >/dev/null 2>&1; then
  IMDS_TOKEN=$(curl -X PUT -fsSL "http://169.254.169.254/latest/api/token" \
    -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
else
  IMDS_TOKEN=$(wget -qO- --method=PUT --header="X-aws-ec2-metadata-token-ttl-seconds: 21600" \
    "http://169.254.169.254/latest/api/token")
fi

INSTANCE_ID=${APP_INSTANCE_ID:-$(fetch_md "instance-id")}
PUBLIC_DNS=${PUBLIC_DNS_OVERRIDE:-$(fetch_md "public-hostname" || true)}
PUBLIC_IP=${PUBLIC_IP_OVERRIDE:-$(fetch_md "public-ipv4" || true)}

APP_ID=${APP_ID:-$INSTANCE_ID}
WS_PUBLIC_URL=${WS_PUBLIC_URL:-}
if [ -z "$WS_PUBLIC_URL" ]; then
  if [ -n "$PUBLIC_DNS" ] && [ "$PUBLIC_DNS" != "unset" ]; then
    WS_PUBLIC_URL="wss://${PUBLIC_DNS}/ws"
  elif [ -n "$PUBLIC_IP" ]; then
    WS_PUBLIC_URL="wss://${PUBLIC_IP}:8080/ws"
  else
    WS_PUBLIC_URL="ws://localhost:8080/ws"
  fi
fi

log "creating application directories at $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
chown -R "$APP_USER":"$APP_GROUP" "$INSTALL_DIR"

TMP_TGZ=$(mktemp /tmp/small-talk.XXXXXX.tgz)
log "downloading artifact from $SMALL_TALK_TARBALL_URL"
curl -fsSL "$SMALL_TALK_TARBALL_URL" -o "$TMP_TGZ"

log "extracting artifact into $INSTALL_DIR"
tar -xzf "$TMP_TGZ" -C "$INSTALL_DIR"
rm -f "$TMP_TGZ"
chown -R "$APP_USER":"$APP_GROUP" "$INSTALL_DIR"

log "rendering env file at $ENV_FILE"
cat >"$ENV_FILE" <<EOF
PORT=${PORT:-8080}
APP_ID=$APP_ID
WS_PUBLIC_URL=$WS_PUBLIC_URL
DIRECTORY_URL=$DIRECTORY_URL
JWT_SECRET=$JWT_SECRET
REFRESH_JWT_SECRET=$REFRESH_JWT_SECRET
CORS_ORIGINS=$CORS_ORIGINS
REDIS_ADDR=$REDIS_ADDR
REDIS_USERNAME=${REDIS_USERNAME:-}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
INTERNAL_API_KEY=${INTERNAL_API_KEY:-}
HEARTBEAT_INTERVAL=${HEARTBEAT_INTERVAL:-5s}
EOF
chown "$APP_USER":"$APP_GROUP" "$ENV_FILE"
chmod 600 "$ENV_FILE"

log "installing systemd unit"
cp "$INSTALL_DIR/deploy/systemd/small-talk-app.service" "$SYSTEMD_DIR/app.service"
systemctl daemon-reload
systemctl enable --now app.service

log "bootstrap complete"
