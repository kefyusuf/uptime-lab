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
  docs/checker/rust-checker.md
  docs/devops/local-development.md
  docs/testing/go-monitoring-foundation.md
  docs/testing/single-checker-execution-slice.md
  docs/testing/public-monitoring-contract.md
  docs/testing/go-public-transport-adapter.md
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
require_text docs/README.md 'testing/public-monitoring-contract.md'
require_text docs/README.md 'testing/go-public-transport-adapter.md'
require_text docs/README.md 'checker/rust-checker.md'
require_text docs/README.md 'testing/single-checker-execution-slice.md'

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
require_text docs/architecture/container-view.md 'Public contract: defined (`contracts/openapi/public.yaml`).'
require_text docs/architecture/container-view.md 'Go public transport adapter: implemented.'
require_text docs/architecture/container-view.md 'Rust Checker: implemented.'
require_text docs/architecture/container-view.md 'Internal checker contract: implemented (`contracts/openapi/internal.yaml`).'
require_text docs/architecture/container-view.md 'No application host ports are published.'
require_text docs/architecture/dependency-rules.md 'Rust Checker -> PostgreSQL is forbidden.'
require_text docs/architecture/dependency-rules.md 'Cross-runtime implementation-code sharing is forbidden.'
require_text docs/architecture/data-ownership.md 'Go exclusively owns durable product state in PostgreSQL.'
require_text docs/backend/go-control-plane.md 'Go Control Plane'
require_text docs/backend/go-control-plane.md 'contracts/openapi/public.yaml'
require_text docs/backend/go-control-plane.md 'POST /monitors'
require_text docs/backend/go-control-plane.md 'GET  /monitors/{monitorId}/latest-result'
require_text docs/backend/go-control-plane.md 'POST /internal/checks/claim'
require_text docs/backend/go-control-plane.md 'PUT /internal/checks/{checkId}/result'
require_text docs/backend/go-control-plane.md 'monitoring.check_runs'
require_text docs/backend/go-control-plane.md 'read-only migration compatibility checker'
require_text docs/checker/rust-checker.md 'Rust Checker: implemented.'
require_text docs/checker/rust-checker.md 'Rust never accesses PostgreSQL directly.'
require_text docs/checker/rust-checker.md 'POST /internal/checks/claim'
require_text docs/checker/rust-checker.md 'PUT /internal/checks/{checkId}/result'
require_text docs/checker/rust-checker.md 'Production destination policy rejects private and non-public addresses.'
require_text docs/checker/rust-checker.md 'No application host ports are published.'
require_text docs/checker/rust-checker.md 'No public CheckRun history or derived availability/status API exists.'
require_text docs/devops/local-development.md 'docker compose up -d db api'
require_text docs/devops/local-development.md 'docker compose exec -T api /usr/local/bin/uptime-lab-migrate up'
require_text docs/devops/local-development.md 'docker compose up -d --wait --wait-timeout 60'
require_text docs/devops/local-development.md 'GET  /monitors/{monitorId}/latest-result'
require_text docs/testing/go-monitoring-foundation.md 'Go architecture tests: 24 passed, 0 failed'
require_text docs/testing/single-checker-execution-slice.md 'Go -> Rust -> policy_rejected -> Go -> PostgreSQL'
require_text docs/testing/single-checker-execution-slice.md 'GET /monitors/{monitorId}/latest-result'
require_text docs/testing/single-checker-execution-slice.md 'Canonical Docker smoke'
require_text docs/testing/single-checker-execution-slice.md 'cargo audit'
require_text docs/testing/public-monitoring-contract.md 'contracts/openapi/public.yaml'
require_text docs/testing/public-monitoring-contract.md 'OpenAPI 3.1.2'
require_text docs/testing/public-monitoring-contract.md '@redocly/cli@2.53.3'
require_text docs/testing/public-monitoring-contract.md 'Go transport conformance'
require_text docs/testing/public-monitoring-contract.md 'GET  /monitors/{monitorId}/latest-result'
require_text docs/testing/go-public-transport-adapter.md 'POST /monitors'
require_text docs/testing/go-public-transport-adapter.md 'GET  /monitors/{monitorId}'
require_text docs/testing/go-public-transport-adapter.md 'GET  /monitors/{monitorId}/latest-result'
require_text docs/testing/go-public-transport-adapter.md 'No application host ports are published'
require_text docs/testing/go-public-transport-adapter.md 'The readiness path is read-only.'
require_text docs/testing/go-public-transport-adapter.md 'The internal Checker contract is implemented.'

require_text docs/architecture/runtime-flows.md 'GET /monitors/{monitorId}/latest-result'
require_text docs/architecture/runtime-flows.md 'GET /monitors/{monitorId}/availability'
if grep -Fq 'derived availability/status remain deferred' "$ROOT/docs/architecture/runtime-flows.md"; then
  fail 'runtime-flows.md must describe current availability as implemented'
fi
if grep -Fq 'Public Monitoring surface is limited to Monitor create/read plus latest terminal result read.' "$ROOT/docs/devops/local-development.md"; then
  fail 'local-development.md must include the current availability operation'
fi
sequence_count="$(grep -c '^sequenceDiagram$' "$ROOT/docs/architecture/runtime-flows.md" || true)"
[[ "$sequence_count" -eq 6 ]] || fail "runtime-flows.md must contain exactly six sequenceDiagram blocks (found $sequence_count)"

require_text docs/architecture/change-flow.md 'Configure HTTP Request Timeout'

FORBIDDEN_DIRS=(
  docs/frontend
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
