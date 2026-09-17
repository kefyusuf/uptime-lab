# Runtime Flows

**Architecture state:** Committed  
**Implementation state:** Conceptual cross-runtime flows; product runtimes are not implemented yet.

## Purpose

This document defines the canonical collaboration patterns between the Web Client, Go Control Plane, Rust Execution Plane, PostgreSQL, and external targets. It intentionally avoids endpoint names and transport DTOs; concrete contracts belong to the later OpenAPI foundation.

## Create Monitor

```mermaid
sequenceDiagram
    actor User
    participant Web as Web Client
    participant Go as Go Control Plane
    participant Monitoring as Monitoring Application/Domain
    participant DB as PostgreSQL

    User->>Web: Configure HTTP/HTTPS monitor
    Web->>Go: Submit through public contract
    Go->>Monitoring: Execute monitor registration use case
    Monitoring->>DB: Persist through owned persistence boundary
    DB-->>Monitoring: Persisted
    Monitoring-->>Go: Monitor accepted
    Go-->>Web: Stable product response
    Web-->>User: Show configured monitor
```

The browser never persists monitor state directly. The Go Control Plane is the durable-state owner and the Monitoring capability mediates product semantics before persistence.

## Execute Due Check

```mermaid
sequenceDiagram
    participant Rust as Rust Checker
    participant Go as Go Control Plane
    participant Target as External Target
    participant Monitoring as Monitoring Application/Domain
    participant DB as PostgreSQL

    Rust->>Go: Request due work through internal contract
    Go-->>Rust: Work description
    Rust->>Target: Execute bounded probe
    Target-->>Rust: Protocol result
    Rust->>Go: Submit normalized result
    Go->>Monitoring: Apply result to product semantics
    Monitoring->>DB: Persist result/state
```

Rust owns bounded execution, not scheduling truth or product persistence. Work descriptions and normalized results cross the process boundary through the internal contract.

## Read Current State

```mermaid
sequenceDiagram
    actor User
    participant Web as Web Client
    participant Go as Go Control Plane
    participant Monitoring as Monitoring Application/Domain
    participant DB as PostgreSQL

    User->>Web: Open monitor status/history
    Web->>Go: Query through public contract
    Go->>Monitoring: Execute read use case
    Monitoring->>DB: Read owned monitor state/history
    DB-->>Monitoring: Durable state
    Monitoring-->>Go: Product view data
    Go-->>Web: Stable public response
    Web-->>User: Render current state/history
```

Read ownership follows the same rule as writes: browser access remains contract-driven and PostgreSQL is never a browser-facing integration surface.

## Failure Boundary

```mermaid
sequenceDiagram
    participant Target as External Target
    participant Rust as Rust Checker
    participant Go as Go Control Plane
    participant Monitoring as Monitoring Application/Domain
    participant Web as Web Client

    Target--xRust: Raw transport/protocol failure
    Rust->>Rust: Classify and normalize probe failure
    Rust->>Go: Submit normalized probe failure
    Go->>Monitoring: Interpret within product semantics
    Monitoring-->>Go: Stable monitor state/error classification
    Go-->>Web: Stable product state/error
```

A raw transport error is an Execution Plane implementation detail. It terminates at the Rust boundary. Only a normalized probe failure crosses into Go, where it is mapped into stable product semantics before anything becomes browser-visible or persistence-worthy.

## Correlation Context

Correlation/request context is a committed cross-runtime design intent. When observability implementation is introduced, the public request, internal work/result exchange, and persisted check result should be attributable to the same logical operation where appropriate.

No tracing backend, OpenTelemetry Collector, trace exporter, or concrete propagation header is implemented or selected by this documentation phase.

## Related Decisions

- [Container View](container-view.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [ADR-0002 — Control Plane and Execution Plane](../adr/0002-control-plane-and-execution-plane.md)
- [ADR-0003 — Contract and Data Ownership](../adr/0003-contract-and-data-ownership.md)
