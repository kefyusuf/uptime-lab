#!/bin/sh
set -eu

case "${SERVICE_NAME:-}" in
  web|api|checker) ;;
  *)
    printf 'Invalid SERVICE_NAME: %s\n' "${SERVICE_NAME:-<unset>}" >&2
    exit 64
    ;;
esac

READY_FILE="/run/uptime-lab/ready"
child_pid=""
stopping=0

log() { printf 'service=%s event=%s\n' "$SERVICE_NAME" "$1"; }

shutdown() {
  if [ "$stopping" -eq 1 ]; then return; fi
  stopping=1
  rm -f "$READY_FILE"
  log stopping
  if [ -n "$child_pid" ]; then kill "$child_pid" 2>/dev/null || true; fi
}

trap 'exit 130' INT
trap 'exit 143' TERM
trap shutdown EXIT

log starting
: > "$READY_FILE"
log ready

tail -f /dev/null &
child_pid=$!
if wait "$child_pid"; then status=0; else status=$?; fi
exit "$status"
