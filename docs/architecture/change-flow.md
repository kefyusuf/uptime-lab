# Change Flow

**Architecture state:** Committed

**Implementation state:** Engineering impact model only; the canonical example is not implemented by this documentation phase.

## Purpose

A feature is not complete merely because one runtime compiles. This document provides a repeatable way to reason about a product change across architecture boundaries without duplicating business logic between Go, Rust, and the Web Client.

## Canonical Example: Configure HTTP Request Timeout

The example is intentionally cross-cutting: a user-configurable timeout changes product policy, durable configuration, contracts, browser interaction, checker execution budgets, tests, security posture, observability, CI, and documentation.

The timeout capability is not implemented by this documentation phase.

## Product Intent

Define the observable behavior first: a user can configure the maximum duration allowed for an HTTP/HTTPS probe, within product-defined safe bounds. The product contract must distinguish user intent from transport-library implementation details.

## Domain/Application Impact

The Go Monitoring capability decides whether timeout is part of the monitor policy and owns validation rules that are product semantics rather than Rust-library defaults. If the value affects monitor invariants, the rule belongs in Go domain/application logic rather than the HTTP handler or Rust adapter.

## Persistence Impact

If the timeout is user-configurable and durable, Go owns its persistence. A later database design may add a field or value object representation, but this document does not choose a table/column schema.

## Contract Impact

The public contract may need to accept and return the timeout configuration. The internal Go-to-Rust contract may need to carry the effective execution budget. These are separate contracts and may evolve independently even when they represent the same product policy.

## Frontend Impact

The Web Client owns presentation, input interaction, and product feedback. It may perform user-experience validation, but Go remains authoritative for product constraints. The frontend must not encode transport-library defaults as business truth.

## Checker Impact

Rust applies the effective execution budget to the probe adapter. It may map the product-level timeout into HTTP-client/runtime primitives, but it does not redefine allowed product values or persist the configuration itself.

## Testing Impact

Use the cheapest layer that proves each behavior:

- Go unit/application tests prove product validation and policy mapping.
- Contract tests prove public/internal representations remain compatible.
- Rust unit/adapter tests prove the execution budget is enforced.
- A narrow integration/E2E flow proves configured policy reaches execution and the resulting state is observable.

Avoid using E2E tests to prove every edge case already covered at a cheaper layer.

## Security Impact

Timeout is an execution budget and therefore part of the checker security model. Product validation must prevent values that effectively disable bounds or make resource exhaustion trivial. Timeout changes must be reviewed together with response-size limits, redirect policy, private/reserved-address controls, port policy, and concurrency bounds where relevant.

## Observability Impact

Operational signals should distinguish a probe that exceeded its configured execution budget from other failure categories. The exact metrics/log fields and tracing backend are deferred until observability implementation exists.

## CI and Documentation Impact

The change affects whichever runtime/contract tests own the modified behavior. CI should run only affected checks plus the stable aggregate gate. Canonical architecture or ADR documentation must change in the same PR if ownership, dependency direction, trust boundaries, or public/internal contract responsibilities materially change.

## Reusable Change-Impact Checklist

For every material product change, ask:

- **Product:** What observable behavior or invariant changes?
- **Go:** Which domain/application policy owns the decision?
- **Persistence:** Does durable state or module ownership change?
- **Public contract:** Does browser/external representation change?
- **Internal contract:** Does Go-to-Rust work/result representation change?
- **Frontend:** What presentation or interaction changes without duplicating business truth?
- **Rust:** Which execution behavior or adapter mapping changes?
- **Security:** Do execution budgets, SSRF controls, redirect behavior, ports, size limits, or concurrency change?
- **Testing:** What is the cheapest meaningful evidence for each behavior?
- **Observability:** What signal should expose the new behavior or failure mode?
- **CI:** Which affected checks must run while preserving the stable aggregate gate?
- **Canonical docs:** Which architecture documents become stale if this change lands?
- **ADR trigger:** Is the decision expensive to reverse, cross-boundary, or a change to an accepted ADR?

## Boundary Rule

A cross-area feature must be expressed through contracts and owned policies, not by sharing implementation code or reaching across persistence boundaries. The change flow is a reasoning model, not permission to couple every runtime to every feature concern.

## Related Decisions

- [Module Boundaries](module-boundaries.md)
- [Dependency Rules](dependency-rules.md)
- [Data Ownership](data-ownership.md)
- [Runtime Flows](runtime-flows.md)
