#!/bin/sh
# Build the server binary, copy it to the production VPS, restart the service,
# and check that the public URL is serving again.
#
# POSIX sh only: works with sh, dash, bash, or ash.
set -eu

HOST="${DEPLOY_HOST:-tao}"
REMOTE_DIR="${DEPLOY_DIR:-/home/dev/coverletter}"
REMOTE_OWNER="${DEPLOY_OWNER:-dev:dev}"
PUBLIC_URL="${DEPLOY_URL:-https://coverletter.pifia.net/}"

repo_root="$(cd "$(dirname "$0")" && pwd)"
cd "$repo_root"

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

echo "==> building for linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$build_dir/coverletter" ./cmd/server

echo "==> copying to $HOST:$REMOTE_DIR"
ssh "$HOST" "mkdir -p '$REMOTE_DIR' && chown '$REMOTE_OWNER' '$REMOTE_DIR'"
rsync -rlptz --checksum --timeout=60 --no-owner --no-group \
  "$build_dir/coverletter" "$HOST:$REMOTE_DIR/coverletter.new"
# Replace by rename so a running binary is never written in place.
ssh "$HOST" "chmod 0755 '$REMOTE_DIR/coverletter.new' && mv -f '$REMOTE_DIR/coverletter.new' '$REMOTE_DIR/coverletter'"

echo "==> restarting coverletter.service"
if ! ssh "$HOST" "systemctl restart coverletter.service"; then
  echo "deploy: failed to restart coverletter.service" >&2
  ssh "$HOST" "systemctl status coverletter.service --no-pager -l" >&2 || true
  exit 1
fi

active=false
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  if ssh "$HOST" "systemctl is-active --quiet coverletter.service"; then
    active=true
    break
  fi
  sleep 1
done
if [ "$active" != true ]; then
  echo "deploy: coverletter.service is not active after the restart" >&2
  ssh "$HOST" "systemctl status coverletter.service --no-pager -l" >&2 || true
  exit 1
fi
echo "==> coverletter.service is active"

echo "==> checking $PUBLIC_URL"
curl_error="$build_dir/curl-error"
status=000
for _ in 1 2 3 4 5 6 7 8 9 10; do
  if status="$(curl --silent --show-error --location --max-time 20 \
    --output "$build_dir/public-response" --write-out '%{http_code}' \
    "$PUBLIC_URL" 2>"$curl_error")"; then
    if [ "$status" = 200 ]; then
      break
    fi
  else
    status=000
  fi
  sleep 1
done
if [ "$status" != 200 ]; then
  echo "deploy: $PUBLIC_URL returned HTTP $status" >&2
  if [ -s "$curl_error" ]; then
    echo "--- request error ---" >&2
    cat "$curl_error" >&2
  fi
  if [ -f "$build_dir/public-response" ]; then
    echo "--- response body (first 20 lines) ---" >&2
    sed -n '1,20p' "$build_dir/public-response" >&2 || true
  fi
  exit 1
fi

echo "==> deploy complete: coverletter.service active, $PUBLIC_URL returned HTTP 200"
