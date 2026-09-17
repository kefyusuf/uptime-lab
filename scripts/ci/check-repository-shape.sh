#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
REQUIRED_SPEC="docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md"
FORBIDDEN_DIRS=(shared common utils helpers)

if [[ ! -f "$ROOT/$REQUIRED_SPEC" ]]; then
  printf 'Missing canonical foundation spec: %s\n' "$REQUIRED_SPEC" >&2
  exit 1
fi

for directory in "${FORBIDDEN_DIRS[@]}"; do
  if [[ -d "$ROOT/$directory" ]]; then
    printf 'Forbidden root business directory: %s/\n' "$directory" >&2
    exit 1
  fi
done

if git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  if git -C "$ROOT" ls-files --error-unmatch .env >/dev/null 2>&1; then
    printf 'Tracked root .env is forbidden. Keep local secrets untracked and introduce .env.example when configuration exists.\n' >&2
    exit 1
  fi
fi
