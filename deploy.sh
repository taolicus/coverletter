#!/bin/sh
# Build the server binary and copy it to the production VPS.
#
# Deployment is deliberately just "copy the binary". systemd and nginx
# are not set up for this host yet, so the script does not restart a
# service or poll a public URL. See features/deployment.md.
#
# POSIX sh only: works with sh, dash, bash, or ash.
set -eu

HOST="${DEPLOY_HOST:-tao}"
REMOTE_DIR="${DEPLOY_DIR:-/home/dev/coverletter}"

repo_root="$(cd "$(dirname "$0")" && pwd)"
cd "$repo_root"

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

echo "==> building for linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$build_dir/coverletter" ./cmd/server

echo "==> copying to $HOST:$REMOTE_DIR"
ssh "$HOST" "mkdir -p '$REMOTE_DIR'"
scp -q "$build_dir/coverletter" "$HOST:$REMOTE_DIR/coverletter.new"
# Replace by rename so a running binary is never written in place.
ssh "$HOST" "chmod 0755 '$REMOTE_DIR/coverletter.new' && mv -f '$REMOTE_DIR/coverletter.new' '$REMOTE_DIR/coverletter'"

echo "==> copied; not restarted (no coverletter.service yet)"
