#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="${1:-/tmp/clearday-backend-release}"
SOURCE_DIR="${SOURCE_DIR%$'\r'}"
APP_DIR="${CLEARDAY_APP_DIR:-/opt/clearday/app}"
BIN_PATH="${CLEARDAY_BIN_PATH:-/opt/clearday/clearday}"
SERVICE_NAME="${CLEARDAY_SERVICE_NAME:-clearday}"
SERVICE_USER="${CLEARDAY_SERVICE_USER:-clearday}"
BACKUP_ROOT="${CLEARDAY_BACKUP_ROOT:-/opt/clearday/backups}"
HEALTH_URL="${CLEARDAY_HEALTH_URL:-http://127.0.0.1:8080/api/health}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_DIR="$BACKUP_ROOT/backend-$TIMESTAMP"
NEW_BIN="/tmp/clearday-$TIMESTAMP"

export PATH="/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/sbin:${PATH:-}"
export HOME="${HOME:-/root}"
export GOPATH="${GOPATH:-$HOME/go}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"
export GOCACHE="${GOCACHE:-$HOME/.cache/go-build}"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run this script with sudo." >&2
  exit 1
fi

if [[ ! -d "$SOURCE_DIR/backend" ]]; then
  CANDIDATE_BACKEND="$(find "$SOURCE_DIR" -maxdepth 4 -type d -name backend -print -quit 2>/dev/null || true)"
  if [[ -n "$CANDIDATE_BACKEND" ]]; then
    SOURCE_DIR="$(dirname "$CANDIDATE_BACKEND")"
  fi
fi

echo "Using update source folder: $SOURCE_DIR"

if [[ ! -d "$SOURCE_DIR/backend" ]]; then
  echo "Release folder must contain backend/." >&2
  echo "Release folder contents:" >&2
  find "$SOURCE_DIR" -maxdepth 4 -type d | sort >&2 || true
  exit 1
fi

if [[ ! -f /etc/clearday.env ]]; then
  echo "/etc/clearday.env is missing." >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"

if [[ -d "$APP_DIR/backend" ]]; then
  cp -a "$APP_DIR/backend" "$BACKUP_DIR/backend"
fi

if [[ -d "$APP_DIR/deploy" ]]; then
  cp -a "$APP_DIR/deploy" "$BACKUP_DIR/deploy"
fi

if [[ -d "$APP_DIR/web" ]]; then
  cp -a "$APP_DIR/web" "$BACKUP_DIR/web"
fi

if [[ -f "$BIN_PATH" ]]; then
  cp -a "$BIN_PATH" "$BACKUP_DIR/clearday"
fi

echo "Building AWS-compatible backend..."
mkdir -p "$GOMODCACHE" "$GOCACHE"
if ! command -v go >/dev/null 2>&1; then
  echo "Go is not installed or is not available in PATH. Run deploy/aws/setup-ubuntu-ec2.sh on the EC2 instance, then rerun this deployment." >&2
  exit 127
fi
go version
pushd "$SOURCE_DIR/backend" >/dev/null
go mod tidy
go mod download
go build -o "$NEW_BIN" ./cmd/clearday-server
popd >/dev/null

restore_previous() {
  echo "Deployment failed; restoring previous backend release." >&2
  if [[ -d "$BACKUP_DIR/backend" ]]; then
    rm -rf "$APP_DIR/backend"
    cp -a "$BACKUP_DIR/backend" "$APP_DIR/backend"
  fi
  if [[ -d "$BACKUP_DIR/deploy" ]]; then
    rm -rf "$APP_DIR/deploy"
    cp -a "$BACKUP_DIR/deploy" "$APP_DIR/deploy"
  fi
  if [[ -d "$BACKUP_DIR/web" ]]; then
    rm -rf "$APP_DIR/web"
    cp -a "$BACKUP_DIR/web" "$APP_DIR/web"
  fi
  if [[ -f "$BACKUP_DIR/clearday" ]]; then
    install -m 0755 "$BACKUP_DIR/clearday" "$BIN_PATH"
  fi
  if systemctl list-unit-files "$SERVICE_NAME.service" >/dev/null 2>&1; then
    systemctl restart "$SERVICE_NAME" || true
  fi
}

trap restore_previous ERR

if systemctl list-unit-files "$SERVICE_NAME.service" >/dev/null 2>&1; then
  systemctl stop "$SERVICE_NAME" || true
fi

mkdir -p "$APP_DIR"
rm -rf "$APP_DIR/backend"
cp -a "$SOURCE_DIR/backend" "$APP_DIR/backend"
rm -f "$APP_DIR/backend/.env"

if [[ -d "$SOURCE_DIR/deploy" ]]; then
  rm -rf "$APP_DIR/deploy"
  cp -a "$SOURCE_DIR/deploy" "$APP_DIR/deploy"
fi

if [[ -d "$SOURCE_DIR/web" ]]; then
  rm -rf "$APP_DIR/web"
  cp -a "$SOURCE_DIR/web" "$APP_DIR/web"
fi

install -m 0755 "$NEW_BIN" "$BIN_PATH"

if id "$SERVICE_USER" >/dev/null 2>&1; then
  chown -R "$SERVICE_USER:$SERVICE_USER" /opt/clearday
fi

if systemctl list-unit-files "$SERVICE_NAME.service" >/dev/null 2>&1; then
  systemctl daemon-reload
  systemctl start "$SERVICE_NAME"
  sleep 2
  curl -fsS "$HEALTH_URL" >/dev/null
  systemctl --no-pager status "$SERVICE_NAME"
else
  echo "Service $SERVICE_NAME.service was not found. Binary installed at $BIN_PATH."
fi

trap - ERR
rm -f "$NEW_BIN"
echo "ClearDay AWS backend update finished. Backup: $BACKUP_DIR"
