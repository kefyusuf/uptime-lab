#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/conventional.sh
source "$SCRIPT_DIR/lib/conventional.sh"

if [[ "$#" -ne 2 ]]; then
  printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2
  exit 2
fi

BASE_SHA="$1"
HEAD_SHA="$2"

if [[ "$BASE_SHA" =~ ^0{40}$ ]]; then
  mapfile -t SUBJECTS < <(git log -1 --format=%s "$HEAD_SHA")
else
  mapfile -t SUBJECTS < <(git log --format=%s "$BASE_SHA..$HEAD_SHA")
fi

if [[ "${#SUBJECTS[@]}" -eq 0 ]]; then
  printf 'No commits found for range %s..%s\n' "$BASE_SHA" "$HEAD_SHA" >&2
  exit 1
fi

FAILED=0
for subject in "${SUBJECTS[@]}"; do
  if ! is_conventional_subject "$subject"; then
    print_conventional_error "$subject"
    FAILED=1
  fi
done

exit "$FAILED"
