#!/usr/bin/env bash
# Temporarily expose the local app to the internet via a Cloudflare quick tunnel.
# Usage: ./share.sh   (Ctrl-C to stop and tear down the public URL)
set -euo pipefail

PORT=8081
cd "$(dirname "$0")"

cleanup() {
  echo
  echo "Shutting down..."
  [[ -n "${APP_PID:-}" ]]   && kill "$APP_PID"   2>/dev/null || true
  [[ -n "${TUN_PID:-}" ]]   && kill "$TUN_PID"   2>/dev/null || true
  [[ -n "${APP_BIN:-}" ]]   && rm -f "$APP_BIN"  2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "Building app..."
APP_BIN="$(mktemp -t matrigonio.XXXXXX)"
go build -o "$APP_BIN" .

echo "Starting app on :$PORT ..."
"$APP_BIN" &
APP_PID=$!

# Wait for the app to come up before opening the tunnel.
for i in {1..30}; do
  if curl -fsS "http://localhost:$PORT/api/checkhealth" >/dev/null 2>&1; then
    echo "App is up."
    break
  fi
  sleep 0.5
done

echo "Opening Cloudflare tunnel... (your public link will appear below)"
echo "Remember: share the URL with /app/ on the end."
echo
# --protocol http2 avoids QUIC/UDP, which is unreliable under WSL2 (HTTP 530 errors).
cloudflared tunnel --protocol http2 --url "http://localhost:$PORT" &
TUN_PID=$!

wait "$TUN_PID"
