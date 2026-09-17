# ADR-0002: Control Plane and Execution Plane separation

## Status

Accepted

## Context

The system needs product/domain coordination and durable state, but it also needs concurrent outbound network execution with strict resource and security boundaries. Combining these responsibilities in one architectural layer would make product rules, persistence, and low-level probe behavior harder to evolve independently.

## Decision

Separate the responsibilities by runtime:

- Go is the Control Plane and owns product/domain coordination, public/internal API semantics, scheduling/due-work coordination, and durable product state.
- Rust is the Execution Plane and owns bounded network probe execution and normalization of transport/protocol outcomes.
- Go does not embed low-level checker implementation as product/domain logic.
- Rust does not own primary product persistence or product policy.

The runtimes communicate through the internal contract.

## Alternatives Considered

### Go performs all probes

Rejected because it would collapse execution-isolation concerns into the Control Plane and remove the deliberate systems-runtime boundary.

### Rust owns checking and persistence

Rejected because it would split product state ownership or force the browser/product API to coordinate with multiple durable-state owners.

### Checker embedded as a library inside Go

Rejected because it would erase the process/runtime contract boundary and increase implementation coupling between product orchestration and probe execution.

## Consequences

Benefits:

- product rules and persistence remain in one Control Plane;
- execution/resource/security concerns remain localized to the Checker;
- each runtime can use idiomatic internal architecture;
- the checker can evolve protocol adapters without becoming a second product-state owner.

Costs:

- a process-boundary contract must be designed, versioned, and tested;
- work/result mapping is explicit rather than implicit function calls;
- local development and observability must eventually correlate behavior across two backend runtimes.

## Related Documentation

- [Container View](../architecture/container-view.md)
- [Module Boundaries](../architecture/module-boundaries.md)
- [Runtime Flows](../architecture/runtime-flows.md)
- [ADR-0003 — Contract and data ownership](0003-contract-and-data-ownership.md)
