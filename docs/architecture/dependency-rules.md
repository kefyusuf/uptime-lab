# Dependency Rules

**Architecture state:** Committed

**Implementation state:** Go, Rust and frontend dependency fitness are implemented.

## Purpose

This is the canonical source for dependency direction across `uptime-lab`.

## Allowed and Forbidden Dependencies

| Consumer | Allowed dependency | Forbidden dependency |
|---|---|---|
| Web Client | Public contract, frontend lower layers | PostgreSQL, Checker internals, Go persistence adapters |
| Go Domain | Domain concepts/value types | HTTP framework, PostgreSQL driver, Rust implementation |
| Go Application | Domain + declared ports | Concrete adapters/platform/database drivers |
| Go HTTP Adapters | Application/domain contracts | PostgreSQL adapters, platform business shortcuts |
| Rust Core | Checker execution abstractions | Concrete HTTP transport, PostgreSQL, Go internals |
| Rust Probe Adapter | Rust core probe contracts | PostgreSQL, Go persistence, browser concerns |
| Rust Control Client | Internal contract mapping + core concepts | PostgreSQL, Go implementation code |
| PostgreSQL Adapter | Go-owned persistence ports/schema | Direct browser/Rust calls |

## Global Forbidden Edges

```text
Browser -> PostgreSQL is forbidden.
Browser -> Checker internal interface is forbidden.
Rust Checker -> PostgreSQL is forbidden.
Go Domain -> infrastructure adapters is forbidden.
Go module A -> Go module B persistence adapter is forbidden.
Cross-runtime implementation-code sharing is forbidden.
```

## Go Dependency Direction

Inside Monitoring:

```text
HTTP / infrastructure adapters
        -> application use cases
        -> domain
```

Ports are declared inward and implemented outward. Domain/application code does not depend on HTTP or PostgreSQL implementation types.

The generic platform HTTP server remains Monitoring-independent; `cmd/api` owns production composition.

## Rust Dependency Direction

`checker-core` defines execution abstractions. `probe-http` and `control-plane-client` depend inward on core concepts. The `checker` crate composes them.

Rust has no PostgreSQL dependency and no database ownership.

## Frontend Dependency Direction

The committed frontend direction remains:

```text
shared <- entities <- features <- widgets <- pages <- app
```

The implemented AST checker enforces this direction, public feature/entity entrypoints, dynamic imports and browser isolation from Node/server code. No widget layer is needed for this slice.

## Cross-Runtime Boundary

Cross-runtime communication is contract-driven.

Current sources:

- public: `contracts/openapi/public.yaml`;
- internal: `contracts/openapi/internal.yaml`.

The public Go adapter implements Monitor create/read, latest terminal result and current availability. Web forwards only these four operations.

The internal Go/Rust contract implements exactly:

~~~text
POST /internal/checks/claim
PUT /internal/checks/{checkId}/result
~~~

Database tables, Go structs, and Rust structs are not cross-runtime contracts by default.

## Enforcement State

Repository fitness currently enforces:

- Go import/dependency direction;
- Rust crate/module direction and no-database ownership;
- public contract semantics;
- internal contract semantics and fixtures;
- local Compose topology;
- documentation current-state markers.

Frontend lint/import-boundary enforcement runs in the required Web CI job through `scripts/ci/check-web-architecture.mjs` and its mutation harness.

## Related Decisions

- [Module Boundaries](module-boundaries.md)
- [Data Ownership](data-ownership.md)
- [Rust Checker](../checker/rust-checker.md)
- [ADR-0002](../adr/0002-control-plane-and-execution-plane.md)
- [ADR-0003](../adr/0003-contract-and-data-ownership.md)
