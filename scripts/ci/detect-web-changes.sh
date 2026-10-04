#!/usr/bin/env bash
set -euo pipefail
[[ "$#" -eq 2 ]] || { printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2;exit 2; }
BASE_SHA="$1";HEAD_SHA="$2"
git cat-file -e "$HEAD_SHA^{commit}" 2>/dev/null || { printf 'Head commit unavailable\n' >&2;exit 1; }
if [[ "$BASE_SHA" == 0000000000000000000000000000000000000000 ]] || ! git cat-file -e "$BASE_SHA^{commit}" 2>/dev/null;then printf 'true\n';exit 0;fi
CHANGED=false
while IFS= read -r path;do
  case "$path" in
    apps/web/*|apps/api/*|apps/checker/*|compose.yaml|compose.web-local.yaml|.env.example|\
    contracts/openapi/public.yaml|contracts/fixtures/public/*|scripts/ci/*web*|\
    scripts/ci/*local-dev*|.github/workflows/ci.yml) CHANGED=true;break;;
  esac
done < <(git diff --no-renames --name-only "$BASE_SHA" "$HEAD_SHA" --)
printf '%s\n' "$CHANGED"
