#!/bin/sh
set -eu

APP_USER="app"
APP_GROUP="app"
APP_BIN="/app/backend/server"
DATA_DIR="/app/data"
ENV_FILE="$DATA_DIR/.env"

mkdir -p "$DATA_DIR"

is_installed() {
  [ -f "$ENV_FILE" ] && grep -Eq '^[[:space:]]*INSTALL_DONE[[:space:]]*=[[:space:]]*true[[:space:]]*$' "$ENV_FILE"
}

if is_installed; then
  chown -R "${APP_USER}:${APP_GROUP}" "$DATA_DIR" 2>/dev/null || true
  exec su-exec "${APP_USER}:${APP_GROUP}" "$APP_BIN"
fi

exec "$APP_BIN"
