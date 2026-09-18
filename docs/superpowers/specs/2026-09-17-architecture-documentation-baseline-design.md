# Architecture Documentation Baseline Design

**Status:** Review candidate
**Date:** 2026-09-17
**Repository:** `kefyusuf/uptime-lab`
**Parent specification:** `docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`
**Scope:** Canonical system-level architecture documentation structure, reading order, content contracts, ADR baseline, diagram conventions, terminology, and documentation quality gates.
**Decision state:** The C4-first documentation direction was approved in discussion. This written specification requires explicit review before an implementation plan is created.

---

## 1. Purpose

The Architecture Documentation Baseline establishes a durable source of architectural truth for `uptime-lab` before the Go, Rust, React, PostgreSQL, and Docker implementation foundations are created.

The objective is not to pre-document code that does not exist. The objective is to make the system boundaries, ownership rules, cross-runtime relationships, dependency direction, data ownership, and expected end-to-end flows explicit enough that later implementation can be reviewed against a stable architecture rather than inventing architecture implicitly through code.

The baseline must answer, in a predictable reading order:

1. What system is being built?
2. Which runtime owns each responsibility?
3. Which dependencies are allowed or forbidden?
4. Who owns durable data?
5. How does a user-facing change move across web, control plane, checker, persistence, tests, CI, and operations?
6. Which decisions are intentionally fixed now, and which remain deferred until implementation creates a real need?

This baseline complements the foundation design. The foundation design defines the overall engineering direction; the architecture documentation baseline defines how that direction becomes navigable, maintainable project documentation.

---

## 2. Goals

The baseline MUST provide:

- a concise architecture index and reading order;
- a C4-style system context view;
- a C4-style container/runtime view;
- explicit runtime and module ownership boundaries;
- explicit dependency-direction rules;
- explicit durable-data ownership rules;
- canonical end-to-end runtime flows;
- a canonical cross-area change flow;
- a shared glossary for architecture terminology;
- an ADR index and a minimal set of material cross-boundary ADRs;
- Mermaid source-controlled diagrams rather than external image-only diagrams;
- links between system-level documentation and future area-specific documentation;
- language that distinguishes implemented reality from planned architecture;
- enough precision for later architecture fitness checks to be derived mechanically.

The baseline SHOULD be understandable by a contributor who has not read the source code and who does not already know the intended TypeScript/Go/Rust split.

---

## 3. Non-goals

This phase MUST NOT:

- create application source code;
- create Go, Rust, React, PostgreSQL, OpenAPI, or Docker scaffolding;
- create speculative package/crate/component diagrams for code that does not yet exist;
- define final public API endpoints;
- define final internal checker API endpoints;
- define database table columns or migrations;
- define concrete event payload schemas;
- select a production hosting platform;
- introduce microservices, brokers, Kubernetes, service mesh, or distributed tracing infrastructure;
- create empty documentation directories merely to match a future target tree;
- create separate `frontend/`, `backend/`, or `checker/` architecture documents before the corresponding runtime foundations exist;
- duplicate repository governance documentation already owned by `docs/devops/repository-governance.md`;
- create an ADR for every minor foundation statement.

The documentation must describe architecture at the level that is already decided, not invent implementation details to make the repository appear more complete.

---

## 4. Chosen approach: C4-first canonical core

The baseline uses a **C4-first canonical core**.

The system is documented from the outside inward:

```text
System purpose
    -> system context
    -> runtime/container boundaries
    -> module and dependency boundaries
    -> data ownership
    -> runtime flows
    -> cross-area change flow
    -> material ADRs
```

This ordering is intentional. A contributor should first understand *why the system is split* before learning framework- or language-specific implementation details.

### 4.1 Why this approach

Three approaches were considered.

#### A. C4-first canonical core — selected

Document the system context and stable cross-runtime boundaries now. Add runtime-specific documentation when real runtime structures exist.

Benefits:

- minimizes speculative documentation;
- prevents frontend/backend/checker documentation silos;
- gives later implementation a stable review target;
- keeps architecture readable without requiring knowledge of framework internals;
- allows future area-specific docs to link back to one canonical system model.

Cost:

