# Dependency Rules

**Architecture state:** Committed

**Implementation state:** Normative rules with Go dependency fitness, public-contract semantic fitness, and public Monitoring transport boundary fitness implemented. Frontend and Rust enforcement remain deferred until those runtimes exist.

## Purpose

This document is the canonical source for dependency direction across `uptime-lab`. It distinguishes architectural dependencies from transport choices so future code can change internally without weakening ownership boundaries.

## Allowed and Forbidden Dependencies

| Consumer | Allowed dependency | Forbidden dependency |
|---|---|---|
| Web Client | Public contract, frontend-owned lower layers | PostgreSQL, Rust checker internals, Go persistence adapters |
| Go Domain | Domain concepts and domain-owned value types | HTTP framework, PostgreSQL driver, Rust implementation, React implementation |
| Go Application | Domain + declared ports + module application boundaries | Concrete persistence/network infrastructure implementation |
| Go Adapters | Application/domain contracts they implement or invoke | Owning product rules only inside transport/infrastructure code |
| Rust Core | Checker execution abstractions and normalized probe concepts | HTTP client implementation types, PostgreSQL, Go internals |
| Rust Probe Adapter | Rust core probe ports | PostgreSQL, browser concerns, Go persistence |
| Rust Control Client | Internal contract + mapping to/from core concepts | Database implementation, browser implementation |
| PostgreSQL Adapter | Go-owned persistence ports and schema responsibility | Direct calls from browser or Rust |

## Global Forbidden Edges

These statements are deliberately phrased as machine-checkable invariants:

```text
Browser -> PostgreSQL is forbidden.
Browser -> Checker internal interface is forbidden.
Rust Checker -> PostgreSQL is forbidden.
Go Domain -> infrastructure adapters is forbidden.
Go module A -> Go module B persistence adapter is forbidden.
Cross-runtime implementation-code sharing is forbidden.
```

## Go Dependency Direction

The intended direction inside a Go business module is:

```text
HTTP / infrastructure adapters
        -> application use cases
        -> domain
```

Ports are declared inward and implemented outward. Domain code must not depend on HTTP framework types, environment variables, database-driver types, loggers, or Rust/React implementation details.

Cross-module behavior is invoked through an application-level interface or an explicit event boundary. Persistence adapters are never treated as module APIs.

## Rust Dependency Direction

Checker core defines execution abstractions. Protocol and control-plane adapters depend inward on those abstractions. Core must not import concrete HTTP client concerns, database drivers, or Go implementation code.

The checker is allowed to know internal contract representations only at the control-plane adapter boundary, where mapping isolates core execution concepts from transport schema changes.

## Frontend Dependency Direction

The committed frontend direction is:

```text
shared <- entities <- features <- widgets <- pages <- app
```

Imports flow toward lower layers. This rule is conceptual until the React foundation creates real source paths and an enforceable lint configuration.

## Cross-Runtime Boundary

Cross-runtime communication is contract-driven. Contracts describe data exchanged between processes; they do not authorize sharing implementation packages across TypeScript, Go, and Rust.

Current boundary state:

- the public Monitoring contract is defined at `contracts/openapi/public.yaml`;
- the Go public transport adapter is implemented for exactly `POST /monitors` and `GET /monitors/{monitorId}`;
- the internal Go/Rust Checker contract remains deferred;
- public network exposure remains deferred even though the Go transport exists.

Database tables, generated ORM types, Rust structs, and Go domain structs are not cross-runtime contracts by default.

## Enforcement State

The repository applies the cheapest available fitness functions as each boundary becomes real:

- Go import/dependency tests enforce domain/application/adapter direction and module isolation, including platform -> Monitoring and Monitoring HTTP -> PostgreSQL/platform prohibitions;
- public Monitoring contract semantics are enforced by repository-owned contract fitness checks over Redocly-bundled JSON;
- frontend lint/import-boundary rules remain deferred until React source exists;
- Rust crate/module dependency checks remain deferred until Rust source exists;
- internal-contract compatibility checks remain deferred because that contract does not exist yet.

This document does not preselect tools for deferred runtime structures.

## Related Decisions

- [Module Boundaries](module-boundaries.md)
- [Data Ownership](data-ownership.md)
- [ADR-0002: Control plane and execution plane](../adr/0002-control-plane-and-execution-plane.md)
- [ADR-0003: Contract and data ownership](../adr/0003-contract-and-data-ownership.md)
