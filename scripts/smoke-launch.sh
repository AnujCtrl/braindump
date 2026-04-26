#!/usr/bin/env bash
# Boot the desktop binary, verify Tauri setup completes (icon decode,
# frontend load, socket bind), send a --toggle, and confirm the running
# instance receives it. Catches the class of regression where `cargo build`
# is green but the binary panics during runtime startup.
#
# Used by CI under xvfb-run, and runnable locally for the same checks.
#
# Usage: scripts/smoke-launch.sh [path/to/braindump-desktop]
set -euo pipefail

BINARY="${1:-./target/debug/braindump-desktop}"
if [[ ! -x "$BINARY" ]]; then
  echo "binary not found or not executable: $BINARY" >&2
  exit 2
fi

DATA_DIR="$(mktemp -d)"
RUNTIME_DIR="$(mktemp -d)"
LOG_FILE="$(mktemp)"

PID=""
cleanup() {
  if [[ -n "$PID" ]]; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$DATA_DIR" "$RUNTIME_DIR" "$LOG_FILE"
}
trap cleanup EXIT

echo "spawning $BINARY"
BRAINDUMP_DATA_DIR="$DATA_DIR" \
XDG_RUNTIME_DIR="$RUNTIME_DIR" \
RUST_LOG=info \
"$BINARY" >"$LOG_FILE" 2>&1 &
PID=$!

# Wait up to 10s for socket to appear; bail early if the process dies.
SOCKET="$RUNTIME_DIR/braindump.sock"
for _ in $(seq 1 20); do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "FAIL: binary died during startup"
    echo "--- log ---"
    cat "$LOG_FILE"
    exit 1
  fi
  if [[ -S "$SOCKET" ]]; then
    break
  fi
  sleep 0.5
done

if [[ ! -S "$SOCKET" ]]; then
  echo "FAIL: socket never appeared at $SOCKET"
  echo "--- log ---"
  cat "$LOG_FILE"
  exit 1
fi

echo "socket bound; sending --toggle"
XDG_RUNTIME_DIR="$RUNTIME_DIR" "$BINARY" --toggle

# Give the listener a moment to log.
sleep 1

if ! grep -q 'ipc command received.*toggle' "$LOG_FILE"; then
  echo "FAIL: toggle command did not reach the running instance"
  echo "--- log ---"
  cat "$LOG_FILE"
  exit 1
fi

echo "SMOKE TEST PASSED"
echo "--- log ---"
cat "$LOG_FILE"
