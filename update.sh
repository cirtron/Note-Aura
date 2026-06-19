#!/usr/bin/env bash
# update.sh — Rebuild and restart Note-Aura on Linux/macOS WITHOUT touching data.
#
# The Linux/macOS counterpart of update.ps1. Run it ON the server, from the
# Note-Aura source directory (where main.go lives). It stops the running server,
# rebuilds the binary from the current source, and starts it again. Your data
# (note-aura.db, uploads/, config.yaml) is never touched — only the program.
#
# Requirements: Go (to build) and the current source present in this directory.
# If the server has no Go, build elsewhere and copy the binary instead (see
# INSTALL.md §9).
#
# Restart strategy (auto-detected):
#   * systemd: if a unit named "$NOTE_AURA_SERVICE" (default: note-aura) exists,
#     it is restarted with systemctl (uses sudo).
#   * otherwise: any running ./note-aura is killed and relaunched in the
#     background (nohup), logging to note-aura.log.
#
# Usage:
#   ./update.sh                       # stop, rebuild, restart
#   ./update.sh --no-start            # stop and rebuild only (don't relaunch)
#   NOTE_AURA_SERVICE=my-unit ./update.sh   # use a different systemd unit name
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -f main.go ]; then
  echo "main.go not found in $(pwd) — run this from the Note-Aura source directory." >&2
  exit 1
fi

BIN="note-aura"
SERVICE="${NOTE_AURA_SERVICE:-note-aura}"
NO_START=0
[ "${1:-}" = "--no-start" ] && NO_START=1

# Detect a real systemd unit by that name.
USE_SYSTEMD=0
if command -v systemctl >/dev/null 2>&1 &&
   systemctl list-unit-files 2>/dev/null | grep -q "^${SERVICE}\.service"; then
  USE_SYSTEMD=1
fi

echo "==> Stopping Note-Aura ..."
if [ "$USE_SYSTEMD" -eq 1 ]; then
  sudo systemctl stop "$SERVICE" || true
else
  pkill -x "$BIN" 2>/dev/null || true
  for _ in $(seq 1 20); do
    pgrep -x "$BIN" >/dev/null 2>&1 || break
    sleep 0.25
  done
fi
echo "    stopped."

echo "==> Building $BIN ..."
if ! go build -o "$BIN" .; then
  echo "go build failed — the old binary and your data are unchanged." >&2
  exit 1
fi
echo "    build OK."

if [ "$NO_START" -eq 1 ]; then
  echo "==> --no-start given; not relaunching."
  exit 0
fi

echo "==> Starting Note-Aura ..."
if [ "$USE_SYSTEMD" -eq 1 ]; then
  sudo systemctl start "$SERVICE"
  systemctl --no-pager status "$SERVICE" | head -n 5 || true
else
  nohup "./$BIN" -config config.yaml >> note-aura.log 2>&1 &
  echo "    started (pid $!); logging to note-aura.log"
fi
echo "Update complete — data preserved."
