# Single-Checker Execution Vertical Slice Implementation Plan

**Status:** Review candidate
**Date:** 2026-09-26
**Repository:** kefyusuf/uptime-lab
**Scope:** Implement the landed Single-Checker Execution Vertical Slice design from internal contract through Go scheduling/persistence, Rust bounded execution, real Docker cross-runtime evidence, canonical documentation, and controlled landing
**Base:** main@685ae8f5f45f9eda01386129d392281e64f15564
**Authoritative design:** docs/superpowers/specs/2026-09-26-single-checker-execution-vertical-slice-design.md
**Authoritative public contract:** contracts/openapi/public.yaml

---

## 1. Purpose

The Single-Checker Execution Vertical Slice design is landed.

The repository currently registers and reads Monitoring targets but still performs no uptime checks.

This plan implements the first real execution loop:

~~~text
registered Monitor
  -> Go decides work is due
  -> Rust claims one work item
  -> Rust performs one bounded HTTP/HTTPS probe
  -> Rust submits one normalized result
  -> Go persists one terminal CheckRun
~~~

The implementation must preserve the accepted ownership boundary:

~~~text
Go   = product truth, scheduling/coordination, API semantics, persistence
Rust = bounded network execution and transport/protocol normalization
~~~

The implementation phase MUST NOT silently become a public-status, Web, multi-worker, broker, authentication, mutable-lifecycle, or private-network-monitoring milestone.

---

## 2. Existing Authority

The implementation does not reopen these landed design decisions.

### Execution topology

- Exactly one logical Checker process is supported.
- Checker concurrency is bounded at four probes.
- Go owns scheduling truth.
- Rust never accesses PostgreSQL.
- Cross-runtime communication is contract-driven.
- Direct internal HTTP is sufficient; no broker is introduced.
- Canonical Compose remains exactly four services: web, db, api, checker.
- No application host port is published.

### Scheduling and identity

- Every persisted Monitor is eligible; no enabled/disabled lifecycle exists.
- A Monitor with no terminal CheckRun is immediately due.
- Fixed cadence is 60 seconds.
- Never-checked ordering uses monitor created_at.
- Subsequent ordering uses latest terminal completed_at.
- Go generates every CheckID as UUID v7.
- Work claim creates durable pending CheckRun state.
- At most one pending CheckRun per Monitor is mechanically enforced.
- issued_at and deadline_at are Go server time.
- deadline_at = issued_at + 20 seconds.
- Rust probe timeout is 10 seconds.
- Expired pending work becomes worker_timeout using stored deadline_at as terminal completion time.

### Internal contract

The internal contract later contains exactly:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

Claim returns at most one CheckWork.

CheckWork contains exactly the execution concepts:

~~~text
checkId
monitorId
targetUrl
timeoutMs
maxRedirects
~~~

with fixed values:

~~~text
timeoutMs    = 10000
maxRedirects = 3
~~~

Result submission is idempotent for an exact duplicate canonical payload.

Canonical duplicate equality compares only:

~~~text
kind
durationMs
httpStatus presence/value
~~~

### Result vocabulary

Rust may submit exactly:

~~~text
http_response
dns_error
policy_rejected
timeout
connect_error
tls_error
protocol_error
internal_error
~~~

Go alone may create:

~~~text
worker_timeout
~~~

Rust result payload is:

~~~text
kind
durationMs
httpStatus?  only for http_response
~~~

durationMs is an integer in the inclusive range 0..20000.

Raw library errors, headers, response bodies, IP addresses, stack traces, and credentials do not cross the internal contract.

### Persistence

Exactly one new Monitoring table is introduced:

~~~text
monitoring.check_runs
~~~

No schedules, monitor_states, incidents, leases, or event-store tables are introduced.

The existing 00001 migration remains byte-identical.

The new schema must enforce pending/terminal row consistency, one pending row per Monitor, duration bounds, HTTP status bounds, and timestamp ordering.

### Probe security

Production execution is deny-by-default for non-public destinations.

The implementation must preserve:

- HTTP/HTTPS only;
- default ports only: HTTP 80, HTTPS 443;
- no private/loopback/link-local/unique-local/multicast/reserved/non-global destinations;
- IPv4-mapped IPv6 normalized according to effective IPv4 destination;
- reject a hostname when any resolved address is forbidden;
- one DNS resolution per request/redirect hop;
- connection attempts only against that validated set;
- no connect-time re-resolution;
- original authority preserved for Host, TLS SNI, and hostname verification;
- serial fallback across validated addresses only after TCP establishment failure;
- no fallback after TLS/protocol/HTTP terminal outcome;
- every redirect fully revalidated;
- HTTPS to HTTP downgrade rejected;
- maximum three redirects;
- direct connections only; ambient proxy configuration ignored;
- ordinary HTTPS certificate and hostname verification; no skip-verify mode;
- maximum four concurrent probes;
- 10-second overall probe timeout;
- response headers bounded to 64 KiB;
- response body not application-consumed;
- no production private-network bypass.

### Product boundary

- contracts/openapi/public.yaml remains byte-identical.
- No public Monitor status/history is introduced.
- No Monitor list/update/delete/enable/disable is introduced.
- No authentication/authorization/CORS/rate limit/public ingress is introduced.
- No React implementation is introduced.
- No TCP/DNS/TLS-only/ICMP probe type is introduced.

---

## 3. Phase Boundary

### In scope

- first internal OpenAPI contract and semantic fitness;
- split public/internal contract CI;
- Go CheckID and normalized result primitives;
- Go ClaimDueCheck and SubmitCheckResult application capabilities;
- additive check_runs SQL migration;
- real PostgreSQL schema invariants;
- atomic due-work claim;
- expired pending reconciliation;
- idempotent/conflicting/late result completion;
- internal Checker HTTP adapter;
- Rust workspace and architecture fitness;
- Rust CI and vulnerability audit;
- production destination policy;
- deterministic resolver/address-policy tests;
- bounded HTTP/HTTPS probe adapter;
- control-plane client adapter;
- cross-runtime fixtures;
- bounded Checker worker loop and lifecycle;
- production Go internal-route composition;
- real Checker container;
- canonical four-service Compose evolution;
- real Docker Go -> Rust -> policy_rejected -> Go persistence smoke;
- canonical documentation and documentation fitness;
- whole-branch verification;
- external review;
- squash landing;
- fresh post-merge main CI.

### Out of scope

- changes to contracts/openapi/public.yaml;
- changes to 00001_create_monitoring_monitors.sql;
- a third SQL migration;
- public CheckRun/status/history endpoints;
- Monitor list/update/delete/enable/disable;
- user-configurable cadence;
- automatic probe retry;
- multi-checker execution;
- worker IDs;
- distributed leases;
- broker/queue/outbox infrastructure;
- private-network monitoring mode;
- arbitrary CIDR allowlists;
- proxy-capable probing;
- insecure TLS;
- custom user CA;
- client certificates;
- response-body assertions;
- incidents/notifications/status pages;
- accounts/teams/tenancy/billing;
- Web implementation;
- public host exposure;
- auth/authz;
- CORS/rate limiting;
- Kubernetes;
- OpenTelemetry backend infrastructure;
- generated cross-runtime source as a requirement.

If implementation appears to require one of these, STOP and reopen design/product scope.

---

## 4. Implementation Discipline

### 4.1 Branch

After this plan lands, create:

~~~text
feat/single-checker-execution-slice
~~~

from the exact landed plan commit.

Do not implement from the design branch or plan branch.

### 4.2 Pull request lifecycle

Open one draft implementation PR immediately:

~~~text
feat(checker): implement single-checker execution slice
~~~

