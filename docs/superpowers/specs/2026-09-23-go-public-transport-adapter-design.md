
# Go Public Transport Adapter Design

**Status:** Review candidate
**Date:** 2026-09-23
**Repository:** kefyusuf/uptime-lab
**Scope:** Go public Monitoring HTTP transport + production composition + schema-compatible readiness design only
**Base:** main@7a9c142b7ccb244486d84d047494506f07ea40ab

---

## 1. Purpose

The Public Monitoring API Contract artifact phase is landed.

The repository now has real Monitoring domain/application behavior for RegisterMonitor and GetMonitor, PostgreSQL persistence, explicit embedded goose migrations, an operational Go HTTP server, and the canonical public OpenAPI contract at contracts/openapi/public.yaml.

The public product contract is defined but not served.

This design defines the next bounded milestone: Go Public Transport Adapter.

The milestone will make the two already-contracted public operations executable through the Go Control Plane without inventing new product behavior. It also closes the schema-readiness precondition locked by PMAC-019 before product handlers become live.

This design does not implement handlers, runtime wiring, migrations, Web/Rust code, auth, CORS, lifecycle, scheduling, results, or public host exposure.

---

## 2. Product / Scope Reassessment

| Candidate | Decision | Reason |
|---|---|---|
| Go transport for POST /monitors + GET /monitors/{monitorId} | Advance now | Contract and application behavior already exist. |
| Schema-compatible readiness | Advance with transport | PMAC-019 requires it before product handlers become live. |
| Public host-port exposure | Defer | Local/public deployment topology is still deliberately undefined. |
| Authentication / authorization | Defer | No identity, tenant, ownership, or RBAC product model is designed. |
| CORS | Defer | No browser origin/deployment model exists. |
| Request rate limits / public edge policy | Defer | Arbitrary public exposure is still forbidden. |
| Mutable monitor lifecycle | Defer | Update/enable/delete product semantics remain undefined. |
| Internal Go-to-Rust contract | Defer | Scheduler/work/result semantics still do not exist. |
| Rust Checker runtime | Defer | Internal process contract remains separately gated. |
| React Web runtime | Defer | Runtime adapter should land and be verified first. |

Therefore the next milestone is a narrow Go adapter around the existing public contract, not a general API framework or a Web/API vertical slice.

---

## 3. Existing Authority

The following are not reopened:

- Go is the Control Plane.
- Go exclusively owns durable product state in PostgreSQL.
- Monitoring owns monitor product semantics.
- Public and internal contracts remain separate.
- The public OpenAPI artifact is the transport source of truth.
- Public operations are exactly POST /monitors and GET /monitors/{monitorId}.
- Monitor fields are exactly id, targetUrl, createdAt.
- Duplicate target URLs remain valid.
- Registration remains non-idempotent.
- POST success is 201 Created + Location + Monitor.
- GET success is 200 OK + Monitor.
- Error mapping remains 400/404/415/422/500.
- Errors use RFC 9457 Problem Details.
- No public security scheme, CORS policy, public server URL, or host exposure exists.
- contracts/openapi/internal.yaml remains absent.
- API startup never auto-applies migrations.
- Readiness must not mutate schema.
- Product handlers must not report ready against an incompatible required schema.
- No new persistent product state is authorized.
- targetUrl follows existing Go domain parser semantics, preserves accepted text exactly, and intentionally has no RFC 3986 format: uri constraint.

---

## 4. Milestone Boundary

### In scope

- hand-written Go HTTP adapter for the two contracted Monitoring operations;
- strict request classification matching the landed status contract;
- response DTO mapping matching the OpenAPI Monitor shape;
- RFC 9457 Problem response mapping;
- production composition of Monitoring repository, module, UUID v7 generator, real clock, and public HTTP adapter;
- generic HTTP server composition without platform -> Monitoring coupling;
- schema-compatible /readyz;
- explicit local-development bootstrap changes required by schema-aware readiness;
- handler/unit/integration/architecture/runtime fitness evidence;
- container-local product-route smoke evidence;
- canonical documentation updates after implementation.

