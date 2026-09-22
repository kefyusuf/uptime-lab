# Public Monitoring API Contract Design

**Status:** Review candidate
**Date:** 2026-09-22
**Repository:** `kefyusuf/uptime-lab`
**Scope:** Public Monitoring HTTP contract semantics only
**Base:** `main@3830f51813d459e085de2d2915a1cb555d4735b0`

---

## 1. Purpose

The Go Monitoring Foundation is landed and provides real, tested application behavior for exactly two use cases:

1. `RegisterMonitor`
2. `GetMonitor`

Those use cases are deliberately not exposed through a product transport yet.

This design defines the next product-facing boundary: the first public Monitoring API contract. It does **not** implement OpenAPI files, HTTP handlers, runtime wiring, mutable lifecycle, scheduling, Checker integration, or Web code.

The goal is to lock a minimal transport contract around behavior that already exists rather than inventing new product semantics at the HTTP boundary.

---

## 2. Product/Scope Reassessment

The landed Go Monitoring Foundation left several possible next directions.

| Candidate | Decision | Reason |
|---|---|---|
| Public Monitoring contract for Register/Get | Advance now | Existing application behavior already supports it without new domain/persistence semantics. |
| Mutable lifecycle | Defer | Enable/disable still requires explicit no-op, concurrency, conflict, and persistence semantics. |
| Internal Go-to-Rust contract | Defer | Work acquisition/result submission depends on scheduler, result model, and execution coordination decisions that do not exist yet. |
| Scheduling/due-work | Defer | Interval/policy/next-due/lease semantics are unresolved. |
| Check-result/history persistence | Defer | Result normalization and product interpretation are unresolved. |
| Rust Checker runtime | Defer | Its process contract and execution policy are not yet defined. |
| React Web runtime | Defer | A stable public product contract should exist before the browser adapter is implemented. |

Therefore the next milestone is **Public Monitoring API Contract Design**.

This is intentionally narrower than a general “API foundation” milestone.

---

## 3. Existing Architectural Authority

This design remains subordinate to the landed architecture:

- Go is the Control Plane.
- Go exclusively owns PostgreSQL durable product state.
- Monitoring owns monitor product semantics.
- Browser access is public-contract-only.
- Rust access is internal-contract-only.
- Rust and browser never access PostgreSQL directly.
- Cross-runtime contracts are reviewable first-class artifacts.
- Runtime implementations conform to contracts rather than defining contracts implicitly.
- Storage rows, Go domain structs, future Rust structs, and future TypeScript models are not automatically public contract models.
- No product HTTP handler may be introduced before this contract gate lands.

The Go Monitoring Foundation additionally locks:

- immutable Monitor state with `id`, `targetUrl`, and `createdAt`;
- duplicate target URLs are allowed;
- Go-generated UUID v7 identities;
- `RegisterMonitor` and `GetMonitor` only;
- no public/internal product transport yet;
- no lifecycle mutation;
- no scheduler/results/incidents/events.

---

## 4. Milestone Boundary

### In scope

- public HTTP resource shape for the existing Monitor representation;
- create/register request semantics;
- get-by-ID semantics;
- HTTP methods and paths;
- status-code mapping;
- JSON media types;
- RFC 9457 error representation;
- UUID/timestamp representation;
- public contract versioning baseline;
- explicit duplicate/non-idempotent registration semantics;
- contract compatibility expectations;
- schema-readiness requirement that must be satisfied before product handlers later become live;
- validation/CI requirements for the future OpenAPI artifact.

### Out of scope

- creating `contracts/openapi/public.yaml` in this design PR;
- creating `contracts/openapi/internal.yaml`;
- Go HTTP product handlers;
- wiring Monitoring into `cmd/api`;
- public host-port exposure;
- React source;
- Rust source;
- authentication or authorization;
- CORS policy;
- list/search/pagination;
- update/delete;
- enable/disable;
- idempotency-key storage;
- scheduling/due-work;
- checker leases;
- result submission;
- history/current-state views;
- incidents/notifications;
- new PostgreSQL columns/tables/indexes;
- generated clients;
- SDK publication;
- rate limiting;
- OpenTelemetry;
- broker/cache infrastructure.

---

