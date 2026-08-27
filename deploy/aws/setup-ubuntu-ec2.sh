#!/usr/bin/env bash
set -euo pipefail

GO_VERSION="${GO_VERSION:-1.24.6}"
SERVICE_USER="${CLEARDAY_SERVICE_USER:-clearday}"
APP_ROOT="${CLEARDAY_ROOT:-/opt/clearday}"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run this script with sudo." >&2
  exit 1
fi

apt-get update
apt-get install -y ca-certificates curl snapd unzip

case "$(uname -m)" in
  x86_64)
    GO_ARCH="amd64"
    AWSCLI_ARCH="x86_64"
    ;;
  aarch64 | arm64)
    GO_ARCH="arm64"
    AWSCLI_ARCH="aarch64"
    ;;
  *)
    echo "Unsupported CPU architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

GO_TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
curl -fsSLo "/tmp/${GO_TARBALL}" "https://go.dev/dl/${GO_TARBALL}"
rm -rf /usr/local/go
tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

if ! command -v aws >/dev/null 2>&1; then
  curl -fsSLo /tmp/awscliv2.zip "https://awscli.amazonaws.com/awscli-exe-linux-${AWSCLI_ARCH}.zip"
  rm -rf /tmp/aws
  unzip -q /tmp/awscliv2.zip -d /tmp
  /tmp/aws/install --update
fi

systemctl enable --now snapd.socket >/dev/null 2>&1 || true
if command -v snap >/dev/null 2>&1; then
  if ! snap list amazon-ssm-agent >/dev/null 2>&1; then
    snap install amazon-ssm-agent --classic
  fi
  snap start amazon-ssm-agent >/dev/null 2>&1 || true
fi
systemctl enable --now snap.amazon-ssm-agent.amazon-ssm-agent.service >/dev/null 2>&1 || systemctl enable --now amazon-ssm-agent.service >/dev/null 2>&1 || true

if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  useradd --system --home "$APP_ROOT" --shell /usr/sbin/nologin "$SERVICE_USER"
fi

mkdir -p "$APP_ROOT/app" "$APP_ROOT/backups"
chown -R "$SERVICE_USER:$SERVICE_USER" "$APP_ROOT"

cat >/etc/systemd/system/clearday.service <<'UNIT'
[Unit]
Description=ClearDay reminder backend
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=clearday
Group=clearday
WorkingDirectory=/opt/clearday/app/backend
EnvironmentFile=/etc/clearday.env
ExecStart=/opt/clearday/clearday
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable clearday.service

cat <<'NEXT'
EC2 setup is ready.

Next:
1. Create /etc/clearday.env with CLEAR_DAY_STORE=dynamodb.
2. Deploy the backend.
3. Start with: sudo systemctl start clearday
NEXT
