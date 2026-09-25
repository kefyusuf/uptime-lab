#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-architecture-docs.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }
expect_success() { local n="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$n"; else fail "$n"; fi; }
expect_failure() { local n="$1"; shift; if "$@" >/dev/null 2>&1; then fail "$n"; else pass "$n"; fi; }

make_fixture() {
  rm -rf "$TMP/repo"
  mkdir -p "$TMP/repo/docs/architecture" "$TMP/repo/docs/adr" "$TMP/repo/docs/backend" "$TMP/repo/docs/devops" "$TMP/repo/docs/testing"

  cat > "$TMP/repo/docs/README.md" <<'DOC'
# Documentation

See [Architecture](architecture/README.md).
See [Public Monitoring Contract](testing/public-monitoring-contract.md).
See [Go Public Transport](testing/go-public-transport-adapter.md).
DOC

  cat > "$TMP/repo/docs/backend/go-control-plane.md" <<'DOC'
# Go Control Plane

The Go Control Plane foundation is implemented.
The public source contract is contracts/openapi/public.yaml.
The runtime serves POST /monitors and GET /monitors/{monitorId}.
Production uses a read-only migration compatibility checker.
DOC

  cat > "$TMP/repo/docs/testing/public-monitoring-contract.md" <<'DOC'
# Public Monitoring Contract Testing

Authoritative source: contracts/openapi/public.yaml.
OpenAPI 3.1.2 is validated with @redocly/cli@2.53.3.
Contract verification is separate from Go transport conformance.
DOC

  cat > "$TMP/repo/docs/testing/go-monitoring-foundation.md" <<'DOC'
# Go Monitoring Foundation Testing

Go architecture tests: 19 passed, 0 failed.
DOC

  cat > "$TMP/repo/docs/testing/go-public-transport-adapter.md" <<'DOC'
# Go Public Monitoring Transport Testing

POST /monitors
GET  /monitors/{monitorId}
No application host ports are published.
The readiness path is read-only.
The internal Checker contract remains deferred.
DOC

  cat > "$TMP/repo/docs/devops/local-development.md" <<'DOC'
# Local Development

docker compose up -d db api
docker compose exec -T api /usr/local/bin/uptime-lab-migrate up
docker compose up -d --wait --wait-timeout 60
DOC

  cat > "$TMP/repo/docs/glossary.md" <<'DOC'
# Architecture Glossary

## Control Plane
Go runtime owning product/domain coordination and durable product state.

## Execution Plane
Rust runtime owning bounded probe execution.

## Canonical Documentation
Repository-owned authoritative architecture documentation.
DOC

  cat > "$TMP/repo/docs/architecture/README.md" <<'DOC'
# Architecture

## Reading Order
- [System Context](system-context.md)
- [Container View](container-view.md)
- [Module Boundaries](module-boundaries.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [Runtime Flows](runtime-flows.md)
- [Change Flow](change-flow.md)
- [Architecture Decision Records](../adr/README.md)
DOC

  cat > "$TMP/repo/docs/architecture/system-context.md" <<'DOC'
# System Context

**Architecture state:** Committed

```mermaid
flowchart LR
  User --> System[uptime-lab]
  System --> Target[External HTTP/HTTPS Target]
```
DOC

  cat > "$TMP/repo/docs/architecture/container-view.md" <<'DOC'
# Container View

**Architecture state:** Committed

Go is the Control Plane. Rust is the Execution Plane. React/TypeScript is the Web Client.
Go exclusively owns durable product state in PostgreSQL.
Rust never accesses PostgreSQL directly.
The browser never accesses PostgreSQL directly.
Cross-runtime communication is contract-driven.

Public contract: defined (`contracts/openapi/public.yaml`).
Go public transport adapter: implemented.
No application host ports are published.
Internal checker contract: deferred.

```mermaid
flowchart LR
  Web -->|Public product API| Go
  Rust -->|Internal control API| Go
  Go -->|Owned persistence| DB[(PostgreSQL)]
  Rust -->|Bounded probe execution| Target
```
DOC

  cat > "$TMP/repo/docs/architecture/module-boundaries.md" <<'DOC'
# Module Boundaries

**Architecture state:** Committed

Monitoring is the initial Go business capability. The checker uses Ports and Adapters. Frontend dependency direction is app -> pages -> widgets -> features -> entities -> shared.
DOC

  cat > "$TMP/repo/docs/architecture/dependency-rules.md" <<'DOC'
# Dependency Rules

Browser -> PostgreSQL is forbidden.
Browser -> Checker internal interface is forbidden.
Rust Checker -> PostgreSQL is forbidden.
Go Domain -> infrastructure adapters is forbidden.
Go module A -> Go module B persistence adapter is forbidden.
Cross-runtime implementation-code sharing is forbidden.
DOC

  cat > "$TMP/repo/docs/architecture/data-ownership.md" <<'DOC'
# Data Ownership

Go exclusively owns durable product state in PostgreSQL.
Cross-module SQL reads are forbidden. Service extraction is not a current goal.
DOC

  cat > "$TMP/repo/docs/architecture/runtime-flows.md" <<'DOC'
# Runtime Flows

## Create Monitor
```mermaid
sequenceDiagram
  User->>Web: Configure monitor
  Web->>Go: Public contract
  Go->>DB: Persist through owned adapter
```

## Execute Due Check
```mermaid
sequenceDiagram
  Rust->>Go: Request due work
  Rust->>Target: Bounded probe
  Rust->>Go: Submit normalized result
```

## Read Current State
```mermaid
sequenceDiagram
  User->>Web: View status
  Web->>Go: Read state
  Go->>DB: Read owned state
```

## Failure Boundary
```mermaid
sequenceDiagram
  Target-->>Rust: Transport failure
  Rust->>Go: normalized probe failure
  Go-->>Web: Stable product error/state
```
DOC

  cat > "$TMP/repo/docs/architecture/change-flow.md" <<'DOC'
# Change Flow

## Canonical Example: Configure HTTP Request Timeout
Product intent -> Go domain/application -> persistence -> public/internal contracts -> frontend -> Rust execution policy -> tests -> security -> observability -> CI/documentation.
DOC

  cat > "$TMP/repo/docs/adr/README.md" <<'DOC'
# Architecture Decision Records

- [ADR-0001](0001-multi-runtime-monorepo.md)
- [ADR-0002](0002-control-plane-and-execution-plane.md)
- [ADR-0003](0003-contract-and-data-ownership.md)
DOC

  for adr in \
    0001-multi-runtime-monorepo.md \
    0002-control-plane-and-execution-plane.md \
    0003-contract-and-data-ownership.md; do
    cat > "$TMP/repo/docs/adr/$adr" <<'DOC'
# Decision

## Status
Accepted

## Context
Context.

## Decision
Decision.

## Alternatives Considered
Alternatives.

## Consequences
Consequences.

## Related Documentation
Related docs.
DOC
  done
}

make_fixture
expect_success "complete fixture passes" "$CHECKER" "$TMP/repo"

make_fixture
rm "$TMP/repo/docs/architecture/runtime-flows.md"
expect_failure "missing runtime flows fails" "$CHECKER" "$TMP/repo"

make_fixture
printf '\nTODO: unresolved placeholder\n' >> "$TMP/repo/docs/architecture/system-context.md"
expect_failure "placeholder token fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/^## Consequences$/d' "$TMP/repo/docs/adr/0001-multi-runtime-monorepo.md"
expect_failure "ADR missing Consequences fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/runtime-flows\.md/d' "$TMP/repo/docs/architecture/README.md"
expect_failure "missing architecture navigation link fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/Rust Checker -> PostgreSQL is forbidden\./d' "$TMP/repo/docs/architecture/dependency-rules.md"
expect_failure "missing Rust database prohibition fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '0,/^sequenceDiagram$/{/^sequenceDiagram$/d;}' "$TMP/repo/docs/architecture/runtime-flows.md"
expect_failure "only three sequence diagrams fails" "$CHECKER" "$TMP/repo"

make_fixture
rm "$TMP/repo/docs/backend/go-control-plane.md"
expect_failure "missing implemented Go backend documentation fails" "$CHECKER" "$TMP/repo"

make_fixture
rm "$TMP/repo/docs/testing/public-monitoring-contract.md"
expect_failure "missing public contract testing guide fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/testing\/public-monitoring-contract\.md/d' "$TMP/repo/docs/README.md"
expect_failure "missing public contract testing navigation fails" "$CHECKER" "$TMP/repo"

make_fixture
rm "$TMP/repo/docs/testing/go-public-transport-adapter.md"
expect_failure "missing public transport implementation guide fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/Go public transport adapter: implemented\./d' "$TMP/repo/docs/architecture/container-view.md"
expect_failure "missing implemented public transport marker fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/No application host ports are published\./d' "$TMP/repo/docs/architecture/container-view.md"
expect_failure "missing transport-vs-exposure distinction fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/read-only migration compatibility checker/d' "$TMP/repo/docs/backend/go-control-plane.md"
expect_failure "missing schema-aware readiness documentation fails" "$CHECKER" "$TMP/repo"

make_fixture
sed -i '/uptime-lab-migrate up/d' "$TMP/repo/docs/devops/local-development.md"
expect_failure "missing explicit migration bootstrap fails" "$CHECKER" "$TMP/repo"

make_fixture
mkdir -p "$TMP/repo/docs/frontend"
expect_failure "speculative frontend docs directory fails" "$CHECKER" "$TMP/repo"

printf '\nArchitecture documentation tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