### Out of scope

- changing the public path set;
- list/search/pagination;
- update/delete or enable/disable;
- idempotency-key support;
- new monitor fields;
- new tables/columns/indexes;
- internal Checker API;
- scheduling/due-work/results/history;
- Rust source;
- React source;
- auth/authz;
- CORS;
- rate limiting;
- public host ports;
- TLS/public ingress;
- OpenTelemetry;
- generated server/client code;
- a Go HTTP framework;
- new root package manifests;
- auto-migration at API startup.

---

## 5. Locked Design Decisions

### GPTA-001 — Exactly two runtime operations

The public Go adapter implements only:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

It must not add GET /monitors, PUT/PATCH/DELETE, lifecycle actions, history/result endpoints, or internal Checker operations.

Unsupported methods may receive ordinary HTTP 405 Method Not Allowed; that does not create an OpenAPI operation. Unknown paths remain 404.

### GPTA-002 — Go standard library HTTP only

No HTTP framework is introduced.

The adapter uses the reviewed Go standard library: net/http, encoding/json, mime, and request contexts.

Expected module location:

~~~text
apps/api/internal/modules/monitoring/adapters/http/
~~~

The package may be imported with a disambiguating alias such as monitoringhttp.

### GPTA-003 — Platform HTTP stays generic; cmd/api stays the composition root

internal/platform/httpserver must not import Monitoring packages.

The Monitoring HTTP adapter owns product routes and implements http.Handler.

The platform server may accept a composed product http.Handler while reserving /livez and /readyz.

cmd/api composes:

~~~text
pgx pool
  -> Monitoring PostgreSQL repository
  -> Monitoring module
  -> Monitoring HTTP adapter
  -> platform HTTP server
~~~

Future modules can be combined by the composition root without changing platform dependency direction.

### GPTA-004 — Narrow transport dependencies

The HTTP adapter does not depend on the concrete PostgreSQL repository.

It depends only on the minimum application behavior required to execute RegisterMonitor.Execute and GetMonitor.Execute.

Transport-owned narrow interfaces/fakes are preferred for handler tests.

Transport DTOs are not domain models and are not cross-runtime source-of-truth types.

### GPTA-005 — Production identity and time remain server-side

Production registration keeps application-owned identity/time injection.

Composition supplies Go-generated UUID v7 and time.Now.

The HTTP request never accepts a client monitor ID or creation timestamp. PostgreSQL continues to persist the application-assigned UUID and timestamp.

The implementation plan must define a safe invariant-preserving UUID v7 -> domain.MonitorID wrapper; ID generation must not move into persistence or transport DTOs.

### GPTA-006 — POST classification exactly matches the landed contract

POST accepts Content-Type application/json. Media type parsing uses standard MIME parsing so parameters do not create accidental false negatives.

| Condition | Status |
|---|---:|
| missing/unsupported/malformed Content-Type | 415 |
| empty body / invalid JSON syntax / multiple JSON documents | 400 |
| valid JSON but not the exact create-object shape | 422 |
| unknown request member | 422 |
| missing targetUrl | 422 |
| targetUrl not a JSON string | 422 |
| targetUrl rejected by Monitoring domain | 422 |
| application persistence failure | 500 |
| success | 201 |

The request object remains exactly one targetUrl member.

The transport preserves the exact target string and does not normalize it. It must not re-implement TargetURL product validation beyond establishing the JSON member as a string.

A two-stage parse is preferred: first establish one valid JSON document, then validate exact object/member/type shape, then delegate product validation to application/domain.

### GPTA-007 — No new size/status product semantics

The landed contract does not define 413 Payload Too Large, rate limits, or public edge policy.

This milestone therefore does not invent an arbitrary URL/body-size product constraint or a new response status at the adapter boundary.

Existing HTTP server time bounds remain.

A later public-exposure/security gate must define request-size, rate-limit, ingress, TLS, and abuse controls together with contract impact.

### GPTA-008 — GET identity parsing maps to existing statuses

monitorId is parsed with domain.ParseMonitorID.

