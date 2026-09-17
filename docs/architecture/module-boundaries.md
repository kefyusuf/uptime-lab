# Module Boundaries

**Architecture state:** Committed  
**Implementation state:** Conceptual boundaries only; runtime source trees are not implemented yet.

## Purpose

This document defines ownership boundaries inside each runtime without inventing package, crate, or feature structures that do not yet exist. The goal is to make later implementation reviewable against stable responsibilities rather than to pre-create folders.

## Go Control Plane

The Go Control Plane is a modular monolith. A business module owns one coherent product capability and exposes behavior through an application-level boundary rather than through its persistence implementation.

Each business module is expected to own:

- its domain model and invariants;
- application use cases;
- ports required from infrastructure;
- persistence responsibility for its own state;
- domain events representing completed domain facts;
- externally exposed application behavior used by HTTP adapters or other modules.

A Go business module does not own framework bootstrapping, generic infrastructure, or another module's persistence tables.

## Monitoring

Monitoring is the initial Go business capability.

It will own monitor configuration semantics, monitor lifecycle rules, due-check coordination, normalized check-result interpretation, and the monitoring persistence boundary when the Go foundation is implemented.

This statement does not create Go directories, packages, tables, or endpoint schemas in the documentation phase.

## Future Business Modules

Capabilities such as Incidents and Notifications are examples of potential future modules. They are not implementation commitments and must not be scaffolded merely because they are plausible.

A future module is introduced only when a real product requirement justifies a separate ownership boundary.

## Rust Checker

The Rust Checker is not a DDD modular monolith. It is an execution runtime organized around Ports and Adapters.

The conceptual responsibilities are:

- **checker core** — execution policy, normalized probe concepts, and inward-facing ports;
- **protocol probe adapters** — HTTP initially, with other protocols introduced only when required;
- **control-plane client adapter** — mapping between internal contract representations and checker-core concepts;
- **binary/composition root** — configuration, dependency construction, lifecycle, and worker loop ownership.

Adapter-specific HTTP client errors, transport types, and framework/library details do not belong in checker-core semantics.

## Frontend

The Web Client uses feature/domain-oriented boundaries rather than backend-style Clean Architecture folders.

The committed dependency direction is:

```text
app -> pages -> widgets -> features -> entities -> shared
```

Higher layers may compose lower layers. Lower layers must not import from higher layers. `shared` is frontend-local technical reuse, not a cross-runtime business model.

Concrete pages, features, state-management libraries, and generated API clients remain deferred to the frontend foundation.

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

The modular monolith is intentionally designed so a business capability can later be extracted if measurable scale, deployment, reliability, or ownership requirements justify that cost.

Service extraction is not a current goal. The architecture prefers strong module boundaries inside one Control Plane process over early service decomposition.

## Related Decisions

- [Container View](container-view.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [ADR-0001: Multi-runtime monorepo](../adr/0001-multi-runtime-monorepo.md)
- [ADR-0002: Control plane and execution plane](../adr/0002-control-plane-and-execution-plane.md)
