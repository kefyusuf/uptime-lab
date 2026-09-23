#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2
  exit 2
fi

BASE_SHA="$1"
HEAD_SHA="$2"
ZERO_SHA="0000000000000000000000000000000000000000"

fail() {
  printf 'Migration history invariant failed: %s\n' "$1" >&2
  exit 1
}

if ! git cat-file -e "$HEAD_SHA^{commit}" 2>/dev/null; then
  fail "head commit is unavailable: $HEAD_SHA"
fi

if [[ "$BASE_SHA" == "$ZERO_SHA" ]]; then
  printf 'Migration history check: PASS\n'
  exit 0
fi

if ! git cat-file -e "$BASE_SHA^{commit}" 2>/dev/null; then
  fail "base commit is unavailable: $BASE_SHA"
fi

while IFS= read -r path; do
  [[ "$path" == apps/api/migrations/*.sql ]] || continue

  if ! git cat-file -e "$HEAD_SHA:$path" 2>/dev/null; then
    fail "protected migration is missing or renamed: $path"
  fi

  base_blob="$(git rev-parse "$BASE_SHA:$path")"
  head_blob="$(git rev-parse "$HEAD_SHA:$path")"
  if [[ "$base_blob" != "$head_blob" ]]; then
    fail "protected migration content changed: $path"
  fi
done < <(git ls-tree -r --name-only "$BASE_SHA" -- apps/api/migrations)

printf 'Migration history check: PASS\n'
