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

LOCAL_DEV=false
while IFS= read -r path; do
  case "$path" in
    compose.yaml|.env.example|deploy/docker/*|\
    scripts/ci/detect-local-dev-changes.sh|scripts/ci/test-detect-local-dev-changes.sh|\
    scripts/ci/check-local-dev.sh|scripts/ci/test-check-local-dev.sh|\
    scripts/ci/smoke-local-dev.sh|scripts/ci/test-smoke-local-dev.sh|\
    docs/devops/local-development.md|.github/workflows/ci.yml)
      LOCAL_DEV=true
      break
      ;;
  esac
done < <(git diff --name-only "$BASE_SHA" "$HEAD_SHA" --)

printf '%s\n' "$LOCAL_DEV"
