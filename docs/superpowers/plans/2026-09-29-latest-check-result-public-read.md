# Latest Check Result Public Read Slice Implementation Plan

**Status:** Review candidate
**Date:** 2026-09-29
**Repository:** kefyusuf/uptime-lab
**Scope:** Implement the landed Latest Check Result Public Read Slice design from public contract through Go application/read persistence, production composition, Docker evidence, canonical documentation, and controlled landing
**Base:** main@64c789fa7aeb86dfbc07cc706084fe045dbda04d
**Authoritative design:** docs/superpowers/specs/2026-09-29-latest-check-result-public-read-design.md
**Authoritative public contract:** contracts/openapi/public.yaml

---

## 1. Purpose

The Latest Check Result Public Read Slice design is landed.

The repository already has a complete execution loop:

~~~text
registered Monitor
  -> Go claims due work
  -> Rust executes one bounded probe
  -> Rust submits one normalized result
  -> Go persists one terminal CheckRun
~~~

The missing product step is public observation of that already-durable execution fact.

This plan implements exactly one new public operation:

~~~text
GET /monitors/{monitorId}/latest-result
~~~

The implementation must preserve the accepted ownership boundary:

~~~text
Go         = public product semantics, application/read orchestration, persistence
PostgreSQL = durable Monitor and CheckRun state
Rust       = bounded execution only
~~~

This phase MUST NOT silently become a history, availability-status, Web, authentication, public-exposure, mutable-Monitor, schema-evolution, or multi-checker milestone.

---

## 2. Existing Authority

The implementation does not reopen these landed design decisions.

### Public route

Exactly one public operation is added:

~~~text
GET /monitors/{monitorId}/latest-result
~~~

Existing operations remain behaviorally and structurally unchanged:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

The existing Monitor success payload remains exactly:

~~~text
id
targetUrl
createdAt
~~~

No latestResult, status, or execution fields are added to Monitor responses.

### Latest semantics

Only terminal rows participate:

~~~text
completed_at IS NOT NULL
~~~

Authoritative ordering is:

~~~text
completed_at DESC
issued_at DESC
id DESC
~~~

Pending CheckRuns are invisible.

A newer pending run does not hide a previous terminal result.

The read path is side-effect free and does not reconcile expired pending work.

### Public response semantics

~~~text
400  invalid textual Monitor UUID
404  Monitor does not exist
204  Monitor exists but has no terminal CheckRun
200  latest terminal CheckRun exists
405  unsupported method, Allow: GET
500  unexpected application/persistence failure
~~~

204 has no body and means only a known Monitor with no terminal result yet.

It is not an availability verdict.

### Public result vocabulary

The public result exposes execution facts only:

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

It does not derive up/down/healthy/unhealthy/degraded/incident semantics.

### Public payload variants

HTTP response:

~~~text
checkId
resultKind = http_response
httpStatus
durationMs
completedAt
~~~

Rust-classified failure:

~~~text
checkId
resultKind
durationMs
completedAt
~~~

with no httpStatus.

Go-owned worker timeout:

~~~text
checkId
resultKind = worker_timeout
completedAt
~~~

with no httpStatus and no durationMs.

### Persistence boundary

A dedicated read-facing persistence port is required.

Do not widen CheckExecutionRepository with public read concerns.

The existing PostgreSQL Repository type may implement the new narrow interface.

The current schema is sufficient.

No migration is authorized.

The landed migration apps/api/migrations/00002_create_monitoring_check_runs.sql remains byte-identical.

The existing partial index is sufficient:

~~~text
check_runs_latest_terminal_idx
  ON monitoring.check_runs (monitor_id, completed_at DESC)
  WHERE completed_at IS NOT NULL
~~~

### Query behavior

Monitor existence and latest terminal result should be distinguished in one bounded read statement.

No row lock is required.

No write transaction is required.

Concurrent completion may produce either the previously committed terminal state or the newly committed terminal state for that statement snapshot.

### Runtime ownership

The new use case belongs to the public Monitoring module.

The internal execution module remains exactly:

~~~text
ClaimDueCheck
SubmitCheckResult
~~~

Rust and the internal Checker contract do not change.

### Security / exposure

No new network exposure is introduced.

Canonical Compose still publishes no application host port.

This phase does not add authentication, authorization, TLS ingress, CORS, rate limiting, or external public deployment.