| Condition | Status |
|---|---:|
| invalid textual UUID | 400 |
| monitor absent | 404 |
| stable application persistence failure | 500 |
| success | 200 |

The transport does not assume or validate a UUID version.

### GPTA-009 — GET must not implicitly become HEAD

Go ServeMux can treat GET specially with respect to HEAD.

The public contract defines GET only.

The implementation must ensure HEAD /monitors/{monitorId} does not execute the GET application use case as an undocumented product operation.

Preferred route shape:

~~~text
/monitors             -> POST only
/monitors/{monitorId} -> GET only
~~~

Unsupported methods return 405 with the appropriate Allow header. OPTIONS remains unsupported; no CORS behavior is invented.

### GPTA-010 — Successful Monitor serialization is explicit

Successful response JSON contains exactly id, targetUrl, createdAt.

Mapping:

- id -> MonitorID.String();
- targetUrl -> exact TargetURL.String();
- createdAt -> UTC RFC 3339 representation.

POST additionally emits Location: /monitors/{canonical-monitor-id}.

No ETag, version, updatedAt, or lifecycle metadata is added.

### GPTA-011 — Problem Details are stable and sanitized

Problem responses use application/problem+json.

The initial runtime emits a deterministic subset:

- type = about:blank;
- title;
- status;
- safe detail.

instance is omitted because no request/correlation identifier contract exists.

No PostgreSQL/pgx error string, SQL, credential, hostname, stack trace, or internal Go type is returned.

Only user-controlled target validation maps to 422. Invalid internally generated ID/time or other unexpected internal failures are 500-class server failures.

### GPTA-012 — Schema compatibility is an exact, read-only migration-set check

PMAC-019 requires schema-compatible readiness before handlers become live.

Readiness must be tied to repository-owned embedded migrations and must never mutate schema.

Important gate finding:

- goose Provider GetVersions/HasPending/Status initialize the provider and may ensure/create the goose version table;
- those runtime-status methods are therefore forbidden in readiness.

The initial compatibility mechanism is:

1. derive the expected migration version set from repository-owned embedded migration sources;
2. query the existing goose version table through a read-only goose database Store API;
3. select applied versions;
4. ignore only goose's version-zero bootstrap record;
5. require exact set equality between applied DB versions and expected embedded versions.

Readiness is false when PostgreSQL is unreachable, the goose version table does not exist, required migrations are missing, DB versions are ahead, an expected version is absent, an unknown applied version exists, or the read-only metadata query fails.

No DDL/DML is permitted in the readiness path.

This exact-set policy is intentionally conservative. A future rolling-deployment compatibility-range model requires a separate design.

### GPTA-013 — Reuse the existing pgx pool

The runtime repository continues to use pgxpool.Pool.

If the pinned goose Store API requires database/sql, implementation may use the pinned pgx stdlib bridge around the existing pgx pool.

It must not open a second independently managed PostgreSQL pool merely for readiness.

The wrapper lifecycle must not close the underlying pgx pool prematurely.

### GPTA-014 — /readyz becomes DB + schema readiness; /livez stays DB-independent

GET /livez remains a process-alive probe with no database call.

GET /readyz returns 200 only when the required PostgreSQL schema is compatible with the application revision. It returns sanitized 503 on connectivity or schema-compatibility failure.

The existing bounded readiness timeout remains.

Product handlers do not add a separate readiness preflight on every request. If a caller bypasses readiness and reaches an unusable DB/schema, existing application persistence mapping remains authoritative and returns contracted 500 behavior; this milestone does not invent product-level 503.

### GPTA-015 — Migrations stay explicit; bootstrap order changes

API startup never runs migrations.

A fresh DB cannot make the API ready until migrations are explicitly applied.

Canonical Compose remains exactly four services. No fifth migration service, profile, restart loop, or auto-migration dependency is introduced.

Conceptual bootstrap:

~~~text
1. start/build db + api
   - db becomes healthy
   - api process may be live but unready

2. explicitly run uptime-lab-migrate up inside the running api container

