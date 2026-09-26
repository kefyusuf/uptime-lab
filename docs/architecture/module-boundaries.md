# Module Boundaries

**Architecture state:** Committed

**Implementation state:** Partial. The initial Go Monitoring module and platform runtime are implemented. Rust and React runtime boundaries remain architectural commitments only.

## Purpose

This document defines ownership boundaries inside each runtime while distinguishing implemented code from deferred responsibilities.

## Go Control Plane

The Go Control Plane is a modular monolith. Business capability code lives under module-owned boundaries; platform runtime concerns remain separate.

The implemented Monitoring source tree contains:

~~~text
internal/modules/monitoring/
├── domain/
├── application/
├── ports/
├── adapters/postgres/
└── module.go

internal/platform/
├── config/
├── database/
├── httpserver/
└── observability/
~~~

The dependency direction is enforced by repository-owned Go architecture fitness checks.

### Domain

Monitoring domain code currently owns:

- MonitorID;
- TargetURL;
- immutable Monitor;
- creation-time validation;
- UTC creation-time normalization.

TargetURL validation is syntactic only. It accepts absolute HTTP/HTTPS targets without userinfo or fragments and preserves the accepted original string. It performs no DNS/network resolution and makes no execution-safety claim.

### Application

Monitoring currently exposes exactly two use cases:

1. RegisterMonitor
2. GetMonitor

RegisterMonitor validates the target, receives a MonitorID from an injected ID generator, receives time from an injected clock, constructs one immutable Monitor, and calls repository Create.

GetMonitor retrieves one monitor by identity and maps persistence failures into stable application errors.

There is no enable/disable, update, delete, list, scheduling, due-work, result submission, history, or incident transition use case.

### Ports

The implemented persistence capability is intentionally narrow:

~~~text
MonitorRepository
├── Create(ctx, monitor)
└── ByID(ctx, id)
~~~

There is no generic Repository[T], Save method, Unit of Work, transaction manager, or global persistence base abstraction.

### PostgreSQL Adapter

The PostgreSQL adapter depends inward on Monitoring domain and ports and uses pgx only at the infrastructure boundary.

It owns hand-written SQL for monitoring.monitors and maps pgx.ErrNoRows into the module-owned not-found signal. Other database failures remain infrastructure errors and do not become application contracts.

### Module Composition

monitoring.Module groups RegisterMonitor and GetMonitor when supplied with:

- MonitorRepository;
- application ID generator;
- Clock.

It does not create platform resources or read environment variables.

The production `cmd/api` binary now constructs the Monitoring module with the PostgreSQL repository and server-owned identity/time dependencies, then composes the Monitoring HTTP adapter outside the module. The module itself still creates no platform resources and reads no environment configuration.

## Deferred Monitoring Responsibilities

The broader committed Monitoring capability will eventually include lifecycle, scheduling/due-work coordination, normalized result interpretation, and additional persistence behavior.

Those responsibilities are **not implemented** in the current foundation.

Before mutable lifecycle is added, concurrency, idempotency/no-op behavior, and persistence semantics must be designed explicitly.

## Rust Checker

The Rust Checker is not implemented yet. The future execution runtime remains organized around Ports and Adapters and will own bounded network execution.

Rust adapters must not own product/domain decisions and must never access PostgreSQL directly.

## Frontend

The React/TypeScript runtime is not implemented yet.

The committed frontend dependency direction remains:

~~~text
app -> pages -> widgets -> features -> entities -> shared
~~~

No frontend package tree is created merely to mirror this architecture before the runtime exists.

## Cross-Boundary Rules

The following rules are normative:

- business logic does not live in Go HTTP handlers;
- Go modules do not read another module's persistence tables directly;
- there is no global shared business model package;
- generic cross-domain repositories are forbidden;
- circular business-module dependencies are forbidden;
- Rust adapters do not own product/domain decisions;
- frontend components do not become persistence or network-probe owners;
- cross-runtime implementation source is not shared as an integration mechanism.

## Extension Without Premature Distribution

The modular monolith preserves a future extraction path, but service extraction is not a current goal.

Strong in-process module boundaries are preferred over premature distribution.

## Related Decisions

- [Container View](container-view.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [Go Control Plane](../backend/go-control-plane.md)
- [ADR-0001: Multi-runtime monorepo](../adr/0001-multi-runtime-monorepo.md)
- [ADR-0002: Control plane and execution plane](../adr/0002-control-plane-and-execution-plane.md)