---

## 3. Phase Boundary

### In scope

- one new public OpenAPI operation;
- public semantic-contract fitness for the new route and result variants;
- preservation tests for the existing Monitor schemas;
- public result fixtures for HTTP response, classified failure, and worker timeout;
- a narrow latest-result read persistence port;
- a public-safe application read model;
- GetLatestCheckResult application use case;
- application error mapping for no-result vs missing Monitor vs persistence failure;
- one PostgreSQL latest-terminal read query;
- deterministic terminal ordering;
- pending-run invisibility;
- worker_timeout read support;
- impossible persisted-shape rejection;
- public HTTP nested-route support;
- exact 200/204/400/404/405/500 behavior;
- production Monitoring module composition;
- real PostgreSQL cmd/api evidence;
- canonical Docker persisted-result -> public-read evidence;
- current-state documentation updates;
- whole-branch verification;
- external review;
- squash landing;
- fresh post-merge main CI.

### Out of scope

- CheckRun history collection;
- pagination or cursors;
- CheckRun-by-ID public read;
- uptime percentages;
- latency aggregation;
- derived Monitor availability/status;
- incidents;
- notifications;
- status pages;
- React implementation;
- public application host ports;
- authentication or authorization;
- CORS;
- rate limiting;
- Monitor list/update/delete/enable/disable;
- target mutation;
- configurable cadence;
- retries;
- multi-checker coordination;
- broker/queue/outbox;
- new probe types;
- private-network monitoring;
- new SQL migration;
- new table/index/materialized view/cache;
- modifying existing migrations;
- Rust changes;
- internal OpenAPI changes;
- CI workflow redesign;
- new HTTP framework/router.

If implementation appears to require any item above, STOP and reopen design/product scope before proceeding.

---

## 4. Implementation Discipline

### 4.1 Implementation branch

After this plan is reviewed and landed, create:

~~~text
feat/latest-check-result-public-read
~~~

from the exact landed plan commit on main.

Do not implement from the design branch or this plan branch.

### 4.2 Pull request lifecycle

Open one draft implementation PR immediately:

~~~text
feat(api): expose latest check result
~~~

Keep it draft through Tasks 1-7.

Every task closes with:

1. RED evidence produced in local/task working state;
2. minimal GREEN implementation;
3. focused verification;
4. explicit scope self-review;
5. one coherent task commit;
6. fresh remote CI on that exact head;
7. no advancement until the required jobs for that task are GREEN.

Task 8 performs whole-branch verification and marks the PR review-ready.

Task 9 performs external review, finding resolution, exact-head squash landing, and fresh main verification.

### 4.3 RED -> GREEN rule

No deliberately failing commit is pushed.

Tests may be written and observed RED locally before implementation.

The committed task head must be GREEN.

### 4.4 Scope-drift review

At every task self-review search for accidental implementation of:

~~~text
/monitors collection GET
/history
/check-runs collection
/status
availability
up/down
pagination/cursor
auth/securitySchemes
CORS
rate limit
public host port
new migration
new table/index
apps/checker changes
contracts/openapi/internal.yaml changes
broker/queue/outbox
React/Web source
~~~

Occurrences are permitted only in negative tests, historical design text, or explicit deferred-scope documentation.

Any implementation occurrence fails the task review.

---

## 5. Current Repository Constraints

### 5.1 Public contract fitness is exact

scripts/ci/check-public-contract.mjs currently asserts exact public path and schema sets.

Task 1 must evolve these exact-set invariants deliberately.

Do not weaken exactness to broad existence checks.

### 5.2 Existing public Monitor compatibility is a hard requirement

The public semantic checker already asserts Monitor properties exactly:

~~~text
id
targetUrl
createdAt
~~~

Those assertions must remain.

The new result route must use separate schemas.

### 5.3 Public fixtures do not currently exist

The repository currently has contracts/fixtures/internal.

The implementation may add contracts/fixtures/public only for the three approved latest-result variants.

Do not create a generic fixture framework.

### 5.4 Execution persistence already exists

Current PostgreSQL execution code owns:

~~~text
ClaimDueCheck
CompleteCheck
~~~

The new public read must be added separately rather than folded into claim/completion logic.

### 5.5 Production composition already proves result persistence

apps/api/cmd/api/main_integration_test.go already proves public Monitor create/read, internal claim, result submission, terminal persistence, and late-result worker_timeout.