- some detailed runtime documentation is deliberately deferred.

#### B. Full documentation tree immediately — rejected

Create every document named in the long-term documentation target now.

Rejected because it would require describing packages, contracts, testing mechanics, and operational details before those structures exist. This would increase documentation drift and create false confidence.

#### C. Runtime-first documentation — rejected

Write Go, Rust, and React architecture independently and connect them later.

Rejected because it creates exactly the silo behavior this repository is designed to avoid: each runtime can become internally coherent while cross-runtime ownership becomes ambiguous.

---

## 5. Baseline documentation topology

The implementation of this design will create or modify only the documents needed for the canonical architecture core:

```text
docs/
├── README.md
├── glossary.md
│
├── architecture/
│   ├── README.md
│   ├── system-context.md
│   ├── container-view.md
│   ├── module-boundaries.md
│   ├── dependency-rules.md
│   ├── data-ownership.md
│   ├── runtime-flows.md
│   └── change-flow.md
│
└── adr/
    ├── README.md
    ├── 0001-multi-runtime-monorepo.md
    ├── 0002-control-plane-and-execution-plane.md
    └── 0003-contract-and-data-ownership.md
```

No additional architecture documents are created in this phase unless review identifies a requirement that cannot be represented clearly in the files above.

---

## 6. Canonical reading order

The architecture index MUST prescribe this reading order:

```text
Repository README
    -> docs/README.md
    -> architecture/README.md
    -> architecture/system-context.md
    -> architecture/container-view.md
    -> architecture/module-boundaries.md
    -> architecture/dependency-rules.md
    -> architecture/data-ownership.md
    -> architecture/runtime-flows.md
    -> architecture/change-flow.md
    -> related ADRs
```

The purpose of the order is progressive disclosure:

- `system-context.md` explains the product and external actors;
- `container-view.md` explains runtime responsibilities;
- `module-boundaries.md` explains ownership inside those runtimes at a conceptual level;
- `dependency-rules.md` explains allowed direction;
- `data-ownership.md` explains durable-state ownership;
- `runtime-flows.md` explains how the pieces collaborate;
- `change-flow.md` explains how engineering changes propagate across the system;
- ADRs explain why expensive-to-reverse decisions were made.

Area-specific documentation introduced later must link back to the relevant canonical document rather than redefine the same system boundary independently.

---

## 7. Documentation status semantics

Because architecture is being documented before all runtime foundations exist, every architecture document must distinguish **architectural commitment** from **implemented state**.

The baseline uses three status terms:

- **Committed** — architecture decision approved and normative for future implementation.
- **Implemented** — corresponding repository/runtime behavior exists and has verification evidence.
- **Deferred** — intentionally not designed or implemented until a named requirement becomes real.

Documents must not describe a committed-but-unimplemented design as if it were already running.

Example:

> The Rust checker **will** obtain work through the Go internal control-plane contract. Direct Rust-to-PostgreSQL access is **forbidden by committed architecture**. The checker runtime itself is not yet implemented.

This convention avoids two common documentation failures:

1. writing aspirational architecture in present tense and accidentally presenting it as current behavior;
2. refusing to document architecture until implementation exists, which allows architecture to be invented implicitly in code.

---

## 8. Content contract: `docs/glossary.md`

The glossary establishes shared vocabulary. It is normative for architecture terminology, not a general programming glossary.

It MUST define at least:

- **Control Plane** — Go runtime that owns product/domain coordination, durable state, and public/internal API semantics.
- **Execution Plane** — Rust runtime responsible for bounded, safe probe execution.
- **Web Client** — React/TypeScript browser application responsible for presentation and interaction.
- **Monitor** — conceptual configured target and policy owned by the monitoring domain.
- **Probe** — one execution attempt against a target through a protocol adapter.
- **Check Result** — normalized result reported by the execution plane to the control plane.
- **Module** — independently owned business capability inside the Go modular monolith.
- **Port** — inward-facing abstraction that defines a required external capability.
- **Adapter** — infrastructure or transport implementation of a port or boundary.
- **Domain Event** — in-process fact representing a completed domain occurrence.
- **Integration Event** — cross-process fact introduced only when reliable process-boundary delivery becomes necessary.
- **Public Contract** — API contract consumed by the browser or future external clients.
- **Internal Contract** — API contract used between controlled runtimes such as Go and Rust.
- **Architecture Fitness Function** — automated rule that detects a forbidden architectural dependency or invariant violation.
- **Canonical Documentation** — repository-owned document that is authoritative for a particular boundary or decision.

