# Architecture Decision Records

Architecture Decision Records (ADRs) capture material decisions that are expensive to reverse or affect multiple architecture boundaries. Routine implementation choices do not require an ADR.

## Lifecycle Statuses

- `Proposed` — under review and not yet normative.
- `Accepted` — approved and normative.
- `Superseded` — replaced by a later ADR; history remains intact.
- `Rejected` — considered and explicitly not selected.

Accepted ADRs are not silently rewritten when the architecture changes. A material replacement is recorded in a new ADR, with cross-links in both directions where practical.

## File Naming

```text
NNNN-kebab-case-title.md
```

Numbers are sequential and stable once published.

## Required Sections

Each ADR contains:

```text
Status
Context
Decision
Alternatives Considered
Consequences
Related Documentation
```

## Baseline ADRs

- [ADR-0001 — Multi-runtime monorepo](0001-multi-runtime-monorepo.md)
- [ADR-0002 — Control Plane and Execution Plane separation](0002-control-plane-and-execution-plane.md)
- [ADR-0003 — Contract and data ownership](0003-contract-and-data-ownership.md)

## ADR Trigger

Create or supersede an ADR when a decision is expensive to reverse, changes multiple boundaries, or modifies an existing accepted architecture decision. Examples include introducing a broker, changing persistence ownership, extracting a module into a service, selecting authentication architecture, or adopting a production deployment platform that constrains the system.