## 5. Locked Decisions

### PMAC-001 — Public and internal contracts remain separate milestones

This milestone designs only the public Monitoring contract.

The future internal Go-to-Rust contract is not created alongside it merely to complete the target repository tree.

Rationale:

- the public contract can map directly to existing `RegisterMonitor` and `GetMonitor`;
- the internal contract would need due-work, result, execution-policy, and coordination semantics that are not yet owned by application use cases;
- an empty or speculative internal contract would violate the repository's no-placeholder/no-future-shell discipline.

---

### PMAC-002 — OpenAPI remains the future source of truth

When this design later moves to contract implementation, the authoritative artifact will be:

```text
contracts/openapi/public.yaml
```

The design PR does not create that file.

The OpenAPI document will define the transport contract. Go handler structs are not the contract source of truth.

Generated clients, if ever introduced, remain derived implementation artifacts outside domain cores.

---

### PMAC-003 — OpenAPI 3.1.2 is the baseline

The initial public contract will use:

```yaml
openapi: 3.1.2
```

At design time, OpenAPI 3.2.1 is the latest published OAS release, but this milestone requires no 3.2-specific capability. The first contract therefore uses the published 3.1.2 line rather than expanding the project's OpenAPI feature baseline without a concrete product need.

The OpenAPI version may be revisited through an explicit later contract decision.

Normative references:

- https://spec.openapis.org/oas/v3.1.2.html
- https://spec.openapis.org/oas/v3.2.1.html

---

### PMAC-004 — The public surface contains exactly two product operations

The initial contract contains:

```text
POST /monitors
GET  /monitors/{monitorId}
```

No other Monitoring product path is part of the first public contract.

Specifically absent:

- `GET /monitors`;
- `PATCH /monitors/{monitorId}`;
- `PUT /monitors/{monitorId}`;
- `DELETE /monitors/{monitorId}`;
- enable/disable actions;
- history/status/check-runs endpoints.

This keeps transport capability equal to existing application capability.

---

### PMAC-005 — No URL-level version prefix yet

The initial paths are not prefixed with `/v1`.

The foundation already decided that URL-level version proliferation is unnecessary before the first stable compatibility commitment.

The OpenAPI `info.version` will identify the pre-stable contract release independently of the URI space.

The first contract implementation should start at:

```text
info.version: 0.1.0
```

This does not declare a stable public compatibility guarantee.

---

### PMAC-006 — JSON naming is explicit and transport-owned

Public JSON uses lower camel case.

The initial Monitor representation is exactly:

```json
{
  "id": "018f...",
  "targetUrl": "https://example.com/health",
  "createdAt": "2026-09-22T02:00:00Z"
}
```

Transport names do not force Go domain field names, database column names, or future TypeScript implementation names to match mechanically.

---

### PMAC-007 — The public Monitor representation mirrors only stable existing state

The response model contains exactly:

- `id`;
- `targetUrl`;
- `createdAt`.

It does not contain:

- `enabled`;
- `updatedAt`;
- version/concurrency tokens;
- interval/timeout/retry policy;
- next due time;
- last result;
- current status;
- incident state;
- checker assignment.

Those fields do not exist in current product semantics and must not be invented by transport.

---

### PMAC-008 — RegisterMonitor request contains only targetUrl

The create request is exactly one required member:

```json
{
  "targetUrl": "https://example.com/health"
}
```

The request schema rejects unknown members rather than silently accepting misspelled or speculative fields.

The target string remains governed by existing domain semantics:

- absolute URI;
- HTTP or HTTPS scheme, case-insensitive;
- hostname required;
- no embedded userinfo;
- no fragment;
- query strings and explicit ports allowed;
- accepted input text preserved.

The public contract does not claim SSRF safety.

---

### PMAC-009 — Registration remains duplicate-permitting and non-idempotent

Duplicate target URLs remain valid.

`POST /monitors` creates a new Monitor identity on every successful execution.

The first contract does not define `Idempotency-Key`, request deduplication, target uniqueness, or client-supplied monitor IDs.

This is deliberate because:

- Go-generated UUID v7 identity is already locked;
- duplicate targets are already valid product behavior;
- adding idempotency storage would introduce new persistence/product behavior beyond this contract milestone.

