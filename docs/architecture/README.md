# Architecture

This directory is the canonical entry point for system-level architecture in `uptime-lab`. It explains cross-runtime truth before runtime-specific implementation detail.

## Purpose

Architecture documentation defines the stable system boundary, runtime ownership, dependency direction, data ownership, cross-runtime behavior, and the decisions that constrain later implementation.

It does not replace runtime-specific documentation. Future frontend, backend, checker, testing, security, and operations documents must link back to the relevant canonical rule here rather than redefine it independently.

## Architecture State Semantics

- **Committed** — approved normative architecture that future implementation must follow.
- **Implemented** — corresponding repository/runtime behavior exists with verification evidence.
- **Deferred** — intentionally delayed until a named requirement becomes real.

A document may describe committed architecture before implementation exists, but it must not present planned behavior as current runtime fact.

## Canonical Reading Order

Read the system from outside inward:

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
- [Container View](container-view.md) — Web Client, Go Control Plane, Rust Execution Plane, PostgreSQL, and semantic relationships.

## Boundaries and Ownership

- [Module Boundaries](module-boundaries.md) — conceptual Go, Rust, and frontend ownership boundaries.
- [Dependency Rules](dependency-rules.md) — allowed and forbidden dependency directions.
- [Data Ownership](data-ownership.md) — durable-state ownership and cross-module persistence rules.

## Runtime and Change Flows

- [Runtime Flows](runtime-flows.md) — canonical create-monitor, execute-check, read-state, and failure-boundary collaboration.
- [Change Flow](change-flow.md) — reusable cross-area impact model using HTTP request timeout configuration as the canonical example.

## Architecture Decision Records

The [ADR index](../adr/README.md) records material decisions that are expensive to reverse or cross multiple architecture boundaries.

The baseline includes:

- ADR-0001 — multi-runtime monorepo;
- ADR-0002 — Control Plane and Execution Plane separation;
- ADR-0003 — contract and data ownership.

## Documentation Ownership

System-level documents in this directory own cross-runtime architectural truth. Area-specific documents may explain implementation details inside one boundary but must not redefine system ownership, data ownership, or dependency direction.

Example: a future `checker/architecture.md` may explain how Rust enforces bounded concurrency, while this architecture core remains authoritative for the rule that Rust owns execution and does not own durable product persistence.

## Architecture Change Policy

A pull request must update the canonical architecture documentation when it materially changes runtime ownership, module ownership, dependency direction, data ownership, contract boundaries, security trust boundaries, process topology, or an accepted ADR.

A new ADR is required when a decision is expensive to reverse, affects multiple boundaries, or supersedes an accepted ADR. Ordinary implementation detail does not require an ADR.

Architecture documentation and code must change together when the implementation would otherwise make the canonical documents false.
