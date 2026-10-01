# Container View

**Architecture state:** Committed

**Implementation state:** Go, PostgreSQL, and the Rust Checker are implemented for the Single-Checker Execution Vertical Slice. Web remains a placeholder and public network exposure remains deferred.

## Purpose

This is the C4 Level 2 view for the current runtime.

## Containers and Responsibilities

### Web Client — React + TypeScript

The React runtime is not implemented. The canonical `web` service remains a hardened placeholder.

The browser never accesses PostgreSQL directly.

### Control Plane — Go

The Go Control Plane is implemented and owns:

- public Monitor create/read and latest terminal result semantics;
- `CheckID` and normalized result semantics;
- due-work selection and fixed cadence;
- CheckRun deadlines and `worker_timeout`;
- public Monitoring HTTP transport;
- internal Checker HTTP transport;
- PostgreSQL persistence and explicit migrations;
- schema-aware readiness and operational health.

Go exclusively owns durable product state in PostgreSQL.

### Checker / Execution Plane — Rust

Rust Checker: implemented.

The Checker owns bounded HTTP/HTTPS execution, production destination policy, protocol/network normalization, internal control-plane transport, bounded worker orchestration, and Checker lifecycle/readiness.

Rust never accesses PostgreSQL directly.

### PostgreSQL

PostgreSQL is the durable application store behind Go-owned boundaries.

The implemented Monitoring schema contains:

~~~text
monitoring.monitors
  id
  target_url
  created_at

monitoring.check_runs
  id
  monitor_id
  issued_at
  deadline_at
  completed_at
  result_kind
  http_status
  duration_ms
~~~

The database mechanically protects pending/terminal CheckRun shape and at most one pending run per Monitor.

### External HTTP/HTTPS Target

Targets are untrusted.

Go registration validation remains syntactic. Execution-time safety belongs to Rust. The production Checker rejects non-public destinations, binds connections to validated DNS results, revalidates redirects, ignores ambient proxy configuration, verifies TLS normally, and applies bounded execution budgets.

### Docker Compose

Canonical Compose runs exactly:

~~~text
web      placeholder
db       PostgreSQL
api      real Go Control Plane
checker  real Rust Checker
~~~

No application host ports are published.

API may be live but unready until migrations are explicitly applied. The real Checker starts after API readiness and uses `http://api:8080` as its internal control-plane endpoint.

## Current Runtime Diagram

~~~mermaid
flowchart LR
    Caller["Container-local caller"]
    Web[Web placeholder]
    Go["Go Control Plane<br/>public + internal HTTP"]
    Checker["Rust Checker<br/>bounded execution"]
    DB[(PostgreSQL)]
    Target["External HTTP/HTTPS target"]

    Caller -->|POST/GET Monitor + GET latest-result| Go
    Checker -->|claim/result internal contract| Go
    Go -->|owned persistence| DB
    Checker -->|validated bounded probe| Target
    Go -->|schema-aware readiness prerequisite| Checker
~~~

## Current Contract State

- Public contract: defined (`contracts/openapi/public.yaml`).
- Go public transport adapter: implemented.
- Rust Checker: implemented.
- Internal checker contract: implemented (`contracts/openapi/internal.yaml`).
- Public network exposure: deferred.

The internal contract contains exactly:

~~~text
POST /internal/checks/claim
PUT /internal/checks/{checkId}/result
~~~

Cross-runtime communication is contract-driven.

PostgreSQL is never used as a Go/Rust integration bus.

## Operational Health and Product Surface

The Go runtime serves:

~~~text
GET  /livez
GET  /readyz
POST /monitors
GET  /monitors/{monitorId}
GET  /monitors/{monitorId}/latest-result
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

`/livez` is database-independent. `/readyz` performs bounded PostgreSQL connectivity plus read-only exact migration compatibility checks.

The public latest-result endpoint exposes one latest terminal execution fact only. Full CheckRun history and derived availability/status remain deferred.

## Ownership Rules

- Web owns presentation, not persistence or probe execution.
- Go owns scheduling truth, product semantics, and durable state.
- Rust owns bounded execution, not scheduling truth or durable state.
- external targets remain untrusted.
- no browser or Checker code accesses PostgreSQL directly.

## Security Boundaries

Three trust transitions remain material:

1. product input entering Go;
2. internal work/result data crossing Go/Rust;
3. target/DNS/network behavior entering Rust.

The second and third transitions are now implemented and verified. Public authentication/authorization, CORS/rate limiting, ingress/TLS, and React remain deferred.

## Related Decisions

- [Module Boundaries](module-boundaries.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [Rust Checker](../checker/rust-checker.md)
- [ADR-0001](../adr/0001-multi-runtime-monorepo.md)
- [ADR-0002](../adr/0002-control-plane-and-execution-plane.md)
- [ADR-0003](../adr/0003-contract-and-data-ownership.md)
