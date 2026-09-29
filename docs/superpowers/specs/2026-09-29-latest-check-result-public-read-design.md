# Latest Check Result Public Read Slice Design

**Status:** Review candidate
**Date:** 2026-09-29
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@bc9fd6cd51abfc14780516b52a0e518edb8ed4e3`
**Scope:** First public read of one Monitor's latest terminal CheckRun
**Decision state:** Product direction selected by the landed Latest Check Result Scope Reassessment. This document defines design only and does not authorize implementation.

---

## 1. Purpose

The repository now executes real checks and persists terminal CheckRuns, but public callers can still observe only Monitor registration data.

The next increment must expose the smallest useful execution fact without introducing derived availability policy, full history, frontend work, or deployment breadth.

The target journey is:

~~~text
registered Monitor
  -> Go schedules work
  -> Rust executes one bounded probe
  -> Go persists one terminal CheckRun
  -> public caller reads the latest terminal execution fact
~~~

This design defines only the final read step.

It does not authorize code, contract, migration, Compose, CI, or runtime changes.

---

## 2. Current implementation truth

The current public Monitoring surface is exactly:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

The internal Checker surface is exactly:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

The durable Monitoring schema contains:

~~~text
monitoring.monitors
monitoring.check_runs
~~~

A terminal CheckRun contains:

~~~text
id
monitor_id
issued_at
deadline_at
completed_at
result_kind
http_status
duration_ms
~~~

The current terminal-result index is:

~~~text
check_runs_latest_terminal_idx
  ON monitoring.check_runs (monitor_id, completed_at DESC)
  WHERE completed_at IS NOT NULL
~~~

The current execution model also guarantees at most one pending CheckRun per Monitor.

The public `Monitor` schema is shared by:

- `POST /monitors` success;
- `GET /monitors/{monitorId}` success.

That schema currently contains only:

~~~text
id
targetUrl
createdAt
~~~

The Single-Checker design deliberately separated execution facts from future product availability/state policy.

---

## 3. Non-goals

This slice does not design or authorize:

- full CheckRun history;
- pagination or cursors;
- retention policy;
- aggregate uptime percentages;
- latency charts or percentiles;
- derived `up`, `down`, `degraded`, or `unknown` status;
- incidents;
- notifications;
- public status pages;
- React implementation;
- browser authentication;
- public ingress or host application ports;
- CORS or rate limiting;
- Monitor list/update/delete/enable/disable;
- target mutation;
- configurable cadence;
- configurable retry policy;
- multi-checker coordination;
- broker/outbox infrastructure;
- new probe types;
- private-network monitoring;
- Rust behavior changes.

The milestone exposes one already-durable terminal execution fact.

---

## 4. Locked design decisions

### LCR-001 — Use a dedicated Monitor-scoped latest-result subresource

The public operation is:

~~~text
GET /monitors/{monitorId}/latest-result
~~~

The existing operations remain unchanged:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

The existing `Monitor` response schema is not extended with execution data.

Rationale:

1. Monitor identity/registration data remains independent from execution-result data.
2. The POST and GET Monitor representations remain stable.
3. Monitor lookup does not gain an execution-query dependency.
4. A caller can request execution data only when needed.
5. Future history can evolve separately without changing Monitor representation semantics.

This route is a public product API, not an internal Checker route.

### LCR-002 — "Latest" means latest terminal completion

The authoritative ordering is:

~~~text
completed_at DESC,
issued_at DESC,
id DESC
~~~

Only rows with:

~~~text
completed_at IS NOT NULL
~~~

participate.

`completed_at` is primary because existing scheduling truth already uses the latest terminal completion instant.

`issued_at` and `id` are deterministic tie-breakers only.

No product meaning is assigned to UUID lexical order.

### LCR-003 — Pending CheckRuns are not public in this slice

The latest-result operation exposes terminal execution facts only.

Rules:

- no terminal run + no pending run -> no result;
- no terminal run + pending run -> no result;
- previous terminal run + newer pending run -> return the previous terminal run;
- terminal run only -> return that terminal run.

The operation does not expose:

- pending state;
- claim state;
- deadline state;
- worker state;
- next due time.

Those are coordination facts, not part of this public slice.

### LCR-004 — Public reads do not reconcile expired work

The latest-result read must be side-effect free.

It must not:

- terminalize expired pending work;
- invoke claim/reconciliation logic;
- update CheckRuns;
- advance scheduling state.

Expired pending work continues to be reconciled by the existing Go execution coordination path.

Therefore a public read may briefly return the previous terminal result, or no result, while an expired pending run has not yet been reconciled.

This is preferable to making a GET operation mutate execution state.

### LCR-005 — Missing Monitor and missing result are distinct

Public response semantics are:

~~~text
400  monitorId is not a valid UUID
404  Monitor does not exist
204  Monitor exists but has no terminal CheckRun
200  latest terminal CheckRun exists
405  unsupported method
500  unexpected persistence/application failure
~~~

`204 No Content` means only:

~~~text
known Monitor, no terminal result yet
~~~

It is not an availability status and is not an error.

### LCR-006 — The public representation is an execution fact

The response does not expose a derived availability verdict.

The public result vocabulary is based on the already-normalized Go-owned execution vocabulary:

~~~text
http_response
dns_error
policy_rejected
timeout
connect_error
tls_error
protocol_error
internal_error
worker_timeout
~~~

These values describe what happened during execution.

They do not mean:

~~~text
up
down
healthy
unhealthy
incident
~~~

### LCR-007 — Public DTOs are distinct from internal Checker DTOs

The public response must not directly reuse:

- internal OpenAPI request schemas;
- Rust transport structs;
- PostgreSQL row structs;
- internal Checker adapter DTOs.

The public field name is:

~~~text
resultKind
~~~

rather than internal request field:

~~~text
kind
~~~

This makes the public mapping explicit.

The public result is modeled as three semantic variants.

#### HTTP response result

~~~json
{
  "checkId": "uuid",
  "resultKind": "http_response",
  "httpStatus": 200,
  "durationMs": 42,
  "completedAt": "date-time"
}
~~~

Required fields:

~~~text
checkId
resultKind
httpStatus
durationMs
completedAt
~~~

#### Rust-classified failure result

~~~json
{
  "checkId": "uuid",
  "resultKind": "policy_rejected",
  "durationMs": 0,
  "completedAt": "date-time"
}
~~~

Allowed result kinds:

~~~text
dns_error
policy_rejected
timeout
connect_error
tls_error
protocol_error
internal_error
~~~

Required fields:

~~~text
checkId
resultKind
durationMs
completedAt
~~~

`httpStatus` must be absent.

#### Go coordination timeout result

~~~json
{
  "checkId": "uuid",
  "resultKind": "worker_timeout",
  "completedAt": "date-time"
}
~~~

Required fields:

~~~text
checkId
resultKind
completedAt
~~~

`httpStatus` and `durationMs` must be absent.

This shape preserves the current durable invariant that `worker_timeout` has no probe duration.

### LCR-008 — Do not expose coordination-only fields

The public result does not contain:

~~~text
monitorId
issuedAt
deadlineAt
targetUrl
worker identity
next due time
raw error text
resolved IP addresses
redirect chain
response headers
response body
~~~

`monitorId` is already represented by the resource path.

`targetUrl` remains owned by the Monitor resource.

`issuedAt` and `deadlineAt` are execution-coordination facts and are not required to understand the latest terminal result.

### LCR-009 — Add one narrow public read use case

The application capability is conceptually:

~~~text
GetLatestCheckResult
~~~

It accepts:

~~~text
MonitorID
~~~

and returns one public-read-safe terminal result fact.

It must distinguish:

~~~text
Monitor not found
No terminal result
Persistence failure
Latest terminal result
~~~

This use case belongs to the public Monitoring module, not the internal Checker execution module.

### LCR-010 — Use a dedicated read persistence port

Do not extend `CheckExecutionRepository` with public query concerns.

Add a narrow read-facing port conceptually equivalent to:

~~~text
LatestCheckResultRepository
  -> LatestTerminalByMonitorID(...)
~~~

The existing execution repository remains responsible only for:

~~~text
ClaimDueCheck
CompleteCheck
~~~

The same PostgreSQL adapter type may implement both interfaces.

This preserves capability-oriented boundaries without introducing a generic repository hierarchy or CQRS framework.

### LCR-011 — Monitor existence and latest-result lookup should use one read query

The PostgreSQL adapter should distinguish Monitor absence from result absence in one statement.

Conceptually:

~~~sql
SELECT monitor identity + latest terminal columns
FROM monitoring.monitors AS monitor
LEFT JOIN LATERAL (
    SELECT ...
    FROM monitoring.check_runs AS run
    WHERE run.monitor_id = monitor.id
      AND run.completed_at IS NOT NULL
    ORDER BY
        run.completed_at DESC,
        run.issued_at DESC,
        run.id DESC
    LIMIT 1
) AS latest ON true
WHERE monitor.id = $1
~~~

Outcomes:

- no outer Monitor row -> Monitor not found;
- Monitor row + NULL latest row -> no terminal result;
- Monitor row + terminal latest row -> return the result.

No locks are required.

### LCR-012 — No migration is required by this design

The existing partial index:

~~~text
(monitor_id, completed_at DESC)
WHERE completed_at IS NOT NULL
~~~

supports the selected access path.

The additional tie-breakers do not justify a new index for this milestone.

Do not add:

- a latest-result table;
- a monitor status table;
- a materialized view;
- duplicated terminal-result columns on `monitoring.monitors`;
- a cache.

If implementation evidence contradicts this assumption, that is a design-change gate rather than permission to add schema opportunistically.

### LCR-013 — The read is snapshot-consistent, not lock-consistent

A single PostgreSQL statement is sufficient.

If a CheckRun completes concurrently with the public read, either of these outcomes is acceptable:

1. the statement sees the newly terminalized result; or
2. the statement sees the previously committed terminal state.

The operation must not block execution with `FOR UPDATE`.

A subsequent read will observe the later committed state.

### LCR-014 — Extend the existing public Monitoring adapter

The current public Monitoring HTTP adapter remains the owner of this route.

Do not create:

- a second public Monitoring server;
- a separate process;
- a new transport framework;
- a second product router.

The adapter gains one explicit dependency for the latest-result use case and one exact nested route.

Existing create/read behavior must remain unchanged.

### LCR-015 — Public OpenAPI gains exactly one operation

`contracts/openapi/public.yaml` remains the authoritative public contract source.

The implementation phase may add exactly:

~~~text
GET /monitors/{monitorId}/latest-result
~~~

for this milestone.

The public contract must not add:

- collection history;
- individual CheckRun lookup;
- status;
- incidents;
- pagination;
- mutable Monitor operations.

The internal OpenAPI contract remains byte-identical.

The exact `info.version` bookkeeping, if repository convention requires a change, does not alter these product semantics and must not broaden scope.

### LCR-016 — Existing Monitor responses remain shape-stable

The successful payloads for:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

remain:

~~~text
id
targetUrl
createdAt
~~~

No `latestResult`, `status`, or execution fields are added to `Monitor`.

This is a compatibility requirement, not merely an implementation preference.

### LCR-017 — Error responses remain deterministic and sanitized

The new operation follows the existing public adapter error discipline.

It must not expose:

- PostgreSQL errors;
- pgx details;
- SQL text;
- internal contract payloads;
- Rust/library errors;
- DNS/TLS internals;
- credentials or filesystem data.

Structured public errors continue to use:

~~~text
application/problem+json
~~~

where a response body is defined.

`204` has no response body.

### LCR-018 — Rust and the internal Checker contract do not change

No Checker behavior is required for this milestone.

The following remain unchanged:

~~~text
contracts/openapi/internal.yaml
apps/checker/**
claim semantics
result submission semantics
probe policy
worker orchestration
~~~

The read slice consumes already-persisted Go-owned product state.

### LCR-019 — No security exposure change

This design adds a public-contract operation to the existing Go product router but does not expose the application to a new network boundary.

Canonical Compose still publishes no application host port.

This milestone does not add:

- authentication;
- authorization;
- public ingress;
- TLS termination;
- CORS;
- rate limiting.

Those remain a separate deployment/public-exposure design problem.

### LCR-020 — Future history remains independent

The latest-result subresource does not establish:

- history retention guarantees;
- cursor semantics;
- page size;
- ordering contract for a history collection;
- incident semantics.

A later history milestone may add a separate collection-oriented resource.

It must preserve the meaning of this route as:

~~~text
one latest terminal execution fact
~~~

---

## 5. Application model

The public read model needs to represent all durable terminal states, including `worker_timeout`.

It therefore must not reuse the current domain `CheckResult` value directly because that value models normalized probe submissions and intentionally excludes Rust submission of `worker_timeout`.

A narrow read model should carry conceptually:

~~~text
CheckID
ResultKind
HTTPStatus?
DurationMS?
CompletedAt
~~~

with invariants derived from the durable terminal row shape.

The mapping boundary must validate that PostgreSQL data satisfies the known result-kind invariants rather than blindly serializing nullable columns.

An impossible durable shape is an internal failure, not a partially populated public response.

---

## 6. HTTP contract semantics

### Request

~~~http
GET /monitors/{monitorId}/latest-result
~~~

No request body.

### Success with terminal result

~~~text
200 OK
Content-Type: application/json
~~~

Body is one of the three public result variants.

### Existing Monitor without terminal result

~~~text
204 No Content
~~~

No body.

### Invalid Monitor ID

~~~text
400 Bad Request
Content-Type: application/problem+json
~~~

### Missing Monitor

~~~text
404 Not Found
Content-Type: application/problem+json
~~~

### Unsupported method

~~~text
405 Method Not Allowed
Allow: GET
~~~

### Unexpected failure

~~~text
500 Internal Server Error
Content-Type: application/problem+json
~~~

---

## 7. Ordering and edge cases

### Multiple terminal runs

Return exactly one row using:

~~~text
completed_at DESC
issued_at DESC
id DESC
~~~

### Same completion timestamp

Use `issued_at` then `id` only as deterministic tie-breakers.

### Previous terminal + current pending

Return the previous terminal result.

### Pending only

Return `204`.

### Expired pending not yet reconciled

Do not mutate it from the read path.

Return the previous terminal result if one exists, otherwise `204`.

### Worker timeout

Return:

~~~text
resultKind = worker_timeout
no durationMs
no httpStatus
completedAt = persisted terminal instant
~~~

### HTTP response

Return the final stored HTTP status regardless of class.

The API does not translate:

~~~text
2xx -> up
5xx -> down
~~~

### Policy rejection

Return:

~~~text
resultKind = policy_rejected
~~~

This remains an execution-policy outcome, not a derived Monitor health state.

---

## 8. Persistence design

The existing PostgreSQL schema remains authoritative.

No new SQL object is required.

The read adapter should use:

- a bounded single-row query;
- the existing Monitor foreign-key relationship;
- the existing terminal partial index;
- no write transaction;
- no row locking.

The read path must not call the execution claim/completion methods.

---

## 9. Public adapter design

The existing handler currently recognizes:

~~~text
/monitors
/monitors/{monitorId}
~~~

Implementation planning must add exact recognition for:

~~~text
/monitors/{monitorId}/latest-result
~~~

without changing current path semantics.

Examples that must remain not-found:

~~~text
/monitors/{id}/latest-result/extra
/monitors/{id}/unknown
/monitors//latest-result
~~~

Existing:

~~~text
/monitors/{id}
~~~

must continue to resolve only to Monitor retrieval.

Route parsing must remain explicit and bounded; no new router dependency is justified by one nested resource.

---

## 10. Composition design

The public Monitoring module conceptually becomes:

~~~text
RegisterMonitor
GetMonitor
GetLatestCheckResult
~~~

The internal execution module remains:

~~~text
ClaimDueCheck
SubmitCheckResult
~~~

The PostgreSQL `Repository` may satisfy:

~~~text
MonitorRepository
LatestCheckResultRepository
CheckExecutionRepository
~~~

through narrow interfaces.

`cmd/api` remains the composition root.

No additional PostgreSQL pool is required.

---

## 11. Contract verification

The public contract verification must prove:

1. exactly one new public operation is added;
2. existing Monitor create/read schemas remain unchanged;
3. the latest-result route is GET-only;
4. 200 response validates the public result variants;
5. 204 has no body;
6. invalid/missing/error responses remain deterministic;
7. internal contract operation count and bytes remain unchanged unless unrelated repository mechanics require normalization.

Contract fixtures should cover at least:

- HTTP response result;
- normalized failure result;
- worker timeout result.

---

## 12. Application verification

Application tests must cover:

- valid Monitor ID with latest result;
- Monitor not found;
- Monitor exists with no terminal result;
- persistence failure;
- all public-safe terminal result categories;
- worker timeout representation without duration;
- invalid durable result shape mapped to internal failure rather than emitted publicly.

No Clock or ID generation is needed for this read use case.

---

## 13. PostgreSQL verification

Real PostgreSQL integration must cover:

- missing Monitor;
- Monitor with no CheckRuns;
- Monitor with pending-only CheckRun;
- one terminal CheckRun;
- multiple terminal CheckRuns;
- deterministic same-completion-time tie behavior;
- previous terminal plus newer pending;
- `http_response` with HTTP status;
- Rust failure without HTTP status;
- `worker_timeout` without duration or HTTP status;
- query cancellation;
- no mutation caused by the read.

The test must verify that the existing migration is sufficient.

Landed migration `00002` remains immutable.

---

## 14. HTTP adapter verification

The public adapter tests must verify:

- exact nested route matching;
- GET success;
- 204 no-result behavior;
- 400 malformed Monitor UUID;
- 404 missing Monitor;
- 405 + `Allow: GET`;
- 500 sanitized error;
- exact JSON field sets for each result variant;
- no coordination-only fields;
- existing POST/GET Monitor behavior remains unchanged.

---

## 15. Production composition verification

Real PostgreSQL `cmd/api` integration must prove:

~~~text
HTTP latest-result request
  -> public Monitoring adapter
  -> GetLatestCheckResult
  -> PostgreSQL read port
  -> correct public response
~~~

This evidence must use the production composition root.

It must not bypass through a test-only handler composition.

---

## 16. Canonical Docker evidence

The existing canonical four-service topology remains:

~~~text
web
db
api
checker
~~~

The smoke should extend the already-proven controlled journey:

~~~text
register Monitor for http://web/
  -> Rust production policy returns policy_rejected
  -> Go persists terminal CheckRun
  -> public latest-result GET returns that terminal fact
~~~

Required assertions:

- response status is `200`;
- `checkId` equals the terminal CheckRun ID observed in PostgreSQL;
- `resultKind = policy_rejected`;
- `durationMs` is a bounded integer;
- `httpStatus` is absent;
- `completedAt` is present and parseable;
- zero pending CheckRuns remain for the selected Monitor.

The smoke must not:

- expose a host application port;
- weaken production destination policy;
- use an external internet target;
- add a test-only private-network bypass.

Existing bootstrap/restart/reset evidence remains intact.

---

## 17. CI change detection

Because this milestone changes the public contract and Go API implementation, the later implementation branch is expected to activate:

~~~text
public-contract
go-api
local-dev
~~~

as appropriate to the changed files.

The design itself does not modify CI.

Path-aware behavior must not be weakened to force jobs to run.

---

## 18. Documentation impact

Implementation must update current-state documentation when the route lands.

At minimum reassess:

- root `README.md`;
- `docs/README.md`;
- Go Control Plane guide;
- public Monitoring contract testing guide;
- relevant runtime-flow documentation;
- Single-Checker testing guide only where the public evidence changes;
- local-development smoke documentation.

Historical design documents are not rewritten as current state.

No ADR is required by this design because:

- runtime ownership does not change;
- data ownership does not change;
- dependency direction does not change;
- no new process or datastore is introduced.

If implementation discovers a material architecture change, ADR need must be reassessed before proceeding.

---

## 19. Verification matrix

| Concern | Required evidence |
|---|---|
| Public contract shape | OpenAPI lint + semantic contract tests |
| Existing Monitor compatibility | public contract fixtures + HTTP adapter tests |
| Latest ordering | PostgreSQL integration |
| Monitor missing vs result missing | application + PostgreSQL + HTTP tests |
| Pending invisibility | PostgreSQL + HTTP tests |
| Worker timeout shape | application + contract + PostgreSQL tests |
| No read-side mutation | PostgreSQL integration |
| Public DTO isolation | architecture/code review + adapter tests |
| Production composition | real PostgreSQL `cmd/api` integration |
| End-to-end observability | canonical Docker smoke |
| Existing execution behavior | unchanged Rust/Go execution tests |
| Aggregate state | path-aware CI + `CI / gate` |

---

## 20. Implementation-plan boundaries

The later implementation plan may decompose work into:

1. public contract RED;
2. application/read-model RED;
3. read persistence port + PostgreSQL RED;
4. public HTTP adapter RED;
5. implementation GREEN by layer;
6. production composition;
7. Docker evidence;
8. current-state documentation;
9. whole-branch verification;
10. external review and landing.

This list is sequencing guidance only.

No implementation task is authorized by this design document itself.

---

## 21. Gate acceptance criteria

This design is GREEN only if review agrees that:

1. the public route is `GET /monitors/{monitorId}/latest-result`;
2. existing Monitor response schemas remain unchanged;
3. latest means latest terminal completion with deterministic tie-breakers;
4. pending CheckRuns remain invisible;
5. the read path has no reconciliation side effects;
6. existing Monitor + no terminal result maps to `204`;
7. missing Monitor maps to `404`;
8. the public response exposes execution facts only;
9. `worker_timeout` is representable without fake duration data;
10. public DTOs are separate from internal Checker DTOs;
11. a dedicated read port is used instead of widening the execution repository;
12. the existing schema/index are sufficient;
13. no migration is authorized;
14. Rust and the internal contract remain unchanged;
15. canonical Docker evidence proves persisted result -> public read;
16. history, availability policy, React, auth, and exposure remain deferred;
17. implementation planning remains a separate gate.

---

## 22. Self-review

### Product value

PASS.

The design exposes the smallest public monitoring output already supported by durable execution state.

### Contract discipline

PASS.

Exactly one new operation is selected, while existing Monitor payloads remain unchanged.

### Architecture

PASS.

Go retains public semantics and durable-state ownership. Rust remains execution-only.

### Persistence

PASS.

The existing table and terminal partial index are sufficient; no schema expansion is selected.

### Read semantics

PASS.

The query is side-effect free and distinguishes missing Monitor from missing terminal result.

### Availability-policy separation

PASS.

Execution facts remain distinct from future product health/status policy.

### YAGNI

PASS.

No history, pagination, status aggregation, cache, new table, router framework, frontend, auth, or distributed infrastructure is introduced.

### Security

PASS at design level.

No new network exposure is selected and no sensitive execution details are added to the response.

### Testability

PASS.

The design provides deterministic PostgreSQL, HTTP, production-composition, and Docker evidence without internet dependence.

### Execution safety

PASS.

This branch must contain only this design document.

---

## 23. Explicit STOP boundary

After this design is reviewed and landed:

~~~text
NEXT:
Latest Check Result Public Read Slice implementation plan
~~~

Do not:

- modify `contracts/openapi/public.yaml`;
- add application/runtime code;
- add SQL migrations;
- change Rust;
- change Compose;
- change CI;
- begin implementation tasks;

until the separate implementation-plan gate is explicitly opened and reviewed.
