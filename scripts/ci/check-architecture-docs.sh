#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"

REQUIRED_FILES=(
  docs/README.md
  docs/glossary.md
  docs/architecture/README.md
  docs/architecture/system-context.md
  docs/architecture/container-view.md
  docs/architecture/module-boundaries.md
  docs/architecture/dependency-rules.md
  docs/architecture/data-ownership.md
  docs/architecture/runtime-flows.md
  docs/architecture/change-flow.md
  docs/adr/README.md
  docs/adr/0001-multi-runtime-monorepo.md
  docs/adr/0002-control-plane-and-execution-plane.md
  docs/adr/0003-contract-and-data-ownership.md
  docs/backend/go-control-plane.md
)

fail() {
  printf 'Architecture documentation check failed: %s\n' "$1" >&2
  exit 1
}

require_file() {
  local path="$1"
  [[ -f "$ROOT/$path" ]] || fail "missing required file: $path"
}

require_text() {
  local path="$1"
  local text="$2"
  grep -Fq -- "$text" "$ROOT/$path" || fail "$path is missing required text: $text"
}

for path in "${REQUIRED_FILES[@]}"; do
  require_file "$path"
done

for path in "${REQUIRED_FILES[@]}"; do
  if grep -Eq '(^|[^A-Za-z])(TODO|TBD)([^A-Za-z]|$)' "$ROOT/$path"; then
    fail "forbidden placeholder token in $path"
  fi
done

require_text docs/README.md 'architecture/README.md'

for link in \
  'system-context.md' \
  'container-view.md' \
  'module-boundaries.md' \
  'dependency-rules.md' \
  'data-ownership.md' \
  'runtime-flows.md' \
  'change-flow.md' \
  '../adr/README.md'; do
  require_text docs/architecture/README.md "$link"
done

ADR_FILES=(
  docs/adr/0001-multi-runtime-monorepo.md
  docs/adr/0002-control-plane-and-execution-plane.md
  docs/adr/0003-contract-and-data-ownership.md
)

for adr in "${ADR_FILES[@]}"; do
  for heading in \
    '## Status' \
    '## Context' \
    '## Decision' \
    '## Alternatives Considered' \
    '## Consequences' \
    '## Related Documentation'; do
    require_text "$adr" "$heading"
  done
  require_text "$adr" 'Accepted'
done

require_text docs/architecture/container-view.md 'Go exclusively owns durable product state in PostgreSQL.'
require_text docs/architecture/container-view.md 'Rust never accesses PostgreSQL directly.'
require_text docs/architecture/container-view.md 'The browser never accesses PostgreSQL directly.'
require_text docs/architecture/container-view.md 'Cross-runtime communication is contract-driven.'
require_text docs/architecture/dependency-rules.md 'Rust Checker -> PostgreSQL is forbidden.'
require_text docs/architecture/dependency-rules.md 'Cross-runtime implementation-code sharing is forbidden.'
require_text docs/architecture/data-ownership.md 'Go exclusively owns durable product state in PostgreSQL.'
require_text docs/backend/go-control-plane.md 'Go Control Plane'

sequence_count="$(grep -c '^sequenceDiagram$' "$ROOT/docs/architecture/runtime-flows.md" || true)"
[[ "$sequence_count" -eq 4 ]] || fail "runtime-flows.md must contain exactly four sequenceDiagram blocks (found $sequence_count)"

require_text docs/architecture/change-flow.md 'Configure HTTP Request Timeout'

FORBIDDEN_DIRS=(
  docs/frontend
  docs/checker
)
for path in "${FORBIDDEN_DIRS[@]}"; do
  [[ ! -e "$ROOT/$path" ]] || fail "speculative documentation path exists: $path"
done

FORBIDDEN_FILES=(
  docs/architecture/component-view.md
  docs/architecture/api-contracts.md
  docs/architecture/event-model.md
)
for path in "${FORBIDDEN_FILES[@]}"; do
  [[ ! -e "$ROOT/$path" ]] || fail "deferred documentation file exists: $path"
done