The new composition evidence should extend this existing production test rather than create a parallel composition harness.

### 5.6 Canonical Docker smoke already has a deterministic terminal result

scripts/ci/smoke-local-dev.sh already creates targetUrl=http://web/ and proves production destination policy yields policy_rejected with one terminal CheckRun and zero pending rows.

The new smoke evidence should read that same persisted result publicly.

No second target or external internet dependency is required.

---

## 6. Target File Map

### Public contract

Modify:

~~~text
contracts/openapi/public.yaml
scripts/ci/check-public-contract.mjs
scripts/ci/test-check-public-contract.mjs
~~~

Create:

~~~text
contracts/fixtures/public/latest-result-http-response.json
contracts/fixtures/public/latest-result-failure.json
contracts/fixtures/public/latest-result-worker-timeout.json
~~~

Expected unchanged:

~~~text
contracts/openapi/internal.yaml
contracts/fixtures/internal/**
~~~

### Application and ports

Create:

~~~text
apps/api/internal/modules/monitoring/ports/latest_check_result_repository.go
apps/api/internal/modules/monitoring/application/get_latest_check_result.go
apps/api/internal/modules/monitoring/application/get_latest_check_result_test.go
~~~

Modify:

~~~text
apps/api/internal/modules/monitoring/application/errors.go
apps/api/internal/modules/monitoring/module.go
~~~

Do not add a generic query bus, CQRS framework, DTO package, shared/common layer, or repository base class.

### PostgreSQL read adapter

Create:

~~~text
apps/api/internal/modules/monitoring/adapters/postgres/latest_check_result.go
apps/api/internal/modules/monitoring/adapters/postgres/latest_check_result_integration_test.go
~~~

No SQL migration file is created or modified.

### Public HTTP adapter

Modify:

~~~text
apps/api/internal/modules/monitoring/adapters/http/handler.go
apps/api/internal/modules/monitoring/adapters/http/handler_test.go
~~~

Do not add an HTTP framework or second public handler tree.

### Production composition

Modify:

~~~text
apps/api/cmd/api/main.go
apps/api/cmd/api/main_integration_test.go
~~~

No second database pool or service process is introduced.

### Docker evidence

Modify:

~~~text
scripts/ci/smoke-local-dev.sh
scripts/ci/test-smoke-local-dev.sh
~~~

compose.yaml is expected to remain unchanged.

### Architecture/documentation

Reassess and modify only where current truth changes:

~~~text
README.md
docs/README.md
docs/backend/go-control-plane.md
docs/testing/public-monitoring-contract.md
docs/testing/single-checker-execution-slice.md
docs/devops/local-development.md
docs/architecture/runtime-flows.md
scripts/ci/check-architecture-docs.sh
scripts/ci/test-architecture-docs.sh
scripts/ci/check-go-architecture.sh
scripts/ci/test-check-go-architecture.sh
~~~

Architecture CI scripts should change only if a new invariant must be made executable.

### Expected unchanged files

~~~text
apps/api/migrations/*.sql
contracts/openapi/internal.yaml
apps/checker/**
compose.yaml
.github/workflows/ci.yml
scripts/ci/detect-*-changes.sh
~~~

A migration, Rust change, internal-contract change, or Compose topology change requires STOP and design reassessment.

---

## 7. Dependency Order

~~~text
Task 1  Public contract
  -> Task 2  Application/read model + read port
  -> Task 3  PostgreSQL latest-terminal read
  -> Task 4  Public HTTP adapter
  -> Task 5  Production module/composition
  -> Task 6  Canonical Docker evidence
  -> Task 7  Canonical documentation
  -> Task 8  Whole-branch verification
  -> Task 9  External review + landing
~~~

Contract semantics are fixed before runtime work; application vocabulary before persistence mapping; persistence before transport wiring; isolated adapter before production composition; Docker after production composition; current-state docs last.

---

# Task 1 — Public contract + semantic fitness

**Goal:** Make the new operation precise before runtime code exists.

**Primary files:** public.yaml, public contract checker/tests, three public fixtures.

## Steps

1. RED the exact public path set so it expects exactly:
   - /monitors
   - /monitors/{monitorId}
   - /monitors/{monitorId}/latest-result
2. RED the new operation:
   - GET only;
   - operationId=getLatestCheckResult;
   - one required monitorId UUID path parameter;
   - responses 200, 204, 400, 404, 500;
   - no security or servers.
3. RED three result schema variants:
   - HTTP response requires checkId/resultKind/httpStatus/durationMs/completedAt;
   - classified failure requires checkId/resultKind/durationMs/completedAt and no httpStatus;
   - worker_timeout requires checkId/resultKind/completedAt and no durationMs/httpStatus.
4. Preserve existing CreateMonitorRequest, Monitor, and Problem exact checks.
5. Add exactly three public fixtures for the approved variants.
6. GREEN public.yaml with exactly one new path and only needed schemas.
7. Keep info.version at the current repository value 0.1.0; this milestone does not introduce API versioning policy.

## Verification

Run existing Redocly/public semantic validation and focused Node tests.

Expected relevant CI: policy, repository, changes, public-contract, CI/gate.

## Self-review

Exactly three public paths; existing Monitor contract unchanged; no internal contract changes; no availability vocabulary.

---

# Task 2 — Application read model + dedicated persistence port

**Goal:** Establish public-safe application semantics without PostgreSQL or HTTP concerns.

## Steps

1. Define a narrow LatestCheckResultRepository capability.
2. Distinguish:
   - Monitor missing;
   - no terminal result;
   - latest terminal result;
   - persistence failure.
3. Define persistence-neutral result data:
   - CheckID;
   - ResultKind;
   - optional HTTPStatus;
   - optional DurationMS;
   - CompletedAt.
4. RED application tests for:
   - HTTP response;
   - all classified failure kinds;
   - worker_timeout;
   - Monitor missing;
   - no terminal result;
   - persistence failure;
   - impossible row shapes.
5. Impossible shapes include:
   - http_response without status or duration;
   - failure with status or without duration;
   - worker_timeout with status or duration;
   - unknown kind.
6. GREEN GetLatestCheckResult with no Clock, ID generator, cache, retry, or status derivation.
7. Wire it only into the public Monitoring module.
8. Leave ExecutionModule and CheckExecutionRepository behaviorally unchanged.

## Verification

Focused application tests + Go architecture checks.

## Self-review

No HTTP/pgx dependency in application; no status policy; execution repository remains claim+complete only.

---

# Task 3 — PostgreSQL latest-terminal read adapter

**Goal:** Prove deterministic, side-effect-free semantics against real PostgreSQL with current schema.

## Steps

1. RED missing Monitor vs existing Monitor/no result.
2. RED pending-only -> no terminal result.
3. RED latest ordering:
   - completed_at DESC;
   - issued_at DESC;
   - id DESC.
4. Include same-completed-at and final UUID tie-break tests.
5. RED previous terminal + newer pending -> previous terminal.
6. RED terminal shapes:
   - http_response;
   - classified failure;
   - worker_timeout.
7. RED no-mutation behavior by comparing rows before/after read.
8. RED context cancellation.
9. GREEN one bounded LEFT JOIN LATERAL query with no FOR UPDATE and no write transaction.
10. Verify 00001 and 00002 unchanged and no 00003 exists.

## Verification

Real PostgreSQL integration tests through current harness.

## Self-review

No migration/index/cache/materialized state; no reconciliation side effect.

---

# Task 4 — Public HTTP adapter

**Goal:** Expose isolated nested route behavior before production composition.

## Steps

1. RED exact route GET /monitors/{id}/latest-result.
2. RED negative paths:
   - /latest-result/extra;
   - /unknown;
   - malformed empty ID shape.
3. RED unsupported methods including HEAD -> 405 + Allow: GET.
4. RED outcome mapping:
   - result -> 200 JSON;
   - no result -> 204 empty;
   - invalid UUID -> 400 Problem;
   - missing Monitor -> 404 Problem;
   - internal/persistence -> 500 Problem.
5. RED exact JSON field sets for all three variants.
6. Ensure absent optional fields are omitted, not null.
7. GREEN by extending existing Monitoring handler only.
8. Preserve existing POST/GET Monitor behavior unchanged.

## Verification

Focused HTTP adapter tests + public contract tests.

## Self-review

No targetUrl duplication, coordination fields, internal DTO reuse, router dependency, or derived status.

---

# Task 5 — Production composition + real PostgreSQL HTTP evidence

**Goal:** Wire approved use case through the production composition root.

## Steps

1. Extend existing cmd/api integration test rather than creating a parallel harness.
2. Existing Monitor before terminal result -> latest-result 204.
3. Use existing internal claim/result route to create known http_response terminal result.
4. Public latest-result GET -> 200 and exact CheckID/status/duration/completedAt.
5. Reuse existing late-result path to create worker_timeout.
6. Public worker_timeout GET -> 200 with CheckID/completedAt and no duration/status.
7. Missing Monitor -> 404.
8. GREEN composeMonitoring wiring:
   repository -> public Module -> public Handler -> existing product router.
9. No second pool/server/router/process.

## Verification

Real PostgreSQL cmd/api integration.

## Self-review

Internal claim/result semantics unchanged; read uses production repository; no test-only adapter in composition proof.

---

# Task 6 — Canonical Docker public-read evidence

**Goal:** Extend existing deterministic cross-runtime smoke to public observability.

## Steps

1. RED shell helper tests for latest-result response parsing/assertions.
2. Keep target http://web/.
3. Keep execution chain:
   Go claim -> Rust -> policy_rejected -> Go -> PostgreSQL.
4. After terminal row is proven, call GET /monitors/$MONITOR_ID/latest-result from inside container network.
5. Bind public payload to PostgreSQL truth:
   - 200;
   - checkId == CHECK_RUN_ID;
   - resultKind == policy_rejected;
   - durationMs == persisted duration;
   - httpStatus absent;
   - completedAt present/parseable;
   - pending count 0;
   - terminal count 1.
6. Preserve normal restart persistence and destructive reset evidence.
7. Do not change policy, target, internet dependency, host port, or Compose topology.

## Verification

Shell tests + real canonical Docker smoke.

## Self-review

compose.yaml unchanged unless a concrete blocker is proven; any topology change triggers STOP.

---

# Task 7 — Canonical documentation + documentation fitness

**Goal:** Update current-state docs after runtime behavior is stable.

## Steps

1. Search stale statements that public Monitoring exposes exactly two operations.
2. Current-state docs must list the third latest-result operation.
3. Document:
   - terminal-only;
   - pending invisible;
   - 204 no terminal result;
   - resultKind is an execution fact;
   - no up/down derivation;
   - worker_timeout omits status/duration;
   - no history;
   - no host application port;
   - no auth/public deployment yet.
4. Extend runtime flow execution -> terminal CheckRun -> public latest-result read.
5. Historical design/scope docs remain historical.
6. Update architecture/doc fitness only where stale current-state assertions require it.

## Verification

Architecture documentation tests/checker + focused Go/contract checks.

## Self-review

No docs claim history/status/React/auth/public internet exposure.

---

# Task 8 — Whole-branch verification + implementation self-review

**Goal:** Prove only the approved slice is present before external review.

## Steps

1. Confirm branch ancestry from exact landed plan commit.
2. Review complete diff surface.
3. Run relevant repository/governance, public-contract, Go unit, real PostgreSQL integration, architecture, govulncheck, local-dev, Docker smoke, documentation fitness, and git diff --check.
4. Compare internal contract and fixtures against implementation base: unchanged.
5. Verify SQL migrations unchanged and no new SQL path.
6. Re-run exact legacy Monitor create/read compatibility tests.
7. Search for history/status/availability/pagination/auth/security/CORS/rate-limit scope drift.
8. Mark PR review-ready only after exact-head CI is GREEN.

Forbidden branch changes include new migration, Rust, internal OpenAPI, Compose topology, auth/public exposure, React, history/status.

---

# Task 9 — External review + exact-head landing

**Goal:** Land only after independent exact-head review.

## Steps

1. Trigger @coderabbitai review with exact implementation SHA.
2. Independently verify every actionable finding.
3. Apply only smallest in-scope fixes with regression evidence.
4. Re-run exact-head CI after fixes.
5. Final gate:
   - PR open;
   - draft false;
   - mergeable true;
   - CI/gate GREEN;
   - external review covers exact head;
   - zero unresolved actionable findings.
6. Squash merge with expected-head protection.
7. Require fresh push CI on resulting main commit.
8. Confirm landed capability is exactly:
   register Monitor -> read Monitor -> internal execution -> read one latest terminal execution fact.

Still absent: history, derived status, React, auth/public ingress, incidents/notifications, mutable Monitor lifecycle, multi-checker.

---

## 8. Verification Matrix

| Concern | RED evidence | GREEN evidence | Final evidence |
|---|---|---|---|
| New public route | semantic test rejects missing path | OpenAPI/checker pass | public-contract CI |
| Existing Monitor compatibility | exact old assertions retained | old tests pass | whole-branch review |
| 204 no-result | app/HTTP RED | mapping passes | production integration |
| Missing Monitor 404 | app/HTTP RED | mapping passes | production integration |
| Latest ordering | PG RED | query passes | go-api CI |
| Same-time determinism | PG tie RED | tie-break pass | go-api CI |
| Pending invisibility | PG RED | terminal-only query | production/Docker |
| No read mutation | DB before/after RED | read-only query | PG integration |
| HTTP result shape | contract/HTTP RED | exact fields | production integration |
| Failure shape | contract/HTTP RED | no status | Docker policy_rejected |
| worker_timeout shape | contract/app RED | no duration/status | production integration |
| Invalid durable shape | application RED | internal failure | Go tests |
| Public DTO isolation | review/tests | explicit mapping | architecture review |
| Execution repo isolation | architecture check | dedicated read port | branch review |
| No migration | diff guard | current schema passes | branch diff |
| Internal contract unchanged | base compare | unchanged | final review |
| Rust unchanged | base compare | unchanged | final review |
| Docker observability | smoke RED | exact persisted result public | local-dev CI |
| Documentation truth | stale fitness if needed | docs pass | final CI |
| Landing safety | exact-head gate | expected-head squash | fresh main CI |

---

## 9. Preferred Task Commit Shape

~~~text
Task 1  feat(contracts): define latest check result read
Task 2  feat(api): add latest result application read
Task 3  feat(api): read latest terminal check result
Task 4  feat(api): expose latest result transport
Task 5  feat(api): compose latest result read
Task 6  test(devops): verify latest result docker read
Task 7  docs(docs): document latest result runtime
~~~

Do not manufacture empty commits merely to preserve numbering.

---

## 10. CI Expectations

No CI workflow modification is expected.

Existing path-aware detection should activate jobs naturally.

Contract changes should activate public-contract.

Go changes should activate go-api.

Smoke/local-dev changes should activate local-dev.

Do not edit detection scripts merely to force a desired matrix.

---

## 11. Implementation STOP Conditions

Immediately stop and reopen design if implementation needs:

- new SQL migration;
- modification of 00001 or 00002;
- new index/status/materialized table;
- pending-state exposure;
- GET-side reconciliation;
- Rust changes;
- internal OpenAPI changes;
- host application port;
- auth/authz;
- up/down derivation;
- history/pagination;
- new router/framework dependency;
- four-service topology change;
- additional probe data persistence.

Convenience is not sufficient justification to cross these boundaries.

---

## 12. Plan Acceptance Criteria

This plan is GREEN only if review agrees that:

1. implementation begins only after this plan lands;
2. implementation uses a separate short-lived feature branch;
3. exactly one public operation is implemented;
4. existing Monitor payloads remain stable;
5. contract work precedes runtime work;
6. application semantics precede persistence/transport wiring;
7. PostgreSQL semantics are proven against real PostgreSQL;
8. pending state remains invisible;
9. read-side reconciliation is forbidden;
10. worker_timeout has no fabricated duration;
11. dedicated read port preserves execution-repository scope;
12. no migration is planned;
13. Rust/internal contract remain unchanged;
14. Docker evidence binds public payload to exact persisted CheckRun truth;
15. current-state docs are updated after runtime stabilizes;
16. whole-branch review precedes external review;
17. external review covers exact head;
18. squash merge uses expected-head protection;
19. fresh post-merge main CI is mandatory;
20. no history/status/Web/auth/public-exposure work enters the milestone.

---

## 13. Explicit STOP Boundary

This plan PR contains documentation only.

Do not begin implementation while this plan is still under review.

After this plan is reviewed, landed, and fresh main CI is GREEN:

~~~text
NEXT:
create feat/latest-check-result-public-read
open draft implementation PR
begin Task 1 — Public contract + semantic fitness
~~~

Until then do not modify public.yaml, Go runtime/application code, PostgreSQL adapter code, SQL migrations, Rust, Compose, CI, or current-state docs for unimplemented behavior.
