# Module Boundaries

**Architecture state:** Committed

**Implementation state:** The Go Monitoring execution boundary and Rust Checker runtime are implemented. React remains deferred.

Current availability is exposed by `GET /monitors/{monitorId}/availability` as a read-only assessment of the latest terminal CheckRun. It returns `status`, `reason`, UTC `evaluatedAt`, and terminal `evidence` (`checkId`, `completedAt`). A known Monitor without a terminal result returns `200 unknown/no_result` with evidence omitted; the raw latest-result route retains its empty `204`. Every matched availability response, including errors and `405`, uses `Cache-Control: no-store`; `HEAD` returns `405` with `Allow: GET`. No new persistence, reconciliation, history, or public deployment is introduced.

## Purpose

This document defines ownership boundaries inside each runtime and the seam between Go and Rust.

## Go Control Plane

The Go Control Plane remains a modular monolith.

Current Monitoring source owns:

~~~text
internal/modules/monitoring/
├── domain/
├── application/
├── ports/
├── adapters/
│   ├── http/          public Monitor transport
│   ├── checkerhttp/   internal Checker transport
│   └── postgres/
└── module.go
~~~

Platform runtime remains under `internal/platform` and stays business-module independent.

### Domain

Monitoring domain owns:

- MonitorID;
- CheckID;
- TargetURL;
- immutable Monitor;
- normalized CheckResult vocabulary and invariants;
- pure Availability policy with explicit outcome and freshness reasons.

Go registration-time TargetURL validation remains syntactic. Execution-time destination safety is Rust-owned.

### Application

Implemented capabilities include:

1. RegisterMonitor
2. GetMonitor
3. ClaimDueCheck
4. SubmitCheckResult
5. GetLatestCheckResult
6. GetMonitorAvailability

Go application policy owns the fixed cadence, CheckRun deadline window, timeout/redirect work values, server-time decisions, no-work semantics, and completion/conflict mapping.

No public list/update/delete/enable/disable/history use case exists. Availability is derived on read, without a materialized status projection.

### Ports

Monitoring defines narrow persistence contracts rather than a generic repository abstraction.

The execution port exposes only atomic claim/completion capabilities needed by the application layer.

### PostgreSQL Adapter

The adapter owns hand-written SQL for `monitoring.monitors` and `monitoring.check_runs`.

Claim is transactionally bounded, uses row locking/`SKIP LOCKED`, reconciles expired pending work, and inserts one pending CheckRun. Completion terminalizes one pending run or validates an exact idempotent duplicate.

### HTTP Adapters

The public adapter serves Monitor registration/read, latest terminal result, and current availability.

The internal Checker adapter serves:

~~~text
POST /internal/checks/claim
PUT /internal/checks/{checkId}/result
~~~

Both adapters depend inward on application behavior and never own persistence/business rules.

## Rust Checker

The Rust runtime is implemented as a workspace with explicit crate boundaries:

~~~text
checker-core
probe-http
control-plane-client
checker
~~~

- `checker-core` owns worker abstractions/orchestration.
- `probe-http` owns execution-time destination policy and HTTP/HTTPS probing.
- `control-plane-client` owns internal HTTP mapping/transport.
- `checker` owns production composition, readiness marker, and stable event formatting.

Rust adapters do not own product scheduling or persistence decisions.

Rust never accesses PostgreSQL directly.

See [Rust Checker](../checker/rust-checker.md).

## Frontend

The React/TypeScript runtime is not implemented.

The committed dependency direction remains:

~~~text
app -> pages -> widgets -> features -> entities -> shared
~~~

## Cross-Boundary Rules

- business logic does not live in Go HTTP handlers;
- Go modules do not bypass another module's application boundary through SQL;
- generic cross-domain repositories are forbidden;
- Rust core does not depend on concrete control-plane/probe transports;
- Rust adapters do not own durable product semantics;
- Rust never accesses PostgreSQL;
- cross-runtime implementation source is not shared as an integration mechanism;
- Go/Rust exchange is contract-driven.

## Extension Without Premature Distribution

One logical Checker process is supported in this milestone. Multi-worker coordination, broker/outbox infrastructure, and service extraction are not current goals.

## Related Decisions

- [Container View](container-view.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [Go Control Plane](../backend/go-control-plane.md)
- [Rust Checker](../checker/rust-checker.md)
- [ADR-0001](../adr/0001-multi-runtime-monorepo.md)
- [ADR-0002](../adr/0002-control-plane-and-execution-plane.md)