Keep it draft through Tasks 1-14.

Every implementation task closes with:

1. RED evidence produced only in local/task working state;
2. GREEN implementation;
3. focused local verification;
4. explicit self-review;
5. one coherent task commit;
6. fresh remote CI on that exact head;
7. STOP before the next task until required jobs are GREEN.

Task 15 performs whole-branch review and marks the PR review-ready.

Task 16 performs independent external review, finding resolution, exact-head landing, and post-merge main verification.

### 4.3 RED -> GREEN rule

No deliberately failing commit is pushed.

A task may use RED tests during implementation, but its committed head must be GREEN.

Evidence-only temporary PRs are unnecessary for this phase unless GitHub-hosted behavior cannot be reproduced locally. If one becomes necessary, it must be explicitly labeled non-mergeable and closed after evidence is captured.

### 4.4 Scope-drift review

At every task self-review search for accidental introduction of:

- public status/history routes;
- GET collection /monitors;
- Monitor lifecycle mutation;
- a third internal operation;
- Rust PostgreSQL access;
- worker identity/leases;
- broker dependencies;
- private-network bypass configuration;
- ambient proxy use;
- insecure TLS;
- user-configurable cadence/retry;
- public host ports;
- auth/securitySchemes;
- Web implementation;
- response-body persistence;
- a root shared/common/utils business package.

Occurrences are permitted only in negative tests, historical design text, or explicit deferred-scope documentation.

---

## 5. Reviewed Tooling Baseline

Current landed baseline:

~~~text
Go                     1.27.1
PostgreSQL image       postgres:18.6-alpine3.24
pgx                    github.com/jackc/pgx/v5 v5.11.0
goose                  github.com/pressly/goose/v3 v3.28.0
govulncheck            v1.8.0
Node contract runtime  24.21.0
Redocly CLI            2.53.3
Alpine runtime         3.24.2
~~~

Rust baseline reviewed at plan authoring:

~~~text
Rust toolchain          1.98.1
Rust builder image     rust:1.98.1-alpine3.24
cargo-audit            0.22.2
~~~

Rust 1.98.1 is the current stable point release at plan authoring and includes a compiler miscompilation fix relative to 1.98.0.

The official Docker Rust image publishes the exact 1.98.1-alpine3.24 tag.

Task 6 must lock the toolchain and Cargo.lock.

Application crate versions are selected minimally when their first behavior is implemented, committed through Cargo.toml/Cargo.lock, and reviewed against the locked design. Do not add a networking crate merely because it is conventional; it must support the required connection-binding, TLS, redirect, proxy, timeout, and header-budget behavior.

Prefer rustls-based TLS and avoid native OpenSSL unless a concrete implementation blocker is proven and reviewed.

---

## 6. Brownfield CI Constraints

The current repository intentionally reflects the pre-Checker phase.

Two existing invariants must evolve carefully.

### 6.1 Contract CI currently forbids internal.yaml

The current public-contract job asserts:

~~~text
contracts/openapi/internal.yaml does not exist
contracts/openapi contains only public.yaml
~~~

Task 1 must replace this historical assertion with separate public/internal contract ownership.

The public contract verifier must remain focused on public.yaml.

A new internal-contract verifier/job must own internal.yaml.

### 6.2 Local-dev fitness currently forbids apps/checker

The current local-dev checker allows only apps/api because Checker is still a placeholder.

Task 6 introduces apps/checker source and must update local-dev fitness to allow the source tree while still requiring the Compose checker service to remain the placeholder until Task 12.

Task 12 then evolves that same fitness contract from placeholder Checker to real Checker runtime.

No task may simply delete the old protections without a replacement invariant.

---

## 7. Expected Complete-Branch File Map

The exact split may vary narrowly for clarity, but the final change surface should remain within these domains.

### Internal contract and CI

~~~text
Create:
contracts/openapi/internal.yaml
scripts/ci/check-internal-contract.mjs
scripts/ci/test-check-internal-contract.mjs
scripts/ci/detect-internal-contract-changes.sh
scripts/ci/test-detect-internal-contract-changes.sh

Modify:
scripts/ci/detect-public-contract-changes.sh
scripts/ci/test-detect-public-contract-changes.sh
.github/workflows/ci.yml
~~~

### Go execution model

Expected additions under:

~~~text
apps/api/internal/modules/monitoring/domain/
apps/api/internal/modules/monitoring/application/
apps/api/internal/modules/monitoring/ports/
apps/api/internal/modules/monitoring/adapters/postgres/
apps/api/internal/modules/monitoring/adapters/checkerhttp/
~~~

Expected production composition changes:

~~~text
apps/api/internal/modules/monitoring/module.go
apps/api/cmd/api/main.go
apps/api/cmd/api/main_test.go
apps/api/cmd/api/main_integration_test.go
~~~

### SQL migration

~~~text
Create:
apps/api/migrations/00002_create_monitoring_check_runs.sql

Modify tests/providers only as required.
~~~

The following remains byte-identical:

~~~text
apps/api/migrations/00001_create_monitoring_monitors.sql
~~~

### Rust Checker

~~~text
Create:
apps/checker/Cargo.toml
apps/checker/Cargo.lock
apps/checker/rust-toolchain.toml
apps/checker/Dockerfile
apps/checker/crates/checker-core/...
apps/checker/crates/probe-http/...
apps/checker/crates/control-plane-client/...
apps/checker/crates/checker/...
~~~

### Rust CI / architecture fitness

~~~text
Create:
scripts/ci/detect-checker-changes.sh
scripts/ci/test-detect-checker-changes.sh
scripts/ci/check-rust-architecture.sh
scripts/ci/test-check-rust-architecture.sh

Modify:
.github/workflows/ci.yml
scripts/ci/check-local-dev.sh
scripts/ci/test-check-local-dev.sh
scripts/ci/detect-local-dev-changes.sh
scripts/ci/test-detect-local-dev-changes.sh
~~~

### Cross-runtime fixtures

Expected:

~~~text
contracts/fixtures/internal/check-work.json
contracts/fixtures/internal/check-result-http.json
contracts/fixtures/internal/check-result-failure.json
~~~

Keep fixtures small. They are compatibility evidence, not a second contract language.

### Local runtime / smoke

~~~text
Modify:
compose.yaml
scripts/ci/smoke-local-dev.sh
scripts/ci/test-smoke-local-dev.sh
scripts/ci/check-local-dev.sh
scripts/ci/test-check-local-dev.sh
~~~

### Canonical documentation

Expected current-state updates after implementation truth exists:

~~~text
README.md
docs/README.md
docs/architecture/container-view.md
docs/architecture/runtime-flows.md
docs/architecture/module-boundaries.md
docs/architecture/dependency-rules.md
docs/architecture/data-ownership.md
docs/backend/go-control-plane.md
docs/devops/local-development.md
docs/testing/go-monitoring-foundation.md
docs/testing/go-public-transport-adapter.md

Create:
docs/checker/rust-checker.md
docs/testing/single-checker-execution-slice.md

Modify:
scripts/ci/check-architecture-docs.sh
scripts/ci/test-architecture-docs.sh
~~~

### Files that must remain unchanged

~~~text
contracts/openapi/public.yaml
apps/api/migrations/00001_create_monitoring_monitors.sql
~~~

---

# Task 1 — Internal contract source + contract CI separation

**Goal:** Land the authoritative two-operation internal contract and evolve CI so public/internal contracts are verified independently without weakening public-contract protection.

**Primary files:**