The glossary must avoid defining future technology choices that are still deferred.

---

## 9. Content contract: `docs/architecture/README.md`

The architecture README is the entry point for system design.

It MUST contain:

1. purpose of the architecture documentation;
2. canonical reading order;
3. current architecture maturity/state;
4. links to all baseline architecture documents;
5. links to ADR index;
6. explanation of Committed / Implemented / Deferred semantics;
7. ownership rule: system-level docs define cross-runtime truth; future area docs define details inside one boundary;
8. documentation-change rule: a change that materially alters architecture must update the canonical architecture doc and, when appropriate, an ADR in the same PR.

It MUST NOT become a second foundation specification. It is an index and navigation surface.

---

## 10. Content contract: `system-context.md`

This document represents the C4 Level 1 view.

### 10.1 Required content

It MUST describe:

- the purpose of `uptime-lab`;
- primary human actor: user/operator;
- repository contributor as an engineering actor only where relevant;
- monitored external HTTP/HTTPS targets;
- `uptime-lab` as one product/system boundary;
- high-level responsibilities owned inside the system versus external systems;
- major security trust boundary introduced by user-configured outbound targets.

### 10.2 Diagram

The main diagram MUST be Mermaid and remain technology-light:

```text
User -> uptime-lab -> External Target
```

The Level 1 diagram must not expose Go packages, Rust crates, React folders, PostgreSQL schemas, or CI jobs.

### 10.3 Security note

The system context must state that outbound target execution creates an SSRF/egress trust boundary and that public internet exposure is not considered safe until the later checker security controls are implemented.

---

## 11. Content contract: `container-view.md`

This document represents the C4 Level 2 runtime/container view.

### 11.1 Canonical containers

The document MUST model:

- **Web Client — React + TypeScript**
  - owns presentation and browser interaction;
  - consumes the public control-plane contract;
  - does not own durable business state;
  - never communicates directly with PostgreSQL or probe targets.

- **Control Plane — Go**
  - owns business/domain rules and coordination;
  - owns public and internal API semantics;
  - exclusively owns durable application persistence;
  - coordinates due work;
  - does not implement low-level network probing.

- **Checker — Rust**
  - obtains due work through the internal control-plane boundary;
  - performs bounded/safe protocol execution;
  - submits normalized results to the control plane;
  - does not own primary durable application state;
  - never accesses PostgreSQL directly.

- **PostgreSQL**
  - durable application store owned by the Go control plane;
  - future schemas may align with Go business-module ownership;
  - no direct browser or checker access.

- **External target**
  - outside trust boundary;
  - initially HTTP/HTTPS only.

- **Docker Compose**
  - local orchestration boundary when its separate foundation is implemented;
  - explicitly infrastructure, not a business/runtime owner.

### 11.2 Relationship labels

Relationships MUST be named by semantics, not only protocol:

```text
Web -> Go: Public product API
Rust -> Go: Internal control API
Go -> PostgreSQL: Owned persistence
Rust -> Target: Bounded probe execution
```

The document may mention HTTP as the intended initial transport but must not define endpoints before the contract phase.

---

## 12. Content contract: `module-boundaries.md`

This document describes conceptual module ownership without inventing concrete source trees beyond already approved direction.

### 12.1 Go

The Go control plane is a modular monolith.

The initial business capability is **Monitoring**.

The document MUST state that future capabilities such as incidents or notifications are examples of potential modules, not directories to create during this phase.

Each future business module must own:

- its domain model;
- its application use cases;
- required ports;
- its persistence adapter/schema responsibility;
- its domain events;
- its externally exposed application boundary.

The document MUST forbid:

- direct reads of another module's persistence tables;
- a global shared business model;
- generic cross-domain repositories;
- circular business-module dependencies.

### 12.2 Rust

The checker is not a DDD modular monolith. It is an execution runtime organized around Ports and Adapters.

