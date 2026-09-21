# Data Ownership

**Architecture state:** Committed

**Implementation state:** Implemented for the initial Monitoring create/read persistence surface. Broader product schemas remain deferred.

## Primary Ownership Rule

Go exclusively owns durable product state in PostgreSQL.

PostgreSQL is a persistence mechanism behind Go-owned boundaries. It is not a shared integration surface among the Web Client, Control Plane, and Checker.

## Runtime Access

### Web Client

The React runtime is not implemented yet. The future browser reads or changes product state through the public Go contract and never accesses PostgreSQL directly.

### Rust Checker

The Rust runtime is not implemented yet. The future checker obtains work and submits normalized results through the internal Go contract and never accesses PostgreSQL directly.

### Go Control Plane

Go owns schema access, persistence mapping, migrations, and interpretation of durable product state.

The implemented production API runtime creates a PostgreSQL pool but exposes no product data endpoints yet.

## Monitoring Schema

The initial module-owned namespace is implemented:

~~~text
monitoring
~~~

The current application table is exactly:

~~~sql
CREATE TABLE monitoring.monitors (
    id uuid PRIMARY KEY,
    target_url text NOT NULL,
    created_at timestamptz NOT NULL
);
~~~

The schema intentionally does **not** include:

- enabled;
- updated_at;
- version/concurrency columns;
- target_url uniqueness;
- scheduling/due-work tables;
- check_runs;
- monitor_states;
- incidents.

Duplicate target URLs are allowed.

## Persistence Boundary

Monitoring owns a create/read repository port and a concrete pgx adapter.

The adapter:

- inserts the application-assigned UUID;
- reads UUID text and reconstructs MonitorID explicitly;
- preserves TargetURL text;
- preserves creation instants with UTC domain semantics;
- maps no-row behavior into a Monitoring-owned not-found signal;
- does not expose pgx errors as application contracts.

No update/delete/list query or transaction abstraction exists yet.

## Migrations

Versioned SQL migrations live under apps/api/migrations and are executed through the separate uptime-lab-migrate binary.

Supported migration commands are:

~~~text
up
down
status
~~~

API startup does **not** auto-apply migrations.

The migration command uses pgx/libpq-compatible PostgreSQL configuration and goose as a library. Migration metadata is platform infrastructure and is not Monitoring business state.

The current migration is real-PostgreSQL verified for up/down/up behavior.

## Go Module Ownership

Within the modular monolith, a business module's tables are implementation details of that module, not a cross-module API.

A module may not query another module's tables to bypass the owning module's application boundary.

Cross-module SQL reads remain forbidden unless a future accepted architecture decision replaces this rule.

## Cross-Module Integration

A future business module may interact through:

1. a declared application-level interface; or
2. a domain/integration event when asynchronous decoupling is justified by a concrete requirement.

An event bus is not required merely because modules are separate. No Kafka, RabbitMQ, Redis broker, outbox, or event-sourcing infrastructure is part of this foundation.

## Extraction Consequence

Clear ownership preserves a future extraction path. That option is a consequence of modular boundaries, not a roadmap commitment.

## Deferred Persistence Decisions

The current foundation does not define:

- mutable lifecycle concurrency semantics;
- read replicas;
- partitioning/sharding;
- retention/archival policy;
- product history/check-result schemas;
- broker/outbox topology;
- production deployment migration orchestration.

Those require concrete product/workload requirements.

## Related Decisions

- [Container View](container-view.md)
- [Module Boundaries](module-boundaries.md)
- [Dependency Rules](dependency-rules.md)
- [Go Control Plane](../backend/go-control-plane.md)
- [ADR-0003: Contract and data ownership](../adr/0003-contract-and-data-ownership.md)