- Create contracts/openapi/internal.yaml
- Create scripts/ci/check-internal-contract.mjs
- Create scripts/ci/test-check-internal-contract.mjs
- Create scripts/ci/detect-internal-contract-changes.sh
- Create scripts/ci/test-detect-internal-contract-changes.sh
- Modify scripts/ci/detect-public-contract-changes.sh
- Modify scripts/ci/test-detect-public-contract-changes.sh
- Modify .github/workflows/ci.yml

## Step 1 — Create implementation branch and draft PR

Create feat/single-checker-execution-slice from the exact main commit containing this plan.

Open the draft PR before product implementation.

## Step 2 — Write internal-contract semantic tests RED

The semantic checker must prove exactly two operations:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

It must reject:

- additional operations;
- copied public Monitor create/read operations;
- GET work acquisition;
- POST result submission;
- worker identity fields;
- arbitrary map payloads;
- raw error/message fields;
- worker_timeout in Rust-submitted result kind;
- securitySchemes/auth invented in this milestone.

## Step 3 — Lock exact claim contract

Claim request:

- no request body;
- 200 returns CheckWork;
- 204 returns no body;
- 405 and 500 use sanitized Problem.

CheckWork exact properties:

~~~text
checkId
monitorId
targetUrl
timeoutMs
maxRedirects
~~~

Rules:

- additionalProperties false;
- checkId format uuid;
- monitorId format uuid;
- targetUrl string with no new RFC uri format constraint;
- timeoutMs const 10000;
- maxRedirects const 3.

## Step 4 — Lock exact result contract

Result request uses application/json.

Represent conditional result shape with a closed schema.

HTTP response form:

~~~text
kind = http_response
durationMs integer 0..20000
httpStatus integer 100..599
~~~

Failure form:

~~~text
kind in:
  dns_error
  policy_rejected
  timeout
  connect_error
  tls_error
  protocol_error
  internal_error

durationMs integer 0..20000
no httpStatus
~~~

Result responses:

~~~text
204 accepted or exact duplicate
400 malformed checkId / syntax
404 unknown checkId
409 expired/conflicting terminal result
415 unsupported media type
422 structurally valid but semantically invalid result
405 unsupported method
500 unexpected failure
~~~

## Step 5 — Keep public contract byte-identical

Do not edit contracts/openapi/public.yaml.

Modify the public-contract job only to stop treating internal.yaml existence as a public-contract failure.

Public verification must still lint/bundle/check public.yaml exactly as before.

Add a self-review SHA/blob comparison against the task base.

## Step 6 — Split path-aware detection

public-contract detector should trigger on:

- public.yaml;
- public contract checker/test/detector files;
- workflow changes.

internal-contract detector should trigger on:

- internal.yaml;
- internal contract checker/test/detector files;
- internal fixture files once introduced;
- workflow changes.

A change to internal.yaml alone must not falsely claim that public.yaml changed.

## Step 7 — Add internal-contract CI job

Use the existing pinned Node 24.21.0 and Redocly 2.53.3 baseline.

The new job must:

- verify exact checked-out SHA;
- lint internal.yaml;
- bundle it;
- run internal semantic tests;
- run the checker against the bundled artifact.

Add internal_contract output to changes job.

Add internal-contract to CI / gate with success|skipped semantics.

## Step 8 — Local verification

At minimum:

~~~text
node scripts/ci/test-check-internal-contract.mjs
node scripts/ci/test-check-public-contract.mjs
./scripts/ci/test-detect-internal-contract-changes.sh
./scripts/ci/test-detect-public-contract-changes.sh
~~~

Run Redocly lint/bundle for both artifacts.

## Step 9 — Self-review

Verify:

- internal.yaml has exactly two operations;
- public.yaml is byte-identical to base;
- no runtime code exists for internal operations yet;
- no SQL migration exists yet;
- no Rust source exists yet;
- no securitySchemes;
- no worker identity;
- no public status/history.

## Step 10 — Commit + exact-head CI

Suggested commit:

~~~text
feat(contracts): define checker internal contract
~~~

Because workflow changes, require:

~~~text
policy             SUCCESS
repository         SUCCESS
changes            SUCCESS
go-api             SUCCESS
local-dev          SUCCESS
public-contract    SUCCESS
internal-contract  SUCCESS
CI / gate          SUCCESS
~~~

Do not start Task 2 until GREEN.

---

# Task 2 — Go CheckID, result model, and application primitives

**Goal:** Establish framework-neutral Go execution semantics before persistence or HTTP wiring.

**Primary areas:**

- Monitoring domain
- Monitoring application
- Monitoring ports

No SQL or internal HTTP adapter yet.

## Step 1 — CheckID RED tests

Create CheckID modeled consistently with MonitorID.

Prove:

- valid UUID parses;
- invalid/empty UUID rejected;
- exact string round-trip;
- no HTTP/database dependency.

Production generation remains UUID v7 and is injected into application behavior.

## Step 2 — Normalized result RED tests

Create closed execution-result vocabulary matching internal.yaml.

Prove:

- each Rust-submittable kind accepted;
- worker_timeout cannot be constructed as a Rust result input;
- http_response requires status 100..599;
- failure kinds reject HTTP status;
- duration accepts 0 and 20000;
- duration rejects negative and >20000;
- result type contains no raw error text.

## Step 3 — Work description/application model

Define application-facing CheckWork with only:

~~~text
CheckID
MonitorID
TargetURL
Timeout
MaxRedirects
~~~

Fixed policy values originate in Go application code:

~~~text
timeout      10 seconds
maxRedirects 3
cadence      60 seconds
window       20 seconds
~~~

Do not make these environment/user configuration.

## Step 4 — Define narrow execution persistence port

Add a Monitoring-owned port sufficient for:

- atomic claim;
- result completion.

The application layer supplies semantic inputs such as:

- now;
- dueBefore;
- generated CheckID;
- deadline;
- validated normalized result.

The adapter owns SQL transactions/locking.

Do not introduce generic repository/unit-of-work abstractions.

## Step 5 — ClaimDueCheck unit tests

With a fake port, injected Clock, and injected ID generator prove:

- now comes from Go server clock;
- dueBefore = now - 60s;
- deadline = now + 20s;
- timeout/maxRedirects fixed;
- generated CheckID is forwarded;
- no work propagates as stable application no-work result;
- persistence failures map to stable application failure;
- ID-generation failure prevents persistence call.

Wasting an unused generated UUID when no work is available is acceptable if that keeps dependencies explicit; durable identity exists only after successful insert.

## Step 6 — SubmitCheckResult unit tests

Prove:

- malformed CheckID rejected before persistence;
- result validation occurs before persistence;
- duration bound is 0..20000;
- now comes from Go server clock;
- unknown run / conflict / persistence errors map to stable application errors;
- exact duplicate success is represented as success, not a second product event.

## Step 7 — Architecture fitness

Extend Go architecture tests only if new package paths require explicit enforcement.

Domain/application remain:

- net/http-free;
- pgx-free;
- goose-free;
- adapter-free;
- platform-free.

## Step 8 — Verification / self-review / commit

Run Go unit + architecture suites.

Suggested commit:

~~~text
feat(api): add check execution application primitives
~~~

Require go-api + aggregate CI GREEN.

STOP before Task 3.

---

# Task 3 — Additive check_runs migration + real PostgreSQL invariants

**Goal:** Add exactly one durable execution table with mechanical row-state and concurrency invariants.

**Primary files:**

- Create apps/api/migrations/00002_create_monitoring_check_runs.sql
- Extend migration integration tests

00001 must remain byte-identical.

## Step 1 — Migration RED integration tests

Prove migration up/down/up against real PostgreSQL.

Expected table:

~~~text
monitoring.check_runs
~~~

Conceptual columns are exactly those in the design:

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