Clients must not assume that retrying an uncertain successful POST is deduplicated.

A later idempotency capability requires an explicit product/storage design.

---

### PMAC-010 — Successful registration returns 201 Created

A successful `POST /monitors` returns:

```text
201 Created
Content-Type: application/json
Location: /monitors/{monitorId}
```

The response body is the created Monitor representation.

The `Location` target identifies the corresponding `GET /monitors/{monitorId}` resource.

---

### PMAC-011 — Successful lookup returns 200 OK

A successful `GET /monitors/{monitorId}` returns:

```text
200 OK
Content-Type: application/json
```

The response body uses the same Monitor representation as registration.

No separate “summary” and “detail” models are introduced for one resource shape.

---

### PMAC-012 — UUID representation remains product identity, not database identity

`monitorId` is represented as a UUID string compatible with the existing MonitorID parser.

Successful responses serialize the canonical RFC 9562 UUID text representation produced by the Go UUID type.

New identities continue to be generated by the Go application as UUID v7.

The contract does not expose a database-generated row identifier.

---

### PMAC-013 — createdAt is an immutable UTC instant

`createdAt` is serialized as an RFC 3339 / OpenAPI `date-time` string.

Successful responses emit the Monitor's UTC-normalized creation instant.

The contract does not expose `updatedAt` because no mutable lifecycle exists.

---

### PMAC-014 — HTTP request/error classification is explicit

The initial status mapping is:

| Condition | Status |
|---|---:|
| Successful registration | 201 |
| Successful lookup | 200 |
| Malformed JSON | 400 |
| Invalid textual `monitorId` | 400 |
| Unsupported request media type | 415 |
| Syntactically valid JSON that violates the create request/domain contract | 422 |
| Monitor does not exist | 404 |
| Stable application persistence failure | 500 |

The current application layer exposes one generic `ErrPersistence`; therefore this contract does not falsely classify all persistence failures as temporary `503 Service Unavailable`.

A future application error taxonomy may justify a narrower 503 mapping.

HTTP 422 follows RFC 9110 semantics for content whose media type and syntax are understood but whose instructions cannot be processed.

---

### PMAC-015 — Errors use RFC 9457 Problem Details

Error responses use:

```text
Content-Type: application/problem+json
```

The first contract uses the RFC 9457 base members where applicable:

- `type`;
- `title`;
- `status`;
- `detail`;
- `instance`.

The initial contract does not invent a custom global error envelope, numeric error catalog, or validation-extension schema before a real consumer requires one.

Generic initial failures may use `about:blank`.

Human-readable `detail` must not expose PostgreSQL/pgx errors, credentials, hostnames, SQL, stack traces, or other implementation internals.

Normative reference:

- https://www.rfc-editor.org/rfc/rfc9457.html

---

### PMAC-016 — Media types stay minimal

Create requests use:

```text
Content-Type: application/json
```

Successful product responses use:

```text
application/json
```

Problem responses use:

```text
application/problem+json
```

No XML, form, multipart, vendor-specific media type, or custom content encoding is part of the first public contract.

---

### PMAC-017 — Authentication remains explicitly absent

The initial contract contains no authentication or authorization scheme.

That does **not** mean the API is approved for arbitrary public internet exposure.

The foundation already states that an unauthenticated initial build must not be presented as production-ready for unrestricted public exposure.

The first contract implementation must not invent API keys, sessions, OAuth, JWT, tenant ownership, or RBAC.

Those require a separate product/security design.

---

### PMAC-018 — CORS and browser deployment remain deferred

No CORS policy is selected in this milestone.

The React runtime is not implemented and no browser origin/deployment model is established.

The later Web/public-exposure phase must define CORS based on actual deployment topology rather than guessing origins now.

---

### PMAC-019 — Product handlers require schema-compatible readiness

The current `/readyz` proves bounded PostgreSQL connectivity only.

That was acceptable while the runtime exposed operational endpoints only.

Before product handlers later become live, readiness must also fail when the database schema required by the product runtime is not compatible with the application revision.

This design locks the semantic requirement but does not choose the implementation mechanism.

Constraints:

- API startup still MUST NOT auto-apply migrations;
- readiness MUST NOT mutate schema;
- the future implementation plan must define a deterministic compatibility check tied to repository-owned migrations;
- missing/outdated required Monitoring schema must prevent the product runtime from reporting ready.

This closes the explicit readiness question deferred by the Go Monitoring Foundation.

---

### PMAC-020 — No public host-port exposure is implied

Defining a public product contract does not change the canonical local Compose exposure model.

The contract design does not publish an application host port.

Future handler verification may use in-process tests and CI-local mechanisms until a separate local/public exposure decision justifies host networking changes.

---

### PMAC-021 — Contract compatibility is mechanically validated when the artifact lands

The future contract implementation must add dedicated, path-aware CI validation for `contracts/openapi/public.yaml`.

At minimum the implementation plan must provide evidence that:

- the document conforms to the selected OpenAPI version;
- references resolve;
- the exact allowed paths/operations are present;
- no internal-checker operations are introduced;
- schemas match the locked Monitor/create/problem shapes;
- duplicate-target semantics are not contradicted;
- no auth/security scheme is silently invented;
- the stable repository aggregate gate still behaves correctly.

The exact validator/linter is selected and pinned in the later implementation plan after compatibility with OAS 3.1.2 is verified.

No generated client is required merely to validate the contract.

---

### PMAC-022 — Runtime conformance comes after contract landing

The sequence is:

```text
public contract design
        ↓
public OpenAPI artifact + contract CI
        ↓
Go public transport adapter
        ↓
future Web consumer
```

The Go transport implementation may not be bundled into the contract-artifact PR.

The contract must be reviewable before runtime code depends on it.

---

### PMAC-023 — Internal Checker contract remains separately gated

No `contracts/openapi/internal.yaml` file is created by the public contract implementation unless a later internal-contract design has first resolved:

- due-work ownership;
- schedule representation;
- work identity;
- execution policy fields;
- lease/claim semantics, if any;
- normalized result vocabulary;
- result idempotency;
- timeout/retry ownership;
- correlation context;
- compatibility behavior.

The public contract milestone does not pre-decide those issues.

---

### PMAC-024 — No new durable product state

This design requires no migration and no schema change.

The existing table remains exactly:

```text
monitoring.monitors
├── id uuid PRIMARY KEY
├── target_url text NOT NULL
└── created_at timestamptz NOT NULL
```

No transport concern may add persistence state merely for convenience.

---

## 6. Contract Shape Summary

The future public OpenAPI baseline is conceptually:

```text
POST /monitors
  request:
    targetUrl
  responses:
    201 Monitor + Location
    400 Problem
    415 Problem
    422 Problem
    500 Problem

GET /monitors/{monitorId}
  responses:
    200 Monitor
    400 Problem
    404 Problem
    500 Problem
```

Shared schemas conceptually include:

```text
CreateMonitorRequest
Monitor
Problem
```

This section is a semantic design summary, not the OpenAPI source file.

---

## 7. Security Boundary

The public contract validates product syntax only.

A registered target is still untrusted.

The contract must not state or imply that registration means:

- DNS was resolved safely;
- private/reserved destinations were rejected at execution time;
- cloud metadata cannot be reached;
- redirect destinations were checked;
- DNS rebinding was prevented;
- execution timeout/body/concurrency limits were enforced.

Those remain future Rust Execution Plane responsibilities at probe time.

Problem responses must not expose internal infrastructure details.

The unauthenticated contract remains non-production-facing until a separate exposure/authentication design says otherwise.

---

## 8. Verification Strategy for the Future Contract Artifact

The later contract implementation must be reviewable without Go transport code.

Expected evidence layers:

| Layer | Evidence |
|---|---|
| File scope | only approved contract/CI/docs paths |
| OpenAPI syntax/schema | pinned validator against OAS 3.1.2 |
| Semantic shape | repository-owned checks for exact paths/operations/key schemas |
| Regression | negative fixtures for forbidden operations/fields where practical |
| Security | no security scheme invented; no SSRF-safety overclaim |
| Architecture | public/internal contracts remain separate |
| Repository CI | path-aware contract job + stable aggregate gate |
| Documentation | contract status and pre-production limitations are explicit |

Runtime conformance tests belong to the later Go transport-adapter phase.

