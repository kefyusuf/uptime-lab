# Architecture

This directory is the canonical entry point for system-level architecture in `uptime-lab`. It explains cross-runtime truth before runtime-specific implementation detail.

## Purpose

Architecture documentation defines the stable system boundary, runtime ownership, dependency direction, data ownership, cross-runtime behavior, and decisions that constrain later implementation.

Runtime-specific implementation detail belongs in area guides such as the [Go Control Plane](../backend/go-control-plane.md) and [Rust Checker](../checker/rust-checker.md).

## Architecture State Semantics

- **Committed** — approved normative architecture.
- **Implemented** — corresponding repository/runtime behavior exists with verification evidence.
- **Deferred** — intentionally delayed until a named requirement becomes real.

Historical design/plan documents may describe a pre-implementation state; current-state documents must describe the repository as it exists now.

## Canonical Reading Order

1. [System Context](system-context.md)
2. [Container View](container-view.md)
3. [Module Boundaries](module-boundaries.md)
4. [Dependency Rules](dependency-rules.md)
5. [Data Ownership](data-ownership.md)
6. [Runtime Flows](runtime-flows.md)
7. [Change Flow](change-flow.md)
8. [Architecture Decision Records](../adr/README.md)

## System Views

- [System Context](system-context.md) — product boundary, actors, external targets, and trust boundary.
- [Container View](container-view.md) — Web, Go Control Plane, Rust Checker, PostgreSQL, and current relationships.

## Boundaries and Ownership

- [Module Boundaries](module-boundaries.md) — Go module and Rust crate ownership.
- [Dependency Rules](dependency-rules.md) — allowed and forbidden dependency directions.
- [Data Ownership](data-ownership.md) — Go-exclusive durable-state ownership.

## Runtime and Change Flows

- [Runtime Flows](runtime-flows.md) — create Monitor, execute due check, read Monitor, and failure collaboration.
- [Change Flow](change-flow.md) — reusable cross-area impact model.

## Runtime-Specific Guides

- [Go Control Plane](../backend/go-control-plane.md)
- [Rust Checker](../checker/rust-checker.md)
- [Single-Checker Execution Testing](../testing/single-checker-execution-slice.md)

## Architecture Decision Records

The [ADR index](../adr/README.md) records material decisions that are expensive to reverse or cross multiple architecture boundaries.

The baseline includes:

- ADR-0001 — multi-runtime monorepo;
- ADR-0002 — Control Plane and Execution Plane separation;
- ADR-0003 — contract and data ownership.

## Documentation Ownership

System-level documents own cross-runtime architectural truth. Area-specific guides explain implementation details without redefining ownership.

For example, the Rust Checker guide explains bounded concurrency and destination policy while this architecture core remains authoritative that Rust owns execution and not durable product persistence.

## Architecture Change Policy

A pull request must update canonical architecture documentation when it materially changes runtime ownership, module ownership, dependency direction, data ownership, contract boundaries, security trust boundaries, process topology, or an accepted ADR.

A new ADR is required only for material, expensive-to-reverse cross-boundary decisions.

Architecture documentation and code must change together when implementation would otherwise make current-state documents false.
## Local Web Ownership

The implemented [React client and restricted gateway](../frontend/local-monitoring-web.md) preserve Go-owned semantics and independent snapshots. [Verification](../testing/local-monitoring-web.md) covers unit, HTTP, Docker and staged browser evidence. Canonical Compose remains private; the explicit override publishes only loopback Web. Remote release remains separate.