No target_url snapshot and no extra lifecycle/scheduling columns.

## Step 2 — Row-shape constraints

Database constraints must reject invalid states.

Pending:

~~~text
completed_at NULL
result_kind NULL
http_status NULL
duration_ms NULL
~~~

Terminal HTTP:

~~~text
completed_at NOT NULL
result_kind = http_response
http_status 100..599
duration_ms 0..20000
~~~

Terminal Rust failure:

~~~text
completed_at NOT NULL
result_kind in closed Rust failure set
http_status NULL
duration_ms 0..20000
~~~

worker_timeout:

~~~text
completed_at NOT NULL
result_kind = worker_timeout
http_status NULL
duration_ms NULL
~~~

Also enforce:

~~~text
deadline_at > issued_at
completed_at >= issued_at when terminal
~~~

## Step 3 — One pending run per Monitor

Add a partial unique index enforcing:

~~~text
UNIQUE monitor_id WHERE completed_at IS NULL
~~~

Real PostgreSQL test must prove a second pending row for the same Monitor fails while multiple terminal historical rows remain valid.

## Step 4 — Query-support index only where justified

Add only an index required by due-work/latest-terminal lookup, for example a Monitor + terminal-completion index.

Do not add speculative analytics/history indexes.

## Step 5 — Readiness compatibility

Because expected migrations are derived from embedded sources, schema-aware readiness must automatically require 00002.

Prove:

- before explicit 00002 application -> incompatible;
- after explicit up -> compatible;
- API startup still never migrates.

## Step 6 — Migration-history guard

Run the immutable migration guard and prove 00001 has identical blob content to main.

The guard must accept new 00002.

## Step 7 — Verification / self-review / commit

Suggested commit:

~~~text
feat(api): add check run persistence schema
~~~

Require repository, go-api, local-dev, aggregate gate GREEN.

STOP before Task 4.

---

# Task 4 — Atomic PostgreSQL claim and result completion

**Goal:** Implement the transactional persistence mechanics for due work and idempotent result completion.

**Primary area:**

~~~text
apps/api/internal/modules/monitoring/adapters/postgres/
~~~

## Step 1 — Real PostgreSQL RED tests first

Cover:

- immediate due Monitor with no CheckRun;
- never-checked ordering by Monitor created_at;
- terminal ordering by latest completed_at;
- Monitor not due before completed_at + 60s;
- no new claim while pending;
- expired pending becomes worker_timeout;
- timeout completed_at equals stored deadline_at;
- next cadence derives from timeout terminal instant;
- deterministic MonitorID tie-break;
- one pending row per Monitor under concurrent claim attempts;
- independent due Monitors can each receive work;
- first result completion;
- exact duplicate result no-op;
- conflicting duplicate rejection;
- late result terminalizes/observes worker_timeout and conflicts;
- unknown CheckID;
- persistence error behavior.

## Step 2 — Atomic claim transaction

Conceptual transaction:

~~~text
BEGIN
terminalize expired pending rows
select oldest due Monitor with no pending row
lock candidate safely
insert pending CheckRun
return target/work identity
COMMIT
~~~

Use PostgreSQL locking/transaction semantics appropriate to the query.

SKIP LOCKED is permitted as a race-safety mechanism but does not authorize multi-worker semantics.

Rely on the partial unique index as the final data-integrity backstop.

## Step 3 — Due selection semantics

The adapter receives Go-computed:

~~~text
now
dueBefore
CheckID
deadline
~~~

Scheduling point:

~~~text
no terminal row -> monitor.created_at
terminal exists -> latest terminal completed_at
~~~

Pending Monitor is excluded.

## Step 4 — Result transaction

Lock the CheckRun row.

Behavior:

~~~text
missing -> not found
terminal exact canonical payload -> success no-op
terminal different payload -> conflict
pending and now >= deadline -> worker_timeout + conflict
pending and valid payload -> terminalize at now + success
~~~

Exact canonical equality compares:

~~~text
result_kind
duration_ms
http_status presence/value
~~~

It does not compare server-owned timestamps.

## Step 5 — Mutation discipline

Terminal rows are immutable.

An exact duplicate does not rewrite completed_at or result columns.

No probe result update/delete path is introduced.

## Step 6 — Verification / self-review / commit

Run unit + race + real PostgreSQL suites.

Suggested commit:

~~~text
feat(api): implement atomic check execution persistence
~~~

Require go-api GREEN and aggregate gate GREEN.

STOP before Task 5.

---

# Task 5 — Internal Checker HTTP adapter

**Goal:** Expose the two internal operations through an isolated Monitoring outward adapter without production wiring yet.

**Expected package:**

~~~text
apps/api/internal/modules/monitoring/adapters/checkerhttp/
~~~

Names may vary narrowly, but do not merge this transport into the public handler package if that obscures the trust boundary.

## Step 1 — Narrow use-case interfaces

Adapter depends only on application-facing interfaces for:

- ClaimDueCheck;
- SubmitCheckResult.

No pgx/postgres/platform/goose imports.

## Step 2 — Claim handler RED tests

Prove exact path/method behavior:

~~~text
POST /internal/checks/claim
~~~

Success/no-work:

- 200 exact CheckWork JSON;
- 204 no body;
- 200 Content-Type application/json;
- exact response fields only.

Reject unsupported methods with 405 + exact Allow.

Unknown/trailing/nested paths remain 404.

Claim has no request body. If body semantics need explicit rejection, lock that behavior in tests and contract consistently; do not invent product payload.

## Step 3 — Result handler RED tests

Exact route:

~~~text
PUT /internal/checks/{checkId}/result
~~~

Prove:

- 204 accepted;
- 204 exact duplicate;
- malformed textual checkId -> 400 without use-case execution;
- unknown -> 404;
- expired/conflicting terminal -> 409;
- missing/unsupported/malformed Content-Type -> 415;
- malformed JSON -> 400;
- valid JSON wrong shape -> 422;
- invalid duration/status/kind -> 422;
- 405 for unsupported known-resource methods;
- raw internal errors never leak.

## Step 4 — Deterministic JSON classification

Use the same disciplined parsing approach as the public adapter:

- mime.ParseMediaType;
- exactly one JSON document;
- exact object shape;
- closed field set;
- map transport syntax vs semantic validation deterministically.

No body-size/413 policy is invented unless design is reopened.

## Step 5 — Problem Details

Use sanitized RFC 9457-compatible internal Problem responses.

Do not expose SQL, Rust, DNS, TLS, filesystem, or credential details.

## Step 6 — Architecture fitness

Expand Go architecture tests to protect checkerhttp from:

- postgres adapter;
- platform;
- pgx;
- goose.

## Step 7 — Verification / self-review / commit

No cmd/api production wiring yet.

Suggested commit:

~~~text
feat(api): add checker internal HTTP adapter
~~~

Require go-api GREEN.

STOP before Task 6.

---

# Task 6 — Rust workspace + Checker CI/architecture foundation

**Goal:** Introduce the Rust source boundary and deterministic CI without replacing the Compose placeholder yet.

**Primary files:**

- apps/checker Cargo workspace
- rust-toolchain.toml
- Cargo.lock
- four committed crates
- Rust change detector
- Rust architecture fitness
- workflow checker job
- local-dev fitness evolution to permit source presence

## Step 1 — Create workspace shape

Create exactly:

~~~text
apps/checker/
├── Cargo.toml
├── Cargo.lock
├── rust-toolchain.toml
└── crates/
    ├── checker-core/
    ├── probe-http/
    ├── control-plane-client/
    └── checker/
~~~

Do not create speculative fifth crates.

## Step 2 — Pin toolchain

Lock Rust 1.98.1.