---

## 9. Explicit Non-Decisions

This design does not decide:

- list pagination;
- display names;
- mutable monitor configuration;
- enable/disable;
- optimistic concurrency/ETags;
- idempotency-key support;
- auth/authz;
- user/tenant ownership;
- CORS;
- rate limiting;
- scheduling interval;
- timeout/retry product policy;
- work leasing;
- checker result vocabulary;
- check history;
- status/incident derivation;
- retention;
- notification behavior;
- production ingress/domain/TLS;
- public SDK generation;
- internal OpenAPI shape.

Each requires a concrete later gate.

---

## 10. Implementation Gate

This PR is design-only.

Its main-to-head diff must contain exactly this specification file.

It MUST NOT add:

```text
contracts/**
apps/api/**
apps/web/**
apps/checker/**
migrations/**
compose.yaml
.github/workflows/**
package.json
Cargo.toml
go.mod
go.work
```

No OpenAPI artifact, runtime source, CI implementation, Docker change, or product handler may be introduced on this branch.

After this design is reviewed and landed, the next gate is a separate **Public Monitoring API Contract implementation plan**.

That plan will define the exact tasks for:

1. `contracts/openapi/public.yaml`;
2. contract validation/fitness tooling;
3. path-aware CI integration;
4. canonical documentation update;
5. whole-contract review and landing.

It will still not implement Go product HTTP handlers unless a later transport-adapter design/plan explicitly authorizes them.

---

## 11. Design Exit Criteria

This gate is GREEN only if review agrees that:

1. the public contract milestone maps only existing Register/Get behavior;
2. public and internal contracts remain separate;
3. no lifecycle/scheduling/result semantics leak into the public contract;
4. no new database state is required;
5. duplicate target URLs remain allowed;
6. registration remains explicitly non-idempotent until separately designed;
7. the initial public paths are only POST /monitors and GET /monitors/{monitorId};
8. response Monitor shape is exactly id/targetUrl/createdAt;
9. error mapping does not overclaim persistence availability classification;
10. RFC 9457 is used rather than a custom global error envelope;
11. auth/CORS/public exposure remain explicitly deferred;
12. product readiness requires schema compatibility before handlers later become live;
13. OpenAPI 3.1.2 is the selected baseline;
14. no generated clients or transport implementation are included;
15. future contract CI is required before the artifact can land.

---

## 12. Self-Review

### Scope alignment

PASS.

The milestone exposes no new domain capability. It designs a transport contract only for application behavior already implemented.

### Product sequencing

PASS.

Public Register/Get can be specified now. Internal Checker work/result semantics remain deferred until their owning application behavior exists.

### Architecture / ADR consistency

PASS.

Go remains Control Plane and sole PostgreSQL owner. Browser access remains public-contract-only. Rust/browser database access remains forbidden.

### Dependency direction

PASS.

No runtime package dependency changes occur in this design. The future contract remains an external boundary rather than a domain model package.

### YAGNI

PASS.

No list/update/delete/lifecycle, idempotency store, auth, CORS, generated client, framework, broker, or speculative internal contract is introduced.

### Persistence safety

PASS.

No migration or schema mutation is required. Generic persistence failures map to 500 rather than being incorrectly promised as transient 503 failures.

### HTTP semantics

PASS.

201/Location, 400, 404, 415, 422, and 500 are used for their intended categories. Product validation remains distinct from malformed transport syntax.

### Error-contract safety

PASS.

RFC 9457 provides the common error representation. No implementation error strings become public contracts.

### Operational safety

PASS.

The design closes the deferred schema-readiness question before future product handlers can report ready against an incompatible database, while preserving explicit migrations and no auto-migration.

### Security

PASS.

Target registration remains syntactic only, authentication is not invented, and the unauthenticated contract is explicitly not approved for arbitrary public exposure.

### Standards choice

PASS.

OAS 3.1.2 is sufficient for the required JSON Schema/OpenAPI surface. The 3.2 feature line is not adopted without a concrete product need.

### Diff hygiene

PASS by design.

The branch must contain exactly this design document.

### Execution safety

PASS.

No contract artifact or runtime implementation may begin from this design branch. A separate reviewed implementation plan is required after landing.
