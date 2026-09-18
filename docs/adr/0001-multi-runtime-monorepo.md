# ADR-0001: Multi-runtime monorepo

## Status

Accepted

## Context

`uptime-lab` is intended to exercise clear responsibility boundaries across a browser client, product/control plane, and bounded network-execution runtime. These responsibilities benefit from different language ecosystems, but product and contract changes frequently cross more than one runtime.

Splitting each runtime into a separate repository at project inception would make atomic cross-runtime review harder and would add versioning/release coordination before independent ownership or deployment requirements exist.

## Decision

Use one repository containing:

- React/TypeScript for the Web Client;
- Go for the Control Plane;
- Rust for the Checker / Execution Plane;
- one initial product/release boundary.

The repository is polyglot, but source ownership remains runtime-specific. Cross-runtime integration happens through contracts, not shared implementation packages.

## Alternatives Considered

### Separate repositories per runtime

Rejected for the initial system because it would increase contract coordination, CI/release overhead, and cross-runtime change friction before independent team or deployment boundaries exist.

### One-language implementation

Rejected because the project deliberately separates product/control concerns from bounded systems/network execution and is intended to demonstrate idiomatic architectures in the selected runtimes.

### Premature microservices repository split

Rejected because process/repository boundaries should follow measurable deployment, scale, reliability, or ownership requirements rather than architecture fashion.

## Consequences

Positive consequences:

- one PR can review a coherent cross-runtime contract change;
- repository governance, documentation, and CI can enforce shared product invariants;
- initial releases can remain coordinated;
- architecture evolution is visible in one history.

Costs:

- CI/tooling must support multiple language ecosystems;
- contributors must understand repository-wide boundaries without treating the monorepo as permission for cross-runtime source coupling;
- path-aware CI becomes important as the repository grows.

## Related Documentation

- [System Context](../architecture/system-context.md)
- [Container View](../architecture/container-view.md)
- [Module Boundaries](../architecture/module-boundaries.md)
- [Dependency Rules](../architecture/dependency-rules.md)