CI must not silently use runner-default Rust.

Use rustfmt and clippy for the same pinned toolchain.

## Step 3 — Minimal initial crate responsibilities

checker-core:

- contract-neutral work/result concepts;
- Probe port;
- ControlPlane port;
- orchestration primitives.

probe-http:

- adapter shell only at first.

control-plane-client:

- adapter shell only at first.

checker:

- binary/composition shell only at first.

Do not prematurely add network behavior in this task.

## Step 4 — Rust architecture fitness RED -> GREEN

Create mechanical checks proving at minimum:

- no postgres/sqlx/diesel/tokio-postgres dependency in Checker product crates;
- checker-core does not depend on probe-http/control-plane-client/checker;
- probe-http may depend inward on checker-core;
- control-plane-client may depend inward on checker-core;
- checker binary may compose all three;
- no root shared/common business crate;
- production configuration source does not contain known private-network bypass controls.

Prefer Cargo metadata plus narrowly scoped source/config checks over brittle grep-only rules where metadata can prove dependency direction.

## Step 5 — Checker path detection

Create detector/tests so apps/checker changes trigger checker job.

Workflow changes conservatively trigger checker.

Cross-runtime internal fixtures should trigger both internal-contract and checker verification once present.

## Step 6 — Add checker CI job

Checker job must verify exact checkout SHA and use the pinned toolchain.

At minimum:

~~~text
cargo fmt --all --check
cargo check --workspace --all-targets --locked
cargo clippy --workspace --all-targets --all-features --locked -- -D warnings
cargo test --workspace --all-targets --locked
~~~

Install and run cargo-audit 0.22.2 against Cargo.lock.

Do not use unpinned cargo install latest.

## Step 7 — Evolve local-dev fitness without replacing runtime

Current local-dev fitness rejects apps/checker.

Change it so:

- apps/api and apps/checker are allowed source apps;
- checker Compose service is STILL required to use the placeholder image in Task 6;
- no Checker Dockerfile is required yet;
- no host ports;
- four-service topology unchanged.

Add fixture tests proving source presence is allowed but premature Compose real-checker wiring still fails.

## Step 8 — Gate evolution

changes job gains checker output.

CI / gate gains checker with success|skipped semantics.

Because workflow changes, exact-head CI should exercise all major jobs.

## Step 9 — Verification / self-review / commit

Suggested commit:

~~~text
build(checker): establish Rust checker workspace
~~~

Require:

~~~text
policy             SUCCESS
repository         SUCCESS
changes            SUCCESS
go-api             SUCCESS
checker            SUCCESS
local-dev          SUCCESS
public-contract    SUCCESS
internal-contract  SUCCESS
CI / gate          SUCCESS
~~~

STOP before Task 7.

---

# Task 7 — Production destination policy and resolver boundary

**Goal:** Implement the security decision layer before any real outbound probe behavior.

**Primary crate:**

~~~text
probe-http
~~~

Network connection itself may remain test-doubled in this task.

## Step 1 — Pure address-policy RED tests

Cover at least:

- IPv4 unspecified;
- loopback;
- RFC1918;
- link-local;
- multicast;
- broadcast/reserved;
- documentation/benchmark ranges;
- IPv6 unspecified;
- loopback;
- link-local;
- unique-local;
- multicast/reserved/non-global;
- IPv4-mapped IPv6 normalization;
- public IPv4/IPv6 allowed.

## Step 2 — Host resolution policy

Use an injectable resolver boundary.

For hostname resolution:

- resolve once per hop;
- empty resolution is failure;
- any forbidden answer rejects the entire hop;
- deduplicate permitted addresses preserving resolver order.

IP literals bypass DNS but use the same address classification.

## Step 3 — Scheme/port policy

Allow only:

~~~text
http  -> effective port 80
https -> effective port 443
~~~

Non-default explicit ports normalize to policy_rejected.

Unsupported schemes normalize to policy_rejected.

userinfo remains rejected.

## Step 4 — Redirect trust policy

Create a reusable hop validator.

Every redirect target:

- resolve relative Location against current URL according to normal URL semantics;
- re-run scheme/userinfo/port/address checks;
- reject HTTPS -> HTTP;
- permit HTTP -> HTTPS if all other checks pass;
- max three redirects.

Do not trust the previous hop's resolution.

## Step 5 — No production bypass

Production config/model must contain no:

- allowPrivate;
- disableSsrf;
- insecureTestMode;
- arbitrary CIDR allowlist;
- skipDnsValidation;
- proxy URL.

Architecture fitness should fail if equivalent bypass controls are introduced later.

## Step 6 — Synthetic deterministic tests

No public internet.

Use resolver/address fixtures for all policy paths.

## Step 7 — Verification / self-review / commit

Suggested commit:

~~~text
feat(checker): add production probe destination policy
~~~

Require checker GREEN.

STOP before Task 8.

---

# Task 8 — Bounded HTTP/HTTPS probe adapter

**Goal:** Perform real safe HTTP/HTTPS mechanics while preserving validated-address binding and fixed execution budgets.

## Step 1 — Select minimal network dependencies deliberately

Choose the smallest Rust HTTP/TLS stack that can mechanically support:

- custom per-hop validated address set;
- direct connections without ambient proxy;
- original Host authority;
- TLS SNI + certificate hostname verification;
- no connect-time DNS re-resolution;
- serial validated-address fallback;
- explicit redirect loop controlled by our code;
- 10-second overall deadline;
- bounded response-header processing;
- body not consumed.

Record dependency rationale in the implementation PR/task self-review.

Commit exact Cargo.lock.

Do not accept a library default that violates the design merely to reduce code.

## Step 2 — Controlled local integration test harness

Use local test servers only in tests.

The test-only injected policy may permit exactly the local fixture addresses.

It must not be reachable from production Checker configuration.

## Step 3 — Connection binding tests

For one hop prove:

- resolver called once;
- connection attempts use only returned validated addresses;
- resolver not called again during TCP fallback;
- fallback is serial;
- fallback occurs only after TCP establishment failure;
- once connected, TLS/protocol/HTTP terminal outcome stops fallback;
- all attempts share the one overall deadline.

## Step 4 — HTTP response mechanics

Use GET.

For final response:

- capture final HTTP status;
- measure duration to final response headers;
- do not consume body;
- do not persist headers.

## Step 5 — Header/resource bounds

Enforce:

~~~text
overall probe timeout 10s
redirects            <= 3
header processing    <= 64 KiB
body consumed        0 bytes
~~~

Oversized/invalid protocol headers must produce a normalized safe failure, not unbounded allocation.

## Step 6 — TLS tests

Prove:

- ordinary valid TLS works where controlled fixtures make this practical;
- hostname/certificate mismatch -> tls_error;
- no skip-verify path;
- connecting to validated IP still validates certificate against original hostname.

## Step 7 — Redirect tests

Cover:

- relative redirect resolution;
- allowed redirect;
- max redirect exceeded;
- redirect to private address -> policy_rejected before connect;
- HTTPS -> HTTP -> policy_rejected;
- non-default port redirect -> policy_rejected.

## Step 8 — Failure normalization

Map terminal outcomes into closed vocabulary:

~~~text
http_response
dns_error
policy_rejected
timeout
connect_error
tls_error
protocol_error
internal_error
~~~

Never expose library error text through result values.

## Step 9 — Verification / self-review / commit

Suggested commit:

~~~text
feat(checker): implement bounded HTTP probe adapter
~~~

Require checker + audit GREEN.

STOP before Task 9.

---

# Task 9 — Control-plane client + cross-runtime fixtures

**Goal:** Implement the Rust adapter for the exact internal contract and prove semantic compatibility with Go-facing contract examples.