The conceptual boundaries are:

- checker core;
- protocol probe adapters;
- control-plane client adapter;
- binary/composition root.

Actual Cargo crate names remain committed by the foundation design but their implementation detail is deferred to the Rust foundation phase.

### 12.3 Frontend

The frontend uses feature/domain-oriented boundaries.

The conceptual direction is:

```text
app -> pages -> widgets -> features -> entities -> shared
```

This baseline documents only dependency direction and responsibility. Concrete frontend features and state libraries remain deferred until the frontend foundation phase.

---

## 13. Content contract: `dependency-rules.md`

This document is the normative source for cross-boundary dependency direction.

### 13.1 Required allow/deny matrix

It MUST include a readable matrix covering at least:

| Consumer | Allowed dependency | Forbidden dependency |
|---|---|---|
| Web Client | Public contract / frontend-owned lower layers | PostgreSQL, Rust checker internals, Go persistence |
| Go Domain | Domain concepts | HTTP framework, PostgreSQL driver, Rust/React implementation |
| Go Application | Domain + declared ports | infrastructure implementation details |
| Go Adapters | Application/domain contracts | business rules owned only by transport/infrastructure |
| Rust Core | checker execution abstractions | reqwest/client implementation, PostgreSQL, Go internals |
| Rust Probe Adapter | Rust core ports | PostgreSQL, browser concerns |
| Rust Control Client | internal API contract + core mapping | database implementation |

### 13.2 Global forbidden edges

The document MUST explicitly forbid:

```text
Browser -> PostgreSQL
Browser -> Checker internal interface
Rust Checker -> PostgreSQL
Go Domain -> infrastructure adapters
Go module A -> Go module B persistence adapter
Cross-runtime implementation-code sharing
```

### 13.3 Future enforcement

The document MUST state that later implementation phases will translate these rules into architecture fitness functions, but this documentation phase does not invent tools before each runtime exists.

---

## 14. Content contract: `data-ownership.md`

This document defines durable-state ownership.

### 14.1 Primary rule

The Go control plane is the only runtime that owns durable product state in PostgreSQL.

The checker submits facts/results through the internal contract. The browser reads or changes product state only through the public contract.

### 14.2 Module ownership

Within Go, each business module owns its persistence boundary.

The initial intended PostgreSQL namespace:

```text
monitoring.*
```

Concrete table schemas are deferred until the Go/persistence implementation plan.

### 14.3 Cross-module access rule

A business module may interact with another module through:

1. an application-level interface, or
2. a domain/integration event when asynchronous decoupling is justified.

It may not query another module's persistence tables as an undocumented integration API.

### 14.4 Extraction consequence

The document MUST explain why this matters: a well-owned module can later be extracted into a separate process if measurable requirements justify it, without first untangling hidden database coupling.

The document must explicitly state that service extraction is **not** a current goal.

---

## 15. Content contract: `runtime-flows.md`

This document explains system behavior across runtime boundaries without locking endpoint names.

It MUST include at least four Mermaid sequence diagrams.

### 15.1 Create monitor flow

```text
User
 -> Web
 -> Go Control Plane
 -> Monitoring application/domain
 -> PostgreSQL
 <- confirmation
 <- Web
```

### 15.2 Execute due check flow

```text
Rust Checker
 -> Go internal boundary: request due work
 <- work description
 -> External Target: bounded probe
 <- protocol result
 -> Go internal boundary: submit normalized result
 -> Go monitoring/domain processing
 -> PostgreSQL: persist result/state
```

### 15.3 Read current state flow

```text
User
 -> Web
 -> Go public boundary
 -> Monitoring application
 -> PostgreSQL
 <- monitor state/history
 <- Web
```

### 15.4 Failure boundary flow

This must show that transport/probe failures are normalized before entering Go domain/application semantics, and that raw Rust transport errors are not persisted or exposed directly as browser-facing product errors.

### 15.5 Correlation context

The document MUST note that correlation/request context is intended to propagate across Go/Rust boundaries once observability implementation exists. It must not claim tracing infrastructure already exists.

---

## 16. Content contract: `change-flow.md`

