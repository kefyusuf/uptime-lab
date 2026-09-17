# Data Ownership

**Architecture state:** Committed  
**Implementation state:** Ownership is committed; concrete schemas, tables, and migrations are deferred to the persistence foundation.

## Primary Ownership Rule

Go exclusively owns durable product state in PostgreSQL.

PostgreSQL is a persistence mechanism behind Go-owned boundaries. It is not a shared integration surface among the Web Client, Control Plane, and Checker.

## Runtime Access

### Web Client

The browser reads or changes product state through the public Go contract. It never accesses PostgreSQL directly and does not derive product state by reading database representations.

### Rust Checker

The checker obtains work and submits normalized results through the internal Go contract. It never accesses PostgreSQL directly, even if direct database access could appear operationally simpler.

### Go Control Plane

Go owns transaction boundaries, schema access, persistence mapping, migrations, and the interpretation of durable product state.

## Go Module Ownership

Within the modular monolith, each business module owns its persistence boundary. A module's tables are implementation detail of that module, not a cross-module API.

A module may not query another module's tables to bypass the owning module's application boundary.

Cross-module SQL reads are forbidden unless a future accepted architecture decision explicitly replaces this rule with a new integration model.

## Cross-Module Integration

A business module may interact with another module through:

1. a declared application-level interface for synchronous behavior; or
2. a domain/integration event when asynchronous decoupling is justified by an actual requirement.

An integration event is not automatically required merely because modules are separate. In-process application boundaries are preferred while the system remains a modular monolith.

## Initial Monitoring Namespace

The initial intended PostgreSQL namespace is:

```text
monitoring.*
```

This names ownership, not a final schema. Concrete tables, columns, indexes, migration tooling, and data-retention policies belong to the Go/persistence implementation plan.

## Extraction Consequence

Clear data ownership preserves an extraction path. If Monitoring or another module later requires an independent process, its state ownership and integration boundary are already identifiable instead of being hidden in cross-module SQL coupling.

That option is a consequence of good boundaries, not a roadmap commitment: service extraction is not a current goal.

## Non-goals

This document does not define:

- table or column schemas;
- migration syntax or migration tooling;
- transaction isolation level;
- read replicas, partitioning, sharding, or database clustering;
- event sourcing;
- a broker or outbox implementation;
- retention or archival policy.

Those decisions require concrete workload or product requirements.

## Related Decisions

- [Container View](container-view.md)
- [Module Boundaries](module-boundaries.md)
- [Dependency Rules](dependency-rules.md)
- [ADR-0003: Contract and data ownership](../adr/0003-contract-and-data-ownership.md)