**Primary crate:**

~~~text
control-plane-client
~~~

## Step 1 — Shared compatibility fixtures

Create a minimal fixture set under:

~~~text
contracts/fixtures/internal/
~~~

Include representative:

- CheckWork;
- http_response result;
- failure result.

Fixtures must conform to internal.yaml.

They are not a replacement contract.

## Step 2 — Claim mapping tests

Prove:

- POST exact claim path;
- no request body;
- 200 exact CheckWork decoding;
- 204 no-work;
- unexpected success shape rejected safely;
- 4xx/5xx mapped to stable client errors;
- no raw response body promoted into product result.

## Step 3 — Result PUT tests

Prove:

- exact result route by CheckID;
- application/json;
- 204 accepted;
- 404/409/415/422/500 stable handling;
- identical retry body can be reused byte-for-byte after ambiguous transport failure.

Prefer constructing/serializing the canonical result payload once and reusing it for retries.

## Step 4 — Bound every control-plane HTTP operation

Every claim and result HTTP operation has one fixed overall client deadline:

~~~text
5 seconds
~~~

The deadline covers the complete HTTP operation, not only TCP connect.

This is separate from:

- the 10-second target probe timeout;
- the 20-second server-side CheckRun acceptance window.

Every in-flight control-plane HTTP operation must also observe Checker shutdown cancellation.

Required behavior:

- claim deadline expiry is an ambiguous claim transport failure unless a definite HTTP response was received;
- result deadline expiry is a result-delivery transport failure eligible only for the bounded same-payload retry policy;
- shutdown cancellation aborts in-flight claim/result HTTP operations promptly;
- shutdown cancellation does not start the 20-second ambiguous-claim recovery pause because the process is terminating;
- shutdown cancellation stops any further result-delivery retry;
- no control-plane HTTP operation may wait indefinitely.

The 5-second deadline is a fixed implementation constant for this milestone and is not user/environment configuration.

## Step 5 — Transport ambiguity classification

The client must let checker-core distinguish:

- no work;
- definite control-plane rejection;
- transport failure before a known response;
- ambiguous claim transport failure;
- operation deadline/cancellation without exposing library-specific error text.

Do not reconstruct or guess an unknown CheckID after ambiguous claim failure.

## Step 6 — Cross-runtime fixture verification

Go tests and Rust tests both consume/validate representative fixtures.

Generated code remains optional.

## Step 7 — Verification / self-review / commit

Self-review must prove the client has the fixed 5-second operation deadline and that shutdown cancellation reaches both claim and result transports.

Suggested commit:

~~~text
feat(checker): add control plane client
~~~

Because fixture/internal-contract paths changed, require checker + internal-contract GREEN.

STOP before Task 10.

---

# Task 10 — Bounded Checker worker loop and process lifecycle

**Goal:** Build the actual single-Checker orchestration without Docker composition yet.

**Primary crates:**

~~~text
checker-core
checker
~~~

## Step 1 — Worker-loop RED tests with fake ports

Inject:

- ControlPlane port;
- Probe port;
- clock/sleeper as needed for deterministic timing.

Prove:

- max four active probes;
- claims are issued serially to fill slots;
- 204 causes fixed one-second wait;
- ordinary control-plane transport failures use bounded backoff;
- ambiguous claim transport failure pauses all new claims for 20 seconds;
- one probe per CheckID;
- result transport retry never re-runs probe;
- identical result payload reused;
- completed slot is released;
- cancellation/shutdown stops new claims and bounded work exits;
- cancellation/shutdown cancels an in-flight claim request;
- cancellation/shutdown cancels an in-flight result request;
- cancellation/shutdown stops result retries;
- a timed-out claim enters the 20-second ambiguous-claim pause only when the Checker remains running;
- shutdown cancellation itself does not enter that recovery pause.

## Step 2 — Result delivery policy

Result submission retries are transport retries only.

They must:

- preserve same CheckID;
- preserve same canonical payload;
- remain bounded;
- each individual HTTP attempt remains subject to the Task 9 five-second overall request deadline;
- stop when delivery becomes impossible/terminal;
- stop immediately when Checker shutdown is cancelled;
- never initiate a second probe.

Do not invent user-configurable retry policy.

## Step 3 — Checker configuration

Production config should contain only operational necessities such as:

- control-plane base URL;
- log level if needed.

Do not expose product/security bypass controls.

Fixed concurrency/cadence/probe budgets remain code/design constants for this milestone.

## Step 4 — Health lifecycle

Checker health means:

- config valid;
- dependencies constructed;
- worker loop started.

It does not mean a target is reachable.

Implement a deterministic local readiness marker compatible with the existing /run/uptime-lab tmpfs approach unless a smaller equally testable mechanism is proven.

## Step 5 — Logging

Stable fields where relevant:

~~~text
check_id
monitor_id
event
result_kind
duration_ms
~~~

Do not log raw target credentials or raw library errors as contract/product fields.

## Step 6 — Verification / self-review / commit

Suggested commit:

~~~text
feat(checker): implement bounded checker worker
~~~

Require checker GREEN.

STOP before Task 11.

---

# Task 11 — Production Go execution composition

**Goal:** Wire the landed Go execution capabilities into the real API process while keeping platform routing module-agnostic.

## Step 1 — Monitoring module composition

Compose:

- CheckRun persistence adapter;
- ClaimDueCheck;
- SubmitCheckResult;
- UUID v7 CheckID generator;
- existing UTC server Clock.

Do not create a second PostgreSQL pool.

## Step 2 — Product router seam

Keep platform httpserver Monitoring-independent.

Compose a product handler/router in cmd/api that delegates:

- existing public Monitoring adapter;
- internal Checker adapter.

Platform /livez and /readyz retain precedence.

No HTTP framework.

## Step 3 — Internal route integration tests with real PostgreSQL

In cmd/api integration tests prove after explicit migrations:

- claim produces 200 CheckWork;
- no work produces 204;
- result PUT terminalizes durable row;
- exact duplicate returns 204;
- conflict/late/unknown mappings;
- public POST/GET behavior unchanged;
- /livez and /readyz unchanged;
- schema incompatibility still makes /readyz unready;
- API startup does not migrate.

## Step 4 — Public contract protection

Confirm public.yaml blob is still identical to main/base.

No public result/status route appears.

## Step 5 — Verification / self-review / commit

Suggested commit:

~~~text
feat(api): compose checker execution endpoints
~~~

Require go-api, internal-contract, public-contract as applicable, aggregate gate GREEN.

STOP before Task 12.

---

# Task 12 — Real Checker image + canonical Compose replacement

**Goal:** Replace only the Checker placeholder service with the real Rust runtime while preserving the four-service topology and no-host-port rule.

**Primary files:**

- apps/checker/Dockerfile
- compose.yaml
- local-dev fitness/tests

## Step 1 — Multi-stage Checker Dockerfile

Use reviewed builder:

~~~text
rust:1.98.1-alpine3.24
~~~

Runtime should align with current:

~~~text
alpine:3.24.2
~~~

Requirements:

- deterministic Cargo.lock build;
- release build;
- CA certificates available for HTTPS;
- non-root USER 10001:10001;
- minimal runtime contents;
- exact Checker binary entrypoint;
- no development/test policy code path.

## Step 2 — Compose Checker service

Replace placeholder build with real Checker image.

Keep:

- exactly four services;
- init true;
- read_only true;
- tmpfs readiness marker;
- depends_on api service_healthy;
- no host ports;
- no host network;
- no external resources.

Add only necessary operational environment such as container-internal Go API base URL.