`change-flow.md` is a first-class architecture artifact, not a general contribution guide.

Its purpose is to show how a product capability crosses architectural boundaries.

The baseline MUST use one concrete canonical example:

> Configure HTTP request timeout.

The flow MUST examine these questions in order:

1. **Product intent** — what user-visible behavior changes?
2. **Domain/application impact** — does Go need a new invariant or policy representation?
3. **Persistence impact** — does durable configuration change?
4. **Contract impact** — public contract, internal contract, both, or neither?
5. **Frontend impact** — where does the user configure or observe it?
6. **Checker impact** — how is the execution policy applied?
7. **Testing impact** — which layer provides the cheapest meaningful evidence?
8. **Security impact** — can the change weaken execution budgets or SSRF controls?
9. **Observability impact** — which signal should expose timeout behavior?
10. **CI/documentation impact** — which checks and canonical docs become affected?

The example must distinguish **conceptual flow** from current implementation. The timeout feature itself is not implemented by this phase.

The document MUST also provide a reusable change-impact checklist so future PR authors can reason about cross-area consequences without copying backend implementation into frontend or checker code.

---

## 17. ADR baseline

The ADR baseline intentionally contains only material decisions that are expensive to reverse and cross multiple runtime boundaries.

### 17.1 `adr/README.md`

The ADR index MUST define:

- purpose of ADRs;
- lifecycle statuses: `Proposed`, `Accepted`, `Superseded`, `Rejected`;
- file naming: `NNNN-kebab-case-title.md`;
- required sections:
  - Status
  - Context
  - Decision
  - Alternatives Considered
  - Consequences
  - Related Documentation
- rule that ordinary implementation details do not require ADRs;
- rule that a superseding ADR links both directions where practical.

### 17.2 ADR-0001 — Multi-runtime monorepo

Decision:

- one repository;
- React/TypeScript web client;
- Go control plane;
- Rust checker runtime;
- one product/release boundary initially.

Alternatives considered:

- separate repositories per runtime;
- one-language implementation;
- premature microservices repository split.

Consequences MUST include simpler cross-runtime review and atomic contract changes, balanced against a more heterogeneous toolchain.

### 17.3 ADR-0002 — Control plane and execution plane separation

Decision:

- Go owns product/domain coordination;
- Rust owns bounded network execution;
- Go does not embed low-level checker implementation;
- Rust does not own primary product persistence.

Alternatives considered:

- Go performs all probes;
- Rust owns both checking and persistence;
- checker embedded as a library inside Go.

Consequences MUST include process-boundary contract cost in exchange for clearer ownership and execution isolation.

### 17.4 ADR-0003 — Contract and data ownership

Decision:

- runtime communication is contract-driven;
- public and internal contracts are separate concepts;
- Go exclusively owns PostgreSQL durable product state;
- Rust returns results through the internal boundary;
- browser communicates only through the public boundary.

Alternatives considered:

- shared database integration between Go and Rust;
- shared cross-language implementation models;
- frontend coupling directly to storage.

Consequences MUST include explicit mapping/contract work in exchange for reduced hidden coupling and a cleaner future extraction path.

---

## 18. Diagram conventions

All baseline diagrams MUST be source-controlled Mermaid diagrams embedded in Markdown.

Rules:

- diagrams must have explanatory prose; diagrams are not self-sufficient documentation;
- node names must represent responsibilities, not transient implementation classes;
- cross-runtime arrows must be labeled by semantic relationship;
- database arrows must make ownership direction clear;
- planned infrastructure must be labeled as planned when not implemented;
- diagrams must not contain decorative detail that is not relevant to the architectural question;
- external image files are not required for the baseline;
- diagrams should remain readable in GitHub's Markdown renderer.

The architecture baseline will primarily use:

- Mermaid `flowchart` for C4-like context/container/boundary views;
- Mermaid `sequenceDiagram` for runtime flows.

A dedicated diagram-rendering toolchain is deferred until a concrete portability or validation need appears.

---

## 19. Cross-document consistency rules

The following statements are canonical and MUST remain consistent in every baseline document:

