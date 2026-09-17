#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/conventional.sh
source "$SCRIPT_DIR/lib/conventional.sh"

if [[ "$#" -ne 1 ]]; then
  printf 'Usage: %s <pull-request-title>\n' "$0" >&2
  exit 2
fi

if ! is_conventional_subject "$1"; then
  print_conventional_error "$1"
  exit 1
fi
