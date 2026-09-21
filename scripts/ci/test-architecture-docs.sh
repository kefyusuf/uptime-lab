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
  mkdir -p "$TMP/repo/docs/architecture" "$TMP/repo/docs/adr" "$TMP/repo/docs/backend"

  cat > "$TMP/repo/docs/README.md" <<'DOC'
# Documentation

See [Architecture](architecture/README.md).
DOC

  cat > "$TMP/repo/docs/backend/go-control-plane.md" <<'DOC'
# Go Control Plane

The Go Control Plane foundation is implemented.
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
mkdir -p "$TMP/repo/docs/frontend"
expect_failure "speculative frontend docs directory fails" "$CHECKER" "$TMP/repo"

printf '\nArchitecture documentation tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
