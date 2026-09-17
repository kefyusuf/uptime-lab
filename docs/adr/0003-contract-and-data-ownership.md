# ADR-0003: Contract and data ownership

## Status

Accepted

## Context

A polyglot system can become tightly coupled even when runtimes are separate if they integrate through shared database tables, generated storage models, or copied implementation types. Clear ownership requires both communication boundaries and durable-state boundaries to be explicit.

## Decision

- Cross-runtime communication is contract-driven.
- Public and internal contracts are separate concepts with different consumers and evolution pressure.
- Go exclusively owns durable product state in PostgreSQL.
- Rust obtains work and submits normalized results through the internal contract; it never accesses PostgreSQL directly.
- The browser uses the public contract only; it never accesses PostgreSQL directly.
- Storage representations, Go domain structs, Rust structs, and TypeScript models are not automatically cross-runtime contracts.

## Alternatives Considered

### Shared database integration between Go and Rust

Rejected because database schemas would become a hidden runtime API, coupling migration timing and persistence implementation to the Checker.

### Shared cross-language implementation models

Rejected because generated or copied implementation structures encourage internal representation changes to become cross-runtime breaking changes.

### Frontend-to-storage coupling

Rejected because it bypasses product/application semantics and would make authorization, validation, migrations, and domain evolution unsafe.

## Consequences

Benefits:

- durable-state ownership is unambiguous;
- migrations remain owned by the Control Plane;
- Rust and frontend implementations can evolve behind stable contracts;
- future module/service extraction is less constrained by hidden database coupling.

Costs:

- explicit mapping is required at public/internal contract boundaries;
- contract compatibility must be tested;
- duplicate-looking structures may legitimately exist in different runtimes because they serve different roles.

## Related Documentation

- [Dependency Rules](../architecture/dependency-rules.md)
- [Data Ownership](../architecture/data-ownership.md)
- [Runtime Flows](../architecture/runtime-flows.md)
- [ADR-0001 — Multi-runtime monorepo](0001-multi-runtime-monorepo.md)
- [ADR-0002 — Control Plane and Execution Plane separation](0002-control-plane-and-execution-plane.md)
