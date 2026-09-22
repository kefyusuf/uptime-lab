#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2
  exit 2
fi

BASE_SHA="$1"
HEAD_SHA="$2"
ZERO_SHA="0000000000000000000000000000000000000000"

if [[ "$BASE_SHA" == "$ZERO_SHA" ]]; then
  printf 'true\n'
  exit 0
fi

if ! git cat-file -e "$HEAD_SHA^{commit}" 2>/dev/null; then
  printf 'Head commit is unavailable: %s\n' "$HEAD_SHA" >&2
  exit 1
fi

if ! git cat-file -e "$BASE_SHA^{commit}" 2>/dev/null; then
  printf 'true\n'
  exit 0
fi

PUBLIC_CONTRACT=false
while IFS= read -r path; do
  case "$path" in
    contracts/openapi/*|\
    scripts/ci/detect-public-contract-changes.sh|scripts/ci/test-detect-public-contract-changes.sh|\
    scripts/ci/check-public-contract.mjs|scripts/ci/test-check-public-contract.mjs|\
    .github/workflows/ci.yml)
      PUBLIC_CONTRACT=true
      break
      ;;
  esac
done < <(git diff --name-only "$BASE_SHA" "$HEAD_SHA" --)

printf '%s\n' "$PUBLIC_CONTRACT"