1. Go is the control plane.
2. Rust is the execution/checker plane.
3. React/TypeScript is the browser presentation/client layer.
4. Go exclusively owns durable product state in PostgreSQL.
5. Rust never accesses PostgreSQL directly.
6. Browser never accesses PostgreSQL directly.
7. Cross-runtime communication is contract-driven.
8. The initial Go business module is Monitoring.
9. Message brokers and distributed worker coordination are not part of the initial architecture.
10. Local Docker Compose is committed as the future canonical local topology but belongs to a later implementation phase.
11. Authentication is not implemented in the initial foundation; therefore the system must not be described as ready for arbitrary public internet exposure.
12. SSRF and bounded outbound execution are architectural constraints for the checker.

If a later ADR changes one of these statements, affected documents must be updated in the same architectural change.

---

## 20. Deferred documentation

The following documents are intentionally deferred.

### `component-view.md`

Deferred until real Go packages, Rust crates, and React boundaries exist. A component diagram before implementation would encode guesses as architecture.

### `api-contracts.md`

Deferred until the OpenAPI contract foundation is designed. This baseline defines contract ownership but not endpoint schemas.

### `event-model.md`

Deferred until the first domain-event implementation exists. The foundation allows in-process domain events but does not require speculative event catalogues.

### `frontend/**`

Deferred until React/TypeScript foundation creates real routing, state, API adapter, and boundary decisions.

### `backend/**`

Deferred until Go foundation creates real monitoring-domain/application/platform structures.

### `checker/**`

Deferred until the Cargo workspace and actual probe/control-plane boundaries exist.

### `testing/**`, `security/**`, `operations/**`

System-level concerns are acknowledged in this baseline. Dedicated detailed documents are introduced when their corresponding implementation foundations make them concrete enough to avoid speculative duplication.

---

## 21. Documentation maintenance policy

### 21.1 Architecture-affecting changes

A pull request must update architecture documentation when it materially changes:

- runtime ownership;
- module ownership;
- dependency direction;
- data ownership;
- contract boundaries;
- security trust boundaries;
- process topology;
- architectural guarantees represented in an ADR.

### 21.2 ADR trigger

A new ADR is required when the decision is expensive to reverse, affects multiple boundaries, or changes an existing accepted ADR.

Examples:

- introducing a broker;
- extracting a Go module into a separate service;
- allowing private-network monitoring;
- changing persistence ownership;
- selecting authentication architecture;
- selecting a production deployment platform when it constrains architecture.

### 21.3 No duplicate truths

Area-specific docs may explain implementation detail but must link to the canonical system-level rule instead of redefining it.

Example:

- `checker/architecture.md` may explain how Rust enforces bounded concurrency;
- `architecture/module-boundaries.md` remains authoritative for the fact that Rust owns execution and not durable product persistence.

---

## 22. Validation and review strategy

The documentation implementation plan must include verification for:

- Markdown text hygiene with `git diff --check`;
- broken relative links where practical with a small deterministic checker rather than a large documentation platform;
- Mermaid syntax/render compatibility where a lightweight, pinned mechanism is available without creating disproportionate CI cost;
- presence of required headings in ADRs;
- absence of forbidden placeholders such as `TODO` and `TBD` in canonical architecture docs;
- consistency of key ownership phrases across architecture documents where mechanical checks are practical;
- repository-scope check proving no application/runtime scaffold is introduced in the docs PR.

The implementation plan may add small documentation-policy scripts if tests justify them. It must not introduce a documentation site generator merely to validate Markdown.

---

## 23. Governance interaction

The repository currently contains tested automation for the desired GitHub governance settings, tracked by Issue #4.

The Architecture Documentation Baseline may be designed, planned, implemented on a short-lived branch, and opened as a pull request while Issue #4 remains open.

However, until `main-protection` is active, the approved merge policy is verified, and Issue #4 is closed:

- the Architecture Documentation Baseline pull request **MUST NOT be merged into `main`**;
- the architecture documentation implementation must not be treated as proof that repository governance is complete;
- Issue #4 remains an explicit operational blocker;
- the implementation plan must preserve the PR-first workflow and must not use the unprotected state as permission for direct-to-`main` changes.

Once the governance bootstrap is run, independently verified through the GitHub API, and Issue #4 is closed, the documentation PR may proceed through its normal CI/review/merge gate.