3. start/wait for the full stack
   - api becomes ready
   - checker may start after api service_healthy
~~~

Expected command shape remains close to:

~~~bash
docker compose up -d --build db api
docker compose exec -T api /usr/local/bin/uptime-lab-migrate up
docker compose up -d --build --wait
~~~

The implementation plan must verify exact Compose behavior before documenting this as canonical.

Normal down/up preserves schema readiness. After down -v, explicit fresh-schema bootstrap is required again.

### GPTA-016 — Startup remains distinct from readiness

The API process may start while PostgreSQL is unavailable or unmigrated, preserving live != ready.

Construction may validate local migration metadata/configuration but must not require a successful live database query during startup.

Database reachability/schema state is evaluated by /readyz.

### GPTA-017 — Architecture fitness expands

Implementation must mechanically protect at minimum:

~~~text
platform -> monitoring module/adapters       forbidden
monitoring HTTP adapter -> postgres adapter forbidden
monitoring HTTP adapter -> pgx/goose        forbidden
monitoring HTTP adapter -> platform         forbidden
domain/application -> net/http              forbidden
~~~

The composition root may import both platform and Monitoring adapters.

The HTTP adapter may depend inward on Monitoring application/domain contracts.

### GPTA-018 — Verification is layered and includes real composition

Handler tests must cover at minimum:

- POST 201 + Location + exact Monitor JSON;
- exact target raw-text preservation, including accepted non-RFC-3986 text;
- malformed JSON -> 400 Problem;
- unsupported/missing Content-Type -> 415 Problem;
- valid JSON wrong shape/member/type -> 422 Problem;
- domain-invalid target -> 422 Problem;
- persistence failure -> sanitized 500 Problem;
- GET 200;
- invalid monitorId -> 400;
- absent monitor -> 404;
- GET persistence failure -> 500;
- HEAD/PUT/PATCH/DELETE/OPTIONS do not become product operations;
- exact response media types;
- no extra response fields.

Real PostgreSQL schema-readiness evidence must prove:

- no migration/version table -> unready without mutation;
- exact required migration set -> ready;
- missing/rolled-back migration -> unready;
- unknown/ahead applied version -> unready;
- connectivity failure -> unready;
- readiness checks do not create/modify migration metadata.

Canonical Docker smoke must evolve to prove:

- fresh DB leaves API unready before explicit migration;
- explicit migration makes API ready;
- container-local POST creates a monitor;
- container-local GET reads the same monitor;
- no host port is required;
- normal down/up preserves schema/data readiness;
- down -v requires explicit bootstrap again;
- Checker still waits for ready API.

The stable CI / gate remains authoritative.

### GPTA-019 — OpenAPI remains authoritative

Implementation conforms to contracts/openapi/public.yaml.

Hand-written Go DTOs/handlers do not become source of truth.

No generated client/server layer is required for this two-operation surface.

If implementation discovers a real contract inconsistency, it must be changed explicitly, semantically tested, and reviewed rather than silently diverged.

The landed targetUrl no-format decision is preserved.

### GPTA-020 — Handler existence does not authorize public exposure

This milestone keeps:

- no host application port;
- no OpenAPI servers entry;
- no authentication/authorization;
- no CORS;
- no rate limit;
- no public ingress/TLS decision.

Public exposure/security remains a later explicit gate.

---

## 6. Expected Runtime Shape

~~~text
container-local HTTP request
          |
          v
platform httpserver
  |  /livez
  |  /readyz -> read-only schema compatibility
  |
  +--> Monitoring HTTP adapter
          |
          +--> POST /monitors
          |      -> RegisterMonitor
          |      -> MonitorRepository.Create
          |
          +--> GET /monitors/{monitorId}
                 -> GetMonitor
                 -> MonitorRepository.ByID
~~~

Persistence ownership does not change.

---

## 7. Expected Production Composition

~~~text
PG environment
   |
   v
pgxpool.Pool
   |------------------------------|
   |                              |
   v                              v
Monitoring Postgres repo     read-only migration compatibility
   |                              |
   v                              v
