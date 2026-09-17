# Dependency Rules

**Architecture state:** Committed  
**Implementation state:** Normative rules; automated runtime-specific fitness functions are deferred until the corresponding code exists.

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

Database tables, generated ORM types, Rust structs, and Go domain structs are not cross-runtime contracts by default.

## Future Enforcement

Later foundation phases should translate these rules into the cheapest runtime-native fitness functions:

- Go import/dependency tests for domain/application/adapter direction and module isolation;
- frontend lint/import-boundary rules for feature-layer direction;
- Rust crate/module dependency checks that keep core isolated from adapters;
- contract compatibility checks at public/internal API boundaries.

This document intentionally does not select the exact tools before the runtime structures exist.

## Related Decisions

- [Module Boundaries](module-boundaries.md)
- [Data Ownership](data-ownership.md)
- [ADR-0002: Control plane and execution plane](../adr/0002-control-plane-and-execution-plane.md)
- [ADR-0003: Contract and data ownership](../adr/0003-contract-and-data-ownership.md)