---

## 24. Acceptance criteria

The Architecture Documentation Baseline is complete only when all of the following are true.

### Navigation

- `docs/README.md` links to the architecture index.
- `docs/architecture/README.md` provides a canonical reading order.
- all baseline architecture documents are reachable from the index.
- ADRs are reachable from the architecture index and ADR index.

### System clarity

- the system context is understandable without source-code knowledge;
- runtime responsibilities do not overlap ambiguously;
- Go control-plane ownership is explicit;
- Rust execution-plane ownership is explicit;
- browser responsibility is explicit;
- direct Rust-to-database and browser-to-database access are explicitly forbidden.

### Modularity and data

- Monitoring is identified as the initial Go business capability without creating hypothetical module shells;
- cross-module database access is explicitly forbidden;
- future module extraction is explained as possible but not assumed;
- durable-state ownership is explicit.

### Cross-area relationships

- create-monitor, execute-check, read-state, and failure-boundary runtime flows are documented;
- the timeout change example crosses product, Go, persistence, contracts, frontend, Rust, tests, security, observability, CI, and documentation;
- future area docs are instructed to link back to canonical system-level boundaries.

### ADRs

- ADR policy/index exists;
- ADR-0001, ADR-0002, and ADR-0003 are complete and accepted;
- alternatives and consequences are recorded rather than only the selected decision.

### Quality

- all project-facing documentation is English;
- diagrams are source-controlled Mermaid;
- no `TODO`, `TBD`, placeholder, or speculative empty documentation shell exists;
- committed architecture and implemented state are distinguished;
- links and text hygiene pass the selected documentation verification checks;
- no runtime/application/Docker/API/database implementation enters the documentation PR.

---

## 25. Decisions locked by this specification

Once this written specification is approved, the following documentation decisions are locked for the baseline:

- **ADB-001:** Use a C4-first canonical architecture core rather than full-tree or runtime-first documentation.
- **ADB-002:** System-level documentation owns cross-runtime architectural truth.
- **ADB-003:** Runtime-specific documentation is deferred until corresponding runtime foundations exist.
- **ADB-004:** Baseline reading order progresses from system context to container boundaries, module/dependency/data ownership, runtime flows, change flow, then ADRs.
- **ADB-005:** Architecture documents distinguish Committed, Implemented, and Deferred state.
- **ADB-006:** Mermaid embedded in Markdown is the baseline diagram source format.
- **ADB-007:** `change-flow.md` is a first-class architecture artifact using request-timeout configuration as the canonical cross-area example.
- **ADB-008:** Only three initial cross-boundary ADRs are created: multi-runtime monorepo, control/execution-plane separation, and contract/data ownership.
- **ADB-009:** No component-level documentation is created before real implementation boundaries exist.
- **ADB-010:** No empty documentation directories or speculative module documents are created.
- **ADB-011:** Go remains the canonical owner of durable product state; this rule must be reflected consistently across all architecture documentation.
- **ADB-012:** Architecture documentation implementation remains PR-first even while the separate GitHub administration blocker is unresolved, and its PR cannot merge until Issue #4 is closed.

---

## 26. Implementation boundary

The implementation plan generated from this design will be documentation-only.

It may create or modify only:

- `docs/README.md`;
- `docs/glossary.md`;
- the baseline `docs/architecture/*.md` files defined by this spec;
- the baseline `docs/adr/*.md` files defined by this spec;
- narrowly scoped documentation-validation scripts/tests if required by the implementation plan;
- CI only if needed to execute those documentation validation checks.

It must not create application runtime scaffolding.

---

## 27. Next gate

After this specification is reviewed and explicitly approved:

1. invoke the writing-plans workflow;
2. create a detailed Architecture Documentation Baseline implementation plan;
3. keep Issue #4 visible as the separate repository-administration blocker;
4. implement the documentation baseline through a short-lived branch and PR only after the implementation plan is approved;
5. do not merge that documentation PR until Issue #4 is closed and repository governance is independently verified;
6. do not begin Docker, Go, Rust, React, OpenAPI, persistence, or product-feature implementation as part of this documentation phase.