Monitoring Module            /readyz
   |
   v
Monitoring HTTP adapter
   |
   v
platform HTTP server
~~~

Composition supplies UUID v7 generation and time.Now.

No migration executes from cmd/api.

---

## 8. Security Boundary

The public handler accepts untrusted input but this is not a public-deployment milestone.

- TargetURL registration remains syntax/product validation only.
- Registration does not establish SSRF safety.
- No probe execution occurs.
- Problem responses are sanitized.
- PostgreSQL errors do not cross the transport boundary.
- Readiness does not mutate schema.
- No user-supplied identity/timestamp is persisted.
- No host port/public ingress is added.

Execution-time DNS/private-network/redirect/rebinding/response-budget controls remain a future Rust execution-plane responsibility.

---

## 9. Implementation Planning Boundary

This design authorizes only a later implementation plan after the design is reviewed and landed.

A later plan should sequence work approximately as:

1. schema-readiness primitive + real PostgreSQL tests;
2. HTTP adapter request/response/error tests;
3. handler implementation;
4. architecture-fitness expansion;
5. production composition;
6. local-dev bootstrap/smoke evolution;
7. canonical documentation;
8. whole-branch verification;
9. external review and landing.

The implementation plan may refine task decomposition but must not reopen locked product semantics without explicit design review.

---

## 10. Acceptance Criteria

The design gate is GREEN only if review agrees that:

1. only POST /monitors and GET /monitors/{monitorId} become live;
2. no new product capability is invented;
3. net/http remains sufficient;
4. platform HTTP remains Monitoring-independent;
5. cmd/api remains the composition root;
6. error classification matches the landed contract;
7. targetUrl is preserved and not re-constrained by RFC 3986 format validation;
8. HEAD does not silently become GET;
9. responses contain only contracted Monitor/Problem surface;
10. UUID v7/time generation remains server-side;
11. readiness becomes schema-compatible before handlers are considered ready;
12. readiness is strictly read-only;
13. compatibility is tied to repository-owned embedded migrations;
14. fresh local bootstrap keeps migrations explicit;
15. Compose remains four services with no host ports;
16. no auth/CORS/rate-limit/public-exposure policy is invented;
17. no internal Checker contract is created;
18. no new durable product state is required;
19. architecture fitness protects the transport boundary;
20. real PostgreSQL + Docker evidence is required before implementation can land.

---

## 11. Decision Self-Review

### Scope alignment

PASS. The milestone makes already-landed application behavior reachable through its already-landed contract and adds no list/update/delete/lifecycle/scheduler/result capability.

### Contract alignment

PASS. The transport remains subordinate to contracts/openapi/public.yaml, including the targetUrl no-format correction.

### Dependency direction

PASS. Monitoring HTTP is an outward adapter, platform stays module-agnostic, and composition stays in cmd/api.

### Persistence safety

PASS. No migration/schema/product-state change is required.

### Readiness safety

PASS after revision. Direct goose Provider status/version methods were rejected because they can ensure/create the version table. The locked mechanism is read-only migration metadata access plus exact expected/applied set comparison.

### Migration ownership

PASS. API startup does not migrate. Local development changes bootstrap order rather than hiding migration execution.

### HTTP semantics

PASS. 400/404/415/422/500 remain the contract. Unsupported methods remain ordinary 405 behavior.

### Security

PASS with explicit boundary. Handlers become executable but arbitrary public exposure remains forbidden.

### YAGNI

PASS. No framework, generator, request ID system, custom error catalog, new status code, migration service, or product field is introduced.

### Brownfield/local-dev safety

PASS by design. The fresh-database bootstrap consequence of schema-readiness is explicitly addressed while preserving the four-service topology.

### Verification

PASS by design. Implementation must prove handler behavior, read-only schema readiness, production composition, real PostgreSQL behavior, Docker bootstrap, and aggregate CI.

### Diff hygiene

PASS by design. This branch must contain exactly this design document.

### Execution safety

PASS. No Go/runtime/Compose implementation is authorized from this branch. A separate reviewed implementation plan is required after this design lands.