## Step 3 — Local-dev fitness RED -> GREEN

Evolve tests from:

~~~text
checker must be placeholder
~~~

to:

~~~text
checker must use apps/checker/Dockerfile and reviewed runtime contract
~~~

Continue requiring web to remain placeholder.

Require exact Rust builder/runtime pins and non-root runtime.

## Step 4 — Make the existing smoke target safe before real Checker CI

The current pre-Checker smoke uses:

~~~text
https://example.com/local-smoke
~~~

That target becomes unsafe once every Monitor is automatically due and the real Checker starts.

In the SAME Task 12 commit that replaces the Checker placeholder, change the existing Monitor smoke target to a deterministic default-port private Compose target such as:

~~~text
http://web/
~~~

Requirements:

- it must remain a valid public Monitoring registration value;
- POST/GET preservation assertions still use the exact target text;
- it must use the default HTTP port so production execution reaches private-address policy rather than non-default-port rejection;
- it must not depend on public internet;
- no pushed head may contain real Checker Compose wiring while the smoke still registers example.com or another external target.

Task 12 does not yet need to assert the resulting CheckRun details; Task 13 adds those complete cross-runtime assertions.

## Step 5 — Change detection

After real runtime exists, checker source/Dockerfile changes should trigger:

- checker job;
- local-dev job.

Update detectors/tests accordingly.

## Step 6 — Verification / self-review / commit

The real Checker may now poll the API safely because every Monitor created by canonical smoke uses a deterministic private Compose target.

Self-review must explicitly search the smoke harness for example.com and other external HTTP targets and require none.

Suggested commit:

~~~text
build(checker): run real checker in local Compose
~~~

Require checker + local-dev + go-api + gate GREEN.

STOP before Task 13.

---

# Task 13 — Real Docker cross-runtime product smoke + CI hardening

**Goal:** Prove the complete implemented journey with real Go, Rust, PostgreSQL, and strict production network policy.

## Step 1 — Preserve explicit migration bootstrap

Fresh DB flow remains:

~~~text
start db + api
/livez available
/readyz unavailable
migration metadata absent
explicit uptime-lab-migrate up
full Compose up
api + checker healthy
~~~

No API or Checker auto-migration.

## Step 2 — Reuse and assert the deterministic private Compose target

Task 12 has already replaced the historical external smoke target with a valid default-port private Compose target such as:

~~~text
http://web/
~~~

Task 13 now treats that target as authoritative cross-runtime evidence.

The purpose is to prove strict production rejection, not successful probing.

Do not reintroduce example.com or any public internet endpoint.

Do not use a non-default port because that would exercise port rejection before private-address rejection.

## Step 3 — Cross-runtime proof

Poll boundedly until persisted evidence proves:

~~~text
Monitor registered
-> Go issued pending CheckRun
-> Rust claimed it
-> Rust resolved private destination
-> policy rejected before connect
-> Rust submitted policy_rejected
-> Go persisted terminal CheckRun
~~~

Verify persisted row through PostgreSQL because public API intentionally exposes no CheckRun state.

Assert:

- result_kind = policy_rejected;
- completed_at non-null;
- duration_ms 0..20000;
- http_status null;
- no pending row remains for that Monitor after the observed terminalization.

## Step 4 — Persistence/restart proof

Normal:

~~~text
docker compose down
docker compose up -d --wait
~~~

must preserve:

- Monitor;
- observed terminal CheckRun.

Do not assert total CheckRun count remains exactly one because the fixed 60-second cadence may legitimately create later runs.

Instead track the original CheckID/result as preserved evidence.

## Step 5 — Destructive reset proof

After down -v:

- old Monitor absent;
- old CheckRun absent;
- API live/unready before migration;
- migration metadata absent;
- explicit migration required again;
- full stack healthy after migration.

## Step 6 — Smoke harness unit tests

Mock the shell dependencies enough to prove:

- bounded polling;
- private target uses default port;
- SQL assertions validate the original CheckID;
- no external internet target;
- reset semantics;
- failures surface stable diagnostics.

## Step 7 — CI hardening

Make sure checker/local-dev/go-api/internal-contract path detection is complete.

Gate must require every triggered job.

Keep CI cost path-aware; unrelated docs should not run Rust/PostgreSQL/Docker unnecessarily.

## Step 8 — Verification / self-review / commit

Suggested commit:

~~~text
test(devops): verify single checker execution loop
~~~

Require all implementation-relevant jobs GREEN.

STOP before Task 14.

---

# Task 14 — Canonical documentation + documentation fitness

**Goal:** Reflect implemented cross-runtime truth without claiming public production readiness.

## Step 1 — Stale-current-state search

Search canonical docs for statements such as:

- Checker is placeholder;
- Rust runtime not implemented;
- internal Checker contract deferred;
- check_runs absent;
- no due-work/result flow;
- only Go create/read is executable.

Historical superpowers design/plan docs remain historical and are not rewritten as current-state docs.

## Step 2 — Create Rust Checker canonical guide

Create:

~~~text
docs/checker/rust-checker.md
~~~

Document:

- workspace/crate boundaries;
- worker loop;
- fixed concurrency;
- internal contract;
- probe policy;
- validated-address binding;
- redirects;
- no proxy;
- TLS;
- health/lifecycle;
- no PostgreSQL;
- no public exposure;
- current limitations.

## Step 3 — Create execution-slice testing guide

Create:

~~~text
docs/testing/single-checker-execution-slice.md
~~~

Document evidence layers:

- internal contract;
- Go unit;
- PG integration;
- Rust architecture;
- Rust policy;
- Rust local probe integration;
- control-plane client;
- worker loop;
- Docker cross-runtime smoke;
- vulnerability audit.

## Step 4 — Update architecture/current-state docs

At minimum review/update:

- README.md;
- docs/README.md;
- container view;
- runtime flows;
- module boundaries;
- dependency rules;
- data ownership;
- Go control-plane guide;
- local-development guide;
- existing testing guides.

Keep public API limitations explicit.

## Step 5 — Documentation fitness

The previous speculative docs/checker prohibition must evolve because Rust Checker is now real.

Require the new Checker guide and execution testing guide.

Fitness must assert current-state markers such as:

- Rust Checker implemented;
- Rust never accesses PostgreSQL;
- internal contract exactly claim/result;
- production policy rejects private destinations;
- public status/history remains deferred;
- no application host ports;
- explicit migration bootstrap.

## Step 6 — Verification / self-review / commit

No runtime behavior changes in this task.

Suggested commit:

~~~text
docs(checker): document execution runtime
~~~

Require repository documentation fitness and aggregate CI GREEN.

STOP before Task 15.

---

# Task 15 — Whole-branch verification + review-ready transition

**Goal:** Prove the complete branch is exactly the reviewed design and prepare one reviewable PR.

## Step 1 — Scope diff

Compare main...HEAD.

Verify the complete branch contains only the planned domains.

Explicitly prove absence of:

- public.yaml change;
- 00001 migration change;
- third migration;
- public CheckRun/status/history route;
- Monitor lifecycle mutation;
- worker identity/lease;
- broker/outbox;
- PostgreSQL dependency in Rust;
- production private-network bypass;
- proxy-enabled production probe;
- insecure TLS;
- public host port;
- Web implementation.

## Step 2 — Contract verification

Require:

- public contract byte-identical to base;
- internal contract exactly two operations;
- Go and Rust compatibility fixtures match contract.

## Step 3 — Go verification

At minimum:

~~~text
gofmt clean
go mod tidy clean
go mod verify
go vet ./...
go test ./...
go test -count=1 -race ./...
real PostgreSQL migrations
real PostgreSQL Monitoring adapter tests
cmd/api integration
Go architecture fitness
govulncheck
~~~

## Step 4 — Rust verification

At minimum:

~~~text
cargo fmt --all --check
cargo check --workspace --all-targets --locked
cargo clippy --workspace --all-targets --all-features --locked -- -D warnings
cargo test --workspace --all-targets --locked
Rust architecture fitness
cargo audit
~~~

## Step 5 — Docker verification

Run real canonical smoke proving:

- fresh live/unready;
- explicit migration;
- real Checker healthy;
- Monitor registration;
- Go -> Rust work;
- strict private-target rejection;
- result PUT;
- terminal CheckRun persistence;
- normal restart persistence;
- destructive reset;
- re-migration requirement.

## Step 6 — Security review

Manually inspect:

- resolver called once per hop;
- validated set bound to connect;
- mixed forbidden/public DNS set rejected;
- no ambient proxy path;
- Host/SNI/certificate identity preserved;
- TLS cannot be disabled;
- no response-body consumption;
- header bound exists;
- production binary has no bypass;
- Rust has no DB client.

## Step 7 — Commit history

Normalize only if necessary.

Every implementation task should remain coherent and understandable.

Do not rewrite already useful task boundaries merely to minimize commit count.

## Step 8 — PR metadata

Update PR body with:

- tasks complete;
- exact HEAD;
- exact CI run;
- test counts;
- scope review;
- security review;
- Docker evidence;
- explicit STOP before external review.

Mark review-ready only after fresh exact-head CI is GREEN.

Do not start Task 16 before this gate closes.

---

# Task 16 — External review + exact-head landing gate

**Goal:** Obtain independent review of the complete implementation and land only a reviewed exact head.

## Step 1 — Trigger external review

Use CodeRabbit or the established independent reviewer on the exact review-ready head.

Record the reviewed commit.

## Step 2 — Validate every finding independently

For each finding classify:

~~~text
valid defect
valid design mismatch
hardening proposal outside scope
false positive
~~~

Do not apply reviewer suggestions mechanically.

## Step 3 — Fix only valid actionable findings

Each fix must be minimal and must preserve landed design.

If a finding requires reopening:

- public API;
- internal operation count;
- scheduling semantics;
- persistence shape;
- SSRF policy;
- multi-worker/broker scope;

STOP and return to design reassessment instead of silently changing the architecture.

## Step 4 — Re-run exact-head CI after fixes

No merge on stale pre-fix CI.

Require all triggered jobs GREEN.

## Step 5 — Resolve review threads

Every actionable thread must be:

- replied to with evidence;
- resolved.

No unresolved actionable finding at landing.

## Step 6 — Final landing conditions

Require simultaneously:

~~~text
whole-branch self-review GREEN
exact-head CI GREEN
public contract protected
internal contract GREEN
Go verification GREEN
Rust verification GREEN
PostgreSQL evidence GREEN
Docker cross-runtime smoke GREEN
Rust vulnerability audit GREEN
external review closed
unresolved actionable threads 0
branch behind main 0
PR mergeable
~~~

## Step 7 — Squash merge with expected-head protection

Expected title:

~~~text
feat(checker): implement single-checker execution slice
~~~

Do not use merge commit or rebase merge if repository policy remains squash-only.

## Step 8 — Fresh main push CI

After merge, verify current main equals the squash commit.

Require fresh push CI GREEN.

Do not count PR CI as post-merge evidence.

## Step 9 — Phase closure

Record exact pre-merge head, squash SHA, post-merge CI run, external findings, and Docker evidence.

Then STOP.

The next work must be a separate product reassessment for current status/history/Web/public exposure value.

---

## 8. Final Phase Acceptance Criteria

The implementation phase is complete only if all are true:

1. internal OpenAPI exists with exactly claim + result operations;
2. public OpenAPI remains byte-identical;
3. Go exclusively owns scheduling/persistence;
4. Rust has no PostgreSQL access;
5. one real Checker runs in Compose;
6. Checker concurrency is max four;
7. fixed 60-second cadence is implemented;
8. UUID v7 CheckID is Go-owned;
9. one pending CheckRun per Monitor is DB-enforced;
10. 20-second acceptance window is implemented;
11. 10-second probe timeout is implemented;
12. durationMs is 0..20000 end-to-end;
13. exact duplicate PUT is idempotent;
14. conflicting/late PUT is rejected;
15. check_runs is the only new product table;
16. 00001 remains immutable;
17. API startup remains non-migrating;
18. readiness requires 00002 after branch contains it;
19. SSRF policy rejects non-public destinations;
20. DNS validation is bound to actual connection attempts;
21. multi-address fallback stays within validated set;
22. redirects are fully revalidated;
23. HTTPS -> HTTP downgrade is rejected;
24. only default ports execute;
25. ambient proxy configuration is ignored;
26. TLS verification cannot be disabled;
27. response headers are bounded;
28. response body is not application-consumed;
29. production binary has no private-network bypass;
30. real Docker smoke proves Go -> Rust -> policy_rejected -> Go persistence;
31. no external internet is required by CI;
32. normal restart preserves durable evidence;
33. destructive reset removes Monitor + CheckRun;
34. canonical Compose remains four services;
35. no application host ports exist;
36. no broker/multi-worker/public-status/Web scope is added;
37. claim and result control-plane HTTP operations have a fixed five-second overall deadline;
38. Checker shutdown cancels in-flight claim/result operations and terminates result retries;
39. canonical docs match implementation truth;
40. external review is closed;
41. post-merge main CI is GREEN.

---

## 9. Plan Self-Review

### Design fidelity

PASS.

Task order follows the landed design and keeps product/security semantics fixed.

### Brownfield safety

PASS.

The plan explicitly handles the two current historical guards that would otherwise fail when internal.yaml and apps/checker first appear. Each is evolved with replacement invariants rather than deleted.

### Dependency direction

PASS.

Internal transport remains an outward Go adapter. PostgreSQL stays behind Go. Rust remains Ports and Adapters and has no DB client.

### Data integrity

PASS.

The migration and persistence tasks precede runtime composition. SQL constraints enforce result and pending-state invariants independently of application code.

### Idempotency/recovery

PASS.

Claim ambiguity, pending timeout, exact duplicate PUT, conflicting result, and result-delivery ambiguity are verified before Docker composition. Control-plane HTTP calls are independently bounded to five seconds, and shutdown cancellation is required to terminate in-flight claim/result operations and stop result retries.

### Security

PASS.

Network policy is implemented before live outbound probe mechanics. Testability cannot create a production private-network bypass.

### CI fitness

PASS.

Internal contract and Rust obtain dedicated path-aware verification. The required aggregate gate evolves rather than being bypassed.

### Runtime sequencing

PASS.

Compose does not switch from placeholder Checker to real Checker until Rust behavior and Go internal routes are independently tested.

### Verification

PASS.

Unit, architecture, contract, real PostgreSQL, Rust integration, cross-runtime fixture, Docker smoke, race, vulnerability, and external-review evidence are all required.

### YAGNI

PASS.

No broker, lease system, public status/history, Web runtime, auth system, private-monitoring mode, generalized events, or configurable scheduling is introduced.

### Execution safety

PASS.

This plan branch must contain exactly this plan document.

No internal.yaml, SQL migration, Go source, Rust source, Compose change, or CI implementation is authorized until this plan itself is reviewed and landed.

---

## 10. Explicit STOP Boundary

After this plan is reviewed and landed:

~~~text
NEXT:
Task 1 — Internal contract source + contract CI separation
~~~

Do not begin Task 1 from the plan branch.

Create the separate implementation branch and draft implementation PR from the exact landed plan commit.
